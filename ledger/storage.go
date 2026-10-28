package ledger

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"ledger/internal/cache"
	"ledger/internal/db"
)

// Глобальные подключения — временное решение до выделения слоя репозиториев.
var (
	sqlDB *sql.DB
	// redisCache == nil означает, что кеш не подключён (например, в тестах), и данные всегда читаются из БД.
	redisCache *cache.Cache
)

// Init подключает ledger к PostgreSQL и Redis. Параметры берутся из переменных окружения.
func Init(ctx context.Context) error {
	if err := InitDB(ctx, db.DSNFromEnv()); err != nil {
		return err
	}

	cfg, err := cache.ConfigFromEnv()
	if err != nil {
		return err
	}
	c, err := cache.Open(ctx, cfg)
	if err != nil {
		return fmt.Errorf("не удалось подключиться к Redis %s: %w", cfg.Addr, err)
	}
	redisCache = c
	log.Printf("Ledger: подключение к Redis %s (db %d) установлено", cfg.Addr, cfg.DB)
	return nil
}

// InitDB подключает ledger только к PostgreSQL, без кеша. Используется в Init и в тестах.
func InitDB(ctx context.Context, dsn string) error {
	conn, err := db.Open(ctx, dsn)
	if err != nil {
		return fmt.Errorf("не удалось подключиться к базе данных: %w", err)
	}
	sqlDB = conn
	log.Print("Ledger: подключение к PostgreSQL установлено")
	return nil
}

func Close() {
	if redisCache != nil {
		redisCache.Close()
	}
	if sqlDB != nil {
		sqlDB.Close()
	}
}

// readCache читает значение из кеша. Ошибки Redis только логируются, чтобы при сбое кеша данные брались из БД.
func readCache(ctx context.Context, key string, v any) bool {
	if redisCache == nil {
		return false
	}

	found, err := redisCache.Get(ctx, key, v)
	if err != nil {
		log.Printf("Ledger: не удалось прочитать кеш %s: %v", key, err)
		return false
	}
	return found
}

func writeCache(ctx context.Context, key string, v any, ttl time.Duration) {
	if redisCache == nil {
		return
	}

	if err := redisCache.Set(ctx, key, v, ttl); err != nil {
		log.Printf("Ledger: не удалось записать кеш %s: %v", key, err)
	}
}

func deleteCache(ctx context.Context, key string) {
	if redisCache == nil {
		return
	}

	if err := redisCache.Delete(ctx, key); err != nil {
		log.Printf("Ledger: не удалось удалить кеш %s: %v", key, err)
	}
}
