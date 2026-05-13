package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/keploy/taskmanager/internal/config"
	"github.com/redis/go-redis/v9"
)

var ErrMiss = errors.New("cache miss")

type Cache struct {
	client *redis.Client
}

func New(ctx context.Context, cfg config.RedisConfig) (*Cache, error) {
	c := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := c.Ping(pingCtx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return &Cache{client: c}, nil
}

func (c *Cache) Close() error { return c.client.Close() }

func (c *Cache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, b, ttl).Err()
}

func (c *Cache) Get(ctx context.Context, key string, dest any) error {
	b, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return ErrMiss
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dest)
}

func (c *Cache) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.client.Del(ctx, keys...).Err()
}

func (c *Cache) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	pipe := c.client.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}
