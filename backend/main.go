package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/Coder266/faja-app/backend/conditions"
	"github.com/Coder266/faja-app/backend/ipma"
	"github.com/Coder266/faja-app/backend/tides"
	"github.com/Coder266/faja-app/backend/web"
)

// viteDevOrigin is the Vite dev server's default origin. As of S9, `npm run
// dev` proxies /api/* to this backend (see frontend/vite.config.ts), so the
// browser only ever talks to the Vite origin and CORS is no longer strictly
// needed for that flow. withCORS is kept anyway as a harmless fallback for
// anyone hitting :8080 directly from a page served on :5174.
const viteDevOrigin = "http://localhost:5174"

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

	// Serve the built frontend (embedded from backend/web/dist, see
	// backend/web/web.go) at "/". Registered on the same mux as the /api/*
	// routes above; ServeMux matches the most specific pattern, so /api/*
	// requests are handled by the API and everything else falls through to
	// the static file server.
	mux.Handle("/", http.FileServer(http.FS(web.DistFS())))

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
