package trip

import (
	"testing"

	"github.com/erandaweligala/taxi-app/internal/models"
)

func TestCanTransition(t *testing.T) {
	legal := []struct{ from, to models.TripState }{
		{models.StateRequested, models.StateAssigned},
		{models.StateRequested, models.StateCancelled},
		{models.StateAssigned, models.StateInProgress},
		{models.StateInProgress, models.StateCompleted},
		{models.StateInProgress, models.StateCancelled},
	}
	for _, c := range legal {
		if !CanTransition(c.from, c.to) {
			t.Errorf("expected %s->%s to be legal", c.from, c.to)
		}
	}

	illegal := []struct{ from, to models.TripState }{
		{models.StateRequested, models.StateInProgress}, // can't skip assignment
		{models.StateCompleted, models.StateInProgress}, // terminal
		{models.StateCancelled, models.StateAssigned},   // terminal
		{models.StateAssigned, models.StateCompleted},   // must start first
	}
	for _, c := range illegal {
		if CanTransition(c.from, c.to) {
			t.Errorf("expected %s->%s to be illegal", c.from, c.to)
		}
	}
}
