package pg

import (
	"context"
	"database/sql"
	"fmt"

	"ledger/internal/domain"
)

type CategoryUnitOfWork struct{ db *sql.DB }

func NewCategoryUnitOfWork(db *sql.DB) *CategoryUnitOfWork {
	return &CategoryUnitOfWork{db: db}
}

var _ domain.CategoryUnitOfWork = (*CategoryUnitOfWork)(nil)

func (u *CategoryUnitOfWork) WithinCategory(ctx context.Context, category string, fn func(domain.BudgetRepository, domain.ExpenseRepository) error) error {
	tx, err := u.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("не удалось начать транзакцию для категории «%s»: %w", category, err)
	}
	defer tx.Rollback()

	// Одинаковая категория получает один advisory lock во всех экземплярах Gateway.
	var locked any
	if err := tx.QueryRowContext(ctx, `SELECT pg_advisory_xact_lock(1789, hashtext($1))`, category).Scan(&locked); err != nil {
		return fmt.Errorf("не удалось заблокировать категорию «%s»: %w", category, err)
	}
	if err := fn(&Budgets{db: tx}, &Expenses{db: tx}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("не удалось завершить транзакцию для категории «%s»: %w", category, err)
	}
	return nil
}
