# Enterprise HTTP Client

Demonstrates tokens, multitenancy, and signed requests against the enterprise server.

## Run

**Start the server** (separate terminal):

```bash
cd examples/http_enterprise/server
go run .
```

**Run the client**:

```bash
cd examples/http_enterprise/client
go run .
```

## What It Does

- Authenticates with Bearer tokens
- Uses HMAC signing for request integrity
- Demonstrates tenant isolation
- Shows idempotency key usage

**Demo tokens** (baked into example):
- `admin-token`
- `user-token`
- `globex-token`

For more details, see [examples/http_enterprise/README.md](../README.md).
