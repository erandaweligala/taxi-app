// Package httpx holds small HTTP helpers shared by the services.
package httpx

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"
)

// WriteJSON serializes v as a JSON response with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// LatLng extracts and validates the lat/lng query parameters.
func LatLng(r *http.Request) (float64, float64, error) {
	latS, lngS := r.URL.Query().Get("lat"), r.URL.Query().Get("lng")
	if latS == "" || lngS == "" {
		return 0, 0, errors.New("missing lat/lng")
	}
	lat, err1 := strconv.ParseFloat(latS, 64)
	lng, err2 := strconv.ParseFloat(lngS, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, errors.New("invalid lat/lng")
	}
	return lat, lng, nil
}

// CORS allows the browser apps (served from a different port) to call the API.
// Permissive by design for the demo; tighten the allowed origin in production.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Logging is a minimal request-logging middleware.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
