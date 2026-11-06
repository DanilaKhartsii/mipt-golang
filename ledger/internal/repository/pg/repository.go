package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"ledger/internal/domain"
)

type queryer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type Budgets struct{ db queryer }
type Expenses struct{ db queryer }

func NewBudgets(db *sql.DB) *Budgets   { return &Budgets{db: db} }
func NewExpenses(db *sql.DB) *Expenses { return &Expenses{db: db} }

var _ domain.BudgetRepository = (*Budgets)(nil)
var _ domain.ExpenseRepository = (*Expenses)(nil)

func (r *Budgets) Upsert(ctx context.Context, b domain.Budget) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO budgets (category, limit_amount) VALUES ($1, $2)
		 ON CONFLICT (category) DO UPDATE SET limit_amount = EXCLUDED.limit_amount`,
		b.Category, b.Limit,
	)
	if err != nil {
		return fmt.Errorf("не удалось сохранить бюджет «%s»: %w", b.Category, err)
	}
	return nil
}

func (r *Budgets) ByCategory(ctx context.Context, category string) (domain.Budget, bool, error) {
	b := domain.Budget{Category: category}
	err := r.db.QueryRowContext(ctx, `SELECT limit_amount FROM budgets WHERE category = $1`, category).Scan(&b.Limit)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Budget{}, false, nil
	}
	if err != nil {
		return domain.Budget{}, false, fmt.Errorf("не удалось получить бюджет категории «%s»: %w", category, err)
	}
	return b, true, nil
}

func (r *Budgets) List(ctx context.Context) ([]domain.Budget, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT category, limit_amount FROM budgets ORDER BY category`)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить бюджеты: %w", err)
	}
	defer rows.Close()

	result := []domain.Budget{}
	for rows.Next() {
		var b domain.Budget
		if err := rows.Scan(&b.Category, &b.Limit); err != nil {
			return nil, fmt.Errorf("не удалось прочитать бюджет: %w", err)
		}
		result = append(result, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("не удалось получить бюджеты: %w", err)
	}
	return result, nil
}

func (r *Expenses) Insert(ctx context.Context, tx domain.Transaction) (domain.Transaction, error) {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO expenses (amount, category, description, date) VALUES ($1, $2, $3, $4) RETURNING id`,
		tx.Amount, tx.Category, tx.Description, tx.Date,
	).Scan(&tx.ID)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("не удалось сохранить транзакцию: %w", err)
	}
	return tx, nil
}

func (r *Expenses) List(ctx context.Context) ([]domain.Transaction, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, amount, category, COALESCE(description, ''), date FROM expenses ORDER BY date DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить транзакции: %w", err)
	}
	defer rows.Close()

	result := []domain.Transaction{}
	for rows.Next() {
		var tx domain.Transaction
		if err := rows.Scan(&tx.ID, &tx.Amount, &tx.Category, &tx.Description, &tx.Date); err != nil {
			return nil, fmt.Errorf("не удалось прочитать транзакцию: %w", err)
		}
		result = append(result, tx)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("не удалось получить транзакции: %w", err)
	}
	return result, nil
}

func (r *Expenses) SumByCategory(ctx context.Context, category string) (float64, error) {
	var total float64
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE category = $1`, category).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("не удалось посчитать расходы по категории «%s»: %w", category, err)
	}
	return total, nil
}

func (r *Expenses) SummaryByPeriod(ctx context.Context, from, to time.Time) ([]domain.CategorySummary, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT category, SUM(amount) FROM expenses WHERE date BETWEEN $1 AND $2 GROUP BY category ORDER BY category`,
		from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("не удалось построить отчёт: %w", err)
	}
	defer rows.Close()

	result := []domain.CategorySummary{}
	for rows.Next() {
		var s domain.CategorySummary
		if err := rows.Scan(&s.Category, &s.Total); err != nil {
			return nil, fmt.Errorf("не удалось прочитать строку отчёта: %w", err)
		}
		result = append(result, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("не удалось построить отчёт: %w", err)
	}
	return result, nil
}
