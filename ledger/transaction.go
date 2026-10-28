package ledger

import (
	"errors"
	"fmt"
	"time"
)

type Transaction struct {
	ID          int
	Amount      float64
	Category    string
	Description string
	Date        time.Time
}

func (tx Transaction) Validate() error {
	if tx.Amount <= 0 {
		return errors.New("сумма транзакции должна быть больше 0")
	}
	if tx.Category == "" {
		return errors.New("категория транзакции не может быть пустой")
	}
	if tx.Date.IsZero() {
		return errors.New("дата транзакции не указана")
	}
	return nil
}

// AddTransaction сохраняет транзакцию и возвращает её с ID из БД.
// Проверка бюджета и вставка выполняются без транзакции БД — для учебной задачи этого достаточно.
func AddTransaction(tx Transaction) (Transaction, error) {
	if err := tx.Validate(); err != nil {
		return Transaction{}, err
	}

	if err := checkBudget(tx); err != nil {
		return Transaction{}, err
	}

	err := sqlDB.QueryRow(
		`INSERT INTO expenses (amount, category, description, date) VALUES ($1, $2, $3, $4) RETURNING id`,
		tx.Amount, tx.Category, tx.Description, tx.Date,
	).Scan(&tx.ID)
	if err != nil {
		return Transaction{}, fmt.Errorf("не удалось сохранить транзакцию: %w", err)
	}
	return tx, nil
}

func ListTransactions() ([]Transaction, error) {
	rows, err := sqlDB.Query(
		`SELECT id, amount, category, COALESCE(description, ''), date FROM expenses ORDER BY date DESC, id DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить транзакции: %w", err)
	}
	defer rows.Close()

	result := []Transaction{}
	for rows.Next() {
		var tx Transaction
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
