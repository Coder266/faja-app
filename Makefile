# faja-app — dev/build convenience targets.
# Two independent apps: Go API in backend/, Vue+Vite SPA in frontend/.
# Run `make` or `make help` to list targets.

BACKEND  := backend
FRONTEND := frontend
EMBED_DIR := $(BACKEND)/web/dist

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| sort \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

## --- Setup ---

.PHONY: install
install: ## Install frontend deps (npm) and tidy Go modules
	cd $(FRONTEND) && npm install
	cd $(BACKEND) && go mod tidy

## --- Dev (live reload) ---

.PHONY: dev
dev: ## Run API (:8080) + Vite dev server (:5173) together; Ctrl-C stops both
	@echo "API on :8080, web on :5173 (Vite proxies /api -> :8080). Ctrl-C to stop."
	@trap 'kill 0' INT TERM; \
		( cd $(BACKEND) && go run . ) & \
		( cd $(FRONTEND) && npm run dev ) & \
		wait

.PHONY: dev-api
dev-api: ## Run only the Go API on :8080
	cd $(BACKEND) && go run .

.PHONY: dev-web
dev-web: ## Run only the Vite dev server on :5173
	cd $(FRONTEND) && npm run dev

## --- Production build (single binary) ---

.PHONY: build
build: build-web ## Build the single deployable binary (embeds the built SPA) -> backend/backend
	rm -rf $(EMBED_DIR)
	mkdir -p $(EMBED_DIR)
	cp -r $(FRONTEND)/dist/. $(EMBED_DIR)/
	cd $(BACKEND) && go build -o backend .
	@echo "Built $(BACKEND)/backend — run it with 'make run' (serves SPA + /api on :8080)."

.PHONY: build-web
build-web: ## Build the frontend to frontend/dist
	cd $(FRONTEND) && npm run build

.PHONY: run
run: build ## Build then run the single binary on :8080
	cd $(BACKEND) && ./backend

## --- Quality checks (what CI runs) ---

.PHONY: check
check: fmt-check vet test type-check ## Run all backend + frontend checks (mirrors CI)

.PHONY: test
test: ## Run backend Go tests
	cd $(BACKEND) && go test ./...

.PHONY: vet
vet: ## Run go vet
	cd $(BACKEND) && go vet ./...

.PHONY: fmt
fmt: ## Format Go code in place
	cd $(BACKEND) && gofmt -w .

.PHONY: fmt-check
fmt-check: ## Fail if any Go file is not gofmt-formatted (as CI does)
	@cd $(BACKEND) && unformatted=$$(gofmt -l .); \
		if [ -n "$$unformatted" ]; then \
			echo "These files are not gofmt-formatted:"; echo "$$unformatted"; exit 1; \
		fi

.PHONY: type-check
type-check: ## Run frontend TypeScript type-check
	cd $(FRONTEND) && npm run type-check

.PHONY: lint
lint: ## Run frontend linter
	cd $(FRONTEND) && npm run lint

## --- Cleanup ---

.PHONY: clean
clean: ## Remove build artifacts (binary, dist, embedded copy) but keep the embed placeholder
	rm -f $(BACKEND)/backend
	rm -rf $(FRONTEND)/dist
	find $(EMBED_DIR) -mindepth 1 ! -name .gitkeep -exec rm -rf {} +
	@touch $(EMBED_DIR)/.gitkeep
	@echo "Cleaned. (kept $(EMBED_DIR)/.gitkeep placeholder for go:embed)"
