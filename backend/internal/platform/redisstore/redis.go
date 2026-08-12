package redisstore

import (
	"context"

	"2pick.app/backend/internal/config"
	"github.com/redis/go-redis/v9"
)

// Open builds a client from validated configuration. It does not dial; callers
// decide when a failure to reach Redis should matter, which keeps startup and
// readiness reporting separate.
func Open(configuration config.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         configuration.Addr,
		Password:     configuration.Password,
		DB:           configuration.DB,
		DialTimeout:  configuration.DialTimeout,
		ReadTimeout:  configuration.ReadTimeout,
		WriteTimeout: configuration.WriteTimeout,
		PoolSize:     configuration.PoolSize,
	})
}

// Ping reports whether Redis is currently reachable.
func Ping(ctx context.Context, client *redis.Client) error {
	return client.Ping(ctx).Err()
}
