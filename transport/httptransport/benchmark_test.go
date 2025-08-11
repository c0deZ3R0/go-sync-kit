package httptransport

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// BenchmarkCompressionThresholds benchmarks the performance of different compression thresholds
// against various batch sizes, as mentioned in the code review
func BenchmarkCompressionThresholds(b *testing.B) {
	// Different compression thresholds to test
	compressionThresholds := []int64{0, 512, 1024, 2048, 4096, 8192}
	// Different batch sizes to test (SyncOptions.BatchSize)
	batchSizes := []int{10, 50, 100, 250, 500, 1000}

	for _, threshold := range compressionThresholds {
		for _, batchSize := range batchSizes {
			b.Run(fmt.Sprintf("Threshold_%dB_BatchSize_%d", threshold, batchSize), func(b *testing.B) {
				benchmarkCompressionThreshold(b, threshold, batchSize)
			})
		}
	}
}

func benchmarkCompressionThreshold(b *testing.B, compressionThreshold int64, batchSize int) {
	// Create mock store
	store := NewMockEventStore()
	
	// Create server with specific compression threshold
	serverOpts := &ServerOptions{
		MaxRequestSize:       10 * 1024 * 1024, // 10MB
		MaxDecompressedSize:  20 * 1024 * 1024, // 20MB
		CompressionEnabled:   true,
		CompressionThreshold: compressionThreshold,
	}
	
	handler := NewSyncHandler(store, slog.Default(), nil, serverOpts)
	
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(handler.handlePush))
	defer server.Close()
	
	// Generate test events for the batch
	events := generateTestEvents(batchSize)
	
	// Marshal events to JSON to get the actual payload size
	data, err := json.Marshal(events)
	if err != nil {
		b.Fatal(err)
	}
	
	b.ResetTimer()
	b.SetBytes(int64(len(data))) // Set bytes processed per operation
	
	// Benchmark the push operation
	for i := 0; i < b.N; i++ {
		// Test uncompressed request
		req, err := http.NewRequest(http.MethodPost, server.URL, bytes.NewReader(data))
		if err != nil {
			b.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			b.Fatalf("Expected 200, got %d", resp.StatusCode)
		}
	}
}

// BenchmarkCompressionRatio benchmarks the compression ratio and performance
// for different payload sizes and content types
func BenchmarkCompressionRatio(b *testing.B) {
	payloadTypes := []struct {
		name     string
		dataFunc func(size int) []JSONEventWithVersion
	}{
		{"Repetitive", generateRepetitiveEvents},
		{"Random", generateRandomEvents},
		{"Mixed", generateMixedEvents},
	}
	
	payloadSizes := []int{100, 500, 1000, 2000}
	
	for _, payloadType := range payloadTypes {
		for _, size := range payloadSizes {
			b.Run(fmt.Sprintf("%s_%devents", payloadType.name, size), func(b *testing.B) {
				benchmarkCompressionRatio(b, payloadType.dataFunc, size)
			})
		}
	}
}

func benchmarkCompressionRatio(b *testing.B, dataFunc func(int) []JSONEventWithVersion, eventCount int) {
	events := dataFunc(eventCount)
	data, err := json.Marshal(events)
	if err != nil {
		b.Fatal(err)
	}
	
	b.ResetTimer()
	b.SetBytes(int64(len(data)))
	
	var totalCompressed int64
	var totalUncompressed int64
	
	for i := 0; i < b.N; i++ {
		// Compress the data
		var compressed bytes.Buffer
		gzWriter := gzip.NewWriter(&compressed)
		_, err := gzWriter.Write(data)
		if err != nil {
			b.Fatal(err)
		}
		gzWriter.Close()
		
		totalUncompressed += int64(len(data))
		totalCompressed += int64(compressed.Len())
	}
	
	// Report compression ratio in the benchmark name
	ratio := float64(totalCompressed) / float64(totalUncompressed)
	b.ReportMetric(ratio, "compression_ratio")
	b.ReportMetric(float64(totalUncompressed-totalCompressed), "bytes_saved")
}

// BenchmarkSafeRequestReader benchmarks the performance of createSafeRequestReader
// with different compression scenarios
func BenchmarkSafeRequestReader(b *testing.B) {
	scenarios := []struct {
		name        string
		compressed  bool
		payloadSize int
		threshold   int64
	}{
		{"Small_Uncompressed_1KB", false, 1024, 2048},
		{"Small_Compressed_1KB", true, 1024, 512},
		{"Medium_Uncompressed_10KB", false, 10*1024, 2048},
		{"Medium_Compressed_10KB", true, 10*1024, 512},
		{"Large_Uncompressed_100KB", false, 100*1024, 2048},
		{"Large_Compressed_100KB", true, 100*1024, 512},
	}
	
	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			benchmarkSafeRequestReader(b, scenario.compressed, scenario.payloadSize, scenario.threshold)
		})
	}
}

func benchmarkSafeRequestReader(b *testing.B, compressed bool, payloadSize int, threshold int64) {
	// Generate test payload
	testData := strings.Repeat("A", payloadSize)
	
	var requestBody []byte
	contentEncoding := ""
	
	if compressed {
		var buf bytes.Buffer
		gzWriter := gzip.NewWriter(&buf)
		gzWriter.Write([]byte(testData))
		gzWriter.Close()
		requestBody = buf.Bytes()
		contentEncoding = "gzip"
	} else {
		requestBody = []byte(testData)
	}
	
	serverOpts := &ServerOptions{
		MaxRequestSize:      10 * 1024 * 1024, // 10MB
		MaxDecompressedSize: 20 * 1024 * 1024, // 20MB
		CompressionThreshold: threshold,
	}
	
	b.ResetTimer()
	b.SetBytes(int64(len(requestBody)))
	
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		if contentEncoding != "" {
			req.Header.Set("Content-Encoding", contentEncoding)
		}
		
		w := httptest.NewRecorder()
		
		reader, cleanup, err := createSafeRequestReader(w, req, serverOpts)
		if err != nil {
			b.Fatal(err)
		}
		
		// Read all data to benchmark the full pipeline
		_, err = bytes.NewBuffer(nil).ReadFrom(reader)
		cleanup()
		
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Helper functions for generating test data

func generateTestEvents(count int) []JSONEventWithVersion {
	events := make([]JSONEventWithVersion, count)
	for i := 0; i < count; i++ {
		events[i] = JSONEventWithVersion{
			Event: JSONEvent{
				ID:          fmt.Sprintf("event-%d", i),
				Type:        fmt.Sprintf("EventType%d", i%5),
				AggregateID: fmt.Sprintf("aggregate-%d", i%10),
				Data:        fmt.Sprintf("Event data for event %d with some variable content", i),
				Metadata: map[string]interface{}{
					"timestamp": fmt.Sprintf("2024-01-01T%02d:%02d:00Z", i%24, i%60),
					"source":    fmt.Sprintf("service-%d", i%3),
					"sequence":  i,
				},
			},
			Version: fmt.Sprintf("%d", i+1),
		}
	}
	return events
}

func generateRepetitiveEvents(count int) []JSONEventWithVersion {
	events := make([]JSONEventWithVersion, count)
	// Repetitive data compresses very well
	repeatData := "This is repetitive data that will compress very well when using gzip compression. "
	for i := 0; i < count; i++ {
		events[i] = JSONEventWithVersion{
			Event: JSONEvent{
				ID:          fmt.Sprintf("repetitive-%d", i),
				Type:        "RepetitiveEvent",
				AggregateID: "repetitive-aggregate",
				Data:        repeatData + repeatData + repeatData, // Triple it for more repetition
				Metadata: map[string]interface{}{
					"index": i,
					"type":  "repetitive",
				},
			},
			Version: fmt.Sprintf("%d", i+1),
		}
	}
	return events
}

func generateRandomEvents(count int) []JSONEventWithVersion {
	events := make([]JSONEventWithVersion, count)
	// Random data compresses poorly
	for i := 0; i < count; i++ {
		events[i] = JSONEventWithVersion{
			Event: JSONEvent{
				ID:          fmt.Sprintf("random-%d-%d", i, i*7919%1000), // Pseudo-random
				Type:        fmt.Sprintf("RandomEvent%d", i*31%17),
				AggregateID: fmt.Sprintf("random-agg-%d", i*13%23),
				Data: fmt.Sprintf("Random data %d with hash %d and value %d",
					i, i*7919, i*31*13),
				Metadata: map[string]interface{}{
					"random_value": i * 7919 % 1000,
					"hash":         i * 31 % 17,
					"computed":     i*13%23 + i*7%11,
				},
			},
			Version: fmt.Sprintf("%d", i+1),
		}
	}
	return events
}

func generateMixedEvents(count int) []JSONEventWithVersion {
	events := make([]JSONEventWithVersion, count)
	// Mix of repetitive and random data
	repetitiveData := "Common data that appears frequently in events and compresses well. "
	for i := 0; i < count; i++ {
		var data string
		if i%3 == 0 {
			// Every third event is repetitive
			data = repetitiveData + repetitiveData
		} else {
			// Other events are more random
			data = fmt.Sprintf("Unique data for event %d with timestamp %d", i, i*7919)
		}
		
		events[i] = JSONEventWithVersion{
			Event: JSONEvent{
				ID:          fmt.Sprintf("mixed-%d", i),
				Type:        fmt.Sprintf("MixedEvent%d", i%7),
				AggregateID: fmt.Sprintf("mixed-agg-%d", i%15),
				Data:        data,
				Metadata: map[string]interface{}{
					"index":      i,
					"is_common":  i%3 == 0,
					"batch":      i / 10,
				},
			},
			Version: fmt.Sprintf("%d", i+1),
		}
	}
	return events
}
