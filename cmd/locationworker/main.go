// Command locationworker consumes the driver-location firehose from Kafka and
// writes positions into the Redis geo index. Decoupling ingest (the API) from
// indexing (this worker) means a spike in pings queues in Kafka instead of
// overwhelming Redis, and the indexer scales independently by adding consumers
// to the group. Run multiple replicas to share the topic's partitions.
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/erandaweligala/taxi-app/internal/config"
	"github.com/erandaweligala/taxi-app/internal/geo"
	"github.com/erandaweligala/taxi-app/internal/kafkax"
	"github.com/erandaweligala/taxi-app/internal/models"
	"github.com/erandaweligala/taxi-app/internal/redisx"
)

func main() {
	cfg := config.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rdb := redisx.New(cfg.RedisAddr)
	if err := redisx.Ping(ctx, rdb); err != nil {
		log.Fatalf("redis: %v", err)
	}
	g := geo.New(rdb)

	reader := kafkax.NewReader(cfg.KafkaBrokers, cfg.LocationTopic, "location-indexer")
	defer reader.Close()

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		<-stop
		cancel()
	}()

	log.Printf("locationworker consuming %s", cfg.LocationTopic)
	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("read: %v", err)
			continue
		}
		var u models.LocationUpdate
		if err := json.Unmarshal(msg.Value, &u); err != nil {
			log.Printf("decode: %v", err) // poison message: skip, don't block the stream
			continue
		}
		if err := g.Upsert(ctx, u); err != nil {
			log.Printf("upsert %s: %v", u.DriverID, err)
		}
	}
}
