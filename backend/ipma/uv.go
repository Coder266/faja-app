package ipma

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"
)

const uvIndexURL = "https://api.ipma.pt/open-data/forecast/meteorology/uv/uv.json"

// uvEntry mirrors one element of IPMA's uv/uv.json payload, a flat array
// (not wrapped in a "data" field like the daily forecast endpoint).
type uvEntry struct {
	GlobalIdLocal int    `json:"globalIdLocal"`
	Data          string `json:"data"`
	IUv           string `json:"iUv"`
}

// UVIndex is today's UV index for Funchal, rounded to the nearest integer,
// with its WHO-scale severity classification in Portuguese and English.
// JSON tags match the aggregated /api/conditions response (see
// docs/stories.md, S5).
type UVIndex struct {
	Index   int    `json:"index"`
	ClassPt string `json:"classPt"`
	ClassEn string `json:"classEn"`
}

func (c *Client) uvIndexURL() string {
	if c.BaseURL != "" {
		return c.BaseURL + "/uv/uv.json"
	}
	return uvIndexURL
}

// FetchUVIndex fetches today's UV index for Funchal. "Today" is determined
// in the Atlantic/Madeira timezone. It never panics; any failure or
// malformed payload is returned as a *Error.
func (c *Client) FetchUVIndex() (UVIndex, error) {
	req, err := http.NewRequest(http.MethodGet, c.uvIndexURL(), nil)
	if err != nil {
		return UVIndex{}, &Error{Op: "build uv index request", Err: err}
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return UVIndex{}, &Error{Op: "fetch uv index", Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return UVIndex{}, &Error{Op: "fetch uv index", Err: fmt.Errorf("unexpected status %d", resp.StatusCode)}
	}

	var payload []uvEntry
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return UVIndex{}, &Error{Op: "decode uv index", Err: err}
	}

	return uvIndexForToday(payload, c.now())
}

// uvIndexForToday extracts the iUv entry matching Funchal's globalIdLocal
// and today's date (in Madeira local time) from a decoded uv.json payload.
func uvIndexForToday(payload []uvEntry, today time.Time) (UVIndex, error) {
	todayStr := today.Format("2006-01-02")

	for _, entry := range payload {
		if entry.GlobalIdLocal != globalIdLocalFunchal || entry.Data != todayStr {
			continue
		}

		f, err := strconv.ParseFloat(entry.IUv, 64)
		if err != nil {
			return UVIndex{}, &Error{Op: "parse iUv", Err: fmt.Errorf("invalid uv value %q: %w", entry.IUv, err)}
		}

		index := int(math.Round(f))
		classPt, classEn := classifyUVIndex(index)
		return UVIndex{Index: index, ClassPt: classPt, ClassEn: classEn}, nil
	}

	return UVIndex{}, &Error{Op: "find today's uv index", Err: fmt.Errorf("no uv entry for globalIdLocal %d on %s", globalIdLocalFunchal, todayStr)}
}

// classifyUVIndex returns the WHO-scale severity classification for a
// rounded UV index, in Portuguese and English.
func classifyUVIndex(index int) (pt, en string) {
	switch {
	case index <= 2:
		return "baixo", "low"
	case index <= 5:
		return "moderado", "moderate"
	case index <= 7:
		return "alto", "high"
	case index <= 10:
		return "muito alto", "very high"
	default:
		return "extremo", "extreme"
	}
}
