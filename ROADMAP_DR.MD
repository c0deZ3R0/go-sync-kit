# Roadmap: Dynamic Conflict Resolution (Strategy + CoR + Specification)

This roadmap upgrades the conflict resolution subsystem of `go-sync-kit` from a basic resolver to a robust, configurable DynamicResolver grounded in proven patterns:
- Strategy (pluggable ConflictResolver implementations)
- Chain of Responsibility (ordered rule dispatch)
- Specification (composable, testable match predicates)
- Builder/Functional Options (ergonomic configuration with validation)
- Optional: Memento (audit/undo), Observer (metrics/logging), Composite (rule grouping), Adapter (domain bindings)

Goals
- Keep go-sync-kit generic and reusable across domains.
- Make conflict handling deterministic, testable, and observable.
- Support config- or code-driven rule wiring without leaking domain specifics into core.

Outcomes
- A DynamicResolver that routes conflicts to Strategy handlers via ordered, composable match rules with clear fallback semantics.
- Strong test coverage (unit, fuzz, integration), tracing/metrics hooks, and optional audit.

---

## Phase 1 – Foundations (types, specs, base strategies)

Deliverables
1) synckit/conflict.go
- Conflict: rich context for a detected conflict (local vs remote), including:
  - EventType, AggregateID
  - ChangedFields/deltas (optional but recommended)
  - Local and Remote versions (and/or vector/cursor snapshot)
  - Origin metadata (node, timestamps), Arbitrary Metadata map
- ResolvedConflict:
  - ResolvedEvents []EventWithVersion (or decision enum: KeepLocal/KeepRemote/Merge/ManualReview)
  - Reasons/Annotations (for audit & telemetry)
- ConflictResolver (Strategy):
  - Resolve(ctx, Conflict) (ResolvedConflict, error)

2) synckit/resolvers.go (baseline Strategy set)
- LastWriteWinsResolver (LWW)
- AdditiveMergeResolver (simple union semantics)
- ManualReviewResolver (flags conflict for human review, carries reasons)

3) synckit/spec.go (Specification predicates)
- type Spec func(Conflict) bool
- Primitives: EventTypeIs, AnyFieldIn, MetadataEq, OriginIs, VersionRelationIs (optional)
- Combinators: And(a,b), Or(a,b), Not(a)

Acceptance Criteria
- Pure, dependency-light packages; no domain leakage
- 90%+ unit coverage for specs and baseline resolvers
- Godoc for all public types

---

## Phase 2 – Dynamic Resolver (ordered rules, options, validation)

Deliverables
1) synckit/resolver.go
- type Rule struct { Matcher Spec; Resolver ConflictResolver; Name string }
- type DynamicResolver struct {
  - rules []Rule (ordered; first-match-wins)
  - fallback ConflictResolver (required)
  - logger (optional)
  - hooks (optional Observer; see below)
}
- Constructor (Uber options pattern):
  - NewDynamicResolver(opts ...Option) (*DynamicResolver, error)
  - Validates options and invariants at construction time; returns error on invalid configuration (e.g., nil fallback or nil rule fields)
- Functional options (Builder style, per Uber guide):
  - type Option interface { apply(*options) } // unexported apply method
  - WithFallback(ConflictResolver)
  - WithRule(name string, matcher Spec, resolver ConflictResolver)
  - WithEventTypeRule(name string, eventType string, resolver ConflictResolver)
  - WithLogger(l)
  - WithHooks(h Hooks) // zero value safe: nil funcs are no-ops
  - WithValidator(v Validator)
- Resolve flow (CoR): iterate rules → first Matcher(conflict)==true → rule.Resolver.Resolve(); else → fallback; if no fallback → error

2) Validation & determinism
- Validator checks (run at build/first use):
  - At least one rule or a non-nil fallback
  - No nil resolvers/specs
  - Optionally detect equivalent/overlapping rules and warn
- Document deterministic order: rules execute in insertion order; first-match-wins

3) Hooks (Observer-lite)
- type Hooks struct {
  - OnRuleMatched(conflict Conflict, rule Rule)
  - OnResolved(conflict Conflict, result ResolvedConflict)
  - OnFallback(conflict Conflict)
  - OnError(conflict Conflict, err error)
}

Acceptance Criteria
- Constructor validates and returns error on invalid configuration (e.g., missing fallback, nil matcher/resolver)
- Compile-time interface compliance checks present (e.g., var _ ConflictResolver = (*LastWriteWinsResolver)(nil))
- 90%+ unit coverage on DynamicResolver (ordering, fallback, validation, hooks)
- Documented first-match-wins semantics and option shapes (Uber pattern)
- Benchmarks for rule dispatch (optional)

---

## Phase 3 – Core Integration (sync engine wiring)

Changes
1) synckit/sync.go
- Remove any legacy ConflictResolver interface duplication; import the new one
- When detecting a conflict, build a Conflict with available metadata (include changed fields when feasible)
- Always delegate to sm.options.ConflictResolver.Resolve()
- Apply ResolvedConflict output to local store; incorporate reasons/metrics into SyncResult

2) synckit/builder.go
- Default a DynamicResolver with LWW fallback when no resolver provided (safe default)
- Inject builder/logger into DynamicResolver when available
- Optional: builder method for registering common rules (e.g., EventType helpers)

3) synckit/manager.go
- In pull path (and push if applicable), remove null guards and rely on default resolver presence
- If ManualReviewResolver result chosen, surface a flag/result payload for UI/application to consume

Acceptance Criteria
- All existing tests green
- New integration tests demonstrating resolver dispatch, fallback, and manual review flow

---

## Phase 4 – Test Hardening (unit, fuzz, integration)

Deliverables
1) Unit tests
- resolvers_test.go: LWW, AdditiveMerge, ManualReview edge cases
- spec_test.go: all primitives/combinators (truth tables)
- resolver_test.go: ordering, overlapping rules, missing fallback, hooks invocation

2) Fuzz tests (where applicable)
- cursor/version + Conflict fuzz to ensure Resolve is deterministic and panic-free

3) Integration tests
- Multi-user and offline scenarios (branching and merge)
- Deterministic outcomes with overlapping rules due to explicit ordering
- Manual review propagation through SyncResult

Acceptance Criteria
- >90% coverage across resolver/spec packages
- Zero data races
- Deterministic outcomes across runs
- Table-driven tests with t.Run subtests; when using t.Parallel(), capture loop vars (tt := tt)
- No mutable globals; dependencies injected via options (e.g., hooks, clock)

---

## Phase 5 – Documentation & Examples

Deliverables
1) README updates
- New section: Dynamic Conflict Resolution
- Explain Strategy + CoR + Specification design
- Show code-based wiring and an outline for config-driven wiring
- Document first-match-wins and fallback requirements

2) Godoc comments
- Public API, resolvers, specs, builder options, hooks

3) Example(s)
- examples/conflict-resolution/configurable: demo app wiring a few rules
- Show optional audit/memento and logging hooks

Acceptance Criteria
- Docs accurately reflect behavior and options
- Example runs green (go run ./examples/conflict-resolution/configurable)

---

## Error & Naming Conventions
- Error wrapping uses fmt.Errorf("context: %w", err); avoid verbose "failed to ..." prefixes
- Exported sentinel errors use Err* naming; unexported package-level errors use err*
- Prefer returning errors to logging inside library code; leave logging decisions to callers
- Custom error types only when callers need errors.As/Is matching; suffix with Error

## Optional Enhancements (post-MVP)

1) Memento (Audit/Undo)
- Capture before/after Conflict/ResolvedConflict as a ResolutionMemento
- Pluggable persistence for audit trails and rollback tooling

2) Observer (Metrics/Logging)
- Export counters/timers: rule_matched, resolved_by, manual_review, duration_ms
- Provide a simple logger facade or integrate with existing logging package

3) Composite (Rule Grouping)
- Group related rules under a common Rule that internally dispatches to sub-rules
- Useful for domain modularity without polluting top-level order

4) Adapter (Domain Bindings)
- Keep go-sync-kit generic: provide adapters in app code to map domain signals into Conflict metadata/specs

5) Config Loader (Out-of-core wiring)
- Optional loader that maps YAML/JSON config → Spec/Rule/Resolver registry
- Strong schema validation + startup diagnostics; visualize active rule graph

---

## Architecture Rationale (Refactoring.Guru patterns)
- Strategy: each ConflictResolver is a small, swappable algorithm → keeps implementations decoupled and testable
- Chain of Responsibility: ordered Rule list; first-match-wins ensures deterministic dispatch and eliminates giant if/else blocks
- Specification: declarative, composable matchers prevent tangled conditionals and enable reuse/testing
- Builder/Options: simplifies construction and validation of complex resolver setups
- Optional Memento/Observer/Composite/Adapter: production hardening without bloating the core

---

## Milestones & Timeline (suggested)
- Week 1: Phase 1 (types/specs/baseline strategies) + unit tests
- Week 2: Phase 2 (DynamicResolver + options + validation + hooks) + unit tests
- Week 3: Phase 3 (integration) + initial integration tests
- Week 4: Phase 4 (fuzz + expanded integration) + performance profiling
- Week 5: Phase 5 (docs/examples) + polish, stabilize for pre-release

---

## Alignment with Architectural Goals
- Abstract Conflict Resolution Interface: ConflictResolver exposes Resolve(ctx, Conflict) (ResolvedConflict, error); strategies like LastWriteWinsResolver, AdditiveMergeResolver, ManualReviewResolver implement it (Strategy pattern).
- Dynamic Resolver Manager: DynamicResolver acts as the manager/dispatcher (aka DynamicResolverManager) maintaining an ordered registry of rules that map conflict metadata (event type, fields, origins) to strategies (Chain of Responsibility + Specification).
- Configuration Layer: A configuration layer is supported via code-based wiring now and an optional config loader later (JSON/YAML → Spec/Rule/Resolver mapping) with schema validation. This allows updates without touching core code.
- Integration with Core Sync Engine: Sync engine constructs Conflict and delegates resolution to the DynamicResolver manager instead of a fixed strategy, passing required metadata for correct selection; SyncResult carries resolution annotations/metrics.

## Definition of Done
- Deterministic, validated DynamicResolver with LWW fallback by default
- Clear public APIs, godoc, and README guidance
- Comprehensive automated tests (unit, fuzz, integration) are green
- Example demonstrates both code-based and (outline for) config-driven rule wiring
- No domain specifics baked into core; adapters/specs supply domain signals externally

This plan keeps go-sync-kit generic, extensible, and easy to reason about while delivering a powerful, production-friendly conflict resolution framework.

