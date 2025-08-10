# Changelog

All notable changes to Go Sync Kit will be documented in this file.

## [v0.10.0] - 2025-08-10 [PRE-RELEASE]

### 🎯 Major Features

#### Real-time SSE Transport
- ✨ **Server-Sent Events Transport**: New SSE transport for real-time event streaming
- ✨ **Cursor-Based Pagination**: Efficient, resumable streaming with cursor checkpoints
- ✨ **Subscribe-Only MVP**: Focused implementation for real-time event consumption
- ✨ **JSON Wire Format**: Cross-platform compatible event serialization
- ✨ **Hybrid Transport Usage**: Combine HTTP (Push/Pull) with SSE (Subscribe) transports

### 🔧 Technical Implementations

#### SSE Server (`transport/sse/server.go`)
- 🔄 **Streaming Handler**: HTTP handler with Server-Sent Events protocol
- 🔄 **Batch Processing**: Configurable batch sizes for optimal performance
- 🔄 **Cursor Management**: Automatic cursor progression and persistence
- 🔄 **Error Handling**: Comprehensive error management using kit's error system
- 🔄 **Event Store Integration**: Works with any `synckit.EventStore` implementation

#### SSE Client (`transport/sse/client.go`)
- 🔄 **Real-time Subscription**: Non-blocking event consumption via `Subscribe()` method
- 🔄 **Event Conversion**: Automatic JSON to `synckit.Event` transformation
- 🔄 **Connection Management**: Robust connection handling with context support
- 🔄 **Transport Interface**: Implements `synckit.Transport` (Subscribe-only MVP)
- 🔄 **Error Recovery**: Graceful handling of connection issues and timeouts

#### Enhanced Cursor Package
- ✨ **Helper Functions**: Added `NewInteger()`, `NewVector()` convenience constructors
- ✨ **Wire Marshaling**: `MustMarshalWire()` and `MustUnmarshalWire()` utilities
- 🔄 **Better Ergonomics**: Simplified cursor creation and manipulation

### 🧪 Testing & Documentation

#### Comprehensive Test Suite
- 🧪 **Integration Tests**: Full SSE server-client communication tests
- 🧪 **Mock Implementations**: `MockEventStore` for testing SSE components
- 🧪 **Example Functions**: Working examples with real event streaming
- 🧪 **Error Scenarios**: Comprehensive error handling and timeout testing

#### Documentation & Examples
- 📚 **Complete README Section**: Added SSE transport to main documentation
- 📚 **Usage Examples**: Server setup, client usage, and hybrid transport patterns
- 📚 **API Documentation**: Comprehensive SSE package documentation
- 📚 **Integration Guides**: How to combine SSE with existing HTTP transports

### 🔗 Integration Features

#### Transport Ecosystem
- 🔄 **Protocol Compatibility**: Standard SSE protocol for broad client support
- 🔄 **Cursor Resumption**: Start streaming from any cursor checkpoint
- 🔄 **Event Filtering**: Server-side event filtering and batching
- 🔄 **Real-time Notifications**: Immediate event delivery as they occur

#### Architecture Benefits
- ✨ **Clean Separation**: SSE transport doesn't complicate existing HTTP transport
- ✨ **Hybrid Usage**: Use HTTP for Push/Pull operations, SSE for real-time Subscribe
- ✨ **Scalable Design**: Supports future RealtimeSyncManager integration
- ✨ **Event Store Agnostic**: Works with SQLite, BadgerDB, and any storage backend

### 📈 Performance & Reliability

#### Streaming Efficiency
- ⚡ **Non-blocking I/O**: Asynchronous event streaming
- ⚡ **Batch Optimization**: Configurable batch sizes for network efficiency
- ⚡ **Memory Management**: Efficient buffering and cursor state management
- ⚡ **Connection Reuse**: Persistent connections for real-time streaming

#### Error Handling & Resilience
- 🔒 **Graceful Degradation**: Handles connection drops and timeouts
- 🔒 **Context Cancellation**: Proper cleanup on client disconnection
- 🔒 **Cursor Recovery**: Resume from last known cursor on reconnection
- 🔒 **Resource Management**: Prevents memory leaks and connection exhaustion

### 🚀 Future Foundation

#### Extensibility
- 🔮 **RealtimeSyncManager Ready**: Designed for future integration
- 🔮 **Authentication Hooks**: Structure ready for auth/authorization middleware
- 🔮 **Metrics Integration**: Foundation for real-time transport metrics
- 🔮 **Compression Support**: Architecture supports future compression features

### ⚠️ Pre-release Notes

- 🚧 **Subscribe-Only MVP**: Currently implements only `Subscribe()` method
- 🚧 **Simple Cursor Parsing**: Basic version parsing (suitable for MVP)
- 🚧 **No Authentication**: Basic implementation without auth (add middleware as needed)
- 🚧 **Single Connection**: Each subscription creates new connection (pool in future)

### 📦 New Files

- `transport/sse/server.go` - SSE server implementation
- `transport/sse/client.go` - SSE client implementation  
- `transport/sse/types.go` - Shared JSON serialization types
- `transport/sse/example_test.go` - Tests and integration examples
- `transport/sse/README.md` - Complete package documentation

---

## [v0.9.0] - 2025-08-09

### 🎯 Major Features

#### SQLite Production Defaults
- ✨ **WAL Mode by Default**: SQLite now enables WAL mode automatically for better concurrency
- ✨ **Connection Pool Management**: Sensible defaults (max open: 25, max idle: 5)
- ✨ **Connection Lifetimes**: Automatic connection management (1 hour max, 5 minutes idle)
- 📚 **Enhanced Documentation**: Comprehensive SQLite configuration guidance

#### HTTP Transport Security & Compression
- ✨ **Automatic Compression**: Gzip compression for payloads >1KB
- 🔒 **Security Hardening**: Protection against zip bombs and decompression attacks
- 🔒 **Size Limits**: Configurable request/response limits with separate compression controls
- 🔒 **Content Validation**: Strict Content-Type validation and error mapping
- ✨ **Client Compression**: Intelligent compression with size limit enforcement

#### Comprehensive Testing
- 🧪 **WAL Integration Tests**: Concurrent write scenarios and production validation
- 🧪 **Compression Test Suite**: Size limit enforcement and attack prevention
- 🧪 **HTTP Transport Tests**: End-to-end security and performance validation
- 🧪 **Error Handling Tests**: Comprehensive error mapping and status code validation

### 🔧 Technical Improvements

#### HTTP Transport Enhancements
- 🔄 **Consistent Error Mapping**: HTTP status codes properly mapped to error types
- 🔄 **Server Configuration**: Enhanced ServerOptions with validation
- 🔄 **Client Options**: New ClientOptions with compression and limit controls
- 🔄 **Request Validation**: Improved Content-Type and size validation

#### SQLite Storage Improvements
- 🔄 **Default Configuration**: Production-ready defaults applied automatically
- 🔄 **Connection Management**: Improved pool configuration and lifetime handling
- 🔄 **WAL Mode**: Enabled by default with proper fallback handling
- 🔄 **Documentation**: Clear guidance for production deployments

### 🛠 Breaking Changes
- ⚠️ **SQLite WAL Mode**: Now enabled by default (was DELETE mode)
- ⚠️ **Connection Pools**: Now enforced by default with sensible limits
- ⚠️ **HTTP Limits**: Size limits now enforced by default for security

### 🐛 Bug Fixes
- 🐛 **HTTP Error Messages**: Fixed test expectations for Go's standard error messages
- 🐛 **Compression Edge Cases**: Proper handling of malformed compressed data
- 🐛 **Connection Pool Stats**: Fixed access to correct database connection metrics
- 🐛 **Test Race Conditions**: Resolved timing issues in integration tests

### 📈 Performance
- ⚡ **WAL Mode**: Better read/write concurrency with SQLite
- ⚡ **Connection Pooling**: Optimized database connection usage
- ⚡ **Compression**: Reduced network overhead for large payloads
- ⚡ **HTTP Transport**: Improved request/response handling efficiency

### 🔒 Security
- 🔐 **Zip Bomb Protection**: Prevents decompression attacks
- 🔐 **Size Limit Enforcement**: Configurable limits for all data transfers
- 🔐 **Input Validation**: Enhanced validation for all HTTP inputs
- 🔐 **Error Sanitization**: Consistent error handling without information leakage

### 📚 Documentation
- 📖 **Updated README**: Comprehensive v0.9.0 feature documentation
- 📖 **SQLite Guide**: Production deployment recommendations
- 📖 **HTTP Transport**: Security and compression configuration examples
- 📖 **Integration Tests**: Examples of proper testing practices

---

## [v0.8.0] - 2025-08-08

### Added
- ✨ Enhanced error handling with comprehensive error mapping
- ✨ HTTP transport improvements with better status code handling
- ✨ Client-side compression with configurable thresholds

---

## [v0.7.1] - 2025-08-08

### Added
- ✨ **HTTP I/O Hardening**: Request/response size limits, compression, timeouts
- ✨ **Server Version Parser**: Inject custom version parsers into HTTP transport
- 📚 **BadgerDB Documentation**: Comprehensive guide for BadgerDB storage
- 🚧 **Conflict Resolution Demo**: New example (work in progress)

### Enhanced
- 🔒 HTTP transport security and stability
- 📚 Transport package documentation consistency
- 🧪 HTTP transport test coverage

### Fixed
- 🐛 Event serialization and state synchronization
- 🐛 Client state initialization

## [v0.6.0] - 2025-08-06

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
