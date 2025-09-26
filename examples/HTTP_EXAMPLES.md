# HTTP Client/Server Examples

These examples demonstrate Go Sync Kit's HTTP transport capabilities using the new **SyncNode** presets.

## 🚀 Quick Start

### 1. Run the Server
```bash
cd http_server
go run main.go
```
Server starts on port 8080 and exposes `/sync` endpoint.

### 2. Run the Client
```bash
# In another terminal
cd http_client  
go run main.go
```
Client connects to server and performs a sync operation.

## 🎯 What These Examples Show

### HTTP Server (`http_server/`)
- Uses `synckit.NewHTTPServerNode()` preset
- SQLite event store (`server.db`)
- HTTP transport in server mode
- Exposes RESTful `/sync` endpoint
- Production-ready server setup

### HTTP Client (`http_client/`) 
- Uses `synckit.NewHTTPClientNode()` preset
- SQLite event store (`client.db`)  
- HTTP transport pointing to server
- Performs sync operations against server
- Real client-server synchronization

## 🔧 Key Benefits

- **Zero Boilerplate**: Presets handle all the setup
- **Production Ready**: Drop-in SQLite → PostgreSQL swap
- **Type Safe**: Full compile-time checking
- **Clean Separation**: Server/client concerns clearly separated

## 🏗️ Architecture

```
┌─────────────┐    HTTP    ┌─────────────┐
│   Client    │◄─────────►│   Server    │
│             │ /sync      │             │  
│ client.db   │            │ server.db   │
└─────────────┘            └─────────────┘
```

## 🌐 Production Notes

**For Production:**
- Replace `sqlite.New()` with `postgres.New()` 
- Add authentication/authorization to HTTP transport
- Configure proper logging and monitoring
- Use environment variables for connection strings
- Add TLS/HTTPS configuration

**Scaling:**
- Multiple clients can sync with single server
- Server can handle concurrent sync requests
- Database handles conflict resolution automatically