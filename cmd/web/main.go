// Command web serves the rider and driver browser apps as static assets. It is
// intentionally separate from the API and gateway: the front end scales and
// deploys independently of the business services, and can sit behind a CDN.
package main

import (
	"io/fs"
	"log"
	"net/http"

	"github.com/erandaweligala/taxi-app/internal/config"
	"github.com/erandaweligala/taxi-app/web"
)

func main() {
	cfg := config.Load()

	// The embedded FS is rooted at the web/ package dir; serve it directly so
	// "/" -> index.html, "/rider/" and "/driver/" -> their apps.
	sub, err := fs.Sub(web.Assets, ".")
	if err != nil {
		log.Fatalf("web assets: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	mux.Handle("GET /", http.FileServer(http.FS(sub)))

	log.Printf("web listening on %s (rider: /rider/  driver: /driver/)", cfg.WebAddr)
	if err := http.ListenAndServe(cfg.WebAddr, mux); err != nil {
		log.Fatalf("web: %v", err)
	}
}
