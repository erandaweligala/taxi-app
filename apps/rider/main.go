// Command rider is the standalone rider front-end. It is a self-contained unit:
// it embeds its own static assets and depends on no internal packages, so it
// builds, deploys, and scales entirely independently of the driver app and the
// business services.
package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
)

//go:embed all:static
var assets embed.FS

func main() {
	addr := os.Getenv("RIDER_ADDR")
	if addr == "" {
		addr = ":8081"
	}
	static, err := fs.Sub(assets, "static")
	if err != nil {
		log.Fatalf("assets: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	mux.Handle("GET /", http.FileServer(http.FS(static)))

	log.Printf("rider app listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("rider: %v", err)
	}
}
