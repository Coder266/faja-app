# faja-app

## Project layout

Monorepo with two independent apps:

- `backend/` — Go module (`github.com/Coder266/faja-app/backend`), plain `net/http`, entry point `main.go`, serves on `:8080`.
- `frontend/` — Vue 3 + Vite + TypeScript app (scaffolded via `npm create vue@latest`).

There is no shared build system tying the two together; run/build each independently. A root **`Makefile`** wraps the common flows so you don't have to memorize them — `make` (or `make help`) lists all targets. Everyday: `make dev` (API + Vite together), `make build`/`make run` (single embedded binary), `make check` (everything CI runs). The raw commands are still documented per-app below.

## Stories and the agent workflow

The product backlog is `docs/stories.md` (stories S1–S10 in three epics: backend data services, frontend poster UI, integration/delivery). It includes the verified IPMA endpoints and the tabuademares.com scraping constraints — treat it as the source of truth for scope and acceptance criteria.

Implementation runs through a two-agent pipeline orchestrated from the main session:

- **`implementer`** (`.claude/agents/implementer.md`, Sonnet) — implements exactly one story (or one round of review fixes) per assignment, writes the story's tests against fixtures (never live network in tests), runs gofmt/vet/`go test` or `type-check`/`build`, and **never commits** — it leaves changes in the working tree.
- **`reviewer`** (`.claude/agents/reviewer.md`, Opus, read-only + Bash) — reviews the uncommitted working tree against the story's acceptance criteria, re-runs verification itself, and replies with `VERDICT: APPROVED` or `VERDICT: CHANGES_REQUESTED` plus numbered `file:line` findings. Only real problems block; style nits are non-blocking notes.
- **`/implement-epic`** (`.claude/skills/implement-epic/SKILL.md`) — the orchestrator. `/implement-epic` runs all remaining stories, `/implement-epic 1` one epic, `/implement-epic S4` one story. Per story it loops implementer → reviewer → fix rounds (same agents continued via SendMessage, max 3 fix rounds) until approval, then commits that story as `S<n>: <title>`. Already-committed `S<n>:` stories are skipped, so the command is resumable. It stops and reports if a story can't pass review in 3 rounds. It never pushes.

If you change the agent contracts (verdict format, commit convention, no-commit rule), update all three files together — the loop depends on them agreeing.

## Dev environment

This project runs inside a devcontainer, connected to via **GoLand** (JetBrains Gateway), not VS Code. This matters for config:

- `customizations.vscode` in `.devcontainer/devcontainer.json` is **inert** here — it's only read by the VS Code Dev Containers extension.
- `customizations.jetbrains` is what actually applies: `backend` picks the IDE (GoLand), `plugins` lists marketplace plugin IDs auto-installed on the JetBrains backend (e.g. `org.jetbrains.plugins.vue`, since GoLand doesn't bundle Vue support the way WebStorm/IntelliJ Ultimate do).

Base image: `mcr.microsoft.com/devcontainers/go:1-bookworm`, plus devcontainer features for Claude Code and Node 22 (`ghcr.io/devcontainers/features/node:1`).

**Any change to `.devcontainer/devcontainer.json` requires a container rebuild to take effect** (JetBrains: rebuild via Gateway; there is no in-place reload).

**Playwright (for UI screenshots/verification)** is provisioned on container create: `playwright` is a `frontend` devDependency and `postCreateCommand` runs `npx playwright install --with-deps chromium`. Browsers are pinned to `/home/vscode/.cache/ms-playwright` via the `PLAYWRIGHT_BROWSERS_PATH` env — set in `containerEnv` so the create-time install and the JetBrains Gateway shell agree on the path (the Gateway shell otherwise defaults browsers into the ephemeral `/.jbdevcontainer` tree, same gotcha as `GH_CONFIG_DIR`). To screenshot the running dev server, drive Chromium with a script under `frontend/` (so the bare `playwright` import resolves from `frontend/node_modules`).

## Go backend

```bash
cd backend
go build ./...
go run .
```

`go build` writes a binary named `backend` into `backend/` — it's gitignored, don't commit it.

## Vue frontend

```bash
cd frontend
npm install
npm run dev
```

Requires Node 22+ (the Vue/Vite scaffolding tool refuses to run on Node 18).

## Dev vs. prod wiring (S9)

The frontend calls the API via a relative path (`/api/conditions`, see
`frontend/src/api/conditions.ts`), so both flows below make that path resolve
without any CORS setup being required.

### Dev: two processes + Vite proxy

```bash
# terminal 1
cd backend && go run .          # API on :8080

# terminal 2
cd frontend && npm run dev      # Vite dev server on :5174 (5173 is the devcontainer)
```

`frontend/vite.config.ts` has a `server.proxy` entry that forwards `/api/*`
requests from the Vite dev server to `http://localhost:8080`. The browser
only ever talks to the Vite origin, so this is not a cross-origin request and
no CORS is involved. (The backend's `withCORS` middleware permitting
`http://localhost:5174` is still in place as a harmless fallback for anyone
hitting `:8080` directly, but the dev flow above no longer depends on it.)

### Prod: one binary

`backend/web/web.go` embeds `backend/web/dist` (via `go:embed`) and
`main.go` serves it at `/`, with `/api/*` still handled by the API on the
same mux — the result is a single deployable binary with no separate static
file host needed. `go:embed` can only reach files inside the backend
module's own tree, so it can't embed `../frontend/dist` directly; the build
copies it in first:

```bash
cd frontend && npm run build              # writes frontend/dist
rm -rf ../backend/web/dist
mkdir -p ../backend/web/dist
cp -r dist/. ../backend/web/dist/
cd ../backend && go build -o backend .     # embeds backend/web/dist, single binary
./backend                                  # serves both the SPA and /api/* on :8080
```

`backend/web/dist` is a gitignored build artifact (like the `backend`
binary itself) — only a placeholder `backend/web/dist/.gitkeep` is committed
so `go build ./...` and `go test ./...` still compile from a clean checkout
without requiring a frontend build first (the embed pattern is
`//go:embed all:dist`; the `all:` prefix is required so the directory still
embeds successfully when it contains only that dotfile placeholder — a plain
`//go:embed dist` fails to compile in that case with "contains no embeddable
files"). After running the real build sequence above, `git status` will show
new untracked files under `backend/web/dist/` from the frontend
build — that's expected and they stay untracked/ignored; don't commit them.
The static handler is a plain `http.FileServer` over the embedded FS (no SPA
fallback to `index.html` for unknown paths — this app is effectively a
single page, so that's not needed today).

## Codebase map (for bugfixing)

All of S1–S10 are implemented and committed. Where things live and how to reproduce a bug offline:

**Backend (`backend/`)** — three data packages plus the wiring:
- `ipma/` — IPMA open-data clients: `airtemp.go` (S1, air temp min/max), `uv.go` (S2, UV index + PT/EN WHO classification), `watertemp.go` (S3, sea-surface temp). One shared `Client` struct; `globalIdLocal` codes are `2310300` (Funchal, air+UV) and `2310326` (Madeira south coast, water).
- `tides/` — `tides.go` + `parse.go` (S4): scrapes `tabuademares.com/pt/madeira/funchal` with a `golang.org/x/net/html` parser (the only third-party dep). Defensive: on markup change it returns `nil` tides + a logged warning, never a crash. Has its own ≥6h cache. Selector strategy + robots.txt findings are documented in the source comments — read them before touching the parser.
- `conditions/` — `conditions.go` (S5): the `Aggregator` behind `GET /api/conditions`. Fetches the four sources concurrently, nulls only the failed field (still 200), 30-min in-memory cache.
- `main.go` — mux wiring, `withCORS` (dev fallback), and the embedded-SPA static handler. `web/web.go` does the `go:embed`.
- **Testing seam:** every client exposes injectable `BaseURL`, `HTTPClient`, and `Now func() time.Time` fields precisely so tests (and repro cases) run against `testdata/` fixtures or `httptest`, never live network. To reproduce a parsing bug, add/adjust a fixture and pin `Now` — don't hit the real API. Errors are typed (`*ipma.Error`); "today" is always computed in `Atlantic/Madeira`.

**Frontend (`frontend/src/`)**:
- `api/conditions.ts` — typed `Conditions` client (matches the S5 JSON; nullable fields). Calls the relative `/api/conditions` (proxied in dev, same-origin in prod).
- `lib/formatConditions.ts` — **pure** formatting/mapping helpers (date `DD/MM`, temp ranges, UV lines, tides, `—` for nulls). **Display/formatting bugs almost always live here**, and it's plain TS so `npm run type-check` covers it.
- `components/PosterBoard.vue` — fetch + `status: loading|ready|error` state, refetch on visibility + hourly + 30s error-retry. `InfoBlock.vue` is presentational (renders `lines` + placeholders). `PosterBackdrop.vue` is decorative. Caveat handwriting font is bundled locally in `assets/fonts/` (no CDN).
- No test runner is configured; type-check is the safety net. CI runs `type-check` + `build` + `lint`.

## Git / GitHub

- Repo remote: private repo under the `Coder266` GitHub account, authenticated locally via `gh auth login`.
- Local git identity for this repo is set to the personal email (not the global/work identity configured elsewhere on this machine) — check `git config user.email` if commits look wrong.
- `gh` CLI is installed via the `ghcr.io/devcontainers/features/github-cli:1` devcontainer feature (not manually `apt`-installed — that gets wiped on rebuild).
- `gh auth login` state persists across rebuilds via a named volume mounted at `/home/vscode/.config/gh` (`GH_CONFIG_DIR` env var points there). This exists because JetBrains Gateway sets `XDG_CONFIG_HOME` to `/.jbdevcontainer/config`, which lives in the container's ephemeral filesystem and is wiped on every rebuild — `GH_CONFIG_DIR` takes precedence over `XDG_CONFIG_HOME` and overrides that.
- If `git push`/`gh` commands fail after creating a new repo with a `refusing to allow an OAuth App to create or update workflow` error, the token is missing the `workflow` scope — run `gh auth refresh -h github.com -s workflow`. Also run `gh auth setup-git` once per fresh login so `git push` picks up gh's credentials (git doesn't do this automatically).
