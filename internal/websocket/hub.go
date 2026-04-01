package websocket

import (
	"encoding/json"
	"log"

	"github.com/sohaib_04/GOmegle/internal/models"
)

// IncomingMessage wraps a message with the sender's client ID.
type IncomingMessage struct {
	ClientID string
	Message  models.Message
}

// sendRequest is used to send a message to a specific client through the
// Run() goroutine, eliminating race conditions on the clients map.
type sendRequest struct {
	ClientID string
	Data     []byte
	Result   chan bool
}

// countRequest is used to get the current client count through the
// Run() goroutine, eliminating race conditions on the clients map.
type countRequest struct {
	Result chan int
}

// MessageHandler is a callback invoked when a message arrives from a client.
type MessageHandler func(clientID string, msg models.Message)

// DisconnectHandler is a callback invoked when a client disconnects.
type DisconnectHandler func(clientID string)

// Hub maintains the set of active clients and broadcasts messages.
// All access to the clients map is serialized through the Run() goroutine
// to avoid races — no mutex needed.
type Hub struct {
	// clients is the set of registered clients, keyed by client ID.
	clients map[string]*Client

	// Register channel for new clients.
	Register chan *Client

	// Unregister channel for departing clients.
	Unregister chan *Client

	// Incoming channel for messages from clients.
	Incoming chan *IncomingMessage

	// send routes send requests through Run() to avoid races.
	send chan sendRequest

	// count routes count requests through Run() to avoid races.
	count chan countRequest

	// OnMessage is called when a message is received from a client.
	OnMessage MessageHandler

	// OnDisconnect is called when a client disconnects.
	OnDisconnect DisconnectHandler
}

// NewHub creates a new Hub instance.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Incoming:   make(chan *IncomingMessage, 256),
		send:       make(chan sendRequest, 256),
		count:      make(chan countRequest),
	}
}

// Run starts the Hub's event loop. This must be called in a goroutine.
// It serializes all client map access, making the Hub fully thread-safe
// without any mutexes.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.clients[client.ID] = client
			log.Printf("client registered: %s (total: %d)", client.ID, len(h.clients))

		case client := <-h.Unregister:
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				close(client.Send)
				log.Printf("client unregistered: %s (total: %d)", client.ID, len(h.clients))

				if h.OnDisconnect != nil {
					h.OnDisconnect(client.ID)
				}
			}

		case incoming := <-h.Incoming:
			if h.OnMessage != nil {
				h.OnMessage(incoming.ClientID, incoming.Message)
			}

		case req := <-h.send:
			client, ok := h.clients[req.ClientID]
			if !ok {
				req.Result <- false
				continue
			}
			select {
			case client.Send <- req.Data:
				req.Result <- true
			default:
				log.Printf("client %s send buffer full, dropping message", req.ClientID)
				req.Result <- false
			}

		case req := <-h.count:
			req.Result <- len(h.clients)
		}
	}
}

// SendToClient sends a message to a specific client by ID.
// This is thread-safe — the request is routed through the Run() goroutine.
func (h *Hub) SendToClient(clientID string, msg models.Message) bool {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("failed to marshal message for client %s: %v", clientID, err)
		return false
	}

	req := sendRequest{
		ClientID: clientID,
		Data:     data,
		Result:   make(chan bool, 1),
	}

	h.send <- req
	return <-req.Result
}

// ClientCount returns the number of currently connected clients.
// This is thread-safe — the request is routed through the Run() goroutine.
func (h *Hub) ClientCount() int {
	req := countRequest{
		Result: make(chan int, 1),
	}
	h.count <- req
	return <-req.Result
}
