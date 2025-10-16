# HTTP Server Examples

This directory contains two versions of the HTTP server example:

## 📚 `main.go` - Simple Version
```bash
go run main.go
```

**Best for:**
- Learning and understanding SyncNode presets
- Quick demos and getting started
- Development and testing

**Features:**
- Minimal code for clarity
- Uses `synckit.NewHTTPServerNode()` preset
- Basic HTTP server with `log.Fatal`

## 🏭 `main_production.go` - Production Version
```bash
go run main_production.go
```

**Best for:**
- Production deployments
- Docker containers
- Kubernetes pods
- Real-world applications

**Features:**
- ✅ **Graceful Shutdown**: Handles SIGINT/SIGTERM signals
- ✅ **HTTP Timeouts**: Read/write/idle timeout configuration
- ✅ **Resource Cleanup**: Proper store and node cleanup with logging
- ✅ **Signal Handling**: 30-second graceful shutdown timeout
- ✅ **Error Handling**: Distinguishes between server errors and shutdown

## 🔄 Usage in Docker

The production version works great in containers:

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o server main_production.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/server .
CMD ["./server"]
```

## 🎯 When to Use Which

- **Development/Learning**: Use `main.go` for simplicity
- **Production**: Use `main_production.go` for proper shutdown handling
- **Docker/K8s**: Always use `main_production.go` for graceful container lifecycle

## 🗃️ Database Files

Both examples create a `server.db` SQLite file that **persists between runs**:

```bash
# Clean slate - delete the database file
rm server.db    # Unix/Mac
del server.db   # Windows

# Then run the example again
go run main.go
```

**Why persistence matters:**
- Events accumulate across runs (useful for testing sync)
- Database schema is created on first run
- Delete the file to start fresh or test initial sync scenarios

---

↩︎ Back to [Documentation Index](../../README.md#-documentation-index)
