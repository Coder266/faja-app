package tides

import (
	"os"
	"testing"
	"time"
)

func readFixture(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/funchal.html")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return body
}

func TestParseTideTimes_FourEventDay(t *testing.T) {
	body := readFixture(t)

	// Fixture's title="sábado, 4 de julho de 2026" row has 4 tide events:
	// high 5:25/17:38, low 11:17/23:49 (see testdata/funchal.html).
	today := time.Date(2026, time.July, 4, 12, 0, 0, 0, time.UTC)

	tides := parseTideTimes(body, today)
	if tides == nil {
		t.Fatal("parseTideTimes() = nil, want a result")
	}

	wantHigh := []string{"05:25", "17:38"}
	wantLow := []string{"11:17", "23:49"}
	if !equalStrings(tides.HighTides, wantHigh) {
		t.Errorf("HighTides = %v, want %v", tides.HighTides, wantHigh)
	}
	if !equalStrings(tides.LowTides, wantLow) {
		t.Errorf("LowTides = %v, want %v", tides.LowTides, wantLow)
	}
}

func TestParseTideTimes_ThreeEventDay(t *testing.T) {
	body := readFixture(t)

	// Fixture's title="domingo, 5 de julho de 2026" row has only 3 tide
	// events (the 4th <td> is present but empty): high 6:02/18:15, low
	// 11:59.
	today := time.Date(2026, time.July, 5, 12, 0, 0, 0, time.UTC)

	tides := parseTideTimes(body, today)
	if tides == nil {
		t.Fatal("parseTideTimes() = nil, want a result")
	}

	wantHigh := []string{"06:02", "18:15"}
	wantLow := []string{"11:59"}
	if !equalStrings(tides.HighTides, wantHigh) {
		t.Errorf("HighTides = %v, want %v", tides.HighTides, wantHigh)
	}
	if !equalStrings(tides.LowTides, wantLow) {
		t.Errorf("LowTides = %v, want %v", tides.LowTides, wantLow)
	}
}

func TestParseTideTimes_TimesAreZeroPadded(t *testing.T) {
	body := readFixture(t)

	// Fixture's title="quarta-feira, 1 de julho de 2026" row has a single-
	// digit-hour high tide "3:41", which must come out as "03:41".
	today := time.Date(2026, time.July, 1, 12, 0, 0, 0, time.UTC)

	tides := parseTideTimes(body, today)
	if tides == nil {
		t.Fatal("parseTideTimes() = nil, want a result")
	}
	if len(tides.HighTides) == 0 || tides.HighTides[0] != "03:41" {
		t.Errorf("HighTides[0] = %v, want 03:41", tides.HighTides)
	}
}

func TestParseTideTimes_NoRowForDate_ReturnsNil(t *testing.T) {
	body := readFixture(t)

	// The fixture only has rows for July 1-6, 2026.
	today := time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC)

	tides := parseTideTimes(body, today)
	if tides != nil {
		t.Errorf("parseTideTimes() = %+v, want nil for a date not in the table", tides)
	}
}

func TestParseTideTimes_MissingTable_ReturnsNil(t *testing.T) {
	html := []byte(`<!doctype html><html><body><p>tabuademares.com redesigned the page</p></body></html>`)
	today := time.Date(2026, time.July, 4, 12, 0, 0, 0, time.UTC)

	tides := parseTideTimes(html, today)
	if tides != nil {
		t.Errorf("parseTideTimes() = %+v, want nil when #tabla_mareas is missing", tides)
	}
}

func TestParseTideTimes_RowWithNoParsableCells_ReturnsNil(t *testing.T) {
	// Today's row exists (matched via title) but the tide-cell markup
	// inside it has been restructured beyond what the parser understands:
	// no tabla_mareas_marea cells at all.
	html := []byte(`<!doctype html><html><body>
		<table id="tabla_mareas">
			<tr title="sábado, 4 de julho de 2026">
				<td class="tabla_mareas_dia">4</td>
			</tr>
		</table>
	</body></html>`)
	today := time.Date(2026, time.July, 4, 12, 0, 0, 0, time.UTC)

	tides := parseTideTimes(html, today)
	if tides != nil {
		t.Errorf("parseTideTimes() = %+v, want nil when today's row has no parsable tide cells", tides)
	}
}

func TestParseTideTimes_MalformedHTML_DoesNotPanic(t *testing.T) {
	html := []byte(`<table id="tabla_mareas"><tr title="not a date at all"><td>`)
	today := time.Date(2026, time.July, 4, 12, 0, 0, 0, time.UTC)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("parseTideTimes() panicked: %v", r)
		}
	}()

	tides := parseTideTimes(html, today)
	if tides != nil {
		t.Errorf("parseTideTimes() = %+v, want nil for malformed/unmatchable HTML", tides)
	}
}

func TestParseTideTimes_UnknownMonthName_SkipsRow(t *testing.T) {
	// A row whose title has a month name the parser doesn't recognize
	// must be skipped defensively, not crash or be misinterpreted.
	html := []byte(`<!doctype html><html><body>
		<table id="tabla_mareas">
			<tr title="sábado, 4 de fictembro de 2026">
				<td class="tabla_mareas_marea">
					<div class="tabla_mareas_marea_hora">5:00</div>
					<div class="tabla_mareas_marea_bajamar_pleamar tabla_mareas_marea_pleamar"></div>
				</td>
			</tr>
		</table>
	</body></html>`)
	today := time.Date(2026, time.July, 4, 12, 0, 0, 0, time.UTC)

	tides := parseTideTimes(html, today)
	if tides != nil {
		t.Errorf("parseTideTimes() = %+v, want nil for an unrecognized month name", tides)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
