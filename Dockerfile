# Multi-stage build for the single deployable faja-app binary.
# Mirrors the "Prod: one binary" flow in CLAUDE.md: build the Vue SPA, copy it
# into backend/web/dist so go:embed can reach it, then compile the Go binary
# that serves both the SPA and /api/* on one port.

# --- Stage 1: build the frontend (Vue + Vite) ---
FROM node:22-bookworm-slim AS frontend
WORKDIR /app/frontend
# Install deps first for better layer caching.
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build          # -> /app/frontend/dist

# --- Stage 2: build the Go binary (embedding the built SPA) ---
FROM golang:1.26-bookworm AS backend
WORKDIR /app/backend
# Cache Go module downloads.
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# Drop in the built SPA where //go:embed all:dist expects it, replacing the
# committed .gitkeep placeholder.
COPY --from=frontend /app/frontend/dist/ ./web/dist/
# Static binary so it runs on the scratch/distroless runtime below.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /faja-app .

# --- Stage 3: minimal runtime ---
# distroless/static ships CA certificates (needed for HTTPS calls to IPMA and
# tabuademares); the tz database is embedded in the binary via the
# time/tzdata import in main.go, so no OS tzdata is required here.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend /faja-app /faja-app
# Documentation only; the process binds to $PORT (Render/Cloud Run inject it)
# and falls back to 8080.
EXPOSE 8080
ENTRYPOINT ["/faja-app"]
