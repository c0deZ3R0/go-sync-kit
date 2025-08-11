# Benchmarks and Fuzzing Tests

This document describes the benchmark and fuzzing tests implemented for go-sync-kit as part of the code review requirements.

## Overview

The following benchmark and fuzzing tests have been implemented to address the code review comment:

> **Benchmarks and fuzzing**
> - Bench: transport compression thresholds vs SyncOptions.BatchSize; vector clock operations; SQLite WAL under concurrent read/write.
> - Fuzz: cursor.UnmarshalWire and gzip request parsing in createSafeRequestReader.

## Benchmark Tests

### 1. HTTP Transport Compression Benchmarks

**Location**: `transport/httptransport/benchmark_test.go`

#### BenchmarkCompressionThresholds
- **Purpose**: Benchmarks the performance of different compression thresholds against various batch sizes (SyncOptions.BatchSize)
- **Test Matrix**: 
  - Compression thresholds: 0, 512, 1024, 2048, 4096, 8192 bytes
  - Batch sizes: 10, 50, 100, 250, 500, 1000 events
- **Metrics**: Throughput (bytes/operation) and compression effectiveness

#### BenchmarkCompressionRatio
- **Purpose**: Measures compression ratios for different payload types
- **Test Types**: Repetitive data (high compression), random data (low compression), mixed data
- **Metrics**: Compression ratio, bytes saved

#### BenchmarkSafeRequestReader
- **Purpose**: Benchmarks the performance of `createSafeRequestReader` with different compression scenarios
- **Test Scenarios**: Small/medium/large payloads, compressed/uncompressed
- **Metrics**: Request processing throughput

### 2. Vector Clock Operations Benchmarks

**Location**: `version/benchmark_test.go`

#### BenchmarkVectorClockOperations
- **Core Operations**: Increment, Compare, Merge, Clone, Serialization, Deserialization
- **Scale Testing**: Tests with vector clocks of different sizes (1-250 nodes)
- **Concurrent Testing**: Parallel benchmark execution for realistic load simulation

#### BenchmarkVectorClockRealWorldScenarios
- **Distributed System**: Simulates 5-node distributed system with periodic syncing
- **Event Sourcing**: Tracks causality in event-driven architecture
- **CRDT Operations**: Conflict-free replicated data type scenarios

### 3. SQLite WAL Concurrent Benchmarks

**Location**: `storage/sqlite/wal_benchmark_test.go`

#### BenchmarkSQLiteWALConcurrentReadWrite
- **Purpose**: Tests SQLite WAL performance under concurrent read/write load
- **Scenarios**: Various reader/writer combinations (1R+1W up to 10R+5W)
- **Comparison**: WAL vs non-WAL performance

#### BenchmarkSQLiteWALScaling
- **Purpose**: Tests how SQLite WAL scales with different connection pool configurations
- **Configurations**: Connection pools from 5/2 up to 100/20 (max open/max idle)

#### BenchmarkSQLiteWALTransactionSizes
- **Purpose**: Benchmarks different transaction batch sizes in WAL mode
- **Sizes**: 1, 5, 10, 25, 50, 100, 250 events per transaction

#### BenchmarkSQLiteWALReadPerformance
- **Purpose**: Specifically benchmarks read performance patterns
- **Patterns**: Sequential, random, aggregate-specific, and range reads
- **Dataset Sizes**: 1K, 5K, 10K, 25K events

## Fuzzing Tests

### 1. Cursor UnmarshalWire Fuzzing

**Location**: `cursor/fuzz_test.go`

#### FuzzUnmarshalWire
- **Purpose**: Tests robustness of `cursor.UnmarshalWire` against malformed input
- **Test Cases**: 
  - Valid cursor data (integer, bytes, string)
  - Edge cases (empty, null, unknown kinds)
  - Malformed JSON and invalid data types
- **Safety**: Ensures no panics on arbitrary input

#### FuzzMarshalWire
- **Purpose**: Tests round-trip behavior of MarshalWire/UnmarshalWire
- **Coverage**: Full uint64 range including edge cases (0, 1, max values)

#### FuzzCursorComparison
- **Purpose**: Tests cursor comparison operations for correctness
- **Properties Tested**: Reflexivity, symmetry, transitivity

### 2. Gzip Request Parsing Fuzzing

**Location**: `transport/httptransport/gzip_fuzz_test.go`

#### FuzzCreateSafeRequestReader
- **Purpose**: Tests robustness of `createSafeRequestReader` against malformed gzip input
- **Attack Vectors**:
  - Malformed gzip headers
  - Invalid compression data
  - Size bomb attacks
  - Content-type violations

#### FuzzGzipDecompression
- **Purpose**: Specifically tests gzip decompression logic
- **Test Cases**: Valid gzip, truncated headers, corrupted data, empty payloads

#### FuzzCompressionSizeLimits
- **Purpose**: Tests size limit enforcement in gzip processing
- **Validation**: Ensures compressed and decompressed size limits are properly enforced

#### FuzzMaxDecompressedReader
- **Purpose**: Tests the `maxDecompressedReader` component specifically
- **Coverage**: Various data sizes and limits to ensure proper bounds checking

## Running the Tests

### Benchmarks

```bash
# Run all benchmarks
go test -bench=. -benchmem ./transport/httptransport ./version ./storage/sqlite

# Run specific benchmark suites
go test -bench=BenchmarkCompressionThresholds -benchmem ./transport/httptransport
go test -bench=BenchmarkVectorClockOperations -benchmem ./version
go test -bench=BenchmarkSQLiteWAL -benchmem ./storage/sqlite

# Run benchmarks with CPU profiling
go test -bench=. -cpuprofile=cpu.prof ./transport/httptransport
go tool pprof cpu.prof
```

### Fuzzing

```bash
# Run fuzzing tests (Go 1.18+)
go test -fuzz=FuzzUnmarshalWire ./cursor
go test -fuzz=FuzzCreateSafeRequestReader ./transport/httptransport

# Run fuzzing for specific duration
go test -fuzz=FuzzUnmarshalWire -fuzztime=30s ./cursor

# Generate fuzzing coverage report
go test -fuzz=FuzzUnmarshalWire -fuzztime=10s -test.fuzzcachedir=/tmp/fuzz ./cursor
```

## Performance Characteristics

### Expected Benchmark Results

1. **Compression Thresholds**:
   - Lower thresholds show better compression ratios but higher CPU usage
   - Optimal threshold varies by payload size and type
   - Batch size impacts both compression effectiveness and throughput

2. **Vector Clock Operations**:
   - Operations scale linearly with vector clock size
   - Merge operations are most expensive
   - Serialization overhead increases with clock complexity

3. **SQLite WAL**:
   - WAL mode shows significant improvement for concurrent reads
   - Write performance scales with connection pool size
   - Transaction batching improves throughput substantially

### Fuzzing Coverage

The fuzzing tests are designed to achieve:
- **Edge case coverage**: Test boundary conditions and error paths
- **Security validation**: Prevent panic-based DoS attacks
- **Data integrity**: Ensure round-trip correctness under all conditions
- **Resource safety**: Validate size limits and resource exhaustion protection

## Integration with CI/CD

These tests should be integrated into the continuous integration pipeline:

```yaml
# Example GitHub Actions workflow
- name: Run Benchmarks
  run: |
    go test -bench=. -benchmem -timeout=10m ./...
    
- name: Run Fuzzing
  run: |
    go test -fuzz=. -fuzztime=60s ./cursor ./transport/httptransport
```

## Monitoring and Alerting

Consider setting up performance regression detection:
- Baseline benchmark results in CI
- Alert on >20% performance degradation
- Track compression ratio changes over time
- Monitor WAL checkpoint frequency and size

## Contributing

When adding new functionality:
1. Add corresponding benchmark tests for performance-critical paths
2. Add fuzzing tests for input parsing and validation logic
3. Ensure benchmarks run in reasonable time (< 30s each)
4. Verify fuzzing tests don't produce excessive false positives
