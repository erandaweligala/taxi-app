// Package region turns a geographic coordinate into a shard key. Sharding by
// geography is the core scaling primitive of the platform: a driver in Lagos
// never competes for capacity with one in Nairobi. The region key is used as
// the Kafka partition key and as the Redis geo-set suffix, so load shards
// naturally end to end.
package region

import "strings"

// base32 alphabet used by the standard geohash encoding.
const base32 = "0123456789bcdefghjkmnpqrstuvwxyz"

// Precision controls how coarse a region shard is. 4 characters of geohash is
// roughly a 20km x 20km cell — about city-district granularity, which keeps a
// busy area on a single shard while still splitting unrelated cities apart.
const Precision = 4

// Key returns the region shard key for a coordinate.
func Key(lat, lng float64) string {
	return Geohash(lat, lng, Precision)
}

// Neighbors returns the region plus the coarse area around a coordinate. A
// proximity search near a shard boundary must consult adjacent cells so a
// nearby driver just over the line is not missed.
func Neighbors(lat, lng float64) []string {
	// A coarse (Precision-1) cell aggregates a region and its immediate
	// surroundings; expanding to it is a cheap, good-enough boundary guard for
	// candidate selection (final ranking is by true distance).
	coarse := Geohash(lat, lng, Precision-1)
	seen := map[string]struct{}{}
	var out []string
	for _, k := range []string{Key(lat, lng), coarse} {
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			out = append(out, k)
		}
	}
	return out
}

// Geohash encodes lat/lng to a geohash string of the given precision.
func Geohash(lat, lng float64, precision int) string {
	latRange := [2]float64{-90, 90}
	lngRange := [2]float64{-180, 180}

	var sb strings.Builder
	even := true
	bit := 0
	ch := 0

	for sb.Len() < precision {
		if even {
			mid := (lngRange[0] + lngRange[1]) / 2
			if lng >= mid {
				ch |= 1 << (4 - bit)
				lngRange[0] = mid
			} else {
				lngRange[1] = mid
			}
		} else {
			mid := (latRange[0] + latRange[1]) / 2
			if lat >= mid {
				ch |= 1 << (4 - bit)
				latRange[0] = mid
			} else {
				latRange[1] = mid
			}
		}
		even = !even
		if bit < 4 {
			bit++
		} else {
			sb.WriteByte(base32[ch])
			bit = 0
			ch = 0
		}
	}
	return sb.String()
}
