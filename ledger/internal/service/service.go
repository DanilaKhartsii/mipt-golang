package service

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"ledger/internal/domain"
)

const (
	budgetsCacheKey = "budgets:all"
	cacheTTL        = 30 * time.Second
	heartbeatPeriod = 400 * time.Millisecond
)

type Service interface {
	SetBudget(context.Context, domain.Budget) error
	ListBudgets(context.Context) ([]domain.Budget, error)
	AddTransaction(context.Context, domain.Transaction) (domain.Transaction, error)
	ListTransactions(context.Context) ([]domain.Transaction, error)
	CategoryTotal(context.Context, string) (float64, error)
	GetReportSummary(context.Context, time.Time, time.Time) (domain.ReportSummary, error)
}

type Cache interface {
	Get(context.Context, string, any) (bool, error)
	Set(context.Context, string, any, time.Duration) error
	Delete(context.Context, ...string) error
}

type ValidationError struct{ Err error }

func (e ValidationError) Error() string { return e.Err.Error() }
func (e ValidationError) Unwrap() error { return e.Err }

type implementation struct {
	budgets  domain.BudgetRepository
	expenses domain.ExpenseRepository
	unit     domain.CategoryUnitOfWork
	cache    Cache
}

func New(budgets domain.BudgetRepository, expenses domain.ExpenseRepository, unit domain.CategoryUnitOfWork, cache Cache) Service {
	return &implementation{budgets: budgets, expenses: expenses, unit: unit, cache: cache}
}

func (s *implementation) SetBudget(ctx context.Context, b domain.Budget) error {
	if err := b.Validate(); err != nil {
		return ValidationError{Err: err}
	}
	if err := s.unit.WithinCategory(ctx, b.Category, func(budgets domain.BudgetRepository, _ domain.ExpenseRepository) error {
		return budgets.Upsert(ctx, b)
	}); err != nil {
		return err
	}
	s.deleteCache(ctx, budgetsCacheKey)
	return nil
}

func (s *implementation) ListBudgets(ctx context.Context) ([]domain.Budget, error) {
	var cached []domain.Budget
	if s.readCache(ctx, budgetsCacheKey, &cached) {
		return cached, nil
	}
	budgets, err := s.budgets.List(ctx)
	if err != nil {
		return nil, err
	}
	s.writeCache(ctx, budgetsCacheKey, budgets)
	return budgets, nil
}

func (s *implementation) AddTransaction(ctx context.Context, tx domain.Transaction) (domain.Transaction, error) {
	if err := tx.Validate(); err != nil {
		return domain.Transaction{}, ValidationError{Err: err}
	}
	var created domain.Transaction
	err := s.unit.WithinCategory(ctx, tx.Category, func(budgets domain.BudgetRepository, expenses domain.ExpenseRepository) error {
		budget, found, err := budgets.ByCategory(ctx, tx.Category)
		if err != nil {
			return err
		}
		if found {
			spent, err := expenses.SumByCategory(ctx, tx.Category)
			if err != nil {
				return err
			}
			total := spent + tx.Amount
			if total > budget.Limit {
				return fmt.Errorf("%w по категории «%s»: %.2f из %.2f", domain.ErrBudgetExceeded, tx.Category, total, budget.Limit)
			}
		}
		created, err = expenses.Insert(ctx, tx)
		return err
	})
	if err != nil {
		return domain.Transaction{}, err
	}
	return created, nil
}

func (s *implementation) ListTransactions(ctx context.Context) ([]domain.Transaction, error) {
	return s.expenses.List(ctx)
}

func (s *implementation) CategoryTotal(ctx context.Context, category string) (float64, error) {
	return s.expenses.SumByCategory(ctx, category)
}

func (s *implementation) GetReportSummary(ctx context.Context, from, to time.Time) (domain.ReportSummary, error) {
	if err := ctx.Err(); err != nil {
		return domain.ReportSummary{}, err
	}
	key := fmt.Sprintf("report:summary:%s:%s", from.Format(time.DateOnly), to.Format(time.DateOnly))
	var cached domain.ReportSummary
	if s.readCache(ctx, key, &cached) {
		if err := ctx.Err(); err != nil {
			return domain.ReportSummary{}, err
		}
		log.Printf("Ledger: отчёт %s взят из кеша", key)
		return cached, nil
	}
	report, err := s.calculateReport(ctx, from, to)
	if err != nil {
		return domain.ReportSummary{}, err
	}
	log.Printf("Ledger: отчёта %s нет в кеше, посчитан по БД", key)
	s.writeCache(ctx, key, report)
	if err := ctx.Err(); err != nil {
		return domain.ReportSummary{}, err
	}
	return report, nil
}

func (s *implementation) calculateReport(ctx context.Context, from, to time.Time) (domain.ReportSummary, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	stopHeartbeat := startReportHeartbeat(ctx)
	defer stopHeartbeat()

	if err := ctx.Err(); err != nil {
		return domain.ReportSummary{}, err
	}
	categories, err := s.expenses.CategoriesByPeriod(ctx, from, to)
	if ctx.Err() != nil {
		return domain.ReportSummary{}, context.Cause(ctx)
	}
	if err != nil {
		return domain.ReportSummary{}, err
	}

	results := make([]domain.CategorySummary, len(categories))
	var workers sync.WaitGroup
	for i, category := range categories {
		if ctx.Err() != nil {
			break
		}
		workers.Add(1)
		go func() {
			defer workers.Done()
			if ctx.Err() != nil {
				return
			}
			total, err := s.expenses.SumByCategoryAndPeriod(ctx, category, from, to)
			if err != nil {
				cancel(err)
				return
			}
			// Каждая горутина пишет только в свой элемент; чтение — после Wait.
			results[i] = domain.CategorySummary{Category: category, Total: total}
		}()
	}
	workers.Wait()
	if err := context.Cause(ctx); err != nil {
		return domain.ReportSummary{}, err
	}

	report := domain.ReportSummary{From: from, To: to, Categories: results}
	for _, category := range results {
		report.Total += category.Total
	}
	report.Total = math.Round(report.Total*100) / 100
	return report, nil
}

func startReportHeartbeat(ctx context.Context) func() {
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(heartbeatPeriod)
		defer func() {
			ticker.Stop()
			if ctx.Err() != nil {
				log.Printf("Ledger: расчёт прерван по ctx.Done(): %v", context.Cause(ctx))
			}
			log.Print("Ledger: heartbeat остановлен")
		}()
		for {
			select {
			case <-ticker.C:
				log.Print("Ledger: идёт расчёт отчёта")
			case <-ctx.Done():
				return
			case <-stop:
				return
			}
		}
	}()
	return func() {
		close(stop)
		<-done
	}
}

func (s *implementation) readCache(ctx context.Context, key string, v any) bool {
	if s.cache == nil {
		return false
	}
	found, err := s.cache.Get(ctx, key, v)
	if err != nil {
		log.Printf("Ledger: не удалось прочитать кеш %s: %v", key, err)
		return false
	}
	return found
}

func (s *implementation) writeCache(ctx context.Context, key string, v any) {
	if s.cache == nil {
		return
	}
	if err := s.cache.Set(ctx, key, v, cacheTTL); err != nil {
		log.Printf("Ledger: не удалось записать кеш %s: %v", key, err)
	}
}

func (s *implementation) deleteCache(ctx context.Context, key string) {
	if s.cache == nil {
		return
	}
	if err := s.cache.Delete(ctx, key); err != nil {
		log.Printf("Ledger: не удалось удалить кеш %s: %v", key, err)
	}
}
