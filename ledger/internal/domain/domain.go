package domain

import (
	"context"
	"errors"
	"time"
)

type Budget struct {
	Category string  `json:"category"`
	Limit    float64 `json:"limit"`
}

func (b Budget) Validate() error {
	if b.Category == "" {
		return errors.New("категория бюджета не может быть пустой")
	}
	if b.Limit <= 0 {
		return errors.New("лимит бюджета должен быть больше 0")
	}
	return nil
}

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

type CategorySummary struct {
	Category string  `json:"category"`
	Total    float64 `json:"total"`
}

type ReportSummary struct {
	From       time.Time         `json:"from"`
	To         time.Time         `json:"to"`
	Categories []CategorySummary `json:"categories"`
	Total      float64           `json:"total"`
}

var ErrBudgetExceeded = errors.New("превышен бюджет")

type BudgetRepository interface {
	Upsert(context.Context, Budget) error
	ByCategory(context.Context, string) (Budget, bool, error)
	List(context.Context) ([]Budget, error)
}

type ExpenseRepository interface {
	Insert(context.Context, Transaction) (Transaction, error)
	List(context.Context) ([]Transaction, error)
	SumByCategory(context.Context, string) (float64, error)
	SummaryByPeriod(context.Context, time.Time, time.Time) ([]CategorySummary, error)
}

// CategoryUnitOfWork выполняет операции с одной категорией в общей транзакции.
type CategoryUnitOfWork interface {
	WithinCategory(context.Context, string, func(BudgetRepository, ExpenseRepository) error) error
}
