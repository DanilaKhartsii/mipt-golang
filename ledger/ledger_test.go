package ledger

import (
	"errors"
	"testing"
	"time"
)

var testDate = time.Date(2026, time.October, 26, 0, 0, 0, 0, time.UTC)

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

// Тест меняет глобальное хранилище, поэтому не параллельный и сбрасывает состояние в каждом подтесте.
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
		if got := len(ListTransactions()); got != 1 {
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
		if got := len(ListTransactions()); got != 1 {
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
		if got := len(ListTransactions()); got != 0 {
			t.Errorf("len(ListTransactions()) = %d, want 0", got)
		}
	})
}

func setupFoodBudget(t *testing.T) {
	t.Helper()

	Reset()
	t.Cleanup(Reset)

	if err := SetBudget(Budget{Category: "еда", Limit: 5000}); err != nil {
		t.Fatalf("SetBudget() error = %v, want nil", err)
	}
}
