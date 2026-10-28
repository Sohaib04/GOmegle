# GOmegle 🎭

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![WebRTC](https://img.shields.io/badge/WebRTC-P2P%20Streaming-333333?style=flat&logo=webrtc)](https://webrtc.org)
[![Fiber v2](https://img.shields.io/badge/Fiber-v2-00ACD7?style=flat)](https://gofiber.io)
[![Docker Ready](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://www.docker.com/)

A high-performance, production-grade anonymous 1:1 random video and text chat platform — an authentic **1:1 Omegle clone** built in **Go** and **WebRTC**.

GOmegle replicates the core experience of classic Omegle: instant matching with random strangers across the globe, peer-to-peer low-latency video and audio streaming, topic-based interest filtering, typing detection, authentic 2-step disconnect confirmation, and hotkey controls, all wrapped in a sleek, modern, mobile-responsive dark theme.

---

## ⚡ Key Highlights

- **1:1 Real-Time Peer-to-Peer Video Chat** — Direct browser-to-browser WebRTC audio and video streaming with STUN resolution and sub-100ms latency.
- **Classic Text Chat** — Lightning-fast anonymous instant messaging.
- **Mode-Isolated Matchmaking** — Strict queue isolation ensures video users only match with video users, and text users only match with text users.
- **Interest-Based Matching** — Add tags (e.g., `music`, `gaming`, `coding`) to find strangers who share common interests, with common interests highlighted upon connection.
- **Authentic Omegle Disconnect Flow** — 2-step disconnect confirmation (`Disconnect` &rarr; `Really?` &rarr; `New Stranger`) with full keyboard shortcut support.
- **In-Call Media Controls** — Live microphone mute/unmute, camera toggle, and front/back camera flip for mobile devices.
- **Stranger Media Status Indicators** — Visual badges indicating when the stranger mutes their microphone or disables their video feed.
- **Typing Indicators** — Live "Stranger is typing..." feedback.
- **Global Online Counter** — Real-time online user count polled dynamically from the backend.
- **Zero Login & Ephemeral by Design** — No user registration, no tracking cookies, no database, and zero persistent message storage.
- **Ultra-Lightweight Go Backend** — Built on Go Fiber v2 with goroutine-isolated read/write WebSocket pumps capable of handling thousands of concurrent connections with minimal memory overhead.

---

## 📸 Capabilities & UI Breakdown

| Feature | Description |
|---|---|
| **Landing Interface** | Clean hero view with instant mode selection (Video or Text) and interest tag manager. |
| **Video Grid** | Dual video layout: larger stranger view with live connection indicators and local picture-in-picture / split preview. |
| **Media Toggles** | Floating controls on local video: Mute Mic, Toggle Camera, Flip Camera (Mobile). |
| **Hotkeys** | <kbd>Esc</kbd> to disconnect / skip / find new stranger; <kbd>M</kbd> to mute audio; <kbd>V</kbd> to toggle camera; <kbd>Enter</kbd> to send. |
| **Fallback & Recovery** | Automatic camera constraint fallbacks for older devices; media permission banners with one-click fallback to Text mode. |
| **Mobile Responsive** | Fully optimized for mobile screens, tablet orientations, and desktop widescreen displays. |

---

## 🏗 Architecture & System Design

```
GOmegle/
├── cmd/
│   └── server/
│       └── main.go              # Server bootstrap, Fiber routes & graceful shutdown
├── internal/
│   ├── handler/
│   │   └── ws_handler.go        # WebSocket upgrade, query param parsing & stats endpoint
│   ├── matchmaking/
│   │   ├── queue.go             # Mode-isolated, interest-ranked matchmaking queue
│   │   └── queue_test.go        # Unit tests for queue & interest overlaps
│   ├── models/
│   │   └── models.go            # Protocol models, message types & session state
│   ├── service/
│   │   ├── chat.go              # Session lifecycle, signaling relay & event orchestration
│   │   └── chat_test.go         # Chat service & WebRTC signaling unit tests
│   └── websocket/
│       ├── client.go            # Per-connection ReadPump & WritePump (128 KB buffer for SDP)
│       └── hub.go               # Thread-safe client connection registry (sync.RWMutex)
├── web/
│   ├── index.html               # Dual video layout + chat interface markup
│   ├── style.css                # Modern responsive dark theme & animations
│   └── app.js                   # WebRTC RTCPeerConnection manager & client state machine
├── Dockerfile                   # Multi-stage container build
├── docker-compose.yml           # Compose specification
├── go.mod
├── go.sum
├── .gitignore
├── .env.example
├── LICENSE                      # MIT License
└── README.md
```

### Data Flow & WebRTC Signaling

```
User A (Initiator)                                Server (GOmegle Hub)                             User B (Receiver)
       │                                                   │                                                │
       ├──── WebSocket Connect (?mode=video) ─────────────►│                                                │
       │                                                   │◄─────── WebSocket Connect (?mode=video) ───────┤
       │                                                   │                                                │
       │                                       [Matchmaking Engine]                                         │
       │                                       Matches A & B in queue                                       │
       │                                                   │                                                │
       │◄─── System: Matched (isInitiator: true) ──────────┼───── System: Matched (isInitiator: false) ────►│
       │                                                   │                                                │
       ├──── webrtc_offer (SDP) ──────────────────────────►│──── webrtc_offer (SDP Relay) ─────────────────►│
       │                                                   │                                                │
       │◄─── webrtc_answer (SDP Relay) ────────────────────│◄─── webrtc_answer (SDP) ───────────────────────┤
       │                                                   │                                                │
       ├──── webrtc_candidate (ICE) ──────────────────────►│──── webrtc_candidate (ICE Relay) ────────────►│
       │◄─── webrtc_candidate (ICE Relay) ─────────────────│◄─── webrtc_candidate (ICE) ────────────────────┤
       │                                                   │                                                │
       │======================= DIRECT PEER-TO-PEER ENCRYPTED MEDIA STREAM =================================│
       │◄───────────────────── (Audio & Video tracks over WebRTC) ─────────────────────────────────────────►│
```

---

## 📡 Message Protocol

Client and server communicate over WebSocket using standard JSON envelopes:

```json
{
  "type": "chat",
  "content": "Hello stranger!",
  "mode": "video",
  "sdp": "...",
  "candidate": { ... },
  "isInitiator": true,
  "mediaState": { "video": true, "audio": true },
  "commonInterests": ["golang", "webrtc"]
}
```

### Message Types

| Message Type | Sender | Description |
|---|---|---|
| `chat` | Client &harr; Server | Text message between paired strangers. |
| `system` | Server &rarr; Client | System events: matched notifications, stranger disconnected, queue updates. |
| `typing` | Client &rarr; Partner | Notifies partner that user is actively typing. |
| `stop_typing` | Client &rarr; Partner | Notifies partner that user stopped typing. |
| `webrtc_offer` | Client &rarr; Partner | WebRTC SDP offer generated by the designated initiator. |
| `webrtc_answer` | Client &rarr; Partner | WebRTC SDP answer generated by the receiver. |
| `webrtc_candidate` | Client &rarr; Partner | Trickle ICE candidates for NAT traversal. |
| `media_toggle` | Client &rarr; Partner | Broadcasts camera on/off or microphone mute/unmute state. |
| `disconnect` | Client &rarr; Server | Ends the active session; notifies the partner. |
| `find_match` | Client &rarr; Server | Requests entry back into the matchmaking queue. |
| `leave_queue` | Client &rarr; Server | Cancels matchmaking while searching. |
| `next` | Client &rarr; Server | Ends current conversation and re-enqueues immediately. |

---

## 🎯 Matchmaking Algorithm

1. **Mode Isolation**: Clients connect via `/ws?mode=video` or `/ws?mode=text`. The queue segments callers so video users never pair with text users.
2. **Interest Scoring**: When matching, the queue inspects all waiting candidates in the requested mode and calculates tag intersections:
   $$\text{score}(A, B) = | \text{interests}_A \cap \text{interests}_B |$$
   The waiting user with the highest overlap is selected first.
3. **FIFO Fallback**: If no common interests exist, the oldest waiting user in that mode is paired immediately.
4. **Initiator Determination**: The first user in the pair is flagged with `isInitiator: true`, instructing their browser to generate the WebRTC offer.
5. **Race-Condition Safety**: If a client disconnects while waiting or during the handshake, pre-session disconnect handlers cleanly re-enqueue the surviving peer.

---

## 🚀 Quick Start

### Prerequisites

- [Go 1.22+](https://golang.org/dl/) (or Docker)
- A modern web browser (Chrome, Firefox, Safari, Edge) with camera/mic permissions enabled.

### 1. Clone the Repository

```bash
git clone https://github.com/Sohaib04/GOmegle.git
cd GOmegle
```

### 2. Run Locally

```bash
# Download dependencies
go mod download

# Start the server
go run ./cmd/server/main.go
```

Open your browser to:
```
http://localhost:8080
```

> **Note on WebRTC Permissions**: WebRTC requires a secure context. `http://localhost` is treated as secure by modern browsers for local development. For remote network testing or production, **HTTPS/WSS is mandatory** for camera and microphone access.

### 3. Run with Custom Port

```bash
PORT=3000 go run ./cmd/server/main.go
```

### 4. Run with Docker / Docker Compose

```bash
# Build and run with Docker Compose
docker compose up --build

# Or run directly with Docker
docker build -t gomegle .
docker run -p 8080:8080 gomegle
```

---

## 🧪 Testing

Run the full test suite including matchmaking queue isolation and chat service signaling relays:

```bash
# Run all unit tests
go test -v ./...

# Run tests with race condition detector
go test -race -v ./...
```

---

## 🌐 Production Deployment & Reverse Proxy

When deploying GOmegle publicly, place it behind a TLS-terminating reverse proxy such as **Caddy** or **Nginx** to provide HTTPS and WSS.

### Caddyfile (Recommended)

```caddy
yourdomain.com {
    reverse_proxy localhost:8080
}
```

### Nginx Configuration

```nginx
server {
    listen 443 ssl http2;
    server_name yourdomain.com;

    ssl_certificate /path/to/fullchain.pem;
    ssl_certificate_key /path/to/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### NAT Traversal (STUN / TURN)

GOmegle is pre-configured with Google's public STUN servers for direct P2P connections. In environments where both peers are behind restrictive symmetric NATs or corporate firewalls, a TURN relay server (such as [Coturn](https://github.com/coturn/coturn)) can be configured in `web/app.js`:

```javascript
const rtcConfig = {
    iceServers: [
        { urls: 'stun:stun.l.google.com:19302' },
        // Add custom TURN server if required:
        // {
        //     urls: 'turn:turn.yourdomain.com:3478',
        //     username: 'user',
        //     credential: 'password'
        // }
    ],
    iceCandidatePoolSize: 10,
};
```

---

## 🔒 Privacy & Open-Source Security

- **No Logging of Conversations**: Messages and WebRTC signaling are relayed purely in memory through Go channels. No chat logs are ever persisted to disk or databases.
- **Random Ephemeral IDs**: Connection IDs are dynamically generated UUIDv4 tokens destroyed immediately upon disconnection.
- **P2P Video & Audio**: Media streams travel directly between peers via end-to-end SRTP encryption; video and audio never touch the GOmegle application server.
- **Sanitized Secrets & Safe Open Source**: Zero hardcoded credentials, API keys, or private IP addresses in source code. All environment overrides use clean `.env` templates.

---

## 🛠 Tech Stack

- **Backend**: [Go](https://go.dev) 1.22+
- **HTTP / WebSockets**: [Fiber v2](https://github.com/gofiber/fiber) & [`gofiber/contrib/websocket`](https://github.com/gofiber/contrib/tree/main/websocket)
- **Real-Time Media**: [WebRTC](https://webrtc.org) (Browser Native `RTCPeerConnection` with STUN/TURN)
- **Unique Identifiers**: [`google/uuid`](https://github.com/google/uuid)
- **Frontend**: Vanilla HTML5, CSS3 (Modern Flex/Grid, CSS custom properties, keyframe animations), ES6+ JavaScript (zero external JS dependencies)

---

## 🤝 Contributing

Contributions are welcome! Please follow these steps:

1. Fork the repository.
2. Create your feature branch (`git checkout -b feature/amazing-feature`).
3. Ensure all tests pass (`go test -v ./...`).
4. Commit your changes (`git commit -m 'Add some amazing feature'`).
5. Push to the branch (`git push origin feature/amazing-feature`).
6. Open a Pull Request.

---

## 📄 License

This project is open-source software licensed under the [MIT License](LICENSE).
