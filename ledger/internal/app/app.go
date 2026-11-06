package app

import (
	"context"
	"fmt"
	"log"

	"ledger/internal/cache"
	"ledger/internal/db"
	"ledger/internal/repository/pg"
	"ledger/internal/service"
)

func New(ctx context.Context) (service.Service, func(), error) {
	conn, err := db.Open(ctx, db.DSNFromEnv())
	if err != nil {
		return nil, nil, fmt.Errorf("не удалось подключиться к базе данных: %w", err)
	}
	log.Print("Ledger: подключение к PostgreSQL установлено")

	cfg, err := cache.ConfigFromEnv()
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	c, err := cache.Open(ctx, cfg)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("не удалось подключиться к Redis %s: %w", cfg.Addr, err)
	}
	log.Printf("Ledger: подключение к Redis %s (db %d) установлено", cfg.Addr, cfg.DB)

	closeFn := func() {
		c.Close()
		conn.Close()
	}
	return service.New(pg.NewBudgets(conn), pg.NewExpenses(conn), pg.NewCategoryUnitOfWork(conn), c), closeFn, nil
}
