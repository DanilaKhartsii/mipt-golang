package service

import (
	"bytes"
	"context"
	"errors"
	"log"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ledger/internal/domain"
)

type reportRepository struct {
	expenseRepository
	categories func(context.Context, time.Time, time.Time) ([]string, error)
	sum        func(context.Context, string, time.Time, time.Time) (float64, error)
}

func (r *reportRepository) CategoriesByPeriod(ctx context.Context, from, to time.Time) ([]string, error) {
	return r.categories(ctx, from, to)
}

func (r *reportRepository) SumByCategoryAndPeriod(ctx context.Context, category string, from, to time.Time) (float64, error) {
	return r.sum(ctx, category, from, to)
}

func TestReportRunsCategoriesConcurrently(t *testing.T) {
	from := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, -1)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	started := make(chan string, 2)
	release := make(chan struct{})
	repo := &reportRepository{
		categories: func(ctx context.Context, gotFrom, gotTo time.Time) ([]string, error) {
			if gotFrom != from || gotTo != to {
				t.Errorf("period = %v..%v, want %v..%v", gotFrom, gotTo, from, to)
			}
			return []string{"еда", "транспорт"}, ctx.Err()
		},
		sum: func(ctx context.Context, category string, gotFrom, gotTo time.Time) (float64, error) {
			if gotFrom != from || gotTo != to {
				t.Errorf("period = %v..%v, want %v..%v", gotFrom, gotTo, from, to)
			}
			started <- category
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-release:
				return map[string]float64{"еда": 1000.10, "транспорт": 500.20}[category], nil
			}
		},
	}
	type result struct {
		report domain.ReportSummary
		err    error
	}
	done := make(chan result, 1)
	go func() {
		report, err := New(nil, repo, nil, nil).GetReportSummary(ctx, from, to)
		done <- result{report: report, err: err}
	}()
	// Ни один расчёт не может закончиться, пока не запущены обе категории.
	for range 2 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("категории не начали рассчитываться параллельно")
		}
	}
	close(release)
	got := <-done
	want := []domain.CategorySummary{{Category: "еда", Total: 1000.10}, {Category: "транспорт", Total: 500.20}}
	if got.err != nil || got.report.From != from || got.report.To != to || got.report.Total != 1500.30 || !slices.Equal(got.report.Categories, want) {
		t.Fatalf("GetReportSummary() = %+v, %v", got.report, got.err)
	}
}

func TestReportCancellationStopsWorkersAndHeartbeat(t *testing.T) {
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(previous)
	var started, finished atomic.Int32
	repo := &reportRepository{
		categories: func(context.Context, time.Time, time.Time) ([]string, error) {
			return []string{"еда", "транспорт"}, nil
		},
		sum: func(ctx context.Context, _ string, _, _ time.Time) (float64, error) {
			started.Add(1)
			defer finished.Add(1)
			<-ctx.Done()
			return 0, ctx.Err()
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*heartbeatPeriod)
	defer cancel()
	report, err := New(nil, repo, nil, nil).GetReportSummary(ctx, time.Now(), time.Now())
	if !errors.Is(err, context.DeadlineExceeded) || report.Categories != nil {
		t.Fatalf("GetReportSummary() = %+v, %v; want no partial report, DeadlineExceeded", report, err)
	}
	if started.Load() != 2 || finished.Load() != 2 {
		t.Fatalf("workers started = %d, finished = %d, want 2, 2", started.Load(), finished.Load())
	}
	// Сообщение остановки записывается до возврата из сервиса, горутина уже завершена.
	for _, message := range []string{"идёт расчёт отчёта", "расчёт прерван по ctx.Done()", "heartbeat остановлен"} {
		if !strings.Contains(logs.String(), message) {
			t.Errorf("missing %q in logs: %s", message, &logs)
		}
	}
}

func TestReportWorkerErrorCancelsOtherWorkers(t *testing.T) {
	wantErr := errors.New("database failure")
	otherStarted := make(chan struct{})
	var finished atomic.Bool
	repo := &reportRepository{
		categories: func(context.Context, time.Time, time.Time) ([]string, error) {
			return []string{"ошибка", "ожидание"}, nil
		},
		sum: func(ctx context.Context, category string, _, _ time.Time) (float64, error) {
			if category == "ошибка" {
				select {
				case <-otherStarted:
					return 0, wantErr
				case <-ctx.Done():
					return 0, ctx.Err()
				}
			}
			close(otherStarted)
			<-ctx.Done()
			finished.Store(true)
			return 0, ctx.Err()
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := New(nil, repo, nil, nil).GetReportSummary(ctx, time.Now(), time.Now())
	if !errors.Is(err, wantErr) || !finished.Load() {
		t.Fatalf("error = %v, sibling finished = %v", err, finished.Load())
	}
}

func TestReportEmptyAndCanceled(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		report, err := New(nil, &expenseRepository{}, nil, nil).GetReportSummary(context.Background(), time.Now(), time.Now())
		if err != nil || report.Categories == nil || len(report.Categories) != 0 || report.Total != 0 {
			t.Fatalf("GetReportSummary() = %+v, %v; want empty report", report, err)
		}
	})
	t.Run("already_canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		// nil-репозиторий подтверждает, что запросов в БД не было.
		_, err := New(nil, nil, nil, nil).GetReportSummary(ctx, time.Now(), time.Now())
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want Canceled", err)
		}
	})
	t.Run("category_query_canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		repo := &reportRepository{categories: func(context.Context, time.Time, time.Time) ([]string, error) {
			cancel()
			return nil, errors.New("SQL query canceled")
		}}
		_, err := New(nil, repo, nil, nil).GetReportSummary(ctx, time.Now(), time.Now())
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want Canceled", err)
		}
	})
}
