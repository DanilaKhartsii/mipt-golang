package ledger

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestConcurrentImportsRespectBudget(t *testing.T) {
	resetDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := fixture.Service.SetBudget(ctx, Budget{Category: "еда", Limit: 100}); err != nil {
		t.Fatal(err)
	}
	transactions := make([]Transaction, 20)
	for i := range transactions {
		transactions[i] = Transaction{Amount: 10, Category: "еда", Date: testDate}
	}
	type result struct {
		summary ImportSummary
		err     error
	}
	start := make(chan struct{})
	results := make(chan result, 3)
	for range 2 {
		go func() {
			<-start
			summary, err := fixture.Service.ImportTransactions(ctx, transactions, 8)
			results <- result{summary: summary, err: err}
		}()
	}
	go func() {
		<-start
		_, err := fixture.Service.AddTransaction(ctx, transactions[0])
		switch {
		case err == nil:
			results <- result{summary: ImportSummary{Accepted: 1}}
		case errors.Is(err, ErrBudgetExceeded):
			results <- result{summary: ImportSummary{Rejected: 1}}
		default:
			results <- result{err: err}
		}
	}()
	close(start)
	var accepted, rejected int64
	for range 3 {
		got := <-results
		if got.err != nil {
			t.Errorf("операция завершилась с ошибкой: %v", got.err)
		}
		accepted += got.summary.Accepted
		rejected += got.summary.Rejected
		previous := -1
		for _, item := range got.summary.Errors {
			if item.Index <= previous || item.Index >= len(transactions) || !strings.Contains(item.Error, ErrBudgetExceeded.Error()) {
				t.Errorf("неожиданная ошибка импорта: %+v", item)
			}
			previous = item.Index
		}
	}
	if accepted != 10 || rejected != 31 {
		t.Fatalf("accepted = %d, rejected = %d, want 10, 31", accepted, rejected)
	}
	total, err := fixture.Service.CategoryTotal(ctx, "еда")
	if err != nil || total != 100 || len(mustListTransactions(t)) != 10 {
		t.Fatalf("total = %v, error = %v; want 100 and 10 saved transactions", total, err)
	}
}

func TestImportDatabaseCancellationReturnsPartialSummary(t *testing.T) {
	resetDB(t)
	conn, err := sql.Open("pgx", os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lock, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	// Тот же advisory lock, который берёт CategoryUnitOfWork при сохранении.
	if _, err := lock.ExecContext(ctx, `SELECT pg_advisory_xact_lock(1789, hashtext($1))`, "ожидание"); err != nil {
		t.Fatal(err)
	}
	importCtx, cancelImport := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancelImport()
	transactions := []Transaction{
		{Amount: 10, Category: "еда", Date: testDate},
		{Amount: 20, Category: "ожидание", Date: testDate},
		{Amount: 30, Category: "еда", Date: testDate},
	}
	summary, err := fixture.Service.ImportTransactions(importCtx, transactions, 1)
	if !errors.Is(err, context.DeadlineExceeded) || summary.Accepted != 1 || summary.Rejected != 1 || len(summary.Errors) != 1 || summary.Errors[0].Index != 1 {
		t.Fatalf("ImportTransactions() = %+v, %v", summary, err)
	}
	if got := mustListTransactions(t); len(got) != 1 || got[0].Amount != 10 {
		t.Fatalf("сохранённые транзакции = %+v, want only first transaction", got)
	}
}
