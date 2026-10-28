package cache

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const pingTimeout = 5 * time.Second

type Config struct {
	Addr     string
	Password string
	DB       int
}

// Cache хранит значения в Redis в виде JSON.
type Cache struct {
	client *redis.Client
}

// ConfigFromEnv читает REDIS_ADDR (по умолчанию localhost:6379), REDIS_DB (по умолчанию 0) и REDIS_PASSWORD.
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		Addr:     cmp.Or(os.Getenv("REDIS_ADDR"), "localhost:6379"),
		Password: os.Getenv("REDIS_PASSWORD"),
	}

	if v := os.Getenv("REDIS_DB"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("REDIS_DB должна быть целым числом: %q", v)
		}
		cfg.DB = n
	}
	return cfg, nil
}

func Open(ctx context.Context, cfg Config) (*Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("Redis недоступен: %w", err)
	}
	return &Cache{client: client}, nil
}

// Get читает значение по ключу в v. Возвращает false, если ключа нет.
func (c *Cache) Get(ctx context.Context, key string, v any) (bool, error) {
	data, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if err := json.Unmarshal(data, v); err != nil {
		return false, fmt.Errorf("некорректный JSON в кеше по ключу %s: %w", key, err)
	}
	return true, nil
}

func (c *Cache) Set(ctx context.Context, key string, v any, ttl time.Duration) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, ttl).Err()
}

func (c *Cache) Delete(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}

func (c *Cache) Close() error {
	return c.client.Close()
}
