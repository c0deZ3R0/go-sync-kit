# Go Sync Kit Documentation

This directory contains organized technical documentation for the Go Sync Kit project.

## 📁 Directory Structure

### `/design/`
Design documents and architectural specifications:
- **POSTGRES_EVENTSTORE_DESIGN.md** - PostgreSQL EventStore with LISTEN/NOTIFY design specification

### `/implementation/`
Implementation guides and technical details:
- *Reserved for future implementation documentation*

### `/testing/`
Testing documentation, benchmarks, and quality assurance:
- **BENCHMARKS_AND_FUZZING.md** - Comprehensive benchmarking and fuzz testing documentation

## 🔍 Finding Documentation

### Core Documentation (Root Level)
- **README.md** - Main project documentation with examples and quick start
- **CHANGELOG.md** - Version history and release notes
- **WARP.md** - Development tool configuration and guidance

### Code Documentation
Most documentation is embedded within the codebase:
- **Package READMEs** - Each package contains detailed usage documentation
- **Example Applications** - `examples/` directory contains working demonstrations
- **Inline Documentation** - Go doc comments throughout the codebase

### Implementation Examples
Instead of separate implementation documentation, refer to:
- `examples/intermediate/07-structured-logging/` - Working structured logging example
- `examples/` - Progressive examples from basic to advanced usage
- Package-specific examples in each module

## 📋 Documentation Guidelines

### When to Add Documentation Here

**Add to `/docs/`:**
- Architecture and design specifications
- Cross-cutting technical decisions
- Testing strategies and benchmarks
- Migration guides
- Performance analysis

**Keep in Root:**
- User-facing documentation (README, CHANGELOG)
- Tool configuration files (WARP.md)

**Keep in Code:**
- API documentation (Go doc comments)
- Usage examples (package examples and `examples/` directory)
- Implementation details (inline comments)

### Documentation Lifecycle

1. **Design Phase** → Document in `/docs/design/`
2. **Implementation Phase** → Document in code and examples
3. **Completion** → Remove redundant design docs, keep architectural decisions

This structure ensures documentation stays current and avoids duplication between specs and actual working code.
