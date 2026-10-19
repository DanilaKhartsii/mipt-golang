package ledger

import (
	"errors"
	"sync"
	"time"
)

type Transaction struct {
	ID          int
	Amount      float64
	Category    string
	Description string
	Date        time.Time
}

var (
	// mu защищает transactions и budgets от одновременного доступа из HTTP-обработчиков.
	mu           sync.RWMutex
	transactions = []Transaction{}
)

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

func AddTransaction(tx Transaction) (Transaction, error) {
	if err := tx.Validate(); err != nil {
		return Transaction{}, err
	}

	mu.Lock()
	defer mu.Unlock()

	if err := checkBudget(tx); err != nil {
		return Transaction{}, err
	}

	tx.ID = len(transactions) + 1
	transactions = append(transactions, tx)
	return tx, nil
}

func ListTransactions() []Transaction {
	mu.RLock()
	defer mu.RUnlock()

	result := make([]Transaction, len(transactions))
	copy(result, transactions)
	return result
}
