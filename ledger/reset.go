package ledger

import "fmt"

// Reset очищает таблицы транзакций и бюджетов и сбрасывает счётчики ID. Нужна тестам, чтобы сценарии не влияли друг на друга.
func Reset() error {
	if _, err := sqlDB.Exec(`TRUNCATE expenses, budgets RESTART IDENTITY`); err != nil {
		return fmt.Errorf("не удалось очистить таблицы: %w", err)
	}
	return nil
}
