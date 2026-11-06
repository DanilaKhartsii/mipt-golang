package testkit

import (
	"context"
	"database/sql"
	"fmt"

	"ledger/internal/db"
	"ledger/internal/repository/pg"
	"ledger/internal/service"
)

// Fixture использует только указанную тестовую БД и не подключает Redis.
type Fixture struct {
	Service service.Service
	db      *sql.DB
}

func Open(ctx context.Context, dsn string) (*Fixture, error) {
	conn, err := db.Open(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &Fixture{Service: service.New(pg.NewBudgets(conn), pg.NewExpenses(conn), pg.NewCategoryUnitOfWork(conn), nil), db: conn}, nil
}

func (f *Fixture) Reset(ctx context.Context) error {
	if _, err := f.db.ExecContext(ctx, `TRUNCATE expenses, budgets RESTART IDENTITY`); err != nil {
		return fmt.Errorf("не удалось очистить таблицы: %w", err)
	}
	return nil
}

func (f *Fixture) Close() error { return f.db.Close() }
