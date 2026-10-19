package api

import (
	"fmt"
	"time"

	"ledger"
)

type CreateTransactionRequest struct {
	Amount      float64 `json:"amount"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Date        string  `json:"date"`
}

type TransactionResponse struct {
	ID          int     `json:"id"`
	Amount      float64 `json:"amount"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Date        string  `json:"date"`
}

type CreateBudgetRequest struct {
	Category string  `json:"category"`
	Limit    float64 `json:"limit"`
}

type BudgetResponse struct {
	Category string  `json:"category"`
	Limit    float64 `json:"limit"`
}

func (r CreateTransactionRequest) toTransaction() (ledger.Transaction, error) {
	tx := ledger.Transaction{
		Amount:      r.Amount,
		Category:    r.Category,
		Description: r.Description,
	}
	if r.Date == "" {
		return tx, nil
	}

	date, err := time.Parse(time.DateOnly, r.Date)
	if err != nil {
		return ledger.Transaction{}, fmt.Errorf("дата транзакции должна быть в формате ГГГГ-ММ-ДД: %q", r.Date)
	}
	tx.Date = date
	return tx, nil
}

func newTransactionResponse(tx ledger.Transaction) TransactionResponse {
	return TransactionResponse{
		ID:          tx.ID,
		Amount:      tx.Amount,
		Category:    tx.Category,
		Description: tx.Description,
		Date:        tx.Date.Format(time.DateOnly),
	}
}

func newTransactionResponses(transactions []ledger.Transaction) []TransactionResponse {
	result := make([]TransactionResponse, 0, len(transactions))
	for _, tx := range transactions {
		result = append(result, newTransactionResponse(tx))
	}
	return result
}

func (r CreateBudgetRequest) toBudget() ledger.Budget {
	return ledger.Budget{
		Category: r.Category,
		Limit:    r.Limit,
	}
}

func newBudgetResponse(b ledger.Budget) BudgetResponse {
	return BudgetResponse{
		Category: b.Category,
		Limit:    b.Limit,
	}
}

func newBudgetResponses(budgets []ledger.Budget) []BudgetResponse {
	result := make([]BudgetResponse, 0, len(budgets))
	for _, b := range budgets {
		result = append(result, newBudgetResponse(b))
	}
	return result
}
