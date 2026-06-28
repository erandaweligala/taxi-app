// Package trip owns the trip lifecycle. The hot path (creating and reading
// active trips) is served from a Redis cache so the latency-sensitive request
// flow never waits on Postgres; the durable projection in Postgres is kept up
// to date asynchronously by the trip worker consuming the Kafka event stream.
package trip

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/erandaweligala/taxi-app/internal/db"
	"github.com/erandaweligala/taxi-app/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// validTransitions encodes the allowed state machine. Rejecting illegal
// transitions keeps the durable history consistent under out-of-order events.
var validTransitions = map[models.TripState]map[models.TripState]bool{
	models.StateRequested:  {models.StateAssigned: true, models.StateCancelled: true},
	models.StateAssigned:   {models.StateInProgress: true, models.StateCancelled: true},
	models.StateInProgress: {models.StateCompleted: true, models.StateCancelled: true},
}

// CanTransition reports whether moving from -> to is legal.
func CanTransition(from, to models.TripState) bool {
	return validTransitions[from][to]
}

// Cache is the Redis-backed active-trip cache used on the hot path.
type Cache struct{ rdb *redis.Client }

func NewCache(rdb *redis.Client) *Cache { return &Cache{rdb: rdb} }

func cacheKey(id string) string { return "trip:" + id }

func (c *Cache) Put(ctx context.Context, t models.Trip) error {
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	// Active trips are hot but bounded; expire idle ones after an hour so the
	// cache tracks only live work.
	return c.rdb.Set(ctx, cacheKey(t.ID), b, time.Hour).Err()
}

func (c *Cache) Get(ctx context.Context, id string) (models.Trip, error) {
	var t models.Trip
	b, err := c.rdb.Get(ctx, cacheKey(id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return t, ErrNotFound
	}
	if err != nil {
		return t, err
	}
	return t, json.Unmarshal(b, &t)
}

var ErrNotFound = errors.New("trip not found")

// Store is the durable Postgres projection, written by the trip worker.
type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Migrate applies the schema. Idempotent.
func (s *Store) Migrate(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, db.Schema)
	return err
}

// Apply persists a trip event: it upserts the trip projection and appends to
// the audit log in one transaction.
func (s *Store) Apply(ctx context.Context, e models.TripEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO trips (id, rider_id, driver_id, region, state, surge, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, now(), now())
		ON CONFLICT (id) DO UPDATE
		SET state = EXCLUDED.state,
		    driver_id = CASE WHEN EXCLUDED.driver_id <> '' THEN EXCLUDED.driver_id ELSE trips.driver_id END,
		    surge = EXCLUDED.surge,
		    updated_at = now()`,
		e.TripID, e.RiderID, e.DriverID, e.Region, string(e.State), e.Surge)
	if err != nil {
		return fmt.Errorf("upsert trip: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO trip_events (trip_id, state, driver_id) VALUES ($1, $2, $3)`,
		e.TripID, string(e.State), e.DriverID)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return tx.Commit(ctx)
}

// Get reads the durable trip projection.
func (s *Store) Get(ctx context.Context, id string) (models.Trip, error) {
	var t models.Trip
	var state string
	err := s.pool.QueryRow(ctx, `
		SELECT id, rider_id, driver_id, region, state, surge, created_at, updated_at
		FROM trips WHERE id = $1`, id).
		Scan(&t.ID, &t.RiderID, &t.DriverID, &t.Region, &state, &t.Surge, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	t.State = models.TripState(state)
	return t, err
}
