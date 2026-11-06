package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"ledger/internal/domain"
)

type budgetRepository struct {
	budget domain.Budget
	found  bool
	reads  int
}

func (r *budgetRepository) Upsert(_ context.Context, b domain.Budget) error {
	r.budget, r.found = b, true
	return nil
}
func (r *budgetRepository) ByCategory(context.Context, string) (domain.Budget, bool, error) {
	r.reads++
	return r.budget, r.found, nil
}
func (r *budgetRepository) List(context.Context) ([]domain.Budget, error) {
	return []domain.Budget{r.budget}, nil
}

type expenseRepository struct {
	spent   float64
	inserts int
}

func (r *expenseRepository) Insert(_ context.Context, tx domain.Transaction) (domain.Transaction, error) {
	r.inserts++
	tx.ID = r.inserts
	return tx, nil
}
func (r *expenseRepository) List(context.Context) ([]domain.Transaction, error) {
	return []domain.Transaction{}, nil
}
func (r *expenseRepository) SumByCategory(context.Context, string) (float64, error) {
	return r.spent, nil
}
func (r *expenseRepository) SummaryByPeriod(context.Context, time.Time, time.Time) ([]domain.CategorySummary, error) {
	return []domain.CategorySummary{}, nil
}

type categoryUnitOfWork struct {
	budgets  domain.BudgetRepository
	expenses domain.ExpenseRepository
	calls    int
}

func (u *categoryUnitOfWork) WithinCategory(_ context.Context, _ string, fn func(domain.BudgetRepository, domain.ExpenseRepository) error) error {
	u.calls++
	return fn(u.budgets, u.expenses)
}

func TestAddTransactionBudgetRule(t *testing.T) {
	date := time.Date(2026, time.October, 26, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		amount     float64
		spent      float64
		budget     bool
		wantErr    error
		wantInsert int
		wantRead   int
	}{
		{name: "within_limit", amount: 2500, spent: 1000, budget: true, wantInsert: 1, wantRead: 1},
		{name: "exact_limit", amount: 4000, spent: 1000, budget: true, wantInsert: 1, wantRead: 1},
		{name: "exceeded", amount: 4001, spent: 1000, budget: true, wantErr: domain.ErrBudgetExceeded, wantRead: 1},
		{name: "without_budget", amount: 10000, budget: false, wantInsert: 1, wantRead: 1},
		{name: "invalid", amount: 0, budget: true, wantRead: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			budgets := &budgetRepository{budget: domain.Budget{Category: "еда", Limit: 5000}, found: tt.budget}
			expenses := &expenseRepository{spent: tt.spent}
			unit := &categoryUnitOfWork{budgets: budgets, expenses: expenses}
			svc := New(budgets, expenses, unit, nil)
			_, err := svc.AddTransaction(context.Background(), domain.Transaction{Amount: tt.amount, Category: "еда", Date: date})
			if tt.name == "invalid" {
				var validationErr ValidationError
				if !errors.As(err, &validationErr) {
					t.Fatalf("AddTransaction() error = %v, want ValidationError", err)
				}
			} else if !errors.Is(err, tt.wantErr) {
				t.Fatalf("AddTransaction() error = %v, want %v", err, tt.wantErr)
			}
			if expenses.inserts != tt.wantInsert || budgets.reads != tt.wantRead || unit.calls != tt.wantRead {
				t.Errorf("inserts = %d, reads = %d, transactions = %d; want %d, %d, %d", expenses.inserts, budgets.reads, unit.calls, tt.wantInsert, tt.wantRead, tt.wantRead)
			}
		})
	}
}
