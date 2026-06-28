// Package config loads service configuration from the environment. Every
// service stays stateless; all connection info comes from env vars so the same
// binary runs unchanged across local docker-compose and any orchestrator.
package config

import (
	"os"
	"strings"
)

type Config struct {
	HTTPAddr      string   // api listen address
	GatewayAddr   string   // websocket gateway listen address
	WebAddr       string   // static web app listen address
	KafkaBrokers  []string // kafka bootstrap brokers
	RedisAddr     string   // redis address
	PostgresDSN   string   // postgres connection string
	LocationTopic string   // topic for the driver-location firehose
	TripTopic     string   // topic for durable trip lifecycle events
}

func Load() Config {
	return Config{
		HTTPAddr:      env("HTTP_ADDR", ":8080"),
		GatewayAddr:   env("GATEWAY_ADDR", ":8090"),
		WebAddr:       env("WEB_ADDR", ":8081"),
		KafkaBrokers:  split(env("KAFKA_BROKERS", "localhost:9092")),
		RedisAddr:     env("REDIS_ADDR", "localhost:6379"),
		PostgresDSN:   env("POSTGRES_DSN", "postgres://taxi:taxi@localhost:5432/taxi?sslmode=disable"),
		LocationTopic: env("LOCATION_TOPIC", "driver-locations"),
		TripTopic:     env("TRIP_TOPIC", "trip-events"),
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func split(s string) []string {
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
