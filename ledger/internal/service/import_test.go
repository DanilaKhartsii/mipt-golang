package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ledger/internal/domain"
)

type importUnitOfWork struct {
	within func(context.Context, string, func(domain.BudgetRepository, domain.ExpenseRepository) error) error
}

func (u *importUnitOfWork) WithinCategory(ctx context.Context, category string, fn func(domain.BudgetRepository, domain.ExpenseRepository) error) error {
	return u.within(ctx, category, fn)
}

type importRepository struct {
	expenseRepository
	insert func(context.Context, domain.Transaction) (domain.Transaction, error)
}

func (r *importRepository) Insert(ctx context.Context, tx domain.Transaction) (domain.Transaction, error) {
	return r.insert(ctx, tx)
}

func TestImportTransactionsResults(t *testing.T) {
	date := time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)
	for _, workers := range []int{1, 4, MaxImportWorkers} {
		t.Run(fmt.Sprint(workers), func(t *testing.T) {
			var inserted atomic.Int64
			unit := &importUnitOfWork{within: func(ctx context.Context, category string, fn func(domain.BudgetRepository, domain.ExpenseRepository) error) error {
				return fn(&budgetRepository{budget: domain.Budget{Limit: 100}, found: category == "лимит"}, &importRepository{
					insert: func(_ context.Context, tx domain.Transaction) (domain.Transaction, error) {
						if tx.Category == "ошибка БД" {
							return domain.Transaction{}, errors.New("не удалось сохранить транзакцию")
						}
						inserted.Add(1)
						return tx, nil
					},
				})
			}}
			transactions := []domain.Transaction{
				{Amount: 100, Category: "лимит", Date: date},
				{Amount: 0, Category: "еда", Date: date},
				{Amount: 101, Category: "лимит", Date: date},
				{Amount: 10, Category: "ошибка БД", Date: date},
				{Amount: 10, Date: date},
				{Amount: 10, Category: "еда"},
				{Amount: 1000, Category: "без бюджета", Date: date},
			}
			summary, err := New(nil, nil, unit, nil).ImportTransactions(context.Background(), transactions, workers)
			if err != nil || summary.Accepted != 2 || summary.Rejected != 5 || len(summary.Errors) != 5 || inserted.Load() != summary.Accepted {
				t.Fatalf("ImportTransactions() = %+v, %v; inserted = %d", summary, err, inserted.Load())
			}
			for i, message := range []string{"сумма", "превышен бюджет", "сохранить", "категория", "дата"} {
				if summary.Errors[i].Index != i+1 || !strings.Contains(summary.Errors[i].Error, message) {
					t.Errorf("Errors[%d] = %+v, want index %d and %q", i, summary.Errors[i], i+1, message)
				}
			}
		})
	}
}

func TestImportTransactionsRunsBoundedWorkers(t *testing.T) {
	const workers = 3
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	started := make(chan struct{}, 30)
	release := make(chan struct{})
	var active, peak, inserted atomic.Int64
	repo := &importRepository{insert: func(gotCtx context.Context, tx domain.Transaction) (domain.Transaction, error) {
		if gotCtx != ctx {
			t.Error("контекст не передан в Insert")
		}
		current := active.Add(1)
		defer active.Add(-1)
		for previous := peak.Load(); current > previous; previous = peak.Load() {
			if peak.CompareAndSwap(previous, current) {
				break
			}
		}
		started <- struct{}{}
		select {
		case <-ctx.Done():
			return domain.Transaction{}, ctx.Err()
		case <-release:
			inserted.Add(1)
			return tx, nil
		}
	}}
	unit := &importUnitOfWork{within: func(gotCtx context.Context, _ string, fn func(domain.BudgetRepository, domain.ExpenseRepository) error) error {
		if gotCtx != ctx {
			t.Error("контекст не передан в WithinCategory")
		}
		return fn(&budgetRepository{}, repo)
	}}
	transactions := make([]domain.Transaction, 30)
	for i := range transactions {
		transactions[i] = domain.Transaction{Amount: 1, Category: "еда", Date: time.Now()}
	}
	type result struct {
		summary domain.ImportSummary
		err     error
	}
	done := make(chan result, 1)
	go func() {
		summary, err := New(nil, nil, unit, nil).ImportTransactions(ctx, transactions, workers)
		done <- result{summary: summary, err: err}
	}()
	for range workers {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("воркеры не начали обработку одновременно")
		}
	}
	close(release)
	select {
	case got := <-done:
		if got.err != nil || got.summary.Accepted != 30 || got.summary.Rejected != 0 || got.summary.Errors == nil || len(got.summary.Errors) != 0 {
			t.Fatalf("ImportTransactions() = %+v, %v", got.summary, got.err)
		}
	case <-ctx.Done():
		t.Fatal("импорт не завершился")
	}
	if peak.Load() != workers || active.Load() != 0 || inserted.Load() != 30 {
		t.Fatalf("peak = %d, active = %d, inserted = %d", peak.Load(), active.Load(), inserted.Load())
	}
}

func TestImportTransactionsCancellationReturnsPartialSummary(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(fmt.Sprintf("deadline_%t", deadline), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			wantErr := context.Canceled
			if deadline {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
				wantErr = context.DeadlineExceeded
			}
			defer cancel()
			var started, finished atomic.Int64
			repo := &importRepository{insert: func(ctx context.Context, tx domain.Transaction) (domain.Transaction, error) {
				defer finished.Add(1)
				if started.Add(1) == 1 {
					return tx, nil
				}
				if !deadline {
					cancel()
				}
				<-ctx.Done()
				return domain.Transaction{}, fmt.Errorf("insert: %w", ctx.Err())
			}}
			unit := &importUnitOfWork{within: func(ctx context.Context, _ string, fn func(domain.BudgetRepository, domain.ExpenseRepository) error) error {
				return fn(&budgetRepository{}, repo)
			}}
			transactions := make([]domain.Transaction, 20)
			for i := range transactions {
				transactions[i] = domain.Transaction{Amount: 1, Category: "еда", Date: time.Now()}
			}
			summary, err := New(nil, nil, unit, nil).ImportTransactions(ctx, transactions, 1)
			if !errors.Is(err, wantErr) || summary.Accepted != 1 || summary.Rejected != 1 || len(summary.Errors) != 1 || summary.Errors[0].Index != 1 {
				t.Fatalf("ImportTransactions() = %+v, %v", summary, err)
			}
			if started.Load() != 2 || finished.Load() != 2 {
				t.Fatalf("started = %d, finished = %d, want 2, 2", started.Load(), finished.Load())
			}
		})
	}
}

func TestImportTransactionsKeepsSuccessOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	unit := &importUnitOfWork{within: func(context.Context, string, func(domain.BudgetRepository, domain.ExpenseRepository) error) error {
		// Сохранение завершилось успешно одновременно с отменой запроса.
		cancel()
		return nil
	}}
	transactions := []domain.Transaction{{Amount: 1, Category: "еда", Date: time.Now()}}
	summary, err := New(nil, nil, unit, nil).ImportTransactions(ctx, transactions, 1)
	if !errors.Is(err, context.Canceled) || summary.Accepted != 1 || summary.Rejected != 0 {
		t.Fatalf("ImportTransactions() = %+v, %v", summary, err)
	}
}

func TestImportTransactionsCancellationWaitsForAllWorkers(t *testing.T) {
	const workers = 4
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var started, finished atomic.Int64
	unit := &importUnitOfWork{within: func(ctx context.Context, _ string, _ func(domain.BudgetRepository, domain.ExpenseRepository) error) error {
		defer finished.Add(1)
		if started.Add(1) == workers {
			cancel()
		}
		<-ctx.Done()
		return ctx.Err()
	}}
	transactions := make([]domain.Transaction, 20)
	for i := range transactions {
		transactions[i] = domain.Transaction{Amount: 1, Category: "еда", Date: time.Now()}
	}
	summary, err := New(nil, nil, unit, nil).ImportTransactions(ctx, transactions, workers)
	if !errors.Is(err, context.Canceled) || summary.Accepted != 0 || summary.Rejected != workers || len(summary.Errors) != workers {
		t.Fatalf("ImportTransactions() = %+v, %v", summary, err)
	}
	if started.Load() != workers || finished.Load() != workers {
		t.Fatalf("started = %d, finished = %d, want %d", started.Load(), finished.Load(), workers)
	}
	for i, item := range summary.Errors {
		if item.Index != i || item.Error != context.Canceled.Error() {
			t.Errorf("Errors[%d] = %+v", i, item)
		}
	}
}

func TestImportTransactionsEmptyInvalidWorkersAndCanceled(t *testing.T) {
	svc := New(nil, nil, nil, nil)
	summary, err := svc.ImportTransactions(context.Background(), nil, DefaultImportWorkers)
	if err != nil || summary.Accepted != 0 || summary.Rejected != 0 || summary.Errors == nil || len(summary.Errors) != 0 {
		t.Fatalf("empty import = %+v, %v", summary, err)
	}
	for _, workers := range []int{-1, 0, MaxImportWorkers + 1} {
		_, err := svc.ImportTransactions(context.Background(), nil, workers)
		var validationErr ValidationError
		if !errors.As(err, &validationErr) {
			t.Fatalf("workers = %d: error = %v, want ValidationError", workers, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	summary, err = svc.ImportTransactions(ctx, []domain.Transaction{{Amount: 1, Category: "еда", Date: time.Now()}}, DefaultImportWorkers)
	if !errors.Is(err, context.Canceled) || summary.Accepted != 0 || summary.Rejected != 0 || len(summary.Errors) != 0 {
		t.Fatalf("canceled import = %+v, %v", summary, err)
	}
}
