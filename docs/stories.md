# Fajã dos Padres — daily conditions board

Web app replicating the laminated daily-conditions poster at Fajã dos Padres (`example.jpg`):
date, air temperature, water temperature, UV index, and high/low tide times, with bilingual
PT/EN labels over blue pebble-shaped panels and handwritten-style values.

## Data sources (verified 2026-07-04)

| Field | Source | Endpoint / page |
|---|---|---|
| Air temperature (min–max) | IPMA | `https://api.ipma.pt/open-data/forecast/meteorology/cities/daily/2310300.json` (Funchal, `globalIdLocal` 2310300) |
| UV index | IPMA | `https://api.ipma.pt/open-data/forecast/meteorology/uv/uv.json` (filter `globalIdLocal == 2310300`, field `iUv`) |
| Water temperature | IPMA | `https://api.ipma.pt/open-data/forecast/oceanography/daily/hp-daily-sea-forecast-day0.json` (Madeira south coast point, `globalIdLocal` 2310326, fields `sstMin`/`sstMax`) |
| Tides | tabuademares.com (IPMA has no tide data) | `https://tabuademares.com/pt/madeira/funchal` (scraped) |

---

## Epic 1 — Backend data services (Go, `backend/`)

### S1. IPMA client: daily air temperature for Funchal

As a visitor, I want to see today's expected air temperature so I know what to wear.

**Acceptance criteria**
- [ ] Backend fetches `cities/daily/2310300.json` and extracts today's `tMin` and `tMax`.
- [ ] Values are exposed as a range (e.g. `25–26 °C` → `{ "airTempMin": 25, "airTempMax": 26 }`), rounded to whole degrees.
- [ ] HTTP client has a timeout (≤ 10 s) and returns a typed error on failure/malformed payload; it never panics.
- [ ] Unit test parses a fixture of the real JSON payload.

### S2. IPMA client: UV index with bilingual classification

As a visitor, I want the UV index and how severe it is, in Portuguese and English.

**Acceptance criteria**
- [ ] Backend fetches `uv/uv.json`, filters `globalIdLocal == 2310300` and `data == today` (Atlantic/Madeira timezone), and reads `iUv` (note: it's a string, e.g. `"9.3"`).
- [ ] `iUv` is rounded to the nearest integer for display (poster shows `8`).
- [ ] Classification follows the WHO scale, returned in both languages:
      0–2 `baixo`/`low`, 3–5 `moderado`/`moderate`, 6–7 `alto`/`high`, 8–10 `muito alto`/`very high`, 11+ `extremo`/`extreme`.
- [ ] Unit tests cover boundary values (2/3, 5/6, 7/8, 10/11) and the today-filter.

### S3. IPMA client: water temperature (sea surface temperature)

As a visitor, I want the water temperature before deciding to swim.

**Acceptance criteria**
- [ ] Backend fetches `hp-daily-sea-forecast-day0.json` and selects the Madeira south coast point `globalIdLocal == 2310326`.
- [ ] Returns a single displayed value (round the midpoint of `sstMin`/`sstMax`; poster shows one number, e.g. `19 °C`).
- [ ] Falls back gracefully (field omitted / null in API response) if the point is missing that day.
- [ ] Unit test parses a fixture of the real payload.

### S4. Tide times scraped from tabuademares.com

As a visitor, I want today's high- and low-tide times so I can plan beach access.

**Acceptance criteria**
- [ ] Backend fetches `https://tabuademares.com/pt/madeira/funchal` and parses **all** of today's tide events: time + type (high/low). A normal day has 3–4 events (poster showed 1 high, 2 low).
- [ ] Result shape: `{ "highTides": ["05:1x", "17:02"], "lowTides": ["10:39", "23:09"] }` — times in `HH:MM`, Madeira local time.
- [ ] Scrape happens at most a few times per day (tide tables are precomputed): cache the parsed result with a TTL of ≥ 6 h and re-scrape on date rollover. Requests send a descriptive `User-Agent`.
- [ ] Parser is defensive: if the page structure changes, the endpoint returns tides as `null` with a logged warning — never a crash or a garbage value.
- [ ] Unit test parses a saved HTML fixture of the page.
- [ ] (Spike, part of this story) Check the page's markup for structured data (JSON-LD/tables) and robots.txt before writing the parser; document the chosen selector strategy in a code comment.

### S5. Aggregated `GET /api/conditions` endpoint

As the frontend, I want one endpoint with everything the poster shows.

**Acceptance criteria**
- [ ] `GET /api/conditions` returns:
      ```json
      {
        "date": "2026-07-04",
        "airTempMin": 25, "airTempMax": 26,
        "waterTemp": 19,
        "uv": { "index": 8, "classPt": "muito alto", "classEn": "very high" },
        "tides": { "highTides": ["17:02"], "lowTides": ["10:39", "23:09"] },
        "updatedAt": "2026-07-04T09:00:00Z"
      }
      ```
- [ ] Upstream sources are fetched concurrently; one failing source nulls only its own field(s) — the endpoint still returns 200 with partial data.
- [ ] Whole response is cached in memory (TTL ~30 min; tides per S4 keep their longer TTL) so page loads don't hammer IPMA/tabuademares.
- [ ] `date` is today in `Atlantic/Madeira`.
- [ ] CORS allows the Vite dev origin (or the Vite dev server proxies `/api` — see S9).
- [ ] Handler test with mocked clients covers the partial-failure case.

---

## Epic 2 — Frontend (Vue 3 + TS, `frontend/`)

### S6. Poster layout and visual design

As a visitor, I want the page to look like the physical poster.

**Acceptance criteria**
- [ ] Light blue page with darker blue organic pebble/blob shapes (SVG or CSS `border-radius` blobs) scattered like the poster; the six info blocks sit on the large blobs in roughly the poster's positions (Data top-left; Temperatura right; Temperatura da água mid-left; UV mid-right; Maré cheia bottom-left; Maré vazia bottom-right).
- [ ] Each block shows the PT label in bold serif with the EN label smaller beneath it, exactly as on the poster: `Data`/`Date`, `Temperatura`/`Temperature`, `Temperatura da água`/`Water temperature`, `UV`, `Maré cheia`/`High tide` (fixing the poster's "Maré cheira" typo), `Maré vazia`/`Low tide`.
- [ ] Values render in a handwriting-style font (bundled locally, e.g. Caveat — no CDN dependency).
- [ ] "Fajã dos Padres" wordmark bottom-left.
- [ ] Layout is responsive: poster-proportioned column on desktop, blocks reflow to a single column on narrow screens.

### S7. Live data display

As a visitor, I want the poster filled in with today's real values.

**Acceptance criteria**
- [ ] On load the app calls `GET /api/conditions` and renders: date as `DD/MM`; air temp as `25–26 °C` (single value if min == max); water temp as `19 °C`; UV as the integer plus both classification lines (`muito alto` / `very high`); each tide time on its own line.
- [ ] A typed client (`Conditions` interface) matches the S5 contract; no `any`.
- [ ] Data is re-fetched when the tab becomes visible again and at least hourly while open, so a kiosk/pinned tab rolls over to the new day.

### S8. Loading, error, and partial-data states

As a visitor, I want the page to stay legible when data is missing.

**Acceptance criteria**
- [ ] While loading, value areas show a subtle placeholder (not layout jumps).
- [ ] A null field renders as `—` in its block; the rest of the poster is unaffected.
- [ ] If the whole request fails, the poster still renders with all values as `—` and an unobtrusive bilingual "data unavailable" note; the app retries automatically.

---

## Epic 3 — Integration & delivery

### S9. Dev and prod wiring

As a developer, I want one obvious way to run the app in dev and ship it as one unit.

**Acceptance criteria**
- [ ] Vite dev server proxies `/api` to `localhost:8080` so `npm run dev` + `go run .` work together with no CORS fuss.
- [ ] Production: `npm run build` output is embedded in the Go binary via `go:embed` and served at `/`, with `/api/*` handled by the API — single deployable binary.
- [ ] README (or CLAUDE.md) documents both flows.

### S10. CI test coverage

As a developer, I want CI to catch regressions in parsers and the endpoint.

**Acceptance criteria**
- [ ] Existing CI runs `go test ./...` (fixtures from S1–S5, no live network calls in tests) and frontend `type-check` + `build`.
- [ ] CI fails on gofmt/vet issues for the backend.

---

## Out of scope (for now)

- Admin/manual override of values (the physical poster is hand-written; the app is API-driven only).
- Historical data, forecasts beyond today, other locations.
- PWA/offline support.
