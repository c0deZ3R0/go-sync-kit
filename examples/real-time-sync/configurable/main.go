package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
	sync "github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/transport/httptransport"
	"github.com/c0deZ3R0/go-sync-kit/version"
)

//go:embed resolver_config.yaml
var cfgYAML []byte

func main() {
	fmt.Println("\n🚨 VECTOR CLOCK ANALYSIS:")
	fmt.Println("The issue is that the sync system is using SEQUENCE NUMBERS for sync decisions,")
	fmt.Println("but VECTOR CLOCKS for conflict detection. This creates a mismatch:")
	fmt.Println("")
	fmt.Println("1. Both Alice & Bob start from sequence 1 (shared baseline)")
	fmt.Println("2. Alice creates event at sequence 2 (vector clock: alice=1, sequence=2)")
	fmt.Println("3. Bob creates event at sequence 2 (vector clock: bob=1, sequence=2)")
	fmt.Println("4. Alice pushes first (server sequence becomes 2)")
	fmt.Println("5. Bob tries to sync: server version=2, his latest=2, so no events to push!")
	fmt.Println("")
	fmt.Println("The vector clocks ARE concurrent (alice=1 || bob=1), but the sync manager")
	fmt.Println("never compares them because it uses sequence numbers to filter events.")
	fmt.Println("")
	fmt.Println("This is a fundamental architectural issue - the sync layer needs to be")
	fmt.Println("vector-clock-aware, not just the conflict detection layer.")
	fmt.Println("")
	fmt.Println("Let's run the test to confirm this analysis...")
	fmt.Println("")
	
	// Create standard logger for SQLite (which expects *log.Logger)
	stdLogger := log.New(os.Stdout, "[VectorClockDemo] ", log.LstdFlags)
	
	// Create slog logger for components that expect *slog.Logger
	slogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	ctx := context.Background()

	// 1) Start server (SQLite + HTTP SyncHandler)
	serverStore := mustVersionedStore("server", "file:demo_server.db", stdLogger)
	defer serverStore.Close()

	serverHandler := httptransport.NewSyncHandler(serverStore, slogger, nil, nil)
	srv := &http.Server{Addr: ":8088", Handler: serverHandler}
	go func() {
		slogger.Info("sync server listening", "addr", srv.Addr)
		_ = srv.ListenAndServe()
	}()
	defer srv.Shutdown(ctx)

	// 2) Clients: Alice and Bob
	aliceStore := mustVersionedStore("alice", "file:demo_alice.db", stdLogger)
	defer aliceStore.Close()
	bobStore := mustVersionedStore("bob", "file:demo_bob.db", stdLogger)
	defer bobStore.Close()

	// Transport (HTTP) + offline wrapper
	baseURL := "http://127.0.0.1:8088"
	
	// Create version parser function that uses our versioned store's ParseVersion method
	versionParser := func(ctx context.Context, versionStr string) (sync.Version, error) {
		return serverStore.ParseVersion(ctx, versionStr)
	}
	
	aliceHTTP := httptransport.NewTransport(baseURL, nil, versionParser, nil)
	bobHTTP := httptransport.NewTransport(baseURL, nil, versionParser, nil)

	aliceNet := NewOfflineTransport(aliceHTTP)
	bobNet := NewOfflineTransport(bobHTTP)

	// 3) Build resolver (pick code or config) 
	// Use code-based resolver to see ManualReviewResolver in action
	resolver, err := buildResolverCode(slogger)
	// resolver, err := buildResolverFromConfig(slogger, cfgYAML)
	if err != nil {
		slogger.Error("resolver build failed", "error", err)
		return
	}

	opts := &sync.SyncOptions{
		BatchSize:        32,
		SyncInterval:     0,
		ConflictResolver: resolver,
	}

	aliceMgr := sync.NewSyncManager(aliceStore, aliceNet, opts, slogger)
	bobMgr := sync.NewSyncManager(bobStore, bobNet, opts, slogger)

	// 4) FORCE CONFLICT: Start both clients from same synchronized baseline
	fmt.Println("\n🔄 Step 1: Both clients sync to establish shared baseline...")
	
	// Both clients start online and sync to establish shared state
	aliceNet.SetOnline(true)
	bobNet.SetOnline(true)
	
	// Sync both to empty server to establish version 0 baseline
	mustSync("alice establishes baseline", aliceMgr, ctx)
	mustSync("bob establishes baseline", bobMgr, ctx)
	
	// Create a shared initial document on the server
	initialDoc := &DocumentEvent{
		id:          "shared-baseline-doc",
		eventType:   "document_updated",
		aggregateID: "conflict-test-doc",
		data: DocumentData{
			Title:     "Shared Baseline Document",
			Content:   "Initial shared content",
			Author:    "system",
			Status:    "draft", // Both will change this!
			Tags:      []string{"baseline"},
			UpdatedAt: time.Now(),
		},
		metadata: map[string]interface{}{
			"origin": "system",
			"baseline": true,
		},
	}
	
	// Alice creates the baseline document and pushes it
	must(aliceStore.Store(ctx, initialDoc, nil))
	mustSync("alice creates shared baseline", aliceMgr, ctx)
	
	// Bob pulls the baseline document so both have same starting state
	mustSync("bob syncs shared baseline", bobMgr, ctx)
	
	fmt.Println("\n✅ Both clients now have SAME baseline: document with status='draft'")
	
	// 5) Alice goes offline and creates her change first
	aliceNet.SetOnline(false)
	fmt.Println("\n💥 Step 2: Alice goes offline and makes status change...")
	
	// Alice makes her change: draft → review
	aliceConflictEvent := &DocumentEvent{
		id:          "alice-status-conflict",
		eventType:   "document_updated",
		aggregateID: "conflict-test-doc", // SAME document as baseline
		data: DocumentData{
			Title:     "Shared Baseline Document",
			Content:   "Alice: Added review comments and improvements",
			Author:    "alice",
			Status:    "review", // Alice wants REVIEW
			Tags:      []string{"baseline", "under-review"},
			UpdatedAt: time.Now(),
		},
		metadata: map[string]interface{}{
			"origin": "alice",
			"action": "status_change",
			"from_status": "draft",
			"to_status": "review",
		},
	}
	must(aliceStore.Store(ctx, aliceConflictEvent, nil))
	fmt.Println("   Alice (offline): draft → review")
	
	// 6) Alice comes online and pushes her change
	aliceNet.SetOnline(true)
	fmt.Println("\n🔥 Step 3: Alice comes online and pushes her status=REVIEW change...")
	mustSync("alice pushes REVIEW status", aliceMgr, ctx)
	
	// 7) Now Bob goes offline and creates his conflicting change
	bobNet.SetOnline(false)
	fmt.Println("\n💥 Step 4: Bob goes offline and creates CONFLICTING status change...")
	
	// Bob makes his CONFLICTING change: draft → published (but Alice already changed it to review!)
	bobConflictEvent := &DocumentEvent{
		id:          "bob-status-conflict",
		eventType:   "document_updated",
		aggregateID: "conflict-test-doc", // SAME document as baseline!
		data: DocumentData{
			Title:     "Shared Baseline Document",
			Content:   "Bob: Ready for production deployment!",
			Author:    "bob",
			Status:    "published", // Bob wants PUBLISHED!
			Tags:      []string{"baseline", "production-ready"},
			UpdatedAt: time.Now().Add(2 * time.Second),
		},
		metadata: map[string]interface{}{
			"origin": "bob",
			"action": "status_change",
			"from_status": "draft", // Bob still thinks it's draft!
			"to_status": "published",
		},
	}
	must(bobStore.Store(ctx, bobConflictEvent, nil))
	fmt.Println("   Bob (offline): draft → published (CONFLICT with Alice's review!)")
	
	// Debug: Check what's in Bob's local store before sync
	fmt.Println("\n🔍 DEBUGGING: What events does Bob have locally?")
	bobEvents, err := bobStore.Load(ctx, nil)
	if err != nil {
		fmt.Printf("Error loading Bob's events: %v\n", err)
	} else {
		fmt.Printf("Bob has %d local events:\n", len(bobEvents))
		for i, ev := range bobEvents {
			if docEv, ok := ev.Event.(*DocumentEvent); ok {
				fmt.Printf("  Event %d: ID=%s, AggregateID=%s, Status=%s, Author=%s\n", 
					i+1, docEv.ID(), docEv.AggregateID(), docEv.data.Status, docEv.data.Author)
			} else {
				fmt.Printf("  Event %d: ID=%s, Type=%s, AggregateID=%s\n", 
					i+1, ev.Event.ID(), ev.Event.Type(), ev.Event.AggregateID())
			}
		}
	}
	
	// Bob comes online and this should create the conflict!
	bobNet.SetOnline(true)
	fmt.Println("\n💥 Step 5: Bob comes online and tries to sync his CONFLICTING change...")
	fmt.Println("   Bob should pull Alice's REVIEW status, detect conflict with his PUBLISHED status,")
	fmt.Println("   and trigger the ManualReviewResolver!")
	
	// Debug: Check what the sync manager considers "local events since version X"
	fmt.Println("\n🔍 DEBUGGING: What events would sync manager push?")
	latestVersion, err := bobStore.LatestVersion(ctx)
	if err != nil {
		fmt.Printf("Error getting Bob's latest version: %v\n", err)
	} else {
		fmt.Printf("Bob's latest local version: %v\n", latestVersion)
	}
	
	// Try to see events since different versions
	eventsFromZero, err := bobStore.Load(ctx, nil)
	if err == nil {
		fmt.Printf("Events from version 0: %d events\n", len(eventsFromZero))
	}
	
	mustSync("bob triggers STATUS CONFLICT resolution", bobMgr, ctx)
	
	// Debug: Check what's in Bob's local store after sync
	fmt.Println("\n🔍 DEBUGGING: What events does Bob have after sync?")
	bobEventsAfter, err := bobStore.Load(ctx, nil)
	if err != nil {
		fmt.Printf("Error loading Bob's events after sync: %v\n", err)
	} else {
		fmt.Printf("Bob has %d local events after sync:\n", len(bobEventsAfter))
		for i, ev := range bobEventsAfter {
			if docEv, ok := ev.Event.(*DocumentEvent); ok {
				fmt.Printf("  Event %d: ID=%s, AggregateID=%s, Status=%s, Author=%s\n", 
					i+1, docEv.ID(), docEv.AggregateID(), docEv.data.Status, docEv.data.Author)
			} else {
				fmt.Printf("  Event %d: ID=%s, Type=%s, AggregateID=%s\n", 
					i+1, ev.Event.ID(), ev.Event.Type(), ev.Event.AggregateID())
			}
		}
	}

	// 8) Final sync to see resolution
	aliceNet.SetOnline(true)
	mustSync("alice final sync", aliceMgr, ctx)
	mustSync("bob final sync", bobMgr, ctx)

	fmt.Println("\nDemo finished. Check logs above for rule matches, decisions, and reasons.")
}

func mustVersionedStore(nodeID, dsn string, logger *log.Logger) sync.EventStore {
	base, err := sqlite.New(&sqlite.Config{
		DataSourceName: dsn,
		EnableWAL:      true,
		Logger:         logger,
	})
	must(err)

	vm := version.NewVectorClockManager()
	vs, err := version.NewVersionedStore(base, nodeID, vm)
	must(err)
	return vs
}

func mustSync(label string, mgr sync.SyncManager, ctx context.Context) {
	res, err := mgr.Sync(ctx)
	if err != nil {
		fmt.Printf("%s: sync error: %v\n", label, err)
		return
	}
	fmt.Printf("%s: pushed=%d pulled=%d conflictsResolved=%d\n", label, res.EventsPushed, res.EventsPulled, res.ConflictsResolved)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
