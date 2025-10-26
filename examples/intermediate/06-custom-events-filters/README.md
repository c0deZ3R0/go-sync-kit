# Custom Events & Filters

Shows selective sync by event type and metadata filtering.

## Run

```bash
cd examples/intermediate/06-custom-events-filters
go run .
```

## What It Does

- Filter events by type during sync
- Filter events by custom metadata fields
- Implement custom event interfaces
- Demonstrate selective synchronization

## Use Cases

- Syncing only specific event types (e.g., orders, not analytics)
- Tenant-specific filtering
- Priority-based sync
- Reducing bandwidth for large event streams

See also: [Events and Storage](../03-events-and-storage/README.md), [HTTP Query Filtering](../../HTTP_EXAMPLES.md)
