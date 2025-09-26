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