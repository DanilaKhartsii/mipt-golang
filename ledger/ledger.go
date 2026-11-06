package ledger

import (
	"context"

	"ledger/internal/app"
	"ledger/internal/domain"
	"ledger/internal/service"
)

type Budget = domain.Budget
type Transaction = domain.Transaction
type CategorySummary = domain.CategorySummary
type ReportSummary = domain.ReportSummary
type Service = service.Service
type ValidationError = service.ValidationError

var ErrBudgetExceeded = domain.ErrBudgetExceeded

func NewService(ctx context.Context) (Service, func(), error) {
	return app.New(ctx)
}
