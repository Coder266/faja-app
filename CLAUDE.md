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

## Git / GitHub

- Repo remote: private repo under the `Coder266` GitHub account, authenticated locally via `gh auth login`.
- Local git identity for this repo is set to the personal email (not the global/work identity configured elsewhere on this machine) — check `git config user.email` if commits look wrong.
- `gh` CLI is installed via the `ghcr.io/devcontainers/features/github-cli:1` devcontainer feature (not manually `apt`-installed — that gets wiped on rebuild).
- `gh auth login` state persists across rebuilds via a named volume mounted at `/home/vscode/.config/gh` (`GH_CONFIG_DIR` env var points there). This exists because JetBrains Gateway sets `XDG_CONFIG_HOME` to `/.jbdevcontainer/config`, which lives in the container's ephemeral filesystem and is wiped on every rebuild — `GH_CONFIG_DIR` takes precedence over `XDG_CONFIG_HOME` and overrides that.
- If `git push`/`gh` commands fail after creating a new repo with a `refusing to allow an OAuth App to create or update workflow` error, the token is missing the `workflow` scope — run `gh auth refresh -h github.com -s workflow`. Also run `gh auth setup-git` once per fresh login so `git push` picks up gh's credentials (git doesn't do this automatically).
