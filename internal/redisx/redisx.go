// Package redisx provides a shared Redis client factory. Redis is the system's
// hot-state store: driver geo indexes, active-trip cache, surge counters, and
// the pub/sub backbone all live here so services stay stateless.
package redisx

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

func New(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         addr,
		PoolSize:     50,
		MinIdleConns: 5,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})
}

// Ping verifies connectivity, retrying briefly so services tolerate infra
// starting up alongside them under docker-compose.
func Ping(ctx context.Context, c *redis.Client) error {
	var err error
	for i := 0; i < 30; i++ {
		if err = c.Ping(ctx).Err(); err == nil {
			return nil
		}
		time.Sleep(time.Second)
	}
	return err
}
