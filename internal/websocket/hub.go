package websocket

import (
	"encoding/json"
	"log"
	"sync"

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
// A sync.RWMutex protects the clients map to prevent race conditions
// and completely eliminate goroutine deadlocks on synchronous message relaying.
type Hub struct {
	// clients is the set of registered clients, keyed by client ID.
	clients map[string]*Client
	mu      sync.RWMutex

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
		Incoming:   make(chan *IncomingMessage, 512),
	}
}

// Run starts the Hub's event loop. This must be called in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.clients[client.ID] = client
			total := len(h.clients)
			h.mu.Unlock()
			log.Printf("client registered: %s (total: %d)", client.ID, total)

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				client.Close()
				total := len(h.clients)
				h.mu.Unlock()
				log.Printf("client unregistered: %s (total: %d)", client.ID, total)

				if h.OnDisconnect != nil {
					go h.OnDisconnect(client.ID)
				}
			} else {
				h.mu.Unlock()
			}

		case incoming := <-h.Incoming:
			if h.OnMessage != nil {
				go h.OnMessage(incoming.ClientID, incoming.Message)
			}
		}
	}
}

// SendToClient sends a message to a specific client by ID.
// Thread-safe and non-blocking — acquires read lock on clients map and queues to client channel.
func (h *Hub) SendToClient(clientID string, msg models.Message) bool {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("failed to marshal message for client %s: %v", clientID, err)
		return false
	}

	h.mu.RLock()
	client, ok := h.clients[clientID]
	h.mu.RUnlock()

	if !ok {
		return false
	}

	return client.SendMessage(data)
}

// IsClientConnected checks if a client with the given ID is actively registered in the Hub.
func (h *Hub) IsClientConnected(clientID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[clientID]
	return ok
}

// ClientCount returns the number of currently connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
