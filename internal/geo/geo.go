// Package geo manages driver location state in Redis. Locations are indexed in
// a geo set per region so proximity queries answer in-memory in single-digit
// milliseconds, and never touch the durable database.
package geo

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/erandaweligala/taxi-app/internal/models"
	"github.com/erandaweligala/taxi-app/internal/region"
	"github.com/redis/go-redis/v9"
)

// freshTTL bounds how long a driver's location is trusted after their last
// ping. Drivers ping every 3-5s; beyond this window we treat them as gone.
const freshTTL = 15 * time.Second

type Store struct{ rdb *redis.Client }

func New(rdb *redis.Client) *Store { return &Store{rdb: rdb} }

func geoKey(reg string) string   { return "geo:drivers:" + reg }
func driverKey(id string) string { return "driver:" + id }

// Upsert records a driver's latest position. An available driver is added to
// the region geo index; one that has gone offline or is mid-trip is removed so
// matching never considers them.
func (s *Store) Upsert(ctx context.Context, u models.LocationUpdate) error {
	reg := region.Key(u.Lat, u.Lng)
	if !u.Available {
		return s.Remove(ctx, u.DriverID, reg)
	}
	pipe := s.rdb.TxPipeline()
	pipe.GeoAdd(ctx, geoKey(reg), &redis.GeoLocation{
		Name:      u.DriverID,
		Longitude: u.Lng,
		Latitude:  u.Lat,
	})
	pipe.HSet(ctx, driverKey(u.DriverID), map[string]any{
		"region": reg,
		"lat":    u.Lat,
		"lng":    u.Lng,
		"ts":     u.Timestamp,
	})
	// The metadata hash carries the freshness TTL; the geo-set member is pruned
	// lazily on search when its hash is found to have expired.
	pipe.Expire(ctx, driverKey(u.DriverID), freshTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// Remove drops a driver from a region's index.
func (s *Store) Remove(ctx context.Context, driverID, reg string) error {
	pipe := s.rdb.TxPipeline()
	pipe.ZRem(ctx, geoKey(reg), driverID)
	pipe.Del(ctx, driverKey(driverID))
	_, err := pipe.Exec(ctx)
	return err
}

// Nearest returns up to limit available drivers near a point, ranked by true
// distance. It searches the region plus its coarse surroundings so a driver
// just over a shard boundary is still found, then prunes any stale entries.
func (s *Store) Nearest(ctx context.Context, lat, lng, radiusKm float64, limit int) ([]models.Candidate, error) {
	var all []models.Candidate
	for _, reg := range region.Neighbors(lat, lng) {
		locs, err := s.rdb.GeoSearchLocation(ctx, geoKey(reg), &redis.GeoSearchLocationQuery{
			GeoSearchQuery: redis.GeoSearchQuery{
				Longitude:  lng,
				Latitude:   lat,
				Radius:     radiusKm,
				RadiusUnit: "km",
				Sort:       "ASC",
				Count:      limit * 3, // over-fetch so pruning still leaves enough
			},
			WithCoord: true,
			WithDist:  true,
		}).Result()
		if err != nil && err != redis.Nil {
			return nil, fmt.Errorf("geosearch %s: %w", reg, err)
		}
		for _, l := range locs {
			if !s.fresh(ctx, l.Name) {
				// Lazy expiry: the metadata TTL lapsed, so evict the stale member.
				s.rdb.ZRem(ctx, geoKey(reg), l.Name)
				continue
			}
			all = append(all, models.Candidate{
				DriverID:   l.Name,
				Lat:        l.Latitude,
				Lng:        l.Longitude,
				DistanceKm: l.Dist,
			})
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].DistanceKm < all[j].DistanceKm })
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// Supply returns the number of indexed drivers in the region around a point.
// Used by surge as the denominator of the demand/supply ratio.
func (s *Store) Supply(ctx context.Context, lat, lng float64) int64 {
	n, _ := s.rdb.ZCard(ctx, geoKey(region.Key(lat, lng))).Result()
	return n
}

func (s *Store) fresh(ctx context.Context, driverID string) bool {
	n, _ := s.rdb.Exists(ctx, driverKey(driverID)).Result()
	return n > 0
}
