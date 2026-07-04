// Package conditions aggregates the IPMA and tide data sources into the
// single /api/conditions response the frontend poster displays (see
// docs/stories.md, S5).
package conditions

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Coder266/faja-app/backend/ipma"
	"github.com/Coder266/faja-app/backend/tides"
)

// madeiraLocation is the reference timezone for "today" throughout this
// package, matching the ipma and tides packages: the poster this app
// replicates is displayed in Madeira.
var madeiraLocation = mustLoadLocation("Atlantic/Madeira")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		// Fall back to UTC rather than panicking; callers still get
		// deterministic (if slightly wrong) behavior in environments
		// without a tzdata database.
		return time.UTC
	}
	return loc
}

// cacheTTL is how long an assembled response is reused before the sources
// are re-fetched. This is separate from (and shorter than) the tides
// package's own internal cache TTL, which the tides.Client keeps regardless
// of this package's cache.
const cacheTTL = 30 * time.Minute

// Conditions is the aggregated response served at GET /api/conditions.
// Fields sourced from an upstream that failed or had no data are left nil
// so they serialize as JSON null, rather than being omitted or zero-valued;
// date and updatedAt are always present.
type Conditions struct {
	Date       string           `json:"date"`
	AirTempMin *int             `json:"airTempMin"`
	AirTempMax *int             `json:"airTempMax"`
	WaterTemp  *int             `json:"waterTemp"`
	UV         *ipma.UVIndex    `json:"uv"`
	Tides      *tides.TideTimes `json:"tides"`
	UpdatedAt  string           `json:"updatedAt"`
}

// airTemperatureFetcher is satisfied by *ipma.Client; defined here so tests
// can inject a mock instead of hitting the live network.
type airTemperatureFetcher interface {
	FetchAirTemperature() (ipma.AirTemperature, error)
}

// uvIndexFetcher is satisfied by *ipma.Client.
type uvIndexFetcher interface {
	FetchUVIndex() (ipma.UVIndex, error)
}

// waterTemperatureFetcher is satisfied by *ipma.Client.
type waterTemperatureFetcher interface {
	FetchWaterTemperature() (temp int, found bool, err error)
}

// tideTimesFetcher is satisfied by *tides.Client.
type tideTimesFetcher interface {
	FetchTideTimes() (*tides.TideTimes, error)
}

// cacheEntry is a previously assembled response, along with enough
// information to tell whether it's still usable.
type cacheEntry struct {
	conditions *Conditions
	date       string // Madeira-local YYYY-MM-DD this was assembled for
	at         time.Time
}

// valid reports whether e is still usable for a request made "now" for
// "todayStr" (Madeira-local YYYY-MM-DD): both within the TTL and still the
// same Madeira-local date it was assembled for. The date check matters
// separately from the TTL so a response assembled late in the evening
// doesn't get served past midnight under a still-fresh TTL.
func (e *cacheEntry) valid(now time.Time, todayStr string) bool {
	return e != nil && e.date == todayStr && now.Sub(e.at) < cacheTTL
}

// Aggregator fetches all upstream sources concurrently and assembles the
// /api/conditions response, caching it in memory.
type Aggregator struct {
	AirTemp airTemperatureFetcher
	UV      uvIndexFetcher
	Water   waterTemperatureFetcher
	Tides   tideTimesFetcher

	// Now returns the current time and is used to determine "today" and to
	// drive the cache's TTL/date-rollover logic. If nil, time.Now is used.
	// Tests can override this to pin "today" and exercise caching without
	// sleeping.
	Now func() time.Time

	mu    sync.Mutex
	cache *cacheEntry
}

func (a *Aggregator) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now().In(madeiraLocation)
}

// Get returns the current aggregated conditions, reusing a cached response
// if one is still valid (see cacheEntry.valid), or assembling a fresh one
// by fetching all sources concurrently.
func (a *Aggregator) Get() *Conditions {
	now := a.now().In(madeiraLocation)
	todayStr := now.Format("2006-01-02")

	a.mu.Lock()
	cached := a.cache
	a.mu.Unlock()
	if cached.valid(now, todayStr) {
		return cached.conditions
	}

	conditions := a.fetch(now, todayStr)

	a.mu.Lock()
	a.cache = &cacheEntry{conditions: conditions, date: todayStr, at: now}
	a.mu.Unlock()

	return conditions
}

// fetch concurrently queries every upstream source and assembles the
// response. A failing (or gracefully-absent) source leaves its own field(s)
// nil and is logged; it never fails the whole response.
func (a *Aggregator) fetch(now time.Time, todayStr string) *Conditions {
	c := &Conditions{
		Date:      todayStr,
		UpdatedAt: now.UTC().Format(time.RFC3339),
	}

	var wg sync.WaitGroup
	wg.Add(4)

	go func() {
		defer wg.Done()
		airTemp, err := a.AirTemp.FetchAirTemperature()
		if err != nil {
			log.Printf("conditions: fetch air temperature: %v", err)
			return
		}
		min, max := airTemp.Min, airTemp.Max
		c.AirTempMin = &min
		c.AirTempMax = &max
	}()

	go func() {
		defer wg.Done()
		uv, err := a.UV.FetchUVIndex()
		if err != nil {
			log.Printf("conditions: fetch uv index: %v", err)
			return
		}
		c.UV = &uv
	}()

	go func() {
		defer wg.Done()
		temp, found, err := a.Water.FetchWaterTemperature()
		if err != nil {
			log.Printf("conditions: fetch water temperature: %v", err)
			return
		}
		if !found {
			return
		}
		c.WaterTemp = &temp
	}()

	go func() {
		defer wg.Done()
		tt, err := a.Tides.FetchTideTimes()
		if err != nil {
			log.Printf("conditions: fetch tide times: %v", err)
			return
		}
		c.Tides = tt
	}()

	wg.Wait()
	return c
}

// Handler returns an http.HandlerFunc serving GET /api/conditions.
func (a *Aggregator) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conditions := a.Get()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(conditions); err != nil {
			log.Printf("conditions: encode response: %v", err)
		}
	}
}
