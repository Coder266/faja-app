package conditions

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Coder266/faja-app/backend/ipma"
	"github.com/Coder266/faja-app/backend/tides"
)

// fixedNow returns a Now func pinned to the given date at noon Madeira
// time, mirroring the ipma/tides packages' test helper of the same name.
func fixedNow(dateStr string) func() time.Time {
	return func() time.Time {
		t, err := time.ParseInLocation("2006-01-02", dateStr, madeiraLocation)
		if err != nil {
			panic(err)
		}
		return t.Add(12 * time.Hour)
	}
}

// mockAirTemp is an airTemperatureFetcher stub for tests.
type mockAirTemp struct {
	calls int32
	temp  ipma.AirTemperature
	err   error
}

func (m *mockAirTemp) FetchAirTemperature() (ipma.AirTemperature, error) {
	atomic.AddInt32(&m.calls, 1)
	return m.temp, m.err
}

// mockUV is a uvIndexFetcher stub for tests.
type mockUV struct {
	calls int32
	uv    ipma.UVIndex
	err   error
}

func (m *mockUV) FetchUVIndex() (ipma.UVIndex, error) {
	atomic.AddInt32(&m.calls, 1)
	return m.uv, m.err
}

// mockWater is a waterTemperatureFetcher stub for tests.
type mockWater struct {
	calls int32
	temp  int
	found bool
	err   error
}

func (m *mockWater) FetchWaterTemperature() (int, bool, error) {
	atomic.AddInt32(&m.calls, 1)
	return m.temp, m.found, m.err
}

// mockTides is a tideTimesFetcher stub for tests.
type mockTides struct {
	calls int32
	tides *tides.TideTimes
	err   error
}

func (m *mockTides) FetchTideTimes() (*tides.TideTimes, error) {
	atomic.AddInt32(&m.calls, 1)
	return m.tides, m.err
}

// allOKAggregator returns an Aggregator whose four mock sources all succeed,
// along with references to the mocks so a test can tweak one to fail.
func allOKAggregator(now func() time.Time) (*Aggregator, *mockAirTemp, *mockUV, *mockWater, *mockTides) {
	airTemp := &mockAirTemp{temp: ipma.AirTemperature{Min: 25, Max: 26}}
	uv := &mockUV{uv: ipma.UVIndex{Index: 8, ClassPt: "muito alto", ClassEn: "very high"}}
	water := &mockWater{temp: 19, found: true}
	tideTimes := &mockTides{tides: &tides.TideTimes{
		HighTides: []string{"17:02"},
		LowTides:  []string{"10:39", "23:09"},
	}}

	agg := &Aggregator{
		AirTemp: airTemp,
		UV:      uv,
		Water:   water,
		Tides:   tideTimes,
		Now:     now,
	}
	return agg, airTemp, uv, water, tideTimes
}

func TestHandler_AllSourcesSucceed(t *testing.T) {
	agg, _, _, _, _ := allOKAggregator(fixedNow("2026-07-04"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/conditions", nil)
	agg.Handler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got Conditions
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if got.Date != "2026-07-04" {
		t.Errorf("Date = %q, want %q", got.Date, "2026-07-04")
	}
	if got.AirTempMin == nil || *got.AirTempMin != 25 {
		t.Errorf("AirTempMin = %v, want 25", got.AirTempMin)
	}
	if got.AirTempMax == nil || *got.AirTempMax != 26 {
		t.Errorf("AirTempMax = %v, want 26", got.AirTempMax)
	}
	if got.WaterTemp == nil || *got.WaterTemp != 19 {
		t.Errorf("WaterTemp = %v, want 19", got.WaterTemp)
	}
	if got.UV == nil || *got.UV != (ipma.UVIndex{Index: 8, ClassPt: "muito alto", ClassEn: "very high"}) {
		t.Errorf("UV = %v, want {8 muito alto very high}", got.UV)
	}
	if got.Tides == nil || len(got.Tides.HighTides) != 1 || got.Tides.HighTides[0] != "17:02" {
		t.Errorf("Tides = %v, want highTides [17:02]", got.Tides)
	}
	if got.UpdatedAt == "" {
		t.Error("UpdatedAt is empty, want a timestamp")
	}
	if _, err := time.Parse(time.RFC3339, got.UpdatedAt); err != nil {
		t.Errorf("UpdatedAt = %q is not RFC3339: %v", got.UpdatedAt, err)
	}
}

func TestHandler_TidesFailurePartialData(t *testing.T) {
	agg, _, _, _, tideTimes := allOKAggregator(fixedNow("2026-07-04"))
	tideTimes.tides = nil
	tideTimes.err = errors.New("scrape failed")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/conditions", nil)
	agg.Handler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (partial failure must still be 200)", rec.Code, http.StatusOK)
	}

	var got Conditions
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if got.Tides != nil {
		t.Errorf("Tides = %v, want nil after a failed fetch", got.Tides)
	}
	// The rest of the response must be unaffected.
	if got.AirTempMin == nil || *got.AirTempMin != 25 {
		t.Errorf("AirTempMin = %v, want 25", got.AirTempMin)
	}
	if got.WaterTemp == nil || *got.WaterTemp != 19 {
		t.Errorf("WaterTemp = %v, want 19", got.WaterTemp)
	}
	if got.UV == nil {
		t.Error("UV = nil, want populated")
	}

	// Confirm the "tides: null" literal appears (not omitted), i.e. the
	// field is a nullable pointer that serializes as JSON null.
	if !containsNullTides(rec.Body.Bytes()) {
		t.Errorf("response body does not contain a null \"tides\" field: %s", rec.Body.String())
	}
}

func containsNullTides(body []byte) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return false
	}
	v, ok := raw["tides"]
	return ok && string(v) == "null"
}

func TestHandler_AirTemperatureFailureNullsBothFields(t *testing.T) {
	agg, airTemp, _, _, _ := allOKAggregator(fixedNow("2026-07-04"))
	airTemp.err = errors.New("upstream error")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/conditions", nil)
	agg.Handler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got Conditions
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if got.AirTempMin != nil || got.AirTempMax != nil {
		t.Errorf("AirTempMin/Max = %v/%v, want both nil", got.AirTempMin, got.AirTempMax)
	}
	if got.Tides == nil {
		t.Error("Tides = nil, want populated (unaffected by air temp failure)")
	}
}

func TestHandler_WaterTemperatureGracefullyAbsent(t *testing.T) {
	agg, _, _, water, _ := allOKAggregator(fixedNow("2026-07-04"))
	water.found = false
	water.err = nil

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/conditions", nil)
	agg.Handler()(rec, req)

	var got Conditions
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if got.WaterTemp != nil {
		t.Errorf("WaterTemp = %v, want nil when the source found no data", got.WaterTemp)
	}
}

func TestGet_CachesWithinTTL(t *testing.T) {
	now := fixedNow("2026-07-04")
	agg, airTemp, uv, water, tideTimes := allOKAggregator(now)

	first := agg.Get()
	second := agg.Get()

	if first != second {
		t.Errorf("Get() returned different pointers within the TTL, want the cached response reused")
	}
	for name, calls := range map[string]int32{
		"air temp": airTemp.calls, "uv": uv.calls, "water": water.calls, "tides": tideTimes.calls,
	} {
		if calls != 1 {
			t.Errorf("%s fetcher called %d times, want 1 (second Get() should hit the cache)", name, calls)
		}
	}
}

func TestGet_RefreshesAfterTTLExpiry(t *testing.T) {
	current := time.Date(2026, time.July, 4, 12, 0, 0, 0, madeiraLocation)
	agg, airTemp, _, _, _ := allOKAggregator(func() time.Time { return current })

	agg.Get()
	current = current.Add(cacheTTL + time.Minute)
	agg.Get()

	if airTemp.calls != 2 {
		t.Errorf("air temp fetcher called %d times, want 2 (cache should expire after TTL)", airTemp.calls)
	}
}

func TestGet_RefreshesOnMadeiraDateRollover(t *testing.T) {
	current := time.Date(2026, time.July, 4, 23, 45, 0, 0, madeiraLocation)
	agg, airTemp, _, _, _ := allOKAggregator(func() time.Time { return current })

	agg.Get()
	// Advance past midnight but well within the TTL window.
	current = current.Add(30 * time.Minute)
	got := agg.Get()

	if airTemp.calls != 2 {
		t.Errorf("air temp fetcher called %d times, want 2 (cache should refresh on date rollover)", airTemp.calls)
	}
	if got.Date != "2026-07-05" {
		t.Errorf("Date = %q, want %q", got.Date, "2026-07-05")
	}
}
