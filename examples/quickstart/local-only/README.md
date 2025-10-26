# Quickstart: Local Only

Local store with null transport. No network needed.

## Run

```bash
cd examples/quickstart/local-only
go run .
```

## What It Does

- In-memory store (`memstore`)
- Null transport (local-only, no network)
- Basic event creation and sync
- Simplest possible go-sync-kit setup

Perfect for understanding core concepts without external dependencies.

## Next Steps

Once you understand local-only sync, try:
- [HTTP Client Quickstart](../http-client/README.md) - Add network sync
- [In-Memory Example](../../inmem/README.md) - Similar pattern with more details
