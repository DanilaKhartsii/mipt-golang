package db

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const pingTimeout = 5 * time.Second

// DSNFromEnv возвращает DATABASE_URL, а если она не задана — собирает DSN из DB_HOST, DB_PORT, DB_USER, DB_PASS, DB_NAME.
func DSNFromEnv() string {
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		return dsn
	}

	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cmp.Or(os.Getenv("DB_USER"), "postgres"), cmp.Or(os.Getenv("DB_PASS"), "postgres")),
		Host:     net.JoinHostPort(cmp.Or(os.Getenv("DB_HOST"), "localhost"), cmp.Or(os.Getenv("DB_PORT"), "5432")),
		Path:     "/" + cmp.Or(os.Getenv("DB_NAME"), "cashapp"),
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("некорректные параметры подключения к PostgreSQL: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("PostgreSQL недоступен: %w", err)
	}
	return db, nil
}
