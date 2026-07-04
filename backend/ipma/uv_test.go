package ipma

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestFetchUVIndex_ParsesFixture(t *testing.T) {
	fixture, err := os.ReadFile("testdata/uv.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/uv/uv.json" {
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

	got, err := client.FetchUVIndex()
	if err != nil {
		t.Fatalf("FetchUVIndex() returned error: %v", err)
	}

	// Fixture has iUv "9.3" for globalIdLocal 2310300 on 2026-07-04, which
	// rounds to 9 -> "muito alto"/"very high".
	want := UVIndex{Index: 9, ClassPt: "muito alto", ClassEn: "very high"}
	if got != want {
		t.Errorf("FetchUVIndex() = %+v, want %+v", got, want)
	}
}

func TestFetchUVIndex_FiltersOtherLocationsAndDates(t *testing.T) {
	// Same fixture as above: entries for a different location on the same
	// date, and for Funchal on other dates, must not be picked instead of
	// the correct globalIdLocal+date match.
	fixture, err := os.ReadFile("testdata/uv.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(fixture)
	}))
	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		Now:     fixedNow("2026-07-05"),
	}

	got, err := client.FetchUVIndex()
	if err != nil {
		t.Fatalf("FetchUVIndex() returned error: %v", err)
	}

	// iUv "8.9" for 2026-07-05 rounds to 9 -> "muito alto"/"very high".
	want := UVIndex{Index: 9, ClassPt: "muito alto", ClassEn: "very high"}
	if got != want {
		t.Errorf("FetchUVIndex() = %+v, want %+v", got, want)
	}
}

func TestClassifyUVIndex_Boundaries(t *testing.T) {
	tests := []struct {
		index  int
		wantPt string
		wantEn string
	}{
		{0, "baixo", "low"},
		{2, "baixo", "low"},
		{3, "moderado", "moderate"},
		{5, "moderado", "moderate"},
		{6, "alto", "high"},
		{7, "alto", "high"},
		{8, "muito alto", "very high"},
		{10, "muito alto", "very high"},
		{11, "extremo", "extreme"},
		{15, "extremo", "extreme"},
	}

	for _, tt := range tests {
		gotPt, gotEn := classifyUVIndex(tt.index)
		if gotPt != tt.wantPt || gotEn != tt.wantEn {
			t.Errorf("classifyUVIndex(%d) = (%q, %q), want (%q, %q)", tt.index, gotPt, gotEn, tt.wantPt, tt.wantEn)
		}
	}
}

func TestFetchUVIndex_RoundsToNearestInteger(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"globalIdLocal":2310300,"data":"2026-07-04","iUv":"7.5"}]`))
	}))
	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		Now:     fixedNow("2026-07-04"),
	}

	got, err := client.FetchUVIndex()
	if err != nil {
		t.Fatalf("FetchUVIndex() returned error: %v", err)
	}

	// 7.5 rounds to 8 -> "muito alto"/"very high" (crosses the 7/8 boundary).
	want := UVIndex{Index: 8, ClassPt: "muito alto", ClassEn: "very high"}
	if got != want {
		t.Errorf("FetchUVIndex() = %+v, want %+v", got, want)
	}
}

func TestFetchUVIndex_NoEntryForToday(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"globalIdLocal":2310300,"data":"2026-07-05","iUv":"5.0"}]`))
	}))
	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		Now:     fixedNow("2026-07-04"),
	}

	_, err := client.FetchUVIndex()
	if err == nil {
		t.Fatal("expected an error when no uv entry matches today, got nil")
	}

	var ipmaErr *Error
	if !errors.As(err, &ipmaErr) {
		t.Errorf("expected error to be *ipma.Error, got %T: %v", err, err)
	}
}

func TestFetchUVIndex_NoEntryForFunchal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"globalIdLocal":1010500,"data":"2026-07-04","iUv":"5.0"}]`))
	}))
	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		Now:     fixedNow("2026-07-04"),
	}

	_, err := client.FetchUVIndex()
	if err == nil {
		t.Fatal("expected an error when no uv entry matches Funchal, got nil")
	}

	var ipmaErr *Error
	if !errors.As(err, &ipmaErr) {
		t.Errorf("expected error to be *ipma.Error, got %T: %v", err, err)
	}
}

func TestFetchUVIndex_MalformedPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{not valid json`))
	}))
	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		Now:     fixedNow("2026-07-04"),
	}

	_, err := client.FetchUVIndex()
	if err == nil {
		t.Fatal("expected an error for malformed payload, got nil")
	}

	var ipmaErr *Error
	if !errors.As(err, &ipmaErr) {
		t.Errorf("expected error to be *ipma.Error, got %T: %v", err, err)
	}
}

func TestFetchUVIndex_InvalidUVValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"globalIdLocal":2310300,"data":"2026-07-04","iUv":"n/a"}]`))
	}))
	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		Now:     fixedNow("2026-07-04"),
	}

	_, err := client.FetchUVIndex()
	if err == nil {
		t.Fatal("expected an error for invalid uv value, got nil")
	}

	var ipmaErr *Error
	if !errors.As(err, &ipmaErr) {
		t.Errorf("expected error to be *ipma.Error, got %T: %v", err, err)
	}
}

func TestFetchUVIndex_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		Now:     fixedNow("2026-07-04"),
	}

	_, err := client.FetchUVIndex()
	if err == nil {
		t.Fatal("expected an error for a non-200 response, got nil")
	}

	var ipmaErr *Error
	if !errors.As(err, &ipmaErr) {
		t.Errorf("expected error to be *ipma.Error, got %T: %v", err, err)
	}
}

func TestFetchUVIndex_Unreachable(t *testing.T) {
	client := &Client{
		BaseURL: "http://127.0.0.1:1", // nothing listens here
		Now:     fixedNow("2026-07-04"),
	}

	_, err := client.FetchUVIndex()
	if err == nil {
		t.Fatal("expected an error when the server is unreachable, got nil")
	}

	var ipmaErr *Error
	if !errors.As(err, &ipmaErr) {
		t.Errorf("expected error to be *ipma.Error, got %T: %v", err, err)
	}
}
