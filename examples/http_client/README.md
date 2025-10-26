# HTTP Client

Minimal client that syncs against a server at `http://localhost:8080/sync`.

## Run

```bash
cd examples/http_client
go run .
```

**Tip**: Start the simple server in another terminal first:

```bash
cd examples/http_server
go run main.go
```

## What It Does

- Connects to HTTP sync endpoint
- Pushes/pulls events via HTTP transport
- Demonstrates basic client-server sync pattern

See also: [HTTP Server](../http_server/README.md), [HTTP Examples](../HTTP_EXAMPLES.md)
