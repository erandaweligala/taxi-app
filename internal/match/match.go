// Package match implements driver selection. It is deliberately stateless: all
// state lives in Redis, so matching workers scale horizontally behind a load
// balancer and can be sharded per region. Candidate selection uses the Redis
// geo index; ranking happens in-memory.
package match

import (
	"context"

	"github.com/erandaweligala/taxi-app/internal/geo"
	"github.com/erandaweligala/taxi-app/internal/models"
)

// searchRadiusKm bounds candidate selection. Tight enough to stay fast, wide
// enough to find drivers in a sparse region.
const searchRadiusKm = 5.0

// candidateLimit caps how many drivers we rank in-memory per request.
const candidateLimit = 10

type Matcher struct{ geo *geo.Store }

func New(g *geo.Store) *Matcher { return &Matcher{geo: g} }

// Best returns the single best driver for a request, plus the ranked shortlist
// it was chosen from. Returns ok=false when no driver is available nearby.
func (m *Matcher) Best(ctx context.Context, req models.RideRequest) (models.Candidate, []models.Candidate, bool, error) {
	candidates, err := m.geo.Nearest(ctx, req.Lat, req.Lng, searchRadiusKm, candidateLimit)
	if err != nil {
		return models.Candidate{}, nil, false, err
	}
	if len(candidates) == 0 {
		return models.Candidate{}, nil, false, nil
	}
	// Candidates arrive already sorted by true distance; nearest wins. This is
	// the natural extension point for richer ranking (driver rating,
	// acceptance rate, heading), all computed in-memory.
	return candidates[0], candidates, true, nil
}
