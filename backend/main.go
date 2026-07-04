package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/Coder266/faja-app/backend/conditions"
	"github.com/Coder266/faja-app/backend/ipma"
	"github.com/Coder266/faja-app/backend/tides"
)

// viteDevOrigin is the Vite dev server's default origin. Permissive CORS for
// this single origin unblocks local frontend development against the
// backend on :8080; S9 finalizes dev/prod wiring (Vite proxying /api, or a
// single embedded-binary deployment) where this will likely be revisited.
const viteDevOrigin = "http://localhost:5173"

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", healthHandler)

	ipmaClient := &ipma.Client{}
	tidesClient := &tides.Client{}
	aggregator := &conditions.Aggregator{
		AirTemp: ipmaClient,
		UV:      ipmaClient,
		Water:   ipmaClient,
		Tides:   tidesClient,
	}
	mux.Handle("/api/conditions", aggregator.Handler())

	log.Println("listening on :8080")
	if err := http.ListenAndServe(":8080", withCORS(mux)); err != nil {
		log.Fatal(err)
	}
}

// withCORS allows the Vite dev origin to call the API during local
// development.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", viteDevOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
