# GOmegle 🎭

A production-quality anonymous random text chat service — an Omegle clone built in Go.

## Quick Start

```bash
# Run the server
go run ./cmd/server/main.go

# Open in browser
open http://localhost:8080

# Override port
PORT=3000 go run ./cmd/server/main.go
```

### API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /` | Web frontend |
| `GET /health` | Health check |
| `GET /api/stats` | Online user count `{"online": N}` |
| `GET /ws?interests=a,b` | WebSocket upgrade |

---

## Features

- **Random text chat** — Connect with a stranger instantly
- **Interest-based matching** — Add tags to find someone with shared interests
- **Typing indicators** — Real-time "Stranger is typing..." feedback
- **Online count** — Live display of connected users
- **Next / Disconnect** — Skip to a new stranger or leave
- **Keyboard shortcuts** — Enter to send, Escape to disconnect

---

## Architecture

```
GOmegle/
├── cmd/
│   └── server/
│       └── main.go              # App bootstrap & wiring
├── internal/
│   ├── handler/
│   │   └── ws_handler.go        # Fiber WebSocket handler + stats API
│   ├── matchmaking/
│   │   └── queue.go             # Interest-aware matchmaking queue
│   ├── models/
│   │   └── models.go            # Domain types (Message, Session)
│   ├── service/
│   │   └── chat.go              # Chat lifecycle orchestration
│   └── websocket/
│       ├── client.go            # Per-connection read/write pumps
│       └── hub.go               # Central connection registry
├── web/
│   ├── index.html               # Chat UI
│   ├── style.css                # Dark theme styles
│   └── app.js                   # WebSocket client + state machine
├── go.mod
├── go.sum
└── README.md
```

### Data Flow

```
Client → WS Upgrade → Handler → Hub.Register → Queue.Enqueue(interests)
                                                      │
                                     Queue.MatchedPairs channel
                                                      │
                              ChatService.matchConsumer ← creates Session
                                                      │
                              Hub.SendToClient("matched" + commonInterests) → Both clients
                                                      │
Client A sends chat → Hub.Incoming → ChatService.relayMessage → Hub.SendToClient(B)
```

---

## Message Protocol

All messages are JSON:

```json
{ "type": "<type>", "content": "<text>" }
```

| Type | Direction | Description |
|------|-----------|-------------|
| `chat` | Both | Chat message |
| `system` | Server → Client | System notification |
| `next` | Client → Server | Disconnect & find new match |
| `typing` | Both | User started typing |
| `stop_typing` | Both | User stopped typing |

---

## Matchmaking Logic

1. User connects with optional interests → enqueued in `Queue`
2. `Enqueue()` scans existing entries — prefers the one with most overlapping interests
3. If no interest overlap, falls back to FIFO (first in queue)
4. Matched pair receives notification with common interests displayed

---

## Concurrency Model

| Component | Mechanism | Purpose |
|-----------|-----------|---------|
| `Hub.Run()` | Single goroutine + channels | Serializes all client map access |
| `Hub.SendToClient` | Channel-based request to Run() | Thread-safe message delivery |
| `Client.ReadPump` | Per-client goroutine | Reads from WS → Hub.Incoming |
| `Client.WritePump` | Per-client goroutine | Drains Send channel → WS |
| `Queue` | `sync.Mutex` | Protects queue slice |
| `ChatService.sessions` | `sync.RWMutex` | Protects session map |
| Match consumer | Goroutine | Listens on matched channel |

---

## Testing

```bash
# Build
go build ./...

# Run server
go run ./cmd/server/main.go

# Open two browser tabs to http://localhost:8080
# Add interests, click Start, chat, press New/Disconnect

# API checks
curl http://localhost:8080/health
curl http://localhost:8080/api/stats
```

---

## Tech Stack

- **Go** — language
- **Fiber v2** — HTTP framework
- **gofiber/contrib/websocket** — WebSocket upgrade
- **google/uuid** — connection IDs
- **Vanilla HTML/CSS/JS** — frontend
- **In-memory storage** — no database (Phase 1-2)

## License

MIT
