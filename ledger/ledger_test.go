package ledger

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"testing"
	"time"

	"ledger/testkit"
)

var testDate = time.Date(2026, time.October, 26, 0, 0, 0, 0, time.UTC)
var fixture *testkit.Fixture

// TestMain подключает тестовую БД из TEST_DATABASE_URL. Без неё тесты хранилища пропускаются.
func TestMain(m *testing.M) {
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		var err error
		fixture, err = testkit.Open(context.Background(), dsn)
		if err != nil {
			log.Fatalf("не удалось подключиться к тестовой БД: %v", err)
		}
	}
	code := m.Run()
	if fixture != nil {
		fixture.Close()
	}
	os.Exit(code)
}

func TestTransactionValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tx      Transaction
		wantErr bool
	}{
		{
			name: "valid",
			tx:   Transaction{Amount: 100, Category: "еда", Description: "обед", Date: testDate},
		},
		{
			name: "valid_without_description",
			tx:   Transaction{Amount: 0.01, Category: "еда", Date: testDate},
		},
		{
			name:    "zero_amount",
			tx:      Transaction{Amount: 0, Category: "еда", Date: testDate},
			wantErr: true,
		},
		{
			name:    "negative_amount",
			tx:      Transaction{Amount: -50, Category: "еда", Date: testDate},
			wantErr: true,
		},
		{
			name:    "empty_category",
			tx:      Transaction{Amount: 100, Category: "", Date: testDate},
			wantErr: true,
		},
		{
			name:    "zero_date",
			tx:      Transaction{Amount: 100, Category: "еда"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.tx.Validate()
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBudgetValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		budget  Budget
		wantErr bool
	}{
		{
			name:   "valid",
			budget: Budget{Category: "еда", Limit: 5000},
		},
		{
			name:    "zero_limit",
			budget:  Budget{Category: "еда", Limit: 0},
			wantErr: true,
		},
		{
			name:    "negative_limit",
			budget:  Budget{Category: "еда", Limit: -100},
			wantErr: true,
		},
		{
			name:    "empty_category",
			budget:  Budget{Category: "", Limit: 5000},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.budget.Validate()
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Тест меняет общую тестовую БД, поэтому не параллельный и сбрасывает состояние в каждом подтесте.
func TestAddTransactionBudget(t *testing.T) {
	t.Run("within_limit", func(t *testing.T) {
		setupFoodBudget(t)

		created, err := AddTransaction(Transaction{Amount: 3000, Category: "еда", Date: testDate})
		if err != nil {
			t.Fatalf("AddTransaction() error = %v, want nil", err)
		}
		if created.ID != 1 {
			t.Errorf("ID = %d, want 1", created.ID)
		}
		if got := len(mustListTransactions(t)); got != 1 {
			t.Errorf("len(ListTransactions()) = %d, want 1", got)
		}
	})

	t.Run("exact_limit", func(t *testing.T) {
		setupFoodBudget(t)

		if _, err := AddTransaction(Transaction{Amount: 5000, Category: "еда", Date: testDate}); err != nil {
			t.Fatalf("AddTransaction() error = %v, want nil", err)
		}
	})

	t.Run("exceeded", func(t *testing.T) {
		setupFoodBudget(t)

		if _, err := AddTransaction(Transaction{Amount: 3000, Category: "еда", Date: testDate}); err != nil {
			t.Fatalf("AddTransaction() error = %v, want nil", err)
		}

		_, err := AddTransaction(Transaction{Amount: 2500, Category: "еда", Date: testDate})
		if !errors.Is(err, ErrBudgetExceeded) {
			t.Fatalf("AddTransaction() error = %v, want %v", err, ErrBudgetExceeded)
		}
		if got := len(mustListTransactions(t)); got != 1 {
			t.Errorf("len(ListTransactions()) = %d, want 1", got)
		}
	})

	t.Run("other_category", func(t *testing.T) {
		setupFoodBudget(t)

		if _, err := AddTransaction(Transaction{Amount: 10000, Category: "транспорт", Date: testDate}); err != nil {
			t.Fatalf("AddTransaction() error = %v, want nil", err)
		}
	})

	t.Run("invalid_transaction", func(t *testing.T) {
		setupFoodBudget(t)

		if _, err := AddTransaction(Transaction{Amount: 0, Category: "еда", Date: testDate}); err == nil {
			t.Fatal("AddTransaction() error = nil, want error")
		}
		if got := len(mustListTransactions(t)); got != 0 {
			t.Errorf("len(ListTransactions()) = %d, want 0", got)
		}
	})
}

func TestConcurrentTransactionsRespectBudget(t *testing.T) {
	resetDB(t)
	if err := SetBudget(Budget{Category: "еда", Limit: 100}); err != nil {
		t.Fatal(err)
	}

	const workers = 20
	start := make(chan struct{})
	results := make(chan error, workers)
	for range workers {
		go func() {
			<-start
			_, err := AddTransaction(Transaction{Amount: 100, Category: "еда", Date: testDate})
			results <- err
		}()
	}
	close(start)

	successes := 0
	conflicts := 0
	for range workers {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrBudgetExceeded):
			conflicts++
		default:
			t.Fatalf("AddTransaction() unexpected error: %v", err)
		}
	}
	if successes != 1 || conflicts != workers-1 {
		t.Errorf("successes = %d, conflicts = %d; want 1, %d", successes, conflicts, workers-1)
	}
	if got := len(mustListTransactions(t)); got != 1 {
		t.Errorf("len(ListTransactions()) = %d, want 1", got)
	}
}

func TestGetReportSummary(t *testing.T) {
	resetDB(t)

	transactions := []Transaction{
		{Amount: 1000.10, Category: "еда", Date: testDate},
		{Amount: 500.20, Category: "еда", Date: testDate.AddDate(0, 0, 1)},
		{Amount: 300, Category: "транспорт", Date: testDate},
		{Amount: 700, Category: "еда", Date: testDate.AddDate(0, 0, 10)},
	}
	for _, tx := range transactions {
		if _, err := AddTransaction(tx); err != nil {
			t.Fatalf("AddTransaction() error = %v, want nil", err)
		}
	}

	report, err := GetReportSummary(context.Background(), testDate, testDate.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("GetReportSummary() error = %v, want nil", err)
	}

	want := []CategorySummary{
		{Category: "еда", Total: 1500.30},
		{Category: "транспорт", Total: 300},
	}
	if !slices.Equal(report.Categories, want) {
		t.Errorf("Categories = %+v, want %+v", report.Categories, want)
	}
	if report.Total != 1800.30 {
		t.Errorf("Total = %v, want 1800.30", report.Total)
	}
}

func setupFoodBudget(t *testing.T) {
	t.Helper()

	resetDB(t)

	if err := SetBudget(Budget{Category: "еда", Limit: 5000}); err != nil {
		t.Fatalf("SetBudget() error = %v, want nil", err)
	}
}

func resetDB(t *testing.T) {
	t.Helper()

	if fixture == nil {
		t.Skip("TEST_DATABASE_URL не задана, тест с БД пропущен")
	}

	mustReset(t)
	t.Cleanup(func() { mustReset(t) })
}

func mustReset(t *testing.T) {
	t.Helper()

	if err := fixture.Reset(context.Background()); err != nil {
		t.Fatalf("Reset() error = %v, want nil", err)
	}
}

func mustListTransactions(t *testing.T) []Transaction {
	t.Helper()

	transactions, err := ListTransactions()
	if err != nil {
		t.Fatalf("ListTransactions() error = %v, want nil", err)
	}
	return transactions
}

func SetBudget(b Budget) error {
	return fixture.Service.SetBudget(context.Background(), b)
}

func ListBudgets() ([]Budget, error) {
	return fixture.Service.ListBudgets(context.Background())
}

func AddTransaction(tx Transaction) (Transaction, error) {
	return fixture.Service.AddTransaction(context.Background(), tx)
}

func ListTransactions() ([]Transaction, error) {
	return fixture.Service.ListTransactions(context.Background())
}

func GetReportSummary(ctx context.Context, from, to time.Time) (ReportSummary, error) {
	if fixture == nil {
		return ReportSummary{}, fmt.Errorf("тестовая БД не подключена")
	}
	return fixture.Service.GetReportSummary(ctx, from, to)
}
