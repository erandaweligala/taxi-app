// Command tripworker consumes the durable trip-event stream. For each event it
// (1) persists the trip projection and audit log to Postgres and (2) fans out a
// real-time notification to the affected rider and driver over the pub/sub
// backbone. Keeping persistence and notification off the API's hot path is what
// lets the request->match flow stay fast while trips remain durable.
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/erandaweligala/taxi-app/internal/config"
	"github.com/erandaweligala/taxi-app/internal/db"
	"github.com/erandaweligala/taxi-app/internal/kafkax"
	"github.com/erandaweligala/taxi-app/internal/models"
	"github.com/erandaweligala/taxi-app/internal/pubsub"
	"github.com/erandaweligala/taxi-app/internal/redisx"
	"github.com/erandaweligala/taxi-app/internal/trip"
)

func main() {
	cfg := config.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := db.Connect(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer pool.Close()
	store := trip.NewStore(pool)
	if err := store.Migrate(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	rdb := redisx.New(cfg.RedisAddr)
	if err := redisx.Ping(ctx, rdb); err != nil {
		log.Fatalf("redis: %v", err)
	}
	pub := pubsub.NewPublisher(rdb)

	reader := kafkax.NewReader(cfg.KafkaBrokers, cfg.TripTopic, "trip-persister")
	defer reader.Close()

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		<-stop
		cancel()
	}()

	log.Printf("tripworker consuming %s", cfg.TripTopic)
	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("read: %v", err)
			continue
		}
		var ev models.TripEvent
		if err := json.Unmarshal(msg.Value, &ev); err != nil {
			log.Printf("decode: %v", err)
			continue
		}
		if err := store.Apply(ctx, ev); err != nil {
			log.Printf("persist %s/%s: %v", ev.TripID, ev.State, err)
			continue // let the offset stay uncommitted-ish; retried on restart
		}
		notify(ctx, pub, ev)
	}
}

// notify fans out human-readable updates to the rider and (when assigned) the
// driver for each lifecycle transition.
func notify(ctx context.Context, pub *pubsub.Publisher, ev models.TripEvent) {
	riderMsg, driverMsg := messages(ev.State)
	now := time.Now().UnixMilli()
	if riderMsg != "" && ev.RiderID != "" {
		_ = pub.Publish(ctx, models.Notification{
			UserID: ev.RiderID, Kind: "trip:" + string(ev.State),
			TripID: ev.TripID, Message: riderMsg, At: now,
		})
	}
	if driverMsg != "" && ev.DriverID != "" {
		_ = pub.Publish(ctx, models.Notification{
			UserID: ev.DriverID, Kind: "trip:" + string(ev.State),
			TripID: ev.TripID, Message: driverMsg, At: now,
		})
	}
}

func messages(state models.TripState) (rider, driver string) {
	switch state {
	case models.StateRequested:
		return "Looking for a nearby driver…", ""
	case models.StateAssigned:
		return "Driver assigned and on the way", "New trip assigned to you"
	case models.StateInProgress:
		return "Your trip has started", "Trip started"
	case models.StateCompleted:
		return "Trip complete — thanks for riding!", "Trip completed"
	case models.StateCancelled:
		return "Your trip was cancelled", "Trip cancelled"
	}
	return "", ""
}
