# Contributing to Go Sync Kit

Thanks for your interest in contributing to Go Sync Kit!

We welcome contributions of all kinds—code, documentation, examples, bug reports, and ideas. The project is a collaborative learning effort; both experienced Go developers and learners are encouraged to participate.

## Ways to Contribute
- Report bugs and request features
- Improve documentation and examples
- Submit code changes and refactors
- Add tests and improve reliability
- Review pull requests and share feedback

## Prerequisites
- Go 1.24+ (module specifies go 1.24.4)
- Git
- Docker (required for some integration tests: PostgreSQL; optional for RabbitMQ)
- GitHub account for forking and PRs

## Development Setup
1. Fork and clone:
   ```bash
   git clone https://github.com/YOUR_USERNAME/go-sync-kit.git
   cd go-sync-kit
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Verify build and run unit tests:
   ```bash
   go test ./...
   go test -race ./...
   ```

4. Try examples:
   - Quickstart (local-only):
     ```bash
     cd examples/quickstart/local-only
     go run .
     ```
   - Quickstart (HTTP client):
     ```bash
     cd examples/quickstart/http-client
     go run .
     ```

## Project Structure (actual)
```
go-sync-kit/
├── synckit/                    # Core sync manager, options, state machine, resolvers
│   ├── statemachine/           # Sync operation state machine
│   └── types/                  # Shared types
├── storage/
│   ├── sqlite/                 # SQLite store (WAL-enabled, production defaults)
│   ├── postgres/               # PostgreSQL EventStore (+ LISTEN/NOTIFY, migrations)
│   └── badger/                 # BadgerDB helper
├── transport/
│   ├── httptransport/          # HTTP client + server handler
│   ├── sse/                    # Server-Sent Events transport (subscribe)
│   └── rabbitmq/               # RabbitMQ transport (Phase 1 complete)
├── projection/                 # Projections and offsets (badger offsets included)
├── version/                    # Versioning (vector clocks, examples, benchmarks)
├── observability/              # Tracing, metrics, health checks
│   └── health/                 # Health endpoints and checks
├── logging/                    # Structured logging helpers
├── cursor/                     # Cursors and wire format
├── errors/                     # Structured error types and helpers
├── examples/                   # Quickstart + intermediate + server-projection-hooks
│   ├── quickstart/{local-only,http-client}
│   ├── intermediate/{03...10-*}
│   ├── observability_basic.go
│   └── observability_enterprise.go
├── docs/                       # Design docs, roadmaps, testing, projections
├── internal/integration-tests/ # Standalone verification program(s) (not go test)
├── docker-compose.test.yml     # PostgreSQL test environment
├── CHANGELOG.md
├── README.md
└── LICENSE
```

## Reporting Issues
Please include:
- Go version (`go version`)
- OS/arch
- go-sync-kit version (e.g., v0.20.0)
- Steps to reproduce
- Expected vs actual behavior
- Minimal code snippet
- Logs or stack traces

Security: If you believe you’ve found a security issue, please avoid posting details publicly. Open an issue with a minimal description and request private follow-up, or contact the maintainer privately if listed. A SECURITY.md will be added in the future.

## Contributing Code

### Git Workflow (Git Flow)

We use Git Flow with these main branches:
- **`main`** - Production-ready, stable releases
- **`develop`** - Integration branch for ongoing development
- **`feature/*`** - Individual features (branched from `develop`)
- **`hotfix/*`** - Emergency fixes to production (branched from `main`)

#### For Feature Development:
1. **Branch from develop**:
   ```bash
   git checkout develop
   git pull origin develop
   git checkout -b feature/your-feature-name
   ```

2. **Make changes and add tests**
   - Follow coding standards below
   - Add comprehensive tests
   - Update documentation if needed

3. **Run tests** (see "Testing" section below)
   ```bash
   go test ./...
   go test -race ./...
   ```

4. **Commit using conventional commits** (see format below)

5. **Push and open a PR against `develop`**:
   ```bash
   git push origin feature/your-feature-name
   ```
   - Open PR targeting `develop` branch
   - Include tests and documentation
   - Link related issues

#### For Hotfixes:
1. **Branch from main**:
   ```bash
   git checkout main
   git pull origin main
   git checkout -b hotfix/fix-critical-issue
   ```

2. **Make minimal fix, test thoroughly**

3. **Open PR against `main`** (will be merged to both `main` and `develop`)

### Coding Standards
- Use standard Go practices (gofmt, go vet).
- Add doc comments for exported symbols and complex logic.
- Handle errors explicitly; avoid panics in library code.
- Use context.Context for cancellation/timeouts.
- Be thread-safe in concurrent code.
- Add unit tests and table-driven tests for new functionality.

### Commit Messages (Conventional Commits)
Format:
```
<type>(<scope>): <description>
```
Types: feat, fix, docs, style, refactor, test, chore

Examples:
- feat(synckit): add event filtering support
- fix(storage/sqlite): resolve WAL mode regression
- docs: expand quickstart instructions
- test(transport/http): add compression tests

### Pull Requests
- Keep PRs focused on one topic.
- Include tests for changes.
- Update docs/examples if APIs change.
- Link related issues (e.g., “Fixes #123”).
- Ensure all tests pass (unit + any relevant integration).

## Testing

Most packages have extensive unit tests. Some integration tests require services:

- All unit tests:
  ```bash
  go test ./...
  go test -race ./...
  ```

- SQLite package tests:
  ```bash
  go test ./storage/sqlite/...
  ```

- HTTP transport tests:
  ```bash
  go test ./transport/httptransport/...
  ```

- RabbitMQ transport:
  - Tests are designed to skip if RabbitMQ is not reachable.
  - To run integration tests with RabbitMQ, use the Makefile in transport/rabbitmq:
    ```bash
    cd transport/rabbitmq
    make docker-up
    make test-integration   # or: go test -v -race -tags=integration -run TestIntegration ./...
    make docker-down
    ```

- PostgreSQL EventStore:
  - Requires a running Postgres. A Docker Compose setup is provided at repo root.
  - Start Postgres, then run tests:
    ```bash
    docker compose -f docker-compose.test.yml up -d postgres  # or: docker-compose ...
    go test ./storage/postgres/...
    ```
  - You can also set a custom connection string:
    ```bash
    POSTGRES_TEST_CONNECTION="postgres://user:pass@host/db?sslmode=disable" go test ./storage/postgres/...
    ```

Notes:
- internal/integration-tests/ contains a standalone program (not go test).
- Some long-running or external tests may be guarded by test logic (skips) or Makefiles.

## Documentation Updates
When making user-facing changes:
- Update README.md where appropriate.
- Add or adjust examples in examples/.
- Update package-level READMEs and code comments.
- Update CHANGELOG.md for notable changes.
- Add or update files under docs/ for design/architecture/testing content.

## Examples
- Quickstart: examples/quickstart/{local-only,http-client}
- Intermediate:
  - 03-events-and-storage
  - 04-conflict-resolution
  - 05-realtime-autosync
  - 06-custom-events-filters
  - 07-structured-logging
  - 09-advanced-observability
  - 10-state-machine-enhancements
- Observability demos:
  - examples/observability_basic.go
  - examples/observability_enterprise.go
- Server projection hooks:
  - examples/server-projection-hooks

## Architecture & Design Guidelines
- Small, focused interfaces; design for testability.
- Keep packages cohesive and minimize cross-package coupling.
- Export only what’s needed for the public API.
- Ensure concurrency correctness; use race detector in CI/local.

## Focus Areas for Contributions
- Documentation and examples
- Test coverage and reliability
- PostgreSQL EventStore enhancements and docs
- RabbitMQ transport improvements (Phase 2 features)
- Observability (metrics/health/tracing integration and examples)

## License
Licensed under the MIT License. See LICENSE for details.

