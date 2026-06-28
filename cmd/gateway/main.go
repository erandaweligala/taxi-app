// Command gateway is the real-time connection tier. It holds the long-lived
// WebSocket connections for riders and drivers and forwards notifications from
// the pub/sub backbone to the right socket. It is deliberately separate from
// the business logic so the socket layer — the sneaky scaling bottleneck at
// 50k+ concurrent connections — scales on its own axis. Any gateway node can
// serve any user because delivery is routed through Redis pub/sub, not node
// affinity.
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/erandaweligala/taxi-app/internal/config"
	"github.com/erandaweligala/taxi-app/internal/pubsub"
	"github.com/erandaweligala/taxi-app/internal/redisx"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(*http.Request) bool { return true }, // demo: allow all origins
}

func main() {
	cfg := config.Load()
	rdb := redisx.New(cfg.RedisAddr)
	if err := redisx.Ping(context.Background(), rdb); err != nil {
		log.Fatalf("redis: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) { serveWS(rdb, w, r) })

	log.Printf("gateway listening on %s", cfg.GatewayAddr)
	if err := http.ListenAndServe(cfg.GatewayAddr, mux); err != nil {
		log.Fatalf("gateway: %v", err)
	}
}

// serveWS upgrades the connection and bridges the user's pub/sub channel to it
// for the lifetime of the socket.
func serveWS(rdb *redis.Client, w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "user_id required", http.StatusBadRequest)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	sub := pubsub.Subscribe(ctx, rdb, userID)
	defer sub.Close()
	ch := sub.Channel()

	// Reader pump: detect client disconnect (and drain control frames) so we
	// can tear down the subscription promptly.
	go func() {
		defer cancel()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	log.Printf("socket open user=%s", userID)
	defer log.Printf("socket closed user=%s", userID)

	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
				return
			}
		case <-ping.C:
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
