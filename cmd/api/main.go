// Command api is the stateless HTTP front door. It accepts the driver-location
// firehose (publishing it to Kafka), serves the latency-sensitive
// request->match path against Redis, drives trip-lifecycle transitions as Kafka
// events, and exposes surge pricing. It holds no local state, so it scales by
// running more replicas behind a load balancer.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/erandaweligala/taxi-app/internal/config"
	"github.com/erandaweligala/taxi-app/internal/geo"
	"github.com/erandaweligala/taxi-app/internal/httpx"
	"github.com/erandaweligala/taxi-app/internal/kafkax"
	"github.com/erandaweligala/taxi-app/internal/match"
	"github.com/erandaweligala/taxi-app/internal/models"
	"github.com/erandaweligala/taxi-app/internal/redisx"
	"github.com/erandaweligala/taxi-app/internal/region"
	"github.com/erandaweligala/taxi-app/internal/surge"
	"github.com/erandaweligala/taxi-app/internal/trip"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

type server struct {
	cfg       config.Config
	geo       *geo.Store
	matcher   *match.Matcher
	surge     *surge.Engine
	trips     *trip.Cache
	locWr     *kafka.Writer
	tripWr    *kafka.Writer
	ingestSem chan struct{} // bounds in-flight location ingest (backpressure)
}

func main() {
	cfg := config.Load()
	ctx := context.Background()

	rdb := redisx.New(cfg.RedisAddr)
	if err := redisx.Ping(ctx, rdb); err != nil {
		log.Fatalf("redis: %v", err)
	}

	// Provision topics so a fresh cluster works without manual setup.
	for _, t := range []string{cfg.LocationTopic, cfg.TripTopic} {
		if err := kafkax.EnsureTopic(ctx, cfg.KafkaBrokers[0], t, 12); err != nil {
			log.Fatalf("ensure topic %s: %v", t, err)
		}
	}

	g := geo.New(rdb)
	s := &server{
		cfg:       cfg,
		geo:       g,
		matcher:   match.New(g),
		surge:     surge.New(rdb, g),
		trips:     trip.NewCache(rdb),
		locWr:     kafkax.NewWriter(cfg.KafkaBrokers, cfg.LocationTopic),
		tripWr:    kafkax.NewWriter(cfg.KafkaBrokers, cfg.TripTopic),
		ingestSem: make(chan struct{}, 5000),
	}
	defer s.locWr.Close()
	defer s.tripWr.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("POST /v1/drivers/{id}/location", s.handleLocation)
	mux.HandleFunc("POST /v1/rides", s.handleRide)
	mux.HandleFunc("POST /v1/trips/{id}/start", s.transition(models.StateInProgress))
	mux.HandleFunc("POST /v1/trips/{id}/complete", s.transition(models.StateCompleted))
	mux.HandleFunc("POST /v1/trips/{id}/cancel", s.transition(models.StateCancelled))
	mux.HandleFunc("GET /v1/trips/{id}", s.handleGetTrip)
	mux.HandleFunc("GET /v1/surge", s.handleSurge)

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: httpx.Logging(mux)}
	go func() {
		log.Printf("api listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
}

// handleLocation is the ingest edge for the driver-location firehose. It does
// the minimum work — validate and publish to Kafka — so it can absorb thousands
// of writes/sec. A bounded semaphore sheds load (503) before the process is
// overwhelmed, protecting the latency-sensitive match path.
func (s *server) handleLocation(w http.ResponseWriter, r *http.Request) {
	select {
	case s.ingestSem <- struct{}{}:
		defer func() { <-s.ingestSem }()
	default:
		http.Error(w, "overloaded", http.StatusServiceUnavailable)
		return
	}

	var u models.LocationUpdate
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	u.DriverID = r.PathValue("id")
	u.Timestamp = time.Now().UnixMilli()

	body, _ := json.Marshal(u)
	if err := s.locWr.WriteMessages(r.Context(), kafka.Message{
		Key:   []byte(region.Key(u.Lat, u.Lng)), // partition by region
		Value: body,
	}); err != nil {
		http.Error(w, "ingest failed", http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// handleRide runs the request->match path: record demand, match the nearest
// driver against Redis, then create and assign the trip as Kafka events.
func (s *server) handleRide(w http.ResponseWriter, r *http.Request) {
	var req models.RideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RiderID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	s.surge.RecordDemand(ctx, req.Lat, req.Lng)
	mult := s.surge.Multiplier(ctx, req.Lat, req.Lng)

	best, shortlist, ok, err := s.matcher.Best(ctx, req)
	if err != nil {
		http.Error(w, "match error", http.StatusInternalServerError)
		return
	}

	t := models.Trip{
		ID:      uuid.NewString(),
		RiderID: req.RiderID,
		Region:  region.Key(req.Lat, req.Lng),
		State:   models.StateRequested,
		Surge:   mult,
	}
	s.emit(ctx, t) // REQUESTED

	if !ok {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"trip": t, "matched": false, "surge": mult,
			"message": "no drivers available nearby",
		})
		return
	}

	t.DriverID = best.DriverID
	t.State = models.StateAssigned
	now := time.Now()
	t.CreatedAt, t.UpdatedAt = now, now
	if err := s.trips.Put(ctx, t); err != nil {
		http.Error(w, "cache error", http.StatusInternalServerError)
		return
	}
	s.emit(ctx, t) // ASSIGNED

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"trip": t, "matched": true, "driver": best,
		"shortlist": shortlist, "surge": mult,
	})
}

// transition returns a handler that moves an active trip to the target state,
// validating the lifecycle state machine and emitting the event.
func (s *server) transition(to models.TripState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id := r.PathValue("id")
		t, err := s.trips.Get(ctx, id)
		if errors.Is(err, trip.ErrNotFound) {
			http.Error(w, "trip not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "lookup error", http.StatusInternalServerError)
			return
		}
		if !trip.CanTransition(t.State, to) {
			http.Error(w, "illegal transition from "+string(t.State)+" to "+string(to), http.StatusConflict)
			return
		}
		t.State = to
		t.UpdatedAt = time.Now()
		if err := s.trips.Put(ctx, t); err != nil {
			http.Error(w, "cache error", http.StatusInternalServerError)
			return
		}
		s.emit(ctx, t)
		httpx.WriteJSON(w, http.StatusOK, t)
	}
}

func (s *server) handleGetTrip(w http.ResponseWriter, r *http.Request) {
	t, err := s.trips.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, trip.ErrNotFound) {
		http.Error(w, "trip not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "lookup error", http.StatusInternalServerError)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, t)
}

func (s *server) handleSurge(w http.ResponseWriter, r *http.Request) {
	lat, lng, err := httpx.LatLng(r)
	if err != nil {
		http.Error(w, "lat and lng required", http.StatusBadRequest)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"region": region.Key(lat, lng),
		"surge":  s.surge.Multiplier(r.Context(), lat, lng),
		"supply": s.geo.Supply(r.Context(), lat, lng),
	})
}

// emit publishes a trip event to Kafka. The trip worker consumes it to persist
// durably and to fan out notifications; the API never blocks on either.
func (s *server) emit(ctx context.Context, t models.Trip) {
	ev := models.TripEvent{
		TripID:   t.ID,
		RiderID:  t.RiderID,
		DriverID: t.DriverID,
		Region:   t.Region,
		State:    t.State,
		Surge:    t.Surge,
		At:       time.Now().UnixMilli(),
	}
	body, _ := json.Marshal(ev)
	if err := s.tripWr.WriteMessages(ctx, kafka.Message{
		Key:   []byte(t.Region),
		Value: body,
	}); err != nil {
		log.Printf("emit trip event %s/%s: %v", t.ID, t.State, err)
	}
}
