# RabbitMQ Transport Makefile
# Quick commands for development and testing

.PHONY: help test test-unit test-integration docker-test docker-up docker-down clean build lint fmt deps

help: ## Show this help message
	@echo 'Usage: make [TARGET]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

deps: ## Download and verify dependencies
	go mod download
	go mod verify
	go mod tidy

fmt: ## Format Go code
	gofmt -s -w .
	go vet ./...

lint: ## Run linting tools
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping..."; \
		go vet ./...; \
	fi

build: ## Build the transport package
	go build ./...

test: test-unit ## Run all tests (unit tests only by default)

test-unit: ## Run unit tests only
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

test-integration: ## Run integration tests (requires RabbitMQ)
	go test -v -race -tags=integration -run TestIntegration ./...

docker-test: docker-up ## Run integration tests with Docker RabbitMQ
	@echo "Starting RabbitMQ container and running integration tests..."
	@if command -v docker-compose >/dev/null 2>&1; then \
		docker-compose -f docker-compose.test.yml run --rm test-runner sh -c "cd /app && go test -v -tags=integration ./transport/rabbitmq -run TestIntegration"; \
	elif docker compose version >/dev/null 2>&1; then \
		docker compose -f docker-compose.test.yml run --rm test-runner sh -c "cd /app && go test -v -tags=integration ./transport/rabbitmq -run TestIntegration"; \
	else \
		echo "Docker Compose not available"; \
		exit 1; \
	fi

docker-up: ## Start RabbitMQ container for development
	@echo "Starting RabbitMQ container..."
	@if command -v docker-compose >/dev/null 2>&1; then \
		docker-compose -f docker-compose.test.yml up -d rabbitmq; \
	elif docker compose version >/dev/null 2>&1; then \
		docker compose -f docker-compose.test.yml up -d rabbitmq; \
	else \
		echo "Docker Compose not available"; \
		exit 1; \
	fi
	@echo "RabbitMQ Management UI: http://localhost:15672 (synckit_user/synckit_pass)"

docker-down: ## Stop and remove containers
	@if command -v docker-compose >/dev/null 2>&1; then \
		docker-compose -f docker-compose.test.yml down; \
	elif docker compose version >/dev/null 2>&1; then \
		docker compose -f docker-compose.test.yml down; \
	else \
		echo "Docker Compose not available"; \
	fi

docker-logs: ## Show RabbitMQ container logs
	@if command -v docker-compose >/dev/null 2>&1; then \
		docker-compose -f docker-compose.test.yml logs rabbitmq; \
	elif docker compose version >/dev/null 2>&1; then \
		docker compose -f docker-compose.test.yml logs rabbitmq; \
	else \
		echo "Docker Compose not available"; \
	fi

clean: docker-down ## Clean up containers, coverage files, and test artifacts
	rm -f coverage.out
	rm -f rabbitmq.test
	go clean -testcache

benchmark: ## Run benchmarks
	go test -bench=. -benchmem ./...

coverage-html: test-unit ## Generate and open HTML coverage report
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Development workflow targets
dev-setup: deps build ## Setup for development (deps + build)

dev-test: fmt lint test ## Development test workflow (format + lint + unit tests)

ci-test: deps build lint test-unit ## CI workflow (deps + build + lint + unit tests)

# Quick checks
check: fmt lint build ## Quick check (format + lint + build)

all: clean deps fmt lint build test ## Run everything
