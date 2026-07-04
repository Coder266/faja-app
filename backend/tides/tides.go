// Package tides scrapes today's high- and low-tide times for Funchal,
// Madeira from tabuademares.com. IPMA (used by the ipma package for
// weather/oceanography) has no tide data, so this is a different upstream
// source with its own package.
//
// Spike findings (2026-07-04), see docs/stories.md S4:
//
//   - robots.txt (https://tabuademares.com/robots.txt) disallows only
//     "/info" and "/actions" for crawlers in general (plus a handful of
//     named bots, e.g. SemrushBot, that this package isn't). The tide page
//     itself, /pt/madeira/funchal, is unrestricted.
//   - The page has no JSON-LD or other structured data for tides — it's a
//     plain HTML table, <table id="tabla_mareas">, listing every day of the
//     current month (one <tr> per day). See parse.go for the selector
//     strategy this package uses against that table.
package tides

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// madeiraLocation is the reference timezone for "today" throughout this
// package, since the poster this app replicates (and the tide table itself)
// is local to Madeira.
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

const pageURL = "https://tabuademares.com/pt/madeira/funchal"

const defaultTimeout = 10 * time.Second

// userAgent is sent on every request per the story's requirement for a
// descriptive User-Agent, so tabuademares.com can identify/contact us if
// our scraping causes trouble, rather than looking like an anonymous bot.
const userAgent = "faja-app-tide-scraper/1.0 (+https://github.com/Coder266/faja-app)"

// cacheTTL is how long a successfully parsed result is reused before
// re-scraping. Tide tables are precomputed once per month, so there's no
// need to hit the page more than a few times a day; 6h comfortably keeps
// scrapes to well under that. See cacheEntry.valid for the other
// invalidation condition (Madeira-local date rollover).
const cacheTTL = 6 * time.Hour

// Error is a typed error returned by this package's Client methods, so
// callers can distinguish tide-scraping failures from other errors without
// string matching. It's only used for hard failures (request/network
// errors, non-200 responses); a page-structure change that defeats parsing
// is not an Error — see FetchTideTimes.
type Error struct {
	Op  string // operation being attempted, e.g. "fetch tide page"
	Err error
}

func (e *Error) Error() string {
	return fmt.Sprintf("tides: %s: %v", e.Op, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}

// TideTimes is today's tide events for Funchal, split into high and low
// tides, each formatted "HH:MM" in Madeira local time. A normal day has 3-4
// events combined. JSON tags match the aggregated /api/conditions response
// (see docs/stories.md, S5).
type TideTimes struct {
	HighTides []string `json:"highTides"`
	LowTides  []string `json:"lowTides"`
}

// cacheEntry is a previously scraped-and-parsed result, along with enough
// information to tell whether it's still usable.
type cacheEntry struct {
	tides *TideTimes // may be nil: a successful scrape that found no usable data
	date  string     // Madeira-local YYYY-MM-DD this was scraped for
	at    time.Time  // when it was scraped, for TTL expiry
}

// valid reports whether e is still usable for a request made "now" for
// "todayStr" (Madeira-local YYYY-MM-DD): both within the TTL and still the
// same Madeira-local date it was scraped for. The date check matters
// separately from the TTL because a scrape made late in the evening (e.g.
// 23:30) would otherwise still look "fresh" under a 6h TTL well into the
// next day, serving yesterday's tides past midnight.
func (e *cacheEntry) valid(now time.Time, todayStr string) bool {
	return e != nil && e.date == todayStr && now.Sub(e.at) < cacheTTL
}

// Client scrapes tabuademares.com for today's tide times.
type Client struct {
	// HTTPClient is used to perform requests. If nil, a client with
	// defaultTimeout is used.
	HTTPClient *http.Client

	// BaseURL overrides the tide page URL for tests. If empty,
	// "https://tabuademares.com/pt/madeira/funchal" is used.
	BaseURL string

	// Now returns the current time and is used to determine "today" and
	// to drive the cache's TTL/date-rollover logic. If nil, time.Now is
	// used. Tests can override this to pin "today" to a fixture's dates,
	// and to exercise TTL expiry and date rollover without sleeping.
	Now func() time.Time

	mu    sync.Mutex
	cache *cacheEntry
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: defaultTimeout}
}

func (c *Client) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now().In(madeiraLocation)
}

func (c *Client) pageURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return pageURL
}

// FetchTideTimes returns today's tide times for Funchal, scraping
// tabuademares.com (or reusing a cached result — see cacheEntry.valid).
//
// The return signature distinguishes two different kinds of "no result":
//
//   - tides == nil, err == nil: the page was fetched fine, but its
//     structure didn't match what this package's parser expects (site
//     markup changed, or no row for today was found). This is logged as a
//     warning by the parser and is the graceful path — callers (e.g. the
//     /api/conditions aggregator in S5) should surface this as a null
//     tides field, not fail the whole response, and never crash.
//   - err != nil: a genuine failure to fetch the page — request/network
//     error or a non-200 status. Always a *Error. Unlike a successful
//     scrape with no usable data, this is not cached, so the next call
//     retries.
func (c *Client) FetchTideTimes() (*TideTimes, error) {
	// Normalize to Madeira local time up front so the cache key
	// (todayStr) and the date parseTideTimes matches against always agree,
	// regardless of what timezone an injected Now returns.
	now := c.now().In(madeiraLocation)
	todayStr := now.Format("2006-01-02")

	c.mu.Lock()
	cached := c.cache
	c.mu.Unlock()
	if cached.valid(now, todayStr) {
		return cached.tides, nil
	}

	tides, err := c.scrape(now)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.cache = &cacheEntry{tides: tides, date: todayStr, at: now}
	c.mu.Unlock()

	return tides, nil
}

// scrape fetches the tide page and parses it for "today" (in Madeira
// local time, per now). It never panics; fetch failures are returned as a
// *Error, while parse failures are handled defensively inside
// parseTideTimes (logged, nil result, nil error).
func (c *Client) scrape(now time.Time) (*TideTimes, error) {
	req, err := http.NewRequest(http.MethodGet, c.pageURL(), nil)
	if err != nil {
		return nil, &Error{Op: "build tide page request", Err: err}
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, &Error{Op: "fetch tide page", Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &Error{Op: "fetch tide page", Err: fmt.Errorf("unexpected status %d", resp.StatusCode)}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &Error{Op: "read tide page", Err: err}
	}

	return parseTideTimes(body, now), nil
}
