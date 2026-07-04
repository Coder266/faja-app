package tides

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchTideTimes_ParsesFixtureAndSendsUserAgent(t *testing.T) {
	fixture, err := os.ReadFile("testdata/funchal.html")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	var gotUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		w.Write(fixture)
	}))
	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		Now: func() time.Time {
			return time.Date(2026, time.July, 4, 9, 0, 0, 0, time.UTC)
		},
	}

	tides, err := client.FetchTideTimes()
	if err != nil {
		t.Fatalf("FetchTideTimes() returned error: %v", err)
	}
	if tides == nil {
		t.Fatal("FetchTideTimes() = nil, want a result")
	}

	wantHigh := []string{"05:25", "17:38"}
	wantLow := []string{"11:17", "23:49"}
	if !equalStrings(tides.HighTides, wantHigh) {
		t.Errorf("HighTides = %v, want %v", tides.HighTides, wantHigh)
	}
	if !equalStrings(tides.LowTides, wantLow) {
		t.Errorf("LowTides = %v, want %v", tides.LowTides, wantLow)
	}

	if gotUserAgent == "" {
		t.Error("request had no User-Agent header, want a descriptive one")
	}
	if gotUserAgent != userAgent {
		t.Errorf("User-Agent = %q, want %q", gotUserAgent, userAgent)
	}
}

func TestFetchTideTimes_CachesWithinTTL(t *testing.T) {
	fixture, err := os.ReadFile("testdata/funchal.html")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.Write(fixture)
	}))
	defer server.Close()

	now := time.Date(2026, time.July, 4, 9, 0, 0, 0, time.UTC)
	client := &Client{
		BaseURL: server.URL,
		Now:     func() time.Time { return now },
	}

	if _, err := client.FetchTideTimes(); err != nil {
		t.Fatalf("first FetchTideTimes() returned error: %v", err)
	}
	if _, err := client.FetchTideTimes(); err != nil {
		t.Fatalf("second FetchTideTimes() returned error: %v", err)
	}

	// Still well within the 6h TTL and the same Madeira-local date.
	now = now.Add(3 * time.Hour)
	if _, err := client.FetchTideTimes(); err != nil {
		t.Fatalf("third FetchTideTimes() returned error: %v", err)
	}

	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Errorf("server received %d requests, want 1 (result should be cached)", got)
	}
}

func TestFetchTideTimes_ReScrapesAfterTTLExpiry(t *testing.T) {
	fixture, err := os.ReadFile("testdata/funchal.html")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.Write(fixture)
	}))
	defer server.Close()

	now := time.Date(2026, time.July, 4, 9, 0, 0, 0, time.UTC)
	client := &Client{
		BaseURL: server.URL,
		Now:     func() time.Time { return now },
	}

	if _, err := client.FetchTideTimes(); err != nil {
		t.Fatalf("first FetchTideTimes() returned error: %v", err)
	}

	// Past the 6h TTL, still the same date.
	now = now.Add(7 * time.Hour)
	if _, err := client.FetchTideTimes(); err != nil {
		t.Fatalf("second FetchTideTimes() returned error: %v", err)
	}

	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Errorf("server received %d requests, want 2 (TTL should have expired)", got)
	}
}

func TestFetchTideTimes_ReScrapesOnDateRollover(t *testing.T) {
	fixture, err := os.ReadFile("testdata/funchal.html")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.Write(fixture)
	}))
	defer server.Close()

	now := time.Date(2026, time.July, 4, 23, 30, 0, 0, madeiraLocation)
	client := &Client{
		BaseURL: server.URL,
		Now:     func() time.Time { return now },
	}

	first, err := client.FetchTideTimes()
	if err != nil {
		t.Fatalf("first FetchTideTimes() returned error: %v", err)
	}
	if first == nil || first.HighTides[0] != "05:25" {
		t.Fatalf("first FetchTideTimes() = %+v, want July 4th's tides", first)
	}

	// Only 1 hour later (well within the 6h TTL), but past midnight: the
	// Madeira-local date has rolled over to July 5th, which must trigger
	// a re-scrape rather than serving July 4th's cached tides.
	now = now.Add(1 * time.Hour)
	second, err := client.FetchTideTimes()
	if err != nil {
		t.Fatalf("second FetchTideTimes() returned error: %v", err)
	}
	if second == nil || second.HighTides[0] != "06:02" {
		t.Fatalf("second FetchTideTimes() = %+v, want July 5th's tides", second)
	}

	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Errorf("server received %d requests, want 2 (date rollover should re-scrape)", got)
	}
}

func TestFetchTideTimes_MalformedPageReturnsNilTidesNoError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>tabuademares.com redesigned this page</body></html>`))
	}))
	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		Now: func() time.Time {
			return time.Date(2026, time.July, 4, 9, 0, 0, 0, time.UTC)
		},
	}

	tides, err := client.FetchTideTimes()
	if err != nil {
		t.Fatalf("FetchTideTimes() returned error: %v, want nil (defensive parse failure)", err)
	}
	if tides != nil {
		t.Errorf("FetchTideTimes() tides = %+v, want nil for an unparsable page", tides)
	}
}

func TestFetchTideTimes_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL}

	_, err := client.FetchTideTimes()
	if err == nil {
		t.Fatal("expected an error for a non-200 response, got nil")
	}

	var tidesErr *Error
	if !errors.As(err, &tidesErr) {
		t.Errorf("expected error to be *tides.Error, got %T: %v", err, err)
	}
}

func TestFetchTideTimes_Unreachable(t *testing.T) {
	client := &Client{BaseURL: "http://127.0.0.1:1"} // nothing listens here

	_, err := client.FetchTideTimes()
	if err == nil {
		t.Fatal("expected an error when the server is unreachable, got nil")
	}

	var tidesErr *Error
	if !errors.As(err, &tidesErr) {
		t.Errorf("expected error to be *tides.Error, got %T: %v", err, err)
	}
}

func TestFetchTideTimes_FetchFailureIsNotCached(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requests, 1)
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fixture, _ := os.ReadFile("testdata/funchal.html")
		w.Write(fixture)
	}))
	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		Now: func() time.Time {
			return time.Date(2026, time.July, 4, 9, 0, 0, 0, time.UTC)
		},
	}

	if _, err := client.FetchTideTimes(); err == nil {
		t.Fatal("expected an error on the first (failing) request, got nil")
	}

	tides, err := client.FetchTideTimes()
	if err != nil {
		t.Fatalf("second FetchTideTimes() returned error: %v, want a successful retry", err)
	}
	if tides == nil {
		t.Fatal("second FetchTideTimes() = nil tides, want a result once the server recovers")
	}
	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Errorf("server received %d requests, want 2 (a failed fetch must not be cached)", got)
	}
}
