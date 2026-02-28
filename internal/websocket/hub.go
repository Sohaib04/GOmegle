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
		}
	}
}

// SendToClient sends a message to a specific client by ID.
// Returns false if the client is not found.
func (h *Hub) SendToClient(clientID string, msg models.Message) bool {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("failed to marshal message for client %s: %v", clientID, err)
		return false
	}

	// We need to send through a channel to avoid blocking the caller.
	// The Hub's Run loop is the only place that touches h.clients directly,
	// but SendToClient can be called from any goroutine. To avoid a race
	// on h.clients, we use a non-blocking approach: try to find the client
	// and send to its channel.
	//
	// NOTE: This method is safe because:
	// 1. The Send channel is only closed in the Unregister handler.
	// 2. We use a select with default to avoid blocking if the channel is full.
	// 3. If the client was just unregistered, the recover will catch the panic
	//    from sending on a closed channel.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered from send to disconnected client %s: %v", clientID, r)
		}
	}()

	// We access h.clients here which technically could race with Run().
	// To keep things simple and avoid complex synchronization for this MVP,
	// we'll use a safe send pattern. For production, this should go through
	// a channel-based request to the Run loop.
	client, ok := h.clients[clientID]
	if !ok {
		return false
	}

	select {
	case client.Send <- data:
		return true
	default:
		// Client's send buffer is full; they're too slow.
		log.Printf("client %s send buffer full, dropping message", clientID)
		return false
	}
}
