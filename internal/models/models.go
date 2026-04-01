package models

import "sync"

// MessageType represents the type of WebSocket message.
type MessageType string

const (
	// MessageTypeChat is a regular chat message between matched users.
	MessageTypeChat MessageType = "chat"
	// MessageTypeSystem is a system notification (e.g., matched, disconnected).
	MessageTypeSystem MessageType = "system"
	// MessageTypeNext signals the user wants to disconnect and find a new match.
	MessageTypeNext MessageType = "next"
	// MessageTypeTyping signals the user is currently typing.
	MessageTypeTyping MessageType = "typing"
	// MessageTypeStopTyping signals the user has stopped typing.
	MessageTypeStopTyping MessageType = "stop_typing"
)

// Message is the WebSocket message envelope exchanged between client and server.
type Message struct {
	Type            MessageType `json:"type"`
	Content         string      `json:"content,omitempty"`
	Interests       []string    `json:"interests,omitempty"`
	CommonInterests []string    `json:"commonInterests,omitempty"`
	OnlineCount     int         `json:"onlineCount,omitempty"`
}

// Session represents an active chat session between two matched users.
type Session struct {
	User1           string
	User2           string
	CommonInterests []string

	mu     sync.Mutex
	closed bool
}

// Close marks the session as closed. Returns true if this call actually
// closed it (i.e., it wasn't already closed). This makes teardown idempotent.
func (s *Session) Close() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	s.closed = true
	return true
}

// IsClosed reports whether the session has been closed.
func (s *Session) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// Partner returns the other user's ID given one user's ID.
// Returns empty string if the given ID is not part of the session.
func (s *Session) Partner(userID string) string {
	if s.User1 == userID {
		return s.User2
	}
	if s.User2 == userID {
		return s.User1
	}
	return ""
}
