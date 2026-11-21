package service

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"

	"ledger/internal/domain"
)

const (
	DefaultImportWorkers = 4
	MaxImportWorkers     = 64
)

// ImportTransactions возвращает результаты всех завершённых попыток, в том числе при отмене.
// Элементы, обработка которых не началась, в частичную сводку не входят.
func (s *implementation) ImportTransactions(ctx context.Context, transactions []domain.Transaction, workers int) (domain.ImportSummary, error) {
	summary := domain.ImportSummary{Errors: []domain.ImportError{}}
	if err := ctx.Err(); err != nil {
		return summary, err
	}
	if workers < 1 || workers > MaxImportWorkers {
		return summary, ValidationError{Err: fmt.Errorf("число воркеров должно быть от 1 до %d", MaxImportWorkers)}
	}

	type job struct {
		index int
		tx    domain.Transaction
	}
	type result struct {
		index int
		err   error
	}
	jobs := make(chan job)
	results := make(chan result)
	collected := make(chan struct{})
	go func() {
		defer close(collected)
		for result := range results {
			if result.err != nil {
				atomic.AddInt64(&summary.Rejected, 1)
				summary.Errors = append(summary.Errors, domain.ImportError{Index: result.index, Error: result.err.Error()})
			} else {
				atomic.AddInt64(&summary.Accepted, 1)
			}
		}
	}()

	var pool sync.WaitGroup
	for range min(workers, len(transactions)) {
		pool.Add(1)
		go func() {
			defer pool.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-jobs:
					if !ok || ctx.Err() != nil {
						return
					}
					_, err := s.AddTransaction(ctx, job.tx)
					// Коллектор работает до закрытия results: не теряем результат
					// уже завершённой операции, даже если контекст отменён.
					results <- result{index: job.index, err: err}
				}
			}
		}()
	}

sendJobs:
	for i, tx := range transactions {
		select {
		case <-ctx.Done():
			break sendJobs
		case jobs <- job{index: i, tx: tx}:
		}
	}
	close(jobs)
	pool.Wait()
	close(results)
	<-collected

	slices.SortFunc(summary.Errors, func(a, b domain.ImportError) int { return cmp.Compare(a.Index, b.Index) })
	return summary, ctx.Err()
}
