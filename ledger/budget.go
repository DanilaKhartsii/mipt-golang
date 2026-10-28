package ledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

type Budget struct {
	Category string  `json:"category"`
	Limit    float64 `json:"limit"`
}

var ErrBudgetExceeded = errors.New("превышен бюджет")

const (
	budgetsCacheKey = "budgets:all"
	budgetsCacheTTL = 30 * time.Second
)

func (b Budget) Validate() error {
	if b.Category == "" {
		return errors.New("категория бюджета не может быть пустой")
	}
	if b.Limit <= 0 {
		return errors.New("лимит бюджета должен быть больше 0")
	}
	return nil
}

func SetBudget(b Budget) error {
	if err := b.Validate(); err != nil {
		return err
	}

	_, err := sqlDB.Exec(
		`INSERT INTO budgets (category, limit_amount) VALUES ($1, $2)
		 ON CONFLICT (category) DO UPDATE SET limit_amount = EXCLUDED.limit_amount`,
		b.Category, b.Limit,
	)
	if err != nil {
		return fmt.Errorf("не удалось сохранить бюджет «%s»: %w", b.Category, err)
	}

	deleteCache(context.Background(), budgetsCacheKey)
	return nil
}

func LoadBudgets(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("не удалось прочитать бюджеты: %w", err)
	}

	var loaded []Budget
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("не удалось разобрать JSON с бюджетами: %w", err)
	}

	for i, b := range loaded {
		if err := SetBudget(b); err != nil {
			return fmt.Errorf("некорректный бюджет #%d: %w", i+1, err)
		}
	}
	return nil
}

func ListBudgets() ([]Budget, error) {
	ctx := context.Background()

	var cached []Budget
	if readCache(ctx, budgetsCacheKey, &cached) {
		return cached, nil
	}

	rows, err := sqlDB.QueryContext(ctx, `SELECT category, limit_amount FROM budgets ORDER BY category`)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить бюджеты: %w", err)
	}
	defer rows.Close()

	result := []Budget{}
	for rows.Next() {
		var b Budget
		if err := rows.Scan(&b.Category, &b.Limit); err != nil {
			return nil, fmt.Errorf("не удалось прочитать бюджет: %w", err)
		}
		result = append(result, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("не удалось получить бюджеты: %w", err)
	}

	writeCache(ctx, budgetsCacheKey, result, budgetsCacheTTL)
	return result, nil
}

func CategoryTotal(category string) (float64, error) {
	var total float64
	err := sqlDB.QueryRow(`SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE category = $1`, category).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("не удалось посчитать расходы по категории «%s»: %w", category, err)
	}
	return total, nil
}

// checkBudget проверяет, что транзакция не превысит бюджет категории. Категории без бюджета не ограничены.
func checkBudget(tx Transaction) error {
	var limit float64
	err := sqlDB.QueryRow(`SELECT limit_amount FROM budgets WHERE category = $1`, tx.Category).Scan(&limit)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("не удалось получить бюджет категории «%s»: %w", tx.Category, err)
	}

	spent, err := CategoryTotal(tx.Category)
	if err != nil {
		return err
	}

	total := spent + tx.Amount
	if total > limit {
		return fmt.Errorf("%w по категории «%s»: %.2f из %.2f", ErrBudgetExceeded, tx.Category, total, limit)
	}
	return nil
}
