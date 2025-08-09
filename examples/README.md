# Go Sync Kit Examples

This directory contains various examples demonstrating different features and use cases of Go Sync Kit.

## Examples

### 1. [Basic Example](./basic/)
A comprehensive notes synchronization application with a real-time dashboard. This example demonstrates:
- Real-time event synchronization between client and server
- Live monitoring dashboard with metrics
- SQLite persistence with WAL mode
- Auto-sync capabilities
- Event terminal with detailed metadata

**Features:**
- Real-time web dashboard
- Event logging and monitoring
- Metrics collection and display
- Graceful shutdown handling

### 2. [Conflict Resolution](./conflict-resolution/)
An advanced example demonstrating conflict resolution in a distributed counter system. This example shows:
- Vector clock-based conflict resolution
- Multiple client synchronization
- Custom conflict resolution strategies
- Event-driven architecture
- Offline-first design

**Features:**
- Multiple client coordination
- Automatic conflict detection and resolution
- Vector clock versioning
- Comprehensive test suite
- PowerShell and batch scripts for easy testing

### 3. [Utilities](./utils/)
Collection of utility functions and examples for testing and development:
- Memory store implementations
- Mock transport for testing
- Cursor synchronization examples
- Test utilities and helpers

## Getting Started

Each example contains its own README.md with specific instructions. Generally:

1. Navigate to the example directory
2. Install dependencies: `go mod tidy`
3. Run the example: `go run .`
4. Follow the specific instructions in each example's README

## Prerequisites

- Go 1.21 or later
- SQLite (for persistence examples)

## Architecture Patterns

These examples demonstrate various architectural patterns:

- **Event Sourcing**: All examples use event-driven state management
- **CQRS**: Command Query Responsibility Segregation patterns
- **Offline-First**: Applications work offline and sync when connected
- **Vector Clocks**: Distributed versioning for conflict resolution
- **Real-time Monitoring**: Live dashboard and metrics collection

## Building Advanced Applications

These examples serve as building blocks for more complex applications. You can combine patterns from different examples to build sophisticated distributed systems with:

- Custom storage backends
- Advanced conflict resolution strategies
- Complex event routing and filtering
- Real-time collaboration features
- Multi-tenant architectures

## Contributing

When adding new examples:

1. Create a new directory under `examples/`
2. Include a comprehensive README.md
3. Add Go modules (`go.mod`, `go.sum`)
4. Include test files and scripts
5. Update this main README.md

## Support

For questions about these examples or Go Sync Kit in general, please check the main project documentation or open an issue in the repository.
