package ipma

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// fixedNow returns a Now func pinned to the given date at noon Madeira time.
func fixedNow(dateStr string) func() time.Time {
	return func() time.Time {
		t, err := time.ParseInLocation("2006-01-02", dateStr, madeiraLocation)
		if err != nil {
			panic(err)
		}
		return t.Add(12 * time.Hour)
	}
}

func TestFetchAirTemperature_ParsesFixture(t *testing.T) {
	fixture, err := os.ReadFile("testdata/daily_2310300.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cities/daily/2310300.json" {
			t.Errorf("unexpected request path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	}))
	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		Now:     fixedNow("2026-07-04"),
	}

	got, err := client.FetchAirTemperature()
	if err != nil {
		t.Fatalf("FetchAirTemperature() returned error: %v", err)
	}

	want := AirTemperature{Min: 21, Max: 26}
	if got != want {
		t.Errorf("FetchAirTemperature() = %+v, want %+v", got, want)
	}
}

func TestFetchAirTemperature_RoundsToWholeDegrees(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"forecastDate":"2026-07-05","tMin":"21.2","tMax":"25.6"}]}`))
	}))
	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		Now:     fixedNow("2026-07-05"),
	}

	got, err := client.FetchAirTemperature()
	if err != nil {
		t.Fatalf("FetchAirTemperature() returned error: %v", err)
	}

	// 21.2 rounds down to 21, 25.6 rounds up to 26.
	want := AirTemperature{Min: 21, Max: 26}
	if got != want {
		t.Errorf("FetchAirTemperature() = %+v, want %+v", got, want)
	}
}

func TestFetchAirTemperature_NoEntryForToday(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"forecastDate":"2026-07-05","tMin":"21.2","tMax":"25.6"}]}`))
	}))
	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		Now:     fixedNow("2026-07-04"),
	}

	_, err := client.FetchAirTemperature()
	if err == nil {
		t.Fatal("expected an error when no forecast entry matches today, got nil")
	}

	var ipmaErr *Error
	if !errors.As(err, &ipmaErr) {
		t.Errorf("expected error to be *ipma.Error, got %T: %v", err, err)
	}
}

func TestFetchAirTemperature_MalformedPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{not valid json`))
	}))
	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		Now:     fixedNow("2026-07-04"),
	}

	_, err := client.FetchAirTemperature()
	if err == nil {
		t.Fatal("expected an error for malformed payload, got nil")
	}

	var ipmaErr *Error
	if !errors.As(err, &ipmaErr) {
		t.Errorf("expected error to be *ipma.Error, got %T: %v", err, err)
	}
}

func TestFetchAirTemperature_InvalidTemperatureValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"forecastDate":"2026-07-04","tMin":"n/a","tMax":"26.0"}]}`))
	}))
	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		Now:     fixedNow("2026-07-04"),
	}

	_, err := client.FetchAirTemperature()
	if err == nil {
		t.Fatal("expected an error for invalid temperature value, got nil")
	}

	var ipmaErr *Error
	if !errors.As(err, &ipmaErr) {
		t.Errorf("expected error to be *ipma.Error, got %T: %v", err, err)
	}
}

func TestFetchAirTemperature_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		Now:     fixedNow("2026-07-04"),
	}

	_, err := client.FetchAirTemperature()
	if err == nil {
		t.Fatal("expected an error for a non-200 response, got nil")
	}

	var ipmaErr *Error
	if !errors.As(err, &ipmaErr) {
		t.Errorf("expected error to be *ipma.Error, got %T: %v", err, err)
	}
}

func TestFetchAirTemperature_Unreachable(t *testing.T) {
	client := &Client{
		BaseURL: "http://127.0.0.1:1", // nothing listens here
		Now:     fixedNow("2026-07-04"),
	}

	_, err := client.FetchAirTemperature()
	if err == nil {
		t.Fatal("expected an error when the server is unreachable, got nil")
	}

	var ipmaErr *Error
	if !errors.As(err, &ipmaErr) {
		t.Errorf("expected error to be *ipma.Error, got %T: %v", err, err)
	}
}
