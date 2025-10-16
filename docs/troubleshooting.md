# Troubleshooting

[\u2190 Back to Documentation Index](../README.md#-documentation-index)

**TL;DR**
- Check clocks (version/cursor alignment) for conflicts
- Review logs (state machine signals) for stalls or errors
- Verify event versions and cursor positions match remote
- Confirm auth tokens and tenant headers are present
- Enable backoff; don't retry immediately on every failure

---

## Time/version conflicts

**Symptoms:**
- High conflict rate (>10% of events)
- Same event exists locally and remotely with different payloads
- Resolver rejecting events unexpectedly

**Likely causes:**
- Client clocks skewed (if using timestamps)
- Vector clock not propagated correctly across transports
- Store not enforcing idempotency (duplicate IDs with different data)
- Resolver logic doesn't match domain requirements

**Try:**
1. Verify all nodes have synchronized time (NTP)
2. Check event IDs are globally unique and stable (not timestamps)
3. Run a full pull (`node.Pull(ctx)`) to resync all events from remote
4. Inspect resolver logic: Does it handle your conflict pattern? (e.g., CRDTs, OT, domain rules)
5. Enable debug logging: `"sync_round_details"` to see conflict trace
6. If using custom resolver, add unit tests with known conflict scenarios

See [Conflict resolution strategies](./overview.md#conflict-resolution-strategies) and [examples/intermediate/04-conflict-resolution](../examples/intermediate/04-conflict-resolution).

## HTTP errors (4xx/5xx)

**Symptoms:**
- Sync fails with `400 Bad Request` or `401 Unauthorized`
- Repeated `500 Internal Server Error`
- Connection timeouts during pull/push

**Likely causes:**
- Auth token expired or invalid
- Tenant ID missing or mismatched
- Payload validation failed (schema mismatch)
- Server overloaded or down

**Try:**

**400/401/403:**
1. Check auth token is present: `curl -H "Authorization: Bearer <token>" http://server/sync`
2. Verify token hasn't expired: check token claims or server logs
3. Confirm tenant header matches if required: `curl -H "X-SyncKit-Tenant: acme" ...`
4. Check payload schema: are you sending the expected event structure?

Example error response:
```json
{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "invalid or expired token",
    "op": "pull"
  }
}
```

**500:**
1. Check server logs for panic or unhandled exception
2. Try smaller batch size (if pushing many events)
3. Verify database/store is accessible (disk space, connectivity)
4. Enable server debug logging

See [examples/http_enterprise](../examples/http_enterprise) for auth setup.

## Zero events pushed/pulled unexpectedly

**Symptoms:**
- `node.Sync()` succeeds but `EventsPulled` and `EventsPushed` are 0
- No errors, but no data transfer
- Works initially, then stops syncing

**Likely causes:**
- Local and remote versions are identical (nothing to sync)
- Cursor/version mismatch; client and server disagree on "since"
- Filter is too restrictive (tenant, event type)
- Network error silently ignored

**Try:**
1. Log the sync result:
   ```go
   result, _ := node.Sync(ctx)
   log.Printf("Pushed: %d, Pulled: %d", result.EventsPushed, result.EventsPulled)
   ```
2. Manually inspect versions:
   ```go
   local, _ := store.MaxVersion(ctx)
   remote, _ := remoteStore.MaxVersion(ctx) // or HTTP call
   log.Printf("Local: %d, Remote: %d", local, remote)
   ```
3. Check filters (tenant, event type) aren't filtering out all events
4. Verify cursor is being read/written correctly (not stuck at version 0)
5. Enable trace-level logging in store to see queries

See [Store](./overview.md#store) for version/cursor mechanics.

## High conflict rate

**Symptoms:**
- Conflict rate > 10% of events
- Logs show repeated "conflict resolved" messages
- Sync latency increases

**Likely causes:**
- Clients are stale (not pulling frequently enough)
- Multiple writers to same aggregate without proper resolution
- Resolver logic is too permissive (accepting conflicting changes)
- Clock skew between nodes

**Try:**
1. Increase sync frequency: lower `SyncInterval` (e.g., 5s instead of 30s)
2. Review resolver logic: is it actually choosing a winner, or accepting both?
3. Add structured logging to resolver:
   ```go
   log.Printf("Conflict: local=%s, remote=%s, winner=%s", local, remote, winner)
   ```
4. Check client clocks are in sync (NTP)
5. Consider domain-specific resolver (CRDT, OT) if LWW doesn't fit

See [Best Practices](./best-practices.md#observability-hooks) for alerting.

## Realtime stalls (SSE)

**Symptoms:**
- SSE connection established but no events arrive
- Events appear after reconnect
- Clients log "keep-alive timeout" or "connection idle"

**Likely causes:**
- Server isn't broadcasting new events to connected clients
- Network middleware (proxy/firewall) closing idle connections
- Client not consuming stream fast enough (backpressure)
- SSE fan-out loop crashed silently

**Try:**
1. Verify server is emitting keep-alive pings (every 30s recommended)
2. Check network proxy timeouts; increase if <5 min
3. Monitor server logs for SSE handler panics or errors
4. Verify client reconnection logic:
   ```go
   // Should auto-reconnect on close
   for {
     resp, _ := http.Get("http://server/events")
     scanner := bufio.NewScanner(resp.Body)
     for scanner.Scan() {
       process(scanner.Text())
     }
     time.Sleep(backoff()) // exponential backoff
   }
   ```
5. Test end-to-end: push event → server broadcasts → client receives

See [examples/HTTP_EXAMPLES.md](../examples/HTTP_EXAMPLES.md) for SSE setup.

## Logs to look for

**Healthy sync round:**
```
sync_round_started: mode=pull
pull_completed: events=42, duration=150ms
resolve_conflicts: count=0
push_started: events=5
push_completed: events=5, duration=200ms
sync_round_completed: total_duration=350ms, pushed=5, pulled=42, conflicts=0
```

**Problematic patterns:**
```
push_stalled: retries=3, backoff=30s    ← Alert!
conflict_resolved: count=15, rate=0.15  ← High rate, investigate
pull_timeout: duration=60000ms           ← Timeout, check network
resolver_error: code=UNSUPPORTED_TYPE    ← Check resolver logic
```

---

[\u21a9\ufe0e Back to Documentation Index](../README.md#-documentation-index)
