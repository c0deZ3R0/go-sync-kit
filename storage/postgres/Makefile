# Makefile for PostgreSQL EventStore

.PHONY: help build test test-integration test-unit docker-up docker-down clean example benchmark

# Default target
help:
	@echo "Available targets:"
	@echo "  build            - Build the PostgreSQL EventStore package"
	@echo "  test             - Run all tests"
	@echo "  test-unit        - Run only unit tests (fast)"
	@echo "  test-integration - Run integration tests (requires PostgreSQL)"
	@echo "  benchmark        - Run benchmark tests"
	@echo "  docker-up        - Start PostgreSQL with Docker Compose"
	@echo "  docker-down      - Stop PostgreSQL Docker containers"
	@echo "  docker-logs      - View PostgreSQL logs"
	@echo "  example          - Run the example application"
	@echo "  clean            - Clean build artifacts and test data"
	@echo "  fmt              - Format code"
	@echo "  lint             - Run linter"
	@echo "  deps             - Download dependencies"

# Build the package
build:
	@echo "Building PostgreSQL EventStore..."
	go build ./...

# Run all tests
test: docker-up
	@echo "Running all tests..."
	go test -v ./...

# Run only unit tests (fast, no external dependencies)
test-unit:
	@echo "Running unit tests..."
	go test -short -v ./...

# Run integration tests (requires PostgreSQL)
test-integration: docker-up
	@echo "Running integration tests..."
	go test -v ./... -run ".*Integration.*|.*Realtime.*|.*Connection.*"

# Run benchmark tests
benchmark: docker-up
	@echo "Running benchmark tests..."
	go test -bench=. -benchmem ./...

# Start PostgreSQL with Docker Compose
docker-up:
	@echo "Starting PostgreSQL with Docker Compose..."
	docker-compose -f ../../docker-compose.test.yml up -d postgres
	@echo "Waiting for PostgreSQL to be ready..."
	@timeout=30; \
	while ! docker exec go-sync-kit-postgres-test pg_isready -U testuser -d eventstore_test > /dev/null 2>&1; do \
		timeout=$$((timeout - 1)); \
		if [ $$timeout -eq 0 ]; then \
			echo "Timeout waiting for PostgreSQL to be ready"; \
			exit 1; \
		fi; \
		sleep 1; \
	done
	@echo "PostgreSQL is ready!"

# Stop PostgreSQL Docker containers
docker-down:
	@echo "Stopping PostgreSQL containers..."
	docker-compose -f ../../docker-compose.test.yml down

# View PostgreSQL logs
docker-logs:
	@echo "Viewing PostgreSQL logs..."
	docker logs go-sync-kit-postgres-test

# Run the example application
example: build docker-up
	@echo "Running example application..."
	./../../postgres-eventstore-example || go run ./example/main.go

# Clean build artifacts and test data
clean:
	@echo "Cleaning up..."
	go clean ./...
	rm -f ../../postgres-eventstore-example
	docker-compose -f ../../docker-compose.test.yml down -v
	docker system prune -f

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	golangci-lint run ./... || echo "golangci-lint not found. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Development setup
dev-setup: deps docker-up
	@echo "Setting up development environment..."
	@echo "PostgreSQL is running at: localhost:5432"
	@echo "Database: eventstore_test"
	@echo "User: testuser"
	@echo "Password: testpass123"
	@echo "Connection string: postgres://testuser:testpass123@localhost:5432/eventstore_test?sslmode=disable"

# Quick development test cycle
dev-test: fmt build test-unit
	@echo "Quick development test cycle completed!"

# Full test cycle for CI/CD
ci-test: fmt build test benchmark
	@echo "Full CI test cycle completed!"

# Database operations
db-connect:
	@echo "Connecting to PostgreSQL..."
	docker exec -it go-sync-kit-postgres-test psql -U testuser -d eventstore_test

db-reset: docker-down docker-up
	@echo "Database reset completed!"

# Generate migration (placeholder for future use)
migration:
	@echo "Migration generation not yet implemented"

# Show database status
db-status:
	@echo "Checking database status..."
	@docker exec go-sync-kit-postgres-test pg_isready -U testuser -d eventstore_test && echo "✓ Database is ready" || echo "✗ Database is not ready"
	@echo "Container status:"
	@docker ps --filter "name=go-sync-kit-postgres-test" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
