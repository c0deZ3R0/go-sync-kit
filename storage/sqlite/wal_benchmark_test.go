package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// BenchmarkSQLiteWALConcurrentReadWrite benchmarks SQLite WAL mode under concurrent read/write load
// as mentioned in the code review
func BenchmarkSQLiteWALConcurrentReadWrite(b *testing.B) {
	scenarios := []struct {
		name        string
		numReaders  int
		numWriters  int
		enableWAL   bool
		batchSize   int
	}{
		{"WAL_1Reader_1Writer", 1, 1, true, 10},
		{"WAL_2Readers_1Writer", 2, 1, true, 10},
		{"WAL_4Readers_2Writers", 4, 2, true, 10},
		{"WAL_8Readers_4Writers", 8, 4, true, 10},
		{"WAL_10Readers_5Writers", 10, 5, true, 10},
		{"NoWAL_1Reader_1Writer", 1, 1, false, 10},
		{"NoWAL_2Readers_1Writer", 2, 1, false, 10},
	}
	
	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			benchmarkConcurrentReadWrite(b, scenario.numReaders, scenario.numWriters, scenario.enableWAL, scenario.batchSize)
		})
	}
}

func benchmarkConcurrentReadWrite(b *testing.B, numReaders, numWriters int, enableWAL bool, batchSize int) {
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "benchmark.db")
	
	config := &Config{
		DataSourceName:  dbPath,
		EnableWAL:       enableWAL,
		MaxOpenConns:    numReaders + numWriters + 5, // Extra connections for overhead
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 5 * time.Minute,
	}
	
	store, err := New(config)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	
	ctx := context.Background()
	
	// Pre-populate some data for readers
	for i := 0; i < 100; i++ {
		event := &MockEvent{
			id:          fmt.Sprintf("initial-%d", i),
			eventType:   "InitialEvent",
			aggregateID: fmt.Sprintf("initial-agg-%d", i%10),
			data:        fmt.Sprintf("Initial data %d", i),
		}
		err := store.Store(ctx, event, cursor.IntegerCursor{Seq: uint64(i + 1)})
		if err != nil {
			b.Fatal(err)
		}
	}
	
	b.ResetTimer()
	
	var wg sync.WaitGroup
	startTime := time.Now()
	
	// Start readers
	for r := 0; r < numReaders; r++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			benchmarkReader(b, store, readerID, startTime)
		}(r)
	}
	
	// Start writers
	for w := 0; w < numWriters; w++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			benchmarkWriter(b, store, writerID, batchSize, startTime)
		}(w)
	}
	
	wg.Wait()
}

func benchmarkReader(b *testing.B, store *SQLiteEventStore, readerID int, startTime time.Time) {
	ctx := context.Background()
	readCount := 0
	
	for time.Since(startTime) < time.Duration(b.N)*time.Nanosecond {
		// Perform different types of reads
		switch readCount % 3 {
		case 0:
			// Read all events
			_, err := store.Load(ctx, cursor.IntegerCursor{Seq: 0})
			if err != nil {
				b.Error(err)
				return
			}
		case 1:
			// Read from a specific point
			_, err := store.Load(ctx, cursor.IntegerCursor{Seq: uint64(readCount % 50)})
			if err != nil {
				b.Error(err)
				return
			}
		case 2:
			// Read by aggregate
			aggregateID := fmt.Sprintf("initial-agg-%d", readCount%10)
			_, err := store.LoadByAggregate(ctx, aggregateID, cursor.IntegerCursor{Seq: 0})
			if err != nil {
				b.Error(err)
				return
			}
		}
		readCount++
	}
}

func benchmarkWriter(b *testing.B, store *SQLiteEventStore, writerID int, batchSize int, startTime time.Time) {
	ctx := context.Background()
	writeCount := 0
	
	for time.Since(startTime) < time.Duration(b.N)*time.Nanosecond {
		// Create batch of events
		events := make([]synckit.EventWithVersion, batchSize)
		baseSeq := uint64(writeCount*batchSize + writerID*1000000) // Avoid sequence conflicts
		
		for i := 0; i < batchSize; i++ {
			events[i] = synckit.EventWithVersion{
				Event: &MockEvent{
					id:          fmt.Sprintf("writer-%d-batch-%d-event-%d", writerID, writeCount, i),
					eventType:   "BenchmarkEvent",
					aggregateID: fmt.Sprintf("benchmark-agg-%d", (writeCount*batchSize+i)%25),
					data: map[string]interface{}{
						"writer_id":   writerID,
						"batch_id":    writeCount,
						"event_id":    i,
						"timestamp":   time.Now().Unix(),
						"payload":     fmt.Sprintf("Benchmark payload for writer %d batch %d event %d", writerID, writeCount, i),
					},
				},
				Version: cursor.IntegerCursor{Seq: baseSeq + uint64(i)},
			}
		}
		
		// Write batch
		err := store.StoreBatch(ctx, events)
		if err != nil {
			b.Error(err)
			return
		}
		
		writeCount++
	}
}

// BenchmarkSQLiteWALScaling benchmarks how SQLite WAL performance scales with different configurations
func BenchmarkSQLiteWALScaling(b *testing.B) {
	connectionConfigs := []struct {
		name         string
		maxOpenConns int
		maxIdleConns int
	}{
		{"Conns_5_2", 5, 2},
		{"Conns_10_5", 10, 5},
		{"Conns_25_5", 25, 5},
		{"Conns_50_10", 50, 10},
		{"Conns_100_20", 100, 20},
	}
	
	for _, connConfig := range connectionConfigs {
		b.Run(connConfig.name, func(b *testing.B) {
			benchmarkWALScaling(b, connConfig.maxOpenConns, connConfig.maxIdleConns)
		})
	}
}

func benchmarkWALScaling(b *testing.B, maxOpenConns, maxIdleConns int) {
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "scaling.db")
	
	config := &Config{
		DataSourceName:  dbPath,
		EnableWAL:       true,
		MaxOpenConns:    maxOpenConns,
		MaxIdleConns:    maxIdleConns,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 5 * time.Minute,
	}
	
	store, err := New(config)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	
	ctx := context.Background()
	
	b.ResetTimer()
	b.SetBytes(1024) // Approximate size per operation
	
	b.RunParallel(func(pb *testing.PB) {
		eventCounter := 0
		for pb.Next() {
			event := &MockEvent{
				id:          fmt.Sprintf("scaling-event-%d-%d", maxOpenConns, eventCounter),
				eventType:   "ScalingEvent",
				aggregateID: fmt.Sprintf("scaling-agg-%d", eventCounter%10),
				data:        fmt.Sprintf("Scaling test data for event %d with max connections %d", eventCounter, maxOpenConns),
			}
			
			err := store.Store(ctx, event, cursor.IntegerCursor{Seq: uint64(eventCounter + 1)})
			if err != nil {
				b.Error(err)
				return
			}
			eventCounter++
		}
	})
}

// BenchmarkSQLiteWALTransactionSizes benchmarks different transaction sizes in WAL mode
func BenchmarkSQLiteWALTransactionSizes(b *testing.B) {
	transactionSizes := []int{1, 5, 10, 25, 50, 100, 250}
	
	for _, txSize := range transactionSizes {
		b.Run(fmt.Sprintf("TxSize_%d", txSize), func(b *testing.B) {
			benchmarkWALTransactionSize(b, txSize)
		})
	}
}

func benchmarkWALTransactionSize(b *testing.B, transactionSize int) {
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "transaction.db")
	
	config := DefaultConfig(dbPath)
	config.EnableWAL = true
	
	store, err := New(config)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	
	ctx := context.Background()
	
	b.ResetTimer()
	b.SetBytes(int64(transactionSize * 512)) // Approximate bytes per transaction
	
	for i := 0; i < b.N; i++ {
		// Create batch of events
		events := make([]synckit.EventWithVersion, transactionSize)
		baseSeq := uint64(i * transactionSize)
		
		for j := 0; j < transactionSize; j++ {
			events[j] = synckit.EventWithVersion{
				Event: &MockEvent{
					id:          fmt.Sprintf("tx-event-%d-%d", i, j),
					eventType:   "TransactionEvent",
					aggregateID: fmt.Sprintf("tx-agg-%d", (i*transactionSize+j)%15),
					data:        fmt.Sprintf("Transaction test data for batch %d event %d", i, j),
				},
				Version: cursor.IntegerCursor{Seq: baseSeq + uint64(j)},
			}
		}
		
		// Store batch in single transaction
		err := store.StoreBatch(ctx, events)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSQLiteWALCheckpointBehavior benchmarks behavior around WAL checkpoint operations
func BenchmarkSQLiteWALCheckpointBehavior(b *testing.B) {
	scenarios := []struct {
		name              string
		eventsBeforeRead  int
		readFrequency     int
		checkpointFreq    int
	}{
		{"Frequent_Checkpoints", 1000, 50, 100},
		{"Moderate_Checkpoints", 2000, 100, 500},
		{"Infrequent_Checkpoints", 5000, 250, 1000},
	}
	
	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			benchmarkWALCheckpointBehavior(b, scenario.eventsBeforeRead, scenario.readFrequency, scenario.checkpointFreq)
		})
	}
}

func benchmarkWALCheckpointBehavior(b *testing.B, eventsBeforeRead, readFrequency, checkpointFreq int) {
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "checkpoint.db")
	
	config := DefaultConfig(dbPath)
	config.EnableWAL = true
	
	store, err := New(config)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	
	ctx := context.Background()
	
	// Pre-populate to simulate realistic WAL size
	for i := 0; i < eventsBeforeRead; i++ {
		event := &MockEvent{
			id:          fmt.Sprintf("prep-event-%d", i),
			eventType:   "PrepEvent",
			aggregateID: fmt.Sprintf("prep-agg-%d", i%20),
			data:        fmt.Sprintf("Preparation data %d with extra content to increase WAL size", i),
		}
		err := store.Store(ctx, event, cursor.IntegerCursor{Seq: uint64(i + 1)})
		if err != nil {
			b.Fatal(err)
		}
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Write an event
		event := &MockEvent{
			id:          fmt.Sprintf("checkpoint-event-%d", i),
			eventType:   "CheckpointEvent",
			aggregateID: fmt.Sprintf("checkpoint-agg-%d", i%25),
			data:        fmt.Sprintf("Checkpoint benchmark data %d", i),
		}
		
		err := store.Store(ctx, event, cursor.IntegerCursor{Seq: uint64(eventsBeforeRead + i + 1)})
		if err != nil {
			b.Fatal(err)
		}
		
		// Periodic reads
		if i%readFrequency == 0 {
			_, err := store.Load(ctx, cursor.IntegerCursor{Seq: uint64(eventsBeforeRead + i - readFrequency/2)})
			if err != nil {
				b.Fatal(err)
			}
		}
		
		// Simulate checkpoint behavior (SQLite handles this automatically, but we can trigger reads that might be affected)
		if i%checkpointFreq == 0 {
			// Perform a larger read operation that might benefit from checkpointing
			_, err := store.LoadByAggregate(ctx, fmt.Sprintf("checkpoint-agg-%d", i%25), cursor.IntegerCursor{Seq: 0})
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

// BenchmarkSQLiteWALReadPerformance specifically benchmarks read performance under WAL mode
func BenchmarkSQLiteWALReadPerformance(b *testing.B) {
	datasetSizes := []int{1000, 5000, 10000, 25000}
	readPatterns := []struct {
		name string
		fn   func(*testing.B, *SQLiteEventStore, int)
	}{
		{"SequentialReads", benchmarkSequentialReads},
		{"RandomReads", benchmarkRandomReads},
		{"AggregateReads", benchmarkAggregateReads},
		{"RangeReads", benchmarkRangeReads},
	}
	
	for _, datasetSize := range datasetSizes {
		for _, pattern := range readPatterns {
			b.Run(fmt.Sprintf("%s_Dataset_%d", pattern.name, datasetSize), func(b *testing.B) {
				store := setupWALStoreWithData(b, datasetSize)
				defer store.Close()
				
				b.ResetTimer()
				pattern.fn(b, store, datasetSize)
			})
		}
	}
}

func setupWALStoreWithData(b *testing.B, datasetSize int) *SQLiteEventStore {
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "read_perf.db")
	
	config := DefaultConfig(dbPath)
	config.EnableWAL = true
	
	store, err := New(config)
	if err != nil {
		b.Fatal(err)
	}
	
	ctx := context.Background()
	
	// Populate with test data
	events := make([]synckit.EventWithVersion, datasetSize)
	for i := 0; i < datasetSize; i++ {
		events[i] = synckit.EventWithVersion{
			Event: &MockEvent{
				id:          fmt.Sprintf("read-perf-event-%d", i),
				eventType:   fmt.Sprintf("ReadPerfEvent%d", i%10),
				aggregateID: fmt.Sprintf("read-perf-agg-%d", i%100),
				data: map[string]interface{}{
					"index":     i,
					"timestamp": time.Now().Unix(),
					"payload":   fmt.Sprintf("Read performance test data for event %d", i),
					"category":  i % 10,
					"batch":     i / 100,
				},
			},
			Version: cursor.IntegerCursor{Seq: uint64(i + 1)},
		}
	}
	
	// Use batch insert for better performance during setup
	err = store.StoreBatch(ctx, events)
	if err != nil {
		b.Fatal(err)
	}
	
	return store
}

func benchmarkSequentialReads(b *testing.B, store *SQLiteEventStore, datasetSize int) {
	ctx := context.Background()
	
	for i := 0; i < b.N; i++ {
		startSeq := uint64((i % (datasetSize / 10)) * 10)
		_, err := store.Load(ctx, cursor.IntegerCursor{Seq: startSeq})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkRandomReads(b *testing.B, store *SQLiteEventStore, datasetSize int) {
	ctx := context.Background()
	
	for i := 0; i < b.N; i++ {
		randomSeq := uint64((i * 7919) % datasetSize) // Pseudo-random access
		_, err := store.Load(ctx, cursor.IntegerCursor{Seq: randomSeq})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkAggregateReads(b *testing.B, store *SQLiteEventStore, datasetSize int) {
	ctx := context.Background()
	
	for i := 0; i < b.N; i++ {
		aggregateID := fmt.Sprintf("read-perf-agg-%d", i%100)
		_, err := store.LoadByAggregate(ctx, aggregateID, cursor.IntegerCursor{Seq: 0})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkRangeReads(b *testing.B, store *SQLiteEventStore, datasetSize int) {
	ctx := context.Background()
	
	for i := 0; i < b.N; i++ {
		startSeq := uint64((i % (datasetSize / 100)) * 100)
		_, err := store.Load(ctx, cursor.IntegerCursor{Seq: startSeq})
		if err != nil {
			b.Fatal(err)
		}
	}
}
