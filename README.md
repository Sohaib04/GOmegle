# GOmegle 🎭

A production-quality anonymous random text chat backend — an Omegle clone built in Go.

## Quick Start

```bash
# Run the server
go run ./cmd/server/main.go

# Server starts on :8080 (override with PORT env var)
PORT=3000 go run ./cmd/server/main.go
```

### Health Check

```bash
curl http://localhost:8080/health
```

### Connect via WebSocket

```
ws://localhost:8080/ws
```

---

## Architecture

```
GOmegle/
├── cmd/
│   └── server/
│       └── main.go              # App bootstrap & wiring
├── internal/
│   ├── handler/
│   │   └── ws_handler.go        # Fiber WebSocket handler
│   ├── matchmaking/
│   │   └── queue.go             # Thread-safe matchmaking queue
│   ├── models/
│   │   └── models.go            # Domain types (Message, Session)
│   ├── service/
│   │   └── chat.go              # Chat lifecycle orchestration
│   └── websocket/
│       ├── client.go            # Per-connection read/write pumps
│       └── hub.go               # Central connection registry
├── go.mod
├── go.sum
└── README.md
```

### Data Flow

```
Client → WS Upgrade → Handler → Hub.Register → Queue.Enqueue
                                                      │
                                     Queue.MatchedPairs channel
                                                      │
                              ChatService.matchConsumer ← creates Session
                                                      │
                              Hub.SendToClient("matched") → Both clients
                                                      │
Client A sends chat → Hub.Incoming → ChatService.relayMessage → Hub.SendToClient(B)
```

---

## Message Protocol

All messages are JSON with this structure:

```json
{ "type": "<type>", "content": "<text>" }
```

| Type     | Direction       | Description                        |
|----------|-----------------|------------------------------------|
| `chat`   | Client → Server | Send a chat message                |
| `chat`   | Server → Client | Receive a chat message             |
| `system` | Server → Client | System notification                |
| `next`   | Client → Server | Disconnect current & find new match|

### System Messages

| Content                                               | When                        |
|-------------------------------------------------------|-----------------------------|
| `You've been matched with a stranger. Say hi!`        | Two users matched           |
| `Stranger has disconnected.`                          | Partner left or pressed next|
| `You are not currently matched with anyone. Please wait...` | Chat sent while unmatched   |

---

## Concurrency Model

| Component              | Mechanism              | Purpose                                    |
|------------------------|------------------------|--------------------------------------------|
| `Hub.Run()`            | Single goroutine + channels | Serializes client map access — no locks |
| `Client.ReadPump`      | Per-client goroutine   | Reads from WS → Hub.Incoming channel       |
| `Client.WritePump`     | Per-client goroutine   | Drains Send channel → writes to WS         |
| `Queue`                | `sync.Mutex`           | Protects queue slice; matching is atomic    |
| `ChatService.sessions` | `sync.RWMutex`         | Protects session map reads/writes           |
| Match consumer         | Goroutine              | Listens on matched channel → creates sessions |

**Key invariants:**
1. Only `Hub.Run()` modifies the clients map — zero races
2. Queue mutex is held briefly — just slice operations
3. Session teardown is idempotent via `Session.Close()` — safe for concurrent disconnect/next

---

## Matchmaking Logic

1. User connects → UUID assigned → registered in Hub → enqueued in `Queue`
2. `Queue.Enqueue()` locks → appends → if `len ≥ 2`, pops first two → sends pair to `matched` channel
3. `ChatService.matchConsumer()` reads pair → creates `Session` → notifies both users
4. Messages relay through `ChatService.HandleMessage` → partner via Hub
5. **Next**: tear down session → notify partner → re-enqueue both
6. **Disconnect**: tear down session → notify partner → re-enqueue partner only

---

## Testing Suggestions

### Unit Tests
```bash
go test -race ./...
```

- `matchmaking.Queue` — enqueue, dequeue, concurrent enqueue, duplicate rejection
- `models.Session` — Close idempotency, Partner lookup
- `service.ChatService` — mock Hub + Queue, test session creation/teardown/relay

### Integration Tests
- Spin up Fiber app in test, connect real WS clients via `gorilla/websocket` test dialer
- Assert: match notification, message relay, next behavior, disconnect notification

### Manual Test
```javascript
// Browser Console — Tab 1
ws1 = new WebSocket("ws://localhost:8080/ws");
ws1.onmessage = (e) => console.log("Tab1:", e.data);

// Browser Console — Tab 2
ws2 = new WebSocket("ws://localhost:8080/ws");
ws2.onmessage = (e) => console.log("Tab2:", e.data);

// Both should receive: {"type":"system","content":"You've been matched..."}

// Send a message from Tab 1
ws1.send(JSON.stringify({type: "chat", content: "hello!"}));
// Tab 2 should receive: {"type":"chat","content":"hello!"}

// Press next from Tab 1
ws1.send(JSON.stringify({type: "next"}));
// Tab 2 receives: {"type":"system","content":"Stranger has disconnected."}
```

---

## Tech Stack

- **Go 1.26** — language
- **Fiber v2** — HTTP framework
- **gofiber/contrib/websocket** — WebSocket upgrade for Fiber
- **google/uuid** — connection IDs
- **In-memory storage** — no database (Phase 1)

## License

MIT
