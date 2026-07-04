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
