// Package ipma provides clients for the IPMA (Instituto Português do Mar e
// da Atmosfera) open-data API, used to fetch weather conditions for Funchal,
// Madeira.
package ipma

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"
)

// madeiraLocation is the reference timezone for "today" throughout this
// package, since the poster this app replicates is displayed in Madeira.
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

// globalIdLocalFunchal is IPMA's location code for Funchal.
const globalIdLocalFunchal = 2310300

const dailyForecastURL = "https://api.ipma.pt/open-data/forecast/meteorology/cities/daily/%d.json"

const defaultTimeout = 10 * time.Second

// Error is a typed error returned by this package's client methods, so
// callers can distinguish IPMA client failures from other errors without
// string matching.
type Error struct {
	Op  string // operation being attempted, e.g. "fetch daily forecast"
	Err error
}

func (e *Error) Error() string {
	return fmt.Sprintf("ipma: %s: %v", e.Op, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}

// dailyForecastResponse mirrors the shape of IPMA's
// forecast/meteorology/cities/daily/{globalIdLocal}.json payload.
type dailyForecastResponse struct {
	Data []dailyForecastEntry `json:"data"`
}

type dailyForecastEntry struct {
	ForecastDate string `json:"forecastDate"`
	TMin         string `json:"tMin"`
	TMax         string `json:"tMax"`
}

// AirTemperature is today's expected air temperature range for Funchal,
// rounded to whole degrees Celsius. JSON tags match the field names in the
// aggregated /api/conditions response (see docs/stories.md, S5).
type AirTemperature struct {
	Min int `json:"airTempMin"`
	Max int `json:"airTempMax"`
}

// Client fetches weather conditions from the IPMA open-data API.
type Client struct {
	// HTTPClient is used to perform requests. If nil, a client with
	// defaultTimeout is used.
	HTTPClient *http.Client

	// BaseURL overrides the IPMA API base for tests. If empty,
	// "https://api.ipma.pt/open-data/forecast/meteorology" is used.
	BaseURL string

	// Now returns the current time and is used to determine "today".
	// If nil, time.Now is used. Tests can override this to pin "today"
	// to match a fixture's dates.
	Now func() time.Time
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

func (c *Client) dailyForecastURL() string {
	if c.BaseURL != "" {
		return fmt.Sprintf(c.BaseURL+"/cities/daily/%d.json", globalIdLocalFunchal)
	}
	return fmt.Sprintf(dailyForecastURL, globalIdLocalFunchal)
}

// FetchAirTemperature fetches today's air temperature range for Funchal.
// "Today" is determined in the Atlantic/Madeira timezone. It never panics;
// any failure or malformed payload is returned as a *Error.
func (c *Client) FetchAirTemperature() (AirTemperature, error) {
	req, err := http.NewRequest(http.MethodGet, c.dailyForecastURL(), nil)
	if err != nil {
		return AirTemperature{}, &Error{Op: "build daily forecast request", Err: err}
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return AirTemperature{}, &Error{Op: "fetch daily forecast", Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return AirTemperature{}, &Error{Op: "fetch daily forecast", Err: fmt.Errorf("unexpected status %d", resp.StatusCode)}
	}

	var payload dailyForecastResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return AirTemperature{}, &Error{Op: "decode daily forecast", Err: err}
	}

	return airTemperatureForToday(payload, c.now())
}

// airTemperatureForToday extracts the tMin/tMax entry matching today's date
// (in Madeira local time) from a decoded daily forecast payload.
func airTemperatureForToday(payload dailyForecastResponse, today time.Time) (AirTemperature, error) {
	todayStr := today.Format("2006-01-02")

	for _, entry := range payload.Data {
		if entry.ForecastDate != todayStr {
			continue
		}

		tMin, err := parseRoundedTemp(entry.TMin)
		if err != nil {
			return AirTemperature{}, &Error{Op: "parse tMin", Err: err}
		}
		tMax, err := parseRoundedTemp(entry.TMax)
		if err != nil {
			return AirTemperature{}, &Error{Op: "parse tMax", Err: err}
		}

		return AirTemperature{Min: tMin, Max: tMax}, nil
	}

	return AirTemperature{}, &Error{Op: "find today's forecast", Err: fmt.Errorf("no forecast entry for date %s", todayStr)}
}

// parseRoundedTemp parses an IPMA temperature string (e.g. "25.0") and
// rounds it to the nearest whole degree.
func parseRoundedTemp(s string) (int, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid temperature value %q: %w", s, err)
	}
	return int(math.Round(f)), nil
}
