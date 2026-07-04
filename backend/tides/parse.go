package tides

import (
	"bytes"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// Selector strategy (see the package doc comment for the spike's
// robots.txt/structured-data findings):
//
// tabuademares.com's Funchal page has no JSON-LD or other structured data
// for tides. It has a plain HTML table, <table id="tabla_mareas">, with one
// <tr> per day of the current month. Each day row carries a
// title="<weekday>, <day> de <month> de <year>" attribute (in Portuguese) —
// e.g. title="sábado, 4 de julho de 2026" — which is what we match "today"
// against. That's deliberately preferred over the row's
// onclick="Day('YYYY-MM-D')" attribute, which encodes the same date but
// zero-pads the month and NOT the day (e.g. "2026-07-4"), an inconsistency
// that's easy to get subtly wrong and more fragile to depend on than a
// full, unambiguous date string.
//
// Within a matched row, each tide event is one
// <td class="tabla_mareas_marea ..."> cell (3 or 4 per day; on a 3-tide day
// the 4th <td> is still present but empty). Each populated cell contains:
//   - a <div class="tabla_mareas_marea_hora ..."> holding the "H:MM" or
//     "HH:MM" time as its text content, and
//   - a sibling <div class="tabla_mareas_marea_bajamar_pleamar ...">
//     additionally carrying the class "tabla_mareas_marea_pleamar" (high
//     tide) or "tabla_mareas_marea_bajamar" (low tide).
//
// Every lookup below is defensive: if the table, the day's row, or a cell's
// expected children aren't found, that cell (or the whole row) is skipped
// rather than producing a garbage value, and parseTideTimes falls back to a
// logged warning and a nil result when nothing usable was found.

// tabladeMareasID is the id of the tide table described above.
const tabladeMareasID = "tabla_mareas"

// titleDateRe extracts "<day> de <month> de <year>" from a row's title
// attribute, e.g. "sábado, 4 de julho de 2026".
var titleDateRe = regexp.MustCompile(`(\d{1,2}) de (\p{L}+) de (\d{4})`)

var portugueseMonths = map[string]time.Month{
	"janeiro":   time.January,
	"fevereiro": time.February,
	"março":     time.March,
	"abril":     time.April,
	"maio":      time.May,
	"junho":     time.June,
	"julho":     time.July,
	"agosto":    time.August,
	"setembro":  time.September,
	"outubro":   time.October,
	"novembro":  time.November,
	"dezembro":  time.December,
}

// tideTimeRe matches the "H:MM"/"HH:MM" text content of a
// tabla_mareas_marea_hora div.
var tideTimeRe = regexp.MustCompile(`^(\d{1,2}):(\d{2})$`)

// parseTideTimes parses a tabuademares.com Funchal page and extracts
// today's tide events (today being "today" in the given, already
// Madeira-local, time). It never panics and never returns a partial/garbage
// value: if the page's structure doesn't match what's documented above
// (site markup changed, today's row is missing, etc.), it logs a warning
// and returns nil.
func parseTideTimes(body []byte, today time.Time) *TideTimes {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		log.Printf("tides: parsing tide page HTML: %v", err)
		return nil
	}

	table := findNode(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "table" && attrEquals(n, "id", tabladeMareasID)
	})
	if table == nil {
		log.Printf("tides: could not find #%s table in tabuademares.com page; site markup may have changed", tabladeMareasID)
		return nil
	}

	rows := findAllNodes(table, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "tr"
	})

	for _, row := range rows {
		title, ok := attr(row, "title")
		if !ok {
			continue
		}
		rowDate, ok := parseTitleDate(title)
		if !ok {
			continue
		}
		if rowDate.Year() != today.Year() || rowDate.Month() != today.Month() || rowDate.Day() != today.Day() {
			continue
		}

		tides := parseTideRow(row)
		if tides == nil {
			log.Printf("tides: found today's row (%s) but no tide events could be parsed from it; site markup may have changed", today.Format("2006-01-02"))
			return nil
		}
		return tides
	}

	log.Printf("tides: no row found for today (%s) in tide table; site markup may have changed", today.Format("2006-01-02"))
	return nil
}

// parseTitleDate parses the "<day> de <month> de <year>" portion of a tide
// table row's title attribute, e.g. "sábado, 4 de julho de 2026". The
// returned time.Time only carries a meaningful Year/Month/Day (in UTC) — it
// isn't intended for anything but a calendar-date comparison.
func parseTitleDate(title string) (time.Time, bool) {
	m := titleDateRe.FindStringSubmatch(strings.ToLower(title))
	if m == nil {
		return time.Time{}, false
	}

	day, err := strconv.Atoi(m[1])
	if err != nil {
		return time.Time{}, false
	}
	month, ok := portugueseMonths[m[2]]
	if !ok {
		return time.Time{}, false
	}
	year, err := strconv.Atoi(m[3])
	if err != nil {
		return time.Time{}, false
	}

	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC), true
}

// parseTideRow extracts the tide events from a single day's <tr>. It
// returns nil if no cell yielded a usable (time, high/low) pair.
func parseTideRow(row *html.Node) *TideTimes {
	cells := findAllNodes(row, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "td" && hasClassToken(n, "tabla_mareas_marea")
	})

	var tides TideTimes
	for _, cell := range cells {
		horaDiv := findNode(cell, func(n *html.Node) bool {
			return n.Type == html.ElementNode && n.Data == "div" && hasClassToken(n, "tabla_mareas_marea_hora")
		})
		if horaDiv == nil {
			// Empty cell: a normal 3-tide day still renders a 4th <td>
			// with no children.
			continue
		}
		timeStr, ok := normalizeTideTime(textContent(horaDiv))
		if !ok {
			continue
		}

		markerDiv := findNode(cell, func(n *html.Node) bool {
			return n.Type == html.ElementNode && n.Data == "div" && hasClassToken(n, "tabla_mareas_marea_bajamar_pleamar")
		})
		if markerDiv == nil {
			continue
		}

		switch {
		case hasClassToken(markerDiv, "tabla_mareas_marea_pleamar"):
			tides.HighTides = append(tides.HighTides, timeStr)
		case hasClassToken(markerDiv, "tabla_mareas_marea_bajamar"):
			tides.LowTides = append(tides.LowTides, timeStr)
		default:
			// Marker div present but neither class found: ambiguous,
			// skip rather than guess.
		}
	}

	if len(tides.HighTides) == 0 && len(tides.LowTides) == 0 {
		return nil
	}
	return &tides
}

// normalizeTideTime validates and zero-pads a tide table's raw time text
// (e.g. "3:41" or "23:49") to "HH:MM".
func normalizeTideTime(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	m := tideTimeRe.FindStringSubmatch(s)
	if m == nil {
		return "", false
	}

	hour, err := strconv.Atoi(m[1])
	if err != nil || hour < 0 || hour > 23 {
		return "", false
	}
	minute, err := strconv.Atoi(m[2])
	if err != nil || minute < 0 || minute > 59 {
		return "", false
	}

	return fmt.Sprintf("%02d:%s", hour, m[2]), true
}

// --- minimal html.Node helpers -------------------------------------------
//
// golang.org/x/net/html gives us a parse tree but no query helpers (it's
// deliberately low-level); these are the handful of primitives the parser
// above needs, kept local rather than pulling in a heavier
// selector/query library.

// findNode returns the first node in n's subtree (including n itself, in
// document order) for which match returns true, or nil.
func findNode(n *html.Node, match func(*html.Node) bool) *html.Node {
	if match(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findNode(c, match); found != nil {
			return found
		}
	}
	return nil
}

// findAllNodes returns every node in n's subtree (including n itself, in
// document order) for which match returns true.
func findAllNodes(n *html.Node, match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if match(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

// attr returns the value of n's attribute named key.
func attr(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

// attrEquals reports whether n has an attribute named key equal to val.
func attrEquals(n *html.Node, key, val string) bool {
	v, ok := attr(n, key)
	return ok && v == val
}

// hasClassToken reports whether n's class attribute contains token as one
// of its whitespace-separated tokens (an exact match, not a substring
// match — e.g. "tabla_mareas_marea" must not match
// "tabla_mareas_marea_border_bottom").
func hasClassToken(n *html.Node, token string) bool {
	class, ok := attr(n, "class")
	if !ok {
		return false
	}
	for _, t := range strings.Fields(class) {
		if t == token {
			return true
		}
	}
	return false
}

// textContent returns the concatenated text of all descendant text nodes
// of n.
func textContent(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}
