# go-sync-kit v0.5.0 Changelog

## Major Features
- Enhanced context handling and cancellation support throughout the library
- Advanced vector clock implementation with validation and safety limits
- Improved error handling system with error codes and metadata
- Enhanced SyncManagerBuilder with validation, timeouts, and compression options
- Built-in metrics collection for sync operations

## Reliability & Safety
- Comprehensive context handling improvements in manager and store components
- Multiple fixes for race conditions in real-time sync implementation
- Additional safety improvements in sync manager
- Enhanced error handling with better error codes and metadata
- Improved context cancellation error handling

## Performance & Efficiency
- Improved efficiency of remote version fetching in push operations
- Enhanced version comparison logic in pull operations
- Optimized exponential backoff delay calculation
- Better batch size validation and handling

## Testing & Quality
- Added comprehensive context tests
- Added context improvement tests
- Enhanced builder validation tests
- Improved mock transport functionality
- Fixed MockTransport 'since' parameter handling

## Bug Fixes
- Fixed race conditions in real-time sync implementation
- Fixed race condition in auto-sync functionality
- Fixed metrics collector initialization in NewRealtimeSyncManager
- Fixed exponential backoff delay calculation
- Fixed sync manager builder validation

## Documentation
- Updated documentation for v0.5.0 release
- Enhanced feature documentation
- Added new builder features documentation
- Improved code examples and usage patterns
- Reorganized documentation structure

## SQLite Store
- Enhanced SQLite store implementation
- Added safety improvements
- Improved error handling
- Better context support
- Optimized query performance

For full details about the release, including breaking changes and migration guides, please see the [v0.5.0 Release Notes](https://github.com/c0deZ3R0/go-sync-kit/releases/tag/v0.5.0).
