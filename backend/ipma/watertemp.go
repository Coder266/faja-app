package ipma

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
)

// globalIdLocalMadeiraSouthCoast is IPMA's location code for the Madeira
// south coast oceanography point used for sea surface temperature. This is
// a different globalIdLocal than globalIdLocalFunchal (used for air
// temperature and UV, which are reported at Funchal city itself) — don't
// conflate the two.
const globalIdLocalMadeiraSouthCoast = 2310326

const oceanographyDailyURL = "https://api.ipma.pt/open-data/forecast/oceanography/daily/hp-daily-sea-forecast-day0.json"

// oceanographyResponse mirrors the shape of IPMA's
// forecast/oceanography/daily/hp-daily-sea-forecast-day0.json payload.
type oceanographyResponse struct {
	Data []oceanographyEntry `json:"data"`
}

type oceanographyEntry struct {
	GlobalIdLocal int `json:"globalIdLocal"`
	// SstMin/SstMax are pointers so we can tell "field omitted or JSON
	// null" (nil) apart from "present but unparseable" (non-nil, invalid
	// numeric string) — the former is a graceful no-data case, the latter
	// is a genuine error. See FetchWaterTemperature.
	SstMin *string `json:"sstMin"`
	SstMax *string `json:"sstMax"`
}

func (c *Client) oceanographyDailyURL() string {
	if c.BaseURL != "" {
		return c.BaseURL + "/oceanography/daily/hp-daily-sea-forecast-day0.json"
	}
	return oceanographyDailyURL
}

// FetchWaterTemperature fetches today's sea surface temperature for the
// Madeira south coast (globalIdLocal 2310326), returning the midpoint of
// sstMin/sstMax rounded to the nearest whole degree Celsius.
//
// The day0 endpoint is a single day's snapshot with no per-entry date field
// to match against (unlike the daily air-temperature forecast, which
// covers several days), so the target entry is selected purely by
// globalIdLocal.
//
// The return signature distinguishes two different kinds of "no result":
//
//   - found == false, err == nil: the Madeira south coast point is either
//     absent from today's payload, or present but with sstMin/sstMax
//     omitted or JSON null. This is the graceful path — callers (e.g. the
//     /api/conditions aggregator in S5) should surface this as a null
//     waterTemp field, not fail the whole response.
//   - err != nil: a genuine failure — request/network error, non-200
//     status, malformed JSON, or an sstMin/sstMax value that is present
//     but not a valid number. Always a *Error.
func (c *Client) FetchWaterTemperature() (temp int, found bool, err error) {
	req, err := http.NewRequest(http.MethodGet, c.oceanographyDailyURL(), nil)
	if err != nil {
		return 0, false, &Error{Op: "build oceanography daily request", Err: err}
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return 0, false, &Error{Op: "fetch oceanography daily forecast", Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, false, &Error{Op: "fetch oceanography daily forecast", Err: fmt.Errorf("unexpected status %d", resp.StatusCode)}
	}

	var payload oceanographyResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, false, &Error{Op: "decode oceanography daily forecast", Err: err}
	}

	return waterTemperatureForMadeiraSouthCoast(payload)
}

// waterTemperatureForMadeiraSouthCoast extracts and rounds the sea surface
// temperature midpoint for globalIdLocalMadeiraSouthCoast from a decoded
// oceanography payload. See FetchWaterTemperature for the found/err
// contract.
func waterTemperatureForMadeiraSouthCoast(payload oceanographyResponse) (int, bool, error) {
	for _, entry := range payload.Data {
		if entry.GlobalIdLocal != globalIdLocalMadeiraSouthCoast {
			continue
		}

		if entry.SstMin == nil || entry.SstMax == nil {
			// Point is present today but the API omitted or nulled the
			// temperature fields: graceful absence, not an error.
			return 0, false, nil
		}

		sstMin, err := strconv.ParseFloat(*entry.SstMin, 64)
		if err != nil {
			return 0, false, &Error{Op: "parse sstMin", Err: fmt.Errorf("invalid sstMin value %q: %w", *entry.SstMin, err)}
		}
		sstMax, err := strconv.ParseFloat(*entry.SstMax, 64)
		if err != nil {
			return 0, false, &Error{Op: "parse sstMax", Err: fmt.Errorf("invalid sstMax value %q: %w", *entry.SstMax, err)}
		}

		midpoint := (sstMin + sstMax) / 2
		return int(math.Round(midpoint)), true, nil
	}

	// No entry for this globalIdLocal today: graceful absence, not an
	// error.
	return 0, false, nil
}
