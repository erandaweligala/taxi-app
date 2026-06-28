// Package pubsub is the real-time fan-out backbone. Notifications are published
// to a per-user channel; whichever gateway node currently holds that user's
// socket is subscribed and forwards the message. This decouples message
// delivery from which node owns a connection, so the socket tier scales freely.
package pubsub

import (
	"context"
	"encoding/json"

	"github.com/erandaweligala/taxi-app/internal/models"
	"github.com/redis/go-redis/v9"
)

// Channel returns the per-user notification channel name.
func Channel(userID string) string { return "notify:" + userID }

type Publisher struct{ rdb *redis.Client }

func NewPublisher(rdb *redis.Client) *Publisher { return &Publisher{rdb: rdb} }

func (p *Publisher) Publish(ctx context.Context, n models.Notification) error {
	b, err := json.Marshal(n)
	if err != nil {
		return err
	}
	return p.rdb.Publish(ctx, Channel(n.UserID), b).Err()
}

// Subscribe returns a redis pubsub subscription for a user's channel.
func Subscribe(ctx context.Context, rdb *redis.Client, userID string) *redis.PubSub {
	return rdb.Subscribe(ctx, Channel(userID))
}
