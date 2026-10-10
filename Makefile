.DEFAULT_GOAL := help
.PHONY: help backend-run backend-test backend-cover backend-lint backend-docker

BACKEND := backend

help: ## List available targets
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "} {printf "  %-16s %s\n", $$1, $$2}'

backend-run: ## Run the API on :8080
	cd $(BACKEND) && go run ./cmd/server

backend-test: ## Run backend tests with the race detector
	cd $(BACKEND) && go test -race ./...

backend-cover: ## Backend coverage report: backend/coverage/index.html
	@profile=$$(mktemp) && trap 'rm -f "$$profile"' EXIT && \
		cd $(BACKEND) && mkdir -p coverage && \
		go test -race -coverprofile="$$profile" ./... && \
		go tool cover -func="$$profile" | tail -1 && \
		go tool cover -html="$$profile" -o coverage/index.html

backend-lint: ## Check formatting and run go vet
	cd $(BACKEND) && test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	cd $(BACKEND) && go vet ./...

backend-docker: ## Build the backend image
	docker build -t sezzle-calculator-backend $(BACKEND)
