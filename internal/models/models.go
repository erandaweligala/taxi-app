// Package models holds DTOs shared across the API surface and the workers.
package models

import "time"

// LocationUpdate is a single driver location ping. Drivers emit these every
// 3-5 seconds; this is the heaviest, most constant write load in the system.
type LocationUpdate struct {
	DriverID  string  `json:"driver_id"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Available bool    `json:"available"`
	Timestamp int64   `json:"ts"` // unix millis, set by the ingest edge
}

// RideRequest is a rider asking to be matched to a driver.
type RideRequest struct {
	RiderID string  `json:"rider_id"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
}

// Candidate is a driver returned by proximity search, ranked for matching.
type Candidate struct {
	DriverID   string  `json:"driver_id"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	DistanceKm float64 `json:"distance_km"`
}

// TripState enumerates the trip lifecycle.
type TripState string

const (
	StateRequested  TripState = "REQUESTED"
	StateAssigned   TripState = "ASSIGNED"
	StateInProgress TripState = "IN_PROGRESS"
	StateCompleted  TripState = "COMPLETED"
	StateCancelled  TripState = "CANCELLED"
)

// Trip is the durable record of a ride.
type Trip struct {
	ID        string    `json:"id"`
	RiderID   string    `json:"rider_id"`
	DriverID  string    `json:"driver_id"`
	Region    string    `json:"region"`
	State     TripState `json:"state"`
	Surge     float64   `json:"surge"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TripEvent is one state transition, published to Kafka. The event stream is
// the source of truth: it gives replayability and an audit trail for free.
type TripEvent struct {
	TripID   string    `json:"trip_id"`
	RiderID  string    `json:"rider_id"`
	DriverID string    `json:"driver_id"`
	Region   string    `json:"region"`
	State    TripState `json:"state"`
	Surge    float64   `json:"surge"`
	At       int64     `json:"at"` // unix millis
}

// Notification is fanned out over pub/sub to a user's open socket.
type Notification struct {
	UserID  string `json:"user_id"`
	Kind    string `json:"kind"`
	TripID  string `json:"trip_id,omitempty"`
	Message string `json:"message"`
	At      int64  `json:"at"`
}
