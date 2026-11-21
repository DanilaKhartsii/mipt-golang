package api

import (
	"cmp"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
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

type ImportErrorResponse struct {
	Index int    `json:"index"`
	Error string `json:"error"`
}

type ImportSummaryResponse struct {
	Accepted int64                 `json:"accepted"`
	Rejected int64                 `json:"rejected"`
	Errors   []ImportErrorResponse `json:"errors"`
}

type CreateBudgetRequest struct {
	Category string  `json:"category"`
	Limit    float64 `json:"limit"`
}

type BudgetResponse struct {
	Category string  `json:"category"`
	Limit    float64 `json:"limit"`
}

type CategorySummaryResponse struct {
	Category string  `json:"category"`
	Total    float64 `json:"total"`
}

type ReportSummaryResponse struct {
	From       string                    `json:"from"`
	To         string                    `json:"to"`
	Categories []CategorySummaryResponse `json:"categories"`
	Total      float64                   `json:"total"`
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

func parseImportWorkers(query url.Values) (int, error) {
	if !query.Has("workers") {
		return ledger.DefaultImportWorkers, nil
	}
	workers, err := strconv.Atoi(query.Get("workers"))
	if err != nil || workers < 1 || workers > ledger.MaxImportWorkers {
		return 0, fmt.Errorf("параметр workers должен быть целым числом от 1 до %d", ledger.MaxImportWorkers)
	}
	return workers, nil
}

func newImportSummaryResponse(summary ledger.ImportSummary, indices []int, conversionErrors []ImportErrorResponse) ImportSummaryResponse {
	result := ImportSummaryResponse{
		Accepted: summary.Accepted,
		Rejected: summary.Rejected + int64(len(conversionErrors)),
		Errors:   make([]ImportErrorResponse, 0, len(summary.Errors)+len(conversionErrors)),
	}
	result.Errors = append(result.Errors, conversionErrors...)
	for _, item := range summary.Errors {
		result.Errors = append(result.Errors, ImportErrorResponse{Index: indices[item.Index], Error: item.Error})
	}
	slices.SortFunc(result.Errors, func(a, b ImportErrorResponse) int { return cmp.Compare(a.Index, b.Index) })
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

// parseReportPeriod читает обязательные параметры from и to в формате ГГГГ-ММ-ДД.
func parseReportPeriod(query url.Values) (time.Time, time.Time, error) {
	from, err := parseQueryDate(query, "from")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	to, err := parseQueryDate(query, "to")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if from.After(to) {
		return time.Time{}, time.Time{}, errors.New("начало периода from не может быть позже конца периода to")
	}
	return from, to, nil
}

func parseQueryDate(query url.Values, name string) (time.Time, error) {
	value := query.Get(name)
	if value == "" {
		return time.Time{}, fmt.Errorf("не указан параметр %s", name)
	}

	date, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("параметр %s должен быть в формате ГГГГ-ММ-ДД: %q", name, value)
	}
	return date, nil
}

func newReportSummaryResponse(report ledger.ReportSummary) ReportSummaryResponse {
	categories := make([]CategorySummaryResponse, 0, len(report.Categories))
	for _, s := range report.Categories {
		categories = append(categories, CategorySummaryResponse{
			Category: s.Category,
			Total:    s.Total,
		})
	}

	return ReportSummaryResponse{
		From:       report.From.Format(time.DateOnly),
		To:         report.To.Format(time.DateOnly),
		Categories: categories,
		Total:      report.Total,
	}
}
