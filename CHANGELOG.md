# Changelog

All notable changes to Go Sync Kit will be documented in this file.

## [v0.5.0] - 2025-08-06

### Added
- ✨ **Real-time Event Terminal**: Live event monitoring with metadata display
- ✨ **Web Dashboard**: Real-time monitoring UI with metrics and event log
- ✨ **Basic Example**: Complete example showcasing core functionality
- 🔄 **Vector Clock Versioning**: Complete implementation with VersionedStore decorator
- 🌐 **HTTP Transport**: Production-ready HTTP transport with context support
- 📊 **Metrics System**: Built-in metrics collection for sync operations
- 🛠 **Builder Pattern**: New builder methods for configuration and validation

### Enhanced
- 🔒 **Context Support**: Comprehensive context handling with timeouts and cancellation
- 🚀 **Error System**: New error handling with codes and metadata
- ⚡ **Vector Clocks**: Enhanced implementation with validation and safety limits
- 📈 **Performance**: Optimized batch processing and sync operations
- 🧪 **Testing**: Extended test coverage to over 90%

### Fixed
- 🐛 Race conditions in real-time sync implementation
- 🐛 Exponential backoff delay calculation
- 🐛 Metrics collector initialization
- 🐛 Mock transport since parameter handling

### Changed
- 🔄 Improved event versioning system
- 🔄 Enhanced conflict resolution strategies
- 🔄 Optimized real-time sync operations
- 🔄 Better error handling and reporting

### Security
- 🔒 Added safety limits to prevent resource exhaustion
- 🔒 Improved thread safety in concurrent operations
- 🔒 Enhanced validation for sync parameters

## [v0.4.0] - 2025-07-15

### Added
- ✨ Vector clock implementation for distributed systems
- ✨ VersionedStore decorator with pluggable strategies
- ✨ VectorClockManager with automatic causal ordering
- 🧪 Comprehensive test suite with 92.3% coverage

### Enhanced
- 🔒 Thread-safe operations with proper synchronization
- 🔄 Improved event versioning
- 📈 Better performance for large event sets

### Fixed
- 🐛 Various concurrency issues
- 🐛 Event ordering bugs
- 🐛 Version comparison edge cases

## [v0.3.0] - 2025-06-30

Initial public release with basic functionality.

### Added
- ✨ Basic event synchronization
- ✨ SQLite storage backend
- ✨ Simple HTTP transport
- ✨ Last-write-wins conflict resolution
- 📚 Initial documentation
