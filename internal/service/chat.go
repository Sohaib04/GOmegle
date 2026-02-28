package service

import (
	"log"
	"sync"

	"github.com/sohaib_04/GOmegle/internal/matchmaking"
	"github.com/sohaib_04/GOmegle/internal/models"
	"github.com/sohaib_04/GOmegle/internal/websocket"
)

// ChatService orchestrates the chat lifecycle: matchmaking, message relay,
// session management, and disconnect handling.
type ChatService struct {
	hub   *websocket.Hub
	queue *matchmaking.Queue

	// sessions maps userID -> their active session. Both users in a session
	// have an entry pointing to the same *Session.
	sessions map[string]*models.Session
	mu       sync.RWMutex
}

// NewChatService creates a new ChatService and wires up Hub callbacks.
func NewChatService(hub *websocket.Hub, queue *matchmaking.Queue) *ChatService {
	cs := &ChatService{
		hub:      hub,
		queue:    queue,
		sessions: make(map[string]*models.Session),
	}

	// Wire up Hub callbacks so the Hub routes events to us.
	hub.OnMessage = cs.HandleMessage
	hub.OnDisconnect = cs.HandleDisconnect

	return cs
}

// Start launches the background goroutine that listens for matched pairs
// from the matchmaking queue and creates chat sessions.
func (cs *ChatService) Start() {
	go cs.matchConsumer()
}

// matchConsumer listens on the matchmaking queue's matched channel and
// creates sessions for each pair.
func (cs *ChatService) matchConsumer() {
	for pair := range cs.queue.MatchedPairs() {
		cs.createSession(pair[0], pair[1])
	}
}

// createSession sets up a new chat session between two users and notifies them.
func (cs *ChatService) createSession(user1, user2 string) {
	session := &models.Session{
		User1: user1,
		User2: user2,
	}

	cs.mu.Lock()
	// If either user already has a session (race between next and match),
	// don't overwrite — re-enqueue both.
	if _, exists := cs.sessions[user1]; exists {
		cs.mu.Unlock()
		log.Printf("service: user %s already in session, re-enqueuing both", user1)
		cs.queue.Enqueue(user1)
		cs.queue.Enqueue(user2)
		return
	}
	if _, exists := cs.sessions[user2]; exists {
		cs.mu.Unlock()
		log.Printf("service: user %s already in session, re-enqueuing both", user2)
		cs.queue.Enqueue(user1)
		cs.queue.Enqueue(user2)
		return
	}

	cs.sessions[user1] = session
	cs.sessions[user2] = session
	cs.mu.Unlock()

	log.Printf("service: session created between %s and %s", user1, user2)

	// Notify both users they've been matched.
	matchedMsg := models.Message{
		Type:    models.MessageTypeSystem,
		Content: "You've been matched with a stranger. Say hi!",
	}
	cs.hub.SendToClient(user1, matchedMsg)
	cs.hub.SendToClient(user2, matchedMsg)
}

// HandleMessage processes an incoming message from a client.
func (cs *ChatService) HandleMessage(clientID string, msg models.Message) {
	switch msg.Type {
	case models.MessageTypeChat:
		cs.relayMessage(clientID, msg)

	case models.MessageTypeNext:
		cs.HandleNext(clientID)

	default:
		log.Printf("service: unknown message type from %s: %s", clientID, msg.Type)
	}
}

// relayMessage forwards a chat message to the sender's matched partner.
func (cs *ChatService) relayMessage(senderID string, msg models.Message) {
	cs.mu.RLock()
	session, exists := cs.sessions[senderID]
	cs.mu.RUnlock()

	if !exists || session == nil {
		// User isn't in a session — send them a system message.
		cs.hub.SendToClient(senderID, models.Message{
			Type:    models.MessageTypeSystem,
			Content: "You are not currently matched with anyone. Please wait...",
		})
		return
	}

	partnerID := session.Partner(senderID)
	if partnerID == "" {
		return
	}

	cs.hub.SendToClient(partnerID, models.Message{
		Type:    models.MessageTypeChat,
		Content: msg.Content,
	})
}

// HandleNext disconnects the user from their current session and re-enqueues
// them for a new match. The partner is notified.
func (cs *ChatService) HandleNext(userID string) {
	cs.mu.Lock()
	session, exists := cs.sessions[userID]
	if !exists || session == nil {
		cs.mu.Unlock()
		// Not in a session — just ensure they're in the queue.
		cs.queue.Enqueue(userID)
		return
	}

	partnerID := session.Partner(userID)

	// Remove both users from the session map.
	delete(cs.sessions, userID)
	delete(cs.sessions, partnerID)
	cs.mu.Unlock()

	// Mark the session as closed (idempotent).
	if session.Close() {
		log.Printf("service: user %s pressed next, ending session with %s", userID, partnerID)

		// Notify the partner that the stranger disconnected.
		if partnerID != "" {
			cs.hub.SendToClient(partnerID, models.Message{
				Type:    models.MessageTypeSystem,
				Content: "Stranger has disconnected.",
			})
			// Re-enqueue the partner so they can find a new match.
			cs.queue.Enqueue(partnerID)
		}
	}

	// Re-enqueue the user who pressed next.
	cs.queue.Enqueue(userID)
}

// HandleDisconnect handles a client disconnecting (closing the WebSocket).
func (cs *ChatService) HandleDisconnect(userID string) {
	log.Printf("service: user %s disconnected", userID)

	// Remove from matchmaking queue if they were waiting.
	cs.queue.Dequeue(userID)

	cs.mu.Lock()
	session, exists := cs.sessions[userID]
	if !exists || session == nil {
		cs.mu.Unlock()
		return
	}

	partnerID := session.Partner(userID)

	// Remove both users from the session map.
	delete(cs.sessions, userID)
	delete(cs.sessions, partnerID)
	cs.mu.Unlock()

	// Mark the session as closed (idempotent).
	if session.Close() {
		log.Printf("service: tearing down session due to disconnect of %s", userID)

		// Notify the partner.
		if partnerID != "" {
			cs.hub.SendToClient(partnerID, models.Message{
				Type:    models.MessageTypeSystem,
				Content: "Stranger has disconnected.",
			})
			// Re-enqueue the partner for a new match.
			cs.queue.Enqueue(partnerID)
		}
	}
}

// EnqueueUser adds a user to the matchmaking queue.
// Called by the handler after a new WebSocket connection is established.
func (cs *ChatService) EnqueueUser(userID string) {
	cs.queue.Enqueue(userID)
}
