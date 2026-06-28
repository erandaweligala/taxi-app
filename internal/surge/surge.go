// Package surge computes a per-region price multiplier from the live
// demand/supply imbalance. Surge is not only business logic: it is a load
// valve. When a region is overheating, a higher multiplier throttles incoming
// demand and protects the request->match path.
package surge

import (
	"context"
	"time"

	"github.com/erandaweligala/taxi-app/internal/geo"
	"github.com/erandaweligala/taxi-app/internal/region"
	"github.com/redis/go-redis/v9"
)

const (
	demandWindow = time.Minute
	maxSurge     = 3.0
	minSurge     = 1.0
)

type Engine struct {
	rdb *redis.Client
	geo *geo.Store
}

func New(rdb *redis.Client, g *geo.Store) *Engine { return &Engine{rdb: rdb, geo: g} }

func demandKey(reg string) string { return "demand:" + reg }

// RecordDemand bumps the rolling demand counter for the region of a request.
// The TTL is (re)applied each time, giving a simple sliding window of recent
// demand without a background sweeper.
func (e *Engine) RecordDemand(ctx context.Context, lat, lng float64) {
	reg := region.Key(lat, lng)
	pipe := e.rdb.Pipeline()
	pipe.Incr(ctx, demandKey(reg))
	pipe.Expire(ctx, demandKey(reg), demandWindow)
	_, _ = pipe.Exec(ctx)
}

// Multiplier returns the current surge factor for the region around a point.
func (e *Engine) Multiplier(ctx context.Context, lat, lng float64) float64 {
	reg := region.Key(lat, lng)
	demand, _ := e.rdb.Get(ctx, demandKey(reg)).Int64()
	supply := e.geo.Supply(ctx, lat, lng)
	return factor(demand, supply)
}

// factor maps a demand/supply ratio onto a bounded multiplier. Below parity
// there is no surge; above it the price scales linearly up to the cap.
func factor(demand, supply int64) float64 {
	if supply <= 0 {
		if demand <= 0 {
			return minSurge
		}
		return maxSurge // demand with zero supply is maximum scarcity
	}
	ratio := float64(demand) / float64(supply)
	if ratio <= 1 {
		return minSurge
	}
	m := minSurge + (ratio - 1)
	if m > maxSurge {
		return maxSurge
	}
	return m
}
