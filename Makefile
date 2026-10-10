.DEFAULT_GOAL := help
.PHONY: help backend-dev backend-test backend-lint \
	frontend-install frontend-dev frontend-test frontend-lint check up down

BACKEND := backend
FRONTEND := frontend

help: ## List available targets
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "} {printf "  %-16s %s\n", $$1, $$2}'

backend-dev: ## Run the API on :8080
	cd $(BACKEND) && go run ./cmd/server

backend-test: ## Run backend tests (race detector) and write backend/coverage/index.html
	@profile=$$(mktemp) && trap 'rm -f "$$profile"' EXIT && \
		cd $(BACKEND) && mkdir -p coverage && \
		go test -race -coverprofile="$$profile" ./... && \
		go tool cover -func="$$profile" | tail -1 && \
		go tool cover -html="$$profile" -o coverage/index.html

backend-lint: ## Check formatting and run go vet
	cd $(BACKEND) && test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	cd $(BACKEND) && go vet ./...

frontend-install: ## Install frontend dependencies
	cd $(FRONTEND) && npm ci

frontend-dev: ## Run the frontend dev server (proxies /api to :8080)
	cd $(FRONTEND) && npm run dev

frontend-test: ## Run frontend tests and write frontend/coverage/index.html
	cd $(FRONTEND) && npm run test:coverage

frontend-lint: ## Lint and type-check the frontend
	cd $(FRONTEND) && npm run lint && npm run typecheck

check: backend-lint backend-test frontend-lint frontend-test ## Lint and test both projects

up: ## Build and run the full stack with Docker Compose on http://localhost:8080
	docker compose up --build

down: ## Stop the Docker Compose stack
	docker compose down
