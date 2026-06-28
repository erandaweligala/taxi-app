// Package db owns the Postgres connection pool and schema. Postgres is the
// durable system of record for trips; only the trip worker writes to it, off
// the hot path.
package db

import (
	"context"
	_ "embed"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var Schema string

// Connect opens a pooled connection, retrying briefly so a service can start
// alongside Postgres under docker-compose.
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	var err error
	for i := 0; i < 30; i++ {
		pool, err = pgxpool.New(ctx, dsn)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				return pool, nil
			}
			pool.Close()
		}
		time.Sleep(time.Second)
	}
	return nil, err
}
