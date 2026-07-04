# faja-app

Monorepo with two independent apps — see [CLAUDE.md](CLAUDE.md) for details on the dev environment and setup.

- `backend/` — Go (`net/http`), serves on `:8080`
- `frontend/` — Vue 3 + Vite + TypeScript

## Getting started

This project runs inside the devcontainer defined in `.devcontainer/`. Open it in GoLand via JetBrains Gateway (or any Dev Containers-compatible tool).

### Backend

```bash
cd backend
go run .
```

### Frontend

```bash
cd frontend
npm install
npm run dev
```
