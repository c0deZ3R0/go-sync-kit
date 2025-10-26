# Quickstart: HTTP Client

Smallest possible client that talks to the simple HTTP server.

## Run

**Start the server** (separate terminal):

```bash
cd examples/http_server
go run main.go
```

**Run the client**:

```bash
cd examples/quickstart/http-client
go run .
```

## What It Does

- Minimal HTTP client setup
- Connects to `http://localhost:8080/sync`
- Demonstrates basic push/pull operations
- Perfect first step after local-only quickstart

See also: [Local-Only Quickstart](../local-only/README.md), [HTTP Server](../../http_server/README.md)
