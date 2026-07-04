# faja-app

## Project layout

Monorepo with two independent apps:

- `backend/` — Go module (`github.com/Coder266/faja-app/backend`), plain `net/http`, entry point `main.go`, serves on `:8080`.
- `frontend/` — Vue 3 + Vite + TypeScript app (scaffolded via `npm create vue@latest`).

There is no shared build system tying the two together; run/build each independently.

## Dev environment

This project runs inside a devcontainer, connected to via **GoLand** (JetBrains Gateway), not VS Code. This matters for config:

- `customizations.vscode` in `.devcontainer/devcontainer.json` is **inert** here — it's only read by the VS Code Dev Containers extension.
- `customizations.jetbrains` is what actually applies: `backend` picks the IDE (GoLand), `plugins` lists marketplace plugin IDs auto-installed on the JetBrains backend (e.g. `org.jetbrains.plugins.vue`, since GoLand doesn't bundle Vue support the way WebStorm/IntelliJ Ultimate do).

Base image: `mcr.microsoft.com/devcontainers/go:1-bookworm`, plus devcontainer features for Claude Code and Node 22 (`ghcr.io/devcontainers/features/node:1`).

**Any change to `.devcontainer/devcontainer.json` requires a container rebuild to take effect** (JetBrains: rebuild via Gateway; there is no in-place reload).

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
cd frontend && npm run dev      # Vite dev server, typically :5173
```

`frontend/vite.config.ts` has a `server.proxy` entry that forwards `/api/*`
requests from the Vite dev server to `http://localhost:8080`. The browser
only ever talks to the Vite origin, so this is not a cross-origin request and
no CORS is involved. (The backend's `withCORS` middleware permitting
`http://localhost:5173` is still in place as a harmless fallback for anyone
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

## Git / GitHub

- Repo remote: private repo under the `Coder266` GitHub account, authenticated locally via `gh auth login`.
- Local git identity for this repo is set to the personal email (not the global/work identity configured elsewhere on this machine) — check `git config user.email` if commits look wrong.
- `gh` CLI is installed via the `ghcr.io/devcontainers/features/github-cli:1` devcontainer feature (not manually `apt`-installed — that gets wiped on rebuild).
- `gh auth login` state persists across rebuilds via a named volume mounted at `/home/vscode/.config/gh` (`GH_CONFIG_DIR` env var points there). This exists because JetBrains Gateway sets `XDG_CONFIG_HOME` to `/.jbdevcontainer/config`, which lives in the container's ephemeral filesystem and is wiped on every rebuild — `GH_CONFIG_DIR` takes precedence over `XDG_CONFIG_HOME` and overrides that.
- If `git push`/`gh` commands fail after creating a new repo with a `refusing to allow an OAuth App to create or update workflow` error, the token is missing the `workflow` scope — run `gh auth refresh -h github.com -s workflow`. Also run `gh auth setup-git` once per fresh login so `git push` picks up gh's credentials (git doesn't do this automatically).
