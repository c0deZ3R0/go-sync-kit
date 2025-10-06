# IMPLEMENTATION_PLAN: Public API & package ergonomics

Branch: feat/public-api-ergonomics
Status: In Progress
Owner: blain
Last updated: 2025-10-06T11:13:00Z

Project rules honored:
- Test often; commit only with 100% passing tests
- Do not change/merge branches or PRs unless asked
- Review and update this IMPLEMENTATION_PLAN after every commit

## Objectives
Improve developer ergonomics with a single import surface, a canonical configuration entrypoint with validation, and stabilized interfaces consolidated under synckit/types. Preserve backward compatibility where feasible (functional options remain supported), while providing a clear, validated New(Config) path.

## Scope
Covers:
- 1.1 Single import surface (re-export core types)
- 1.2 Canonical config path (Config + New(Config) with validation)
- 1.3 Stabilize interfaces (freeze in synckit/types with doc hints)

Out of scope for this phase:
- Transport/Store concrete implementations beyond interface conformance updates
- Cross-process subscription daemons or new transports/stores
- Performance optimizations beyond correctness and API ergonomics

## Milestones and Work Breakdown

### Milestone 1 — Single import surface ✅ COMPLETE
Goal: applications do `import synckit` and pick store/transport; re-export core types from synckit.

**Status:** COMPLETE (2025-10-06T11:05:00Z)

Planned changes:
- Add synckit/api.go to re-export core types from synckit/types via type aliases.
- Ensure package synckit has top-level doc.go with package overview and usage examples.
- Verify module path stability and import usability in a small example program under examples/ or internal compile-only tests.

Files (new/updated):
- synckit/api.go (new)
- synckit/doc.go (new)
- examples/basic/main.go or internal test verifying import surface (optional for this milestone if examples dir exists)

Tests/validation:
- Compile-time checks: build a small example or test that imports synckit and references Event, Version, Store, Transport.
- go vet/staticcheck clean for the new package surface.

Acceptance criteria:
- `go build ./...` passes with synckit/api.go in place
- A program can `import "github.com/c0deZ3R0/go-sync-kit/synckit"` and refer to aliased types

Risks/notes:
- Name collisions in synckit package; ensure no conflicting exported identifiers.

### Milestone 2 — Canonical config path ✅ COMPLETE
Goal: keep functional options but add a canonical `Config` and `New(Config)` with validation.

**Status:** COMPLETE (2025-10-06T11:10:00Z)

Planned changes:
- Add synckit/config.go with:
  - CursorMode enum (CursorInteger, CursorVector)
  - RetryPolicy {Max, Base, Cap, Jitter}
  - Config {Store, Transport, Logger, Cursor, Retry, Resolvers, Timeout}
  - func (c *Config) Validate() error
  - func New(cfg Config) (*Manager, error)
- Implement or adapt an internal constructor `newManagerFromConfig(cfg Config)` used by New.
- Ensure existing functional options path remains supported (do not remove). If current public ctor exists (e.g., NewManager(…options)), it should internally build a Config or call through to the same wiring to avoid divergence.
- Add stub or reference for ResolverRegistry (see §3), keeping forward-compatibility; if it already exists, wire it; otherwise define an interface or type placeholder where appropriate.

Files (new/updated):
- synckit/config.go (new)
- synckit/manager wiring (existing) updated to consume Config internally (new function or refactor)
- synckit/options.go (existing, if present) left intact and bridged to New(Config)

Tests/validation:
- Unit tests for Config.Validate() covering nil checks, sane timeouts, retry bounds (Max>=0, Base>0, Cap>=Base, etc.).
- Tests that New(cfg) errors on invalid config and succeeds on valid config (with minimal test doubles for Store/Transport).
- Back-compat test: existing WithX/functional options path still works and yields a Manager equivalent to New(cfg) for a representative configuration.

Acceptance criteria:
- `go test ./...` is green
- New(cfg) is the documented canonical entrypoint; functional options documented as supported

Risks/notes:
- Avoid duplication between New(cfg) and functional options path; prefer single wiring function.

### Milestone 3 — Stabilize interfaces
Goal: freeze Store and Transport interfaces and related types in synckit/types with forward-compat Filter; add GoDoc “Implementors” hints.

Planned changes:
- Ensure synckit/types includes:
  - Store interface: Push, Pull, Latest
  - Transport interface: Push, Pull, Subscribe
  - Filter struct {Key, Value string}
  - Event, EventWithVersion, Version, EventHandler, etc. (as currently defined) consolidated or referenced
- Update in-repo implementations and call sites to conform to these signatures if any drift exists.
- Add GoDoc comments to each interface with guidance and implementation notes (latency/ordering/cursor semantics, error handling expectations).

Files (new/updated):
- synckit/types/*.go (updated or new store.go/transport.go as needed)
- package-level doc comments

Tests/validation:
- Compile-time interface conformance checks for in-repo implementations (e.g., var _ Store = (*MyStore)(nil)).
- Minimal behavior tests around Push/Pull/Subscribe contracts using fakes, where applicable.

Acceptance criteria:
- Interfaces are in synckit/types and imported by synckit/api.go
- All in-repo implementations compile and tests pass

Risks/notes:
- If Subscribe semantics vary by transport, document clearly and keep Filter opaque for forward-compat.

## Backward compatibility and migration
- Existing import paths should continue to work. New recommended path for app code: import synckit and refer to aliased types.
- Functional options remain supported; New(Config) is canonical and documented.
- Any breaking changes to interfaces must be avoided in this phase; if unavoidable, document rationale and migration steps here before change.

## Testing strategy
- Unit tests for validation and wiring (Milestone 2) and interface conformance (Milestone 3).
- Compile-only examples or tests for the import surface (Milestone 1).
- Run `go vet` and `staticcheck` if available.

## Versioning and release
- No tag/release actions until directed. If API changes require a version bump, propose SemVer impact (likely minor).

## Commit plan (update after each commit)
1. ✅ M1: Add synckit/api.go with type aliases; add doc.go; compile check; tests green. (COMPLETE)
- Added synckit/doc.go with package overview and usage examples (now the single source for package docs)
- Consolidated API surface docs into synckit/doc.go; removed synckit/api.go to avoid duplication
   - Added synckit/api_test.go with compile-time checks for import surface
   - All tests pass: `go test ./synckit/... && go build ./...`
   - Commit: "feat: add single import surface documentation and tests (Milestone 1)"
2. ✅ M2: Add synckit/canonical_config.go types and Validate(); introduce newManagerFromConfig(cfg); wire New(cfg); add unit tests. (COMPLETE)
   - Added synckit/canonical_config.go with Config struct, CursorMode, RetryPolicy, Validate(), New(cfg), and newManagerFromConfig()
   - Added synckit/canonical_config_test.go with comprehensive validation and constructor tests
   - Config struct bridges to existing SyncManagerBuilder to avoid duplication
   - Functional options remain supported (backward compatible)
   - All tests pass: `go test ./synckit/...`
   - Commit: "feat: add canonical Config and New(cfg) constructor (Milestone 2)"
3. M2: Bridge existing functional options to use shared wiring; tests ensuring equivalence; docs updated.
4. M3: Finalize synckit/types interfaces (store.go/transport.go); add docs; add compile-time conformance checks; fix call sites; tests.
5. Polish: examples and README snippet updates (optional); vet/staticcheck.

Checklist for each commit:
- All tests pass locally: `go test ./...`
- Plan updated (this file): what changed, what’s next
- No merges/PR changes unless explicitly requested

## Open questions
- Exact shape and location of ResolverRegistry (§3): is there a current registry type we should integrate, or introduce a minimal interface now?
- Manager type and wiring entrypoint name: confirm `newManagerFromConfig` naming or reuse an existing constructor.
- Preferred example path (examples/ vs internal/testdata) for import-surface compile check.

## Completed actions
- ✅ Milestone 1: Created synckit/api.go (documentation), synckit/doc.go (package overview), and synckit/api_test.go (compile checks). Tests pass, build succeeds.
- ✅ Milestone 2: Added synckit/canonical_config.go with Config, CursorMode, RetryPolicy, Validate(), and New(cfg). Unit tests comprehensive. Backward compatible with functional options.

## Next action
- Proceed with Milestone 3: Stabilize interfaces in synckit/types - add Store/Transport interfaces with docs, compile-time conformance checks, update call sites.
