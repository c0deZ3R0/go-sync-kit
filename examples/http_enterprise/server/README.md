# Enterprise HTTP Server

Production-style server with authentication (Bearer/HMAC), multitenancy, rate limiting, and structured logging.

## Run

```bash
cd examples/http_enterprise/server
go run .
```

Server listens on `:8080`.

## Next Steps

Run the matching client:

```bash
cd examples/http_enterprise/client
go run .
```

## Features

- Bearer token authentication
- HMAC request signing
- Multitenancy support
- Rate limiting
- Structured error responses
- Idempotency keys

For deeper documentation, see [examples/http_enterprise/README.md](../README.md).
