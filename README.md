# GOmegle 🎭

A production-quality anonymous random video and text chat service — an Omegle clone built in Go and WebRTC.

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
| `GET /ws?mode=video&interests=a,b` | WebSocket upgrade (`mode=video` or `mode=text`) |

---

## Features

- **1:1 Real-Time Video Chat** — Direct peer-to-peer WebRTC video and audio streams with STUN support
- **Random Text Chat** — Classic distraction-free anonymous text chat
- **Interest-Based Matching** — Add tags to find someone with shared interests
- **Mode Isolation** — Video users match with Video users; Text users match with Text users
- **Camera & Mic Controls** — Live mute/unmute audio, toggle camera on/off, flip camera on mobile
- **Typing Indicators** — Real-time "Stranger is typing..." feedback
- **Live Online Counter** — Real-time display of connected users
- **Next / Disconnect** — Skip to a new stranger instantly (`Esc`) or leave session
- **Responsive Layout** — Responsive split view on desktop and compact layout on mobile

---

## Architecture

```
GOmegle/
├── cmd/
│   └── server/
│       └── main.go              # App bootstrap & wiring
├── internal/
│   ├── handler/
│   │   └── ws_handler.go        # Fiber WebSocket handler (mode & interests) + stats API
│   ├── matchmaking/
│   │   ├── queue.go             # Mode-isolated, interest-aware matchmaking queue
│   │   └── queue_test.go        # Matchmaking unit tests
│   ├── models/
│   │   └── models.go            # Domain types (Message, Session, WebRTC signaling)
│   ├── service/
│   │   ├── chat.go              # Chat & WebRTC signaling lifecycle orchestration
│   │   └── chat_test.go         # Chat service & signaling unit tests
│   └── websocket/
│       ├── client.go            # Per-connection read/write pumps (128KB buffer for SDP)
│       └── hub.go               # Central connection registry
├── web/
│   ├── index.html               # Dual video layout + chat UI
│   ├── style.css                # Dark modern theme & responsive video grid
│   └── app.js                   # WebRTC peer connection manager + state machine
├── go.mod
├── go.sum
└── README.md
```

### Data Flow

```
Client → WS Upgrade (?mode=video) → Handler → Hub.Register → Queue.Enqueue(interests, mode)
                                                                    │
                                                   Queue.MatchedPairs channel
                                                                    │
                                            ChatService.matchConsumer ← creates Session
                                                                    │
                             Hub.SendToClient("matched" + isInitiator) → Both clients
                                                                    │
Client A (Initiator) ──[webrtc_offer]──► Hub.Incoming ──► ChatService ──► Client B
Client B (Answer)    ◄──[webrtc_answer]─ Hub.Incoming ◄── ChatService ◄── Client B
Client A ◄───────────── [Direct P2P WebRTC Media Stream] ─────────────► Client B
```

---

## Message Protocol

All messages are JSON envelopes:

```json
{
  "type": "<type>",
  "content": "<text>",
  "mode": "video",
  "sdp": "<sdp_payload>",
  "candidate": { ... },
  "isInitiator": true,
  "mediaState": { "video": true, "audio": true }
}
```

| Type | Direction | Description |
|------|-----------|-------------|
| `chat` | Both | Chat message |
| `system` | Server → Client | System notification (matched, disconnected) |
| `next` | Client → Server | Disconnect & find new match |
| `typing` | Both | User started typing |
| `stop_typing` | Both | User stopped typing |
| `webrtc_offer` | Both (Relay) | SDP Offer for WebRTC video connection |
| `webrtc_answer` | Both (Relay) | SDP Answer for WebRTC video connection |
| `webrtc_candidate` | Both (Relay) | Trickle ICE candidate |
| `media_toggle` | Both (Relay) | Video/Audio mute state notification |

---

## Matchmaking Logic

1. User connects specifying mode (`video` or `text`) and optional interests.
2. `Enqueue()` looks only at waiting users with the **same mode**.
3. It prefers the user with the most overlapping interests.
4. If no interest overlap exists, it falls back to FIFO within that mode.
5. Once matched, `User1` is assigned the `isInitiator: true` flag to start the WebRTC offer.

---

## Testing

```bash
# Build
go build ./...

# Run unit tests
go test -v ./...

# Run server
go run ./cmd/server/main.go

# Open two browser tabs or windows to http://localhost:8080
# Select "Video Chat", grant camera permissions, and chat!
```

---

## Tech Stack

- **Go** — backend language
- **Fiber v2** — HTTP framework
- **gofiber/contrib/websocket** — WebSocket upgrade
- **WebRTC** — direct browser peer-to-peer audio & video streaming
- **google/uuid** — connection IDs
- **Vanilla HTML5/CSS3/ES6+** — high-performance responsive frontend

## License

MIT
