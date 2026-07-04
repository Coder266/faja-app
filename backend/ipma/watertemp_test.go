package ipma

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestFetchWaterTemperature_ParsesFixture(t *testing.T) {
	fixture, err := os.ReadFile("testdata/oceanography_day0.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oceanography/daily/hp-daily-sea-forecast-day0.json" {
			t.Errorf("unexpected request path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL}

	temp, found, err := client.FetchWaterTemperature()
	if err != nil {
		t.Fatalf("FetchWaterTemperature() returned error: %v", err)
	}
	if !found {
		t.Fatal("FetchWaterTemperature() found = false, want true")
	}

	// Fixture has sstMin "18.5"/sstMax "19.9" for globalIdLocal 2310326;
	// midpoint 19.2 rounds to 19.
	if temp != 19 {
		t.Errorf("FetchWaterTemperature() temp = %d, want 19", temp)
	}
}

func TestFetchWaterTemperature_MissingPointFallsBackGracefully(t *testing.T) {
	fixture, err := os.ReadFile("testdata/oceanography_day0_missing.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(fixture)
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL}

	temp, found, err := client.FetchWaterTemperature()
	if err != nil {
		t.Fatalf("FetchWaterTemperature() returned error: %v, want nil (graceful absence)", err)
	}
	if found {
		t.Errorf("FetchWaterTemperature() found = true, want false; temp = %d", temp)
	}
}

func TestFetchWaterTemperature_NullFieldsFallBackGracefully(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"globalIdLocal":2310326,"sstMin":null,"sstMax":null}]}`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL}

	temp, found, err := client.FetchWaterTemperature()
	if err != nil {
		t.Fatalf("FetchWaterTemperature() returned error: %v, want nil (graceful absence)", err)
	}
	if found {
		t.Errorf("FetchWaterTemperature() found = true, want false; temp = %d", temp)
	}
}

func TestFetchWaterTemperature_OmittedFieldsFallBackGracefully(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"globalIdLocal":2310326}]}`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL}

	temp, found, err := client.FetchWaterTemperature()
	if err != nil {
		t.Fatalf("FetchWaterTemperature() returned error: %v, want nil (graceful absence)", err)
	}
	if found {
		t.Errorf("FetchWaterTemperature() found = true, want false; temp = %d", temp)
	}
}

func TestFetchWaterTemperature_RoundsMidpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"globalIdLocal":2310326,"sstMin":"18.0","sstMax":"19.9"}]}`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL}

	temp, found, err := client.FetchWaterTemperature()
	if err != nil {
		t.Fatalf("FetchWaterTemperature() returned error: %v", err)
	}
	if !found {
		t.Fatal("FetchWaterTemperature() found = false, want true")
	}

	// Midpoint (18.0+19.9)/2 = 18.95, rounds to 19.
	if temp != 19 {
		t.Errorf("FetchWaterTemperature() temp = %d, want 19", temp)
	}
}

func TestFetchWaterTemperature_IgnoresOtherLocations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"globalIdLocal":1010500,"sstMin":"20.0","sstMax":"21.0"},{"globalIdLocal":2310326,"sstMin":"16.0","sstMax":"18.0"}]}`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL}

	temp, found, err := client.FetchWaterTemperature()
	if err != nil {
		t.Fatalf("FetchWaterTemperature() returned error: %v", err)
	}
	if !found {
		t.Fatal("FetchWaterTemperature() found = false, want true")
	}

	// Must pick the Madeira south coast entry (17.0), not the other
	// location's (20.5).
	if temp != 17 {
		t.Errorf("FetchWaterTemperature() temp = %d, want 17", temp)
	}
}

func TestFetchWaterTemperature_InvalidSstMinValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"globalIdLocal":2310326,"sstMin":"n/a","sstMax":"19.0"}]}`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL}

	_, _, err := client.FetchWaterTemperature()
	if err == nil {
		t.Fatal("expected an error for invalid sstMin value, got nil")
	}

	var ipmaErr *Error
	if !errors.As(err, &ipmaErr) {
		t.Errorf("expected error to be *ipma.Error, got %T: %v", err, err)
	}
}

func TestFetchWaterTemperature_InvalidSstMaxValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"globalIdLocal":2310326,"sstMin":"18.0","sstMax":"n/a"}]}`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL}

	_, _, err := client.FetchWaterTemperature()
	if err == nil {
		t.Fatal("expected an error for invalid sstMax value, got nil")
	}

	var ipmaErr *Error
	if !errors.As(err, &ipmaErr) {
		t.Errorf("expected error to be *ipma.Error, got %T: %v", err, err)
	}
}

func TestFetchWaterTemperature_MalformedPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{not valid json`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL}

	_, _, err := client.FetchWaterTemperature()
	if err == nil {
		t.Fatal("expected an error for malformed payload, got nil")
	}

	var ipmaErr *Error
	if !errors.As(err, &ipmaErr) {
		t.Errorf("expected error to be *ipma.Error, got %T: %v", err, err)
	}
}

func TestFetchWaterTemperature_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL}

	_, _, err := client.FetchWaterTemperature()
	if err == nil {
		t.Fatal("expected an error for a non-200 response, got nil")
	}

	var ipmaErr *Error
	if !errors.As(err, &ipmaErr) {
		t.Errorf("expected error to be *ipma.Error, got %T: %v", err, err)
	}
}

func TestFetchWaterTemperature_Unreachable(t *testing.T) {
	client := &Client{BaseURL: "http://127.0.0.1:1"} // nothing listens here

	_, _, err := client.FetchWaterTemperature()
	if err == nil {
		t.Fatal("expected an error when the server is unreachable, got nil")
	}

	var ipmaErr *Error
	if !errors.As(err, &ipmaErr) {
		t.Errorf("expected error to be *ipma.Error, got %T: %v", err, err)
	}
}
