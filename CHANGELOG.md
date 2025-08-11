# Changelog

All notable changes to Go Sync Kit will be documented in this file.

## [v0.10.0] - 2025-08-10

### 🎯 Major Features

#### PostgreSQL EventStore with LISTEN/NOTIFY
- ✨ **PostgreSQL Backend**: Complete PostgreSQL implementation of EventStore interface
- ✨ **Real-time Notifications**: PostgreSQL LISTEN/NOTIFY for instant event streaming
- ✨ **Stream-based Subscriptions**: Subscribe to events for specific aggregates
- ✨ **Global Subscriptions**: Subscribe to all events across all streams  
- ✨ **Event Type Filtering**: Subscribe to events of specific types
- ✨ **Connection Recovery**: Automatic reconnection with exponential backoff
- ✨ **Production Ready**: ACID transactions, connection pooling, prepared statements

#### Advanced Database Features
- 🔄 **JSONB Storage**: Efficient JSON storage and querying with GIN indexes
- 🔄 **Generated Columns**: Automatic stream name generation for LISTEN/NOTIFY
- 🔄 **Optimized Indexing**: B-tree and GIN indexes for maximum query performance
- 🔄 **Batch Operations**: High-performance batch inserts with transactions
- 📊 **Connection Pool Monitoring**: Real-time statistics and health checks

### 🔧 Technical Improvements

#### Real-time Event Streaming
- 🌐 **Notification Listener**: Dedicated LISTEN/NOTIFY connection management
- 🌐 **Subscription Manager**: Thread-safe subscription lifecycle management
- 🌐 **Event Filtering**: Client-side filtering by event type and aggregate
- 🌐 **Payload Parsing**: JSON notification payloads with full event metadata

#### Integration and Testing
- 🧪 **Docker Compose**: PostgreSQL test environment with automatic setup
- 🧪 **Integration Tests**: Comprehensive real-time notification testing
- 🧪 **Connection Recovery**: Automated reconnection testing
- 🧪 **Benchmark Suite**: Performance testing for high-throughput scenarios
- 🛠 **Development Tools**: Makefile with complete development workflow

### 📚 Documentation & Examples
- 📖 **Comprehensive README**: Complete PostgreSQL EventStore documentation
- 📖 **Real-time Example**: Working demonstration of LISTEN/NOTIFY features
- 📖 **Configuration Guide**: Production deployment recommendations
- 📖 **Migration Guide**: SQLite to PostgreSQL migration path
- 📖 **Troubleshooting**: Common issues and solutions

### 🔒 Production Features
- 🔐 **SSL/TLS Support**: Secure database connections
- 🔐 **Connection String Masking**: Safe logging without exposing credentials
- 🔐 **Resource Management**: Proper cleanup and connection handling
- 🔐 **Error Handling**: Detailed error types and context preservation

### 📈 Performance Optimizations
- ⚡ **Prepared Statements**: Reduced query parsing overhead
- ⚡ **Connection Pooling**: Optimized concurrent database access
- ⚡ **Batch Processing**: Efficient multi-event transactions
- ⚡ **Index Strategy**: Query-optimized database schema

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
