package models

import "sync"

// Chat modes supported by GOmegle.
const (
	ModeText  = "text"
	ModeVideo = "video"
)

// MessageType represents the type of WebSocket message.
type MessageType string

const (
	// MessageTypeChat is a regular chat message between matched users.
	MessageTypeChat MessageType = "chat"
	// MessageTypeSystem is a system notification (e.g., matched, disconnected).
	MessageTypeSystem MessageType = "system"
	// MessageTypeNext signals the user wants to disconnect and find a new match.
	MessageTypeNext MessageType = "next"
	// MessageTypeDisconnect signals the user wants to end their current chat session.
	MessageTypeDisconnect MessageType = "disconnect"
	// MessageTypeFindMatch signals the user wants to enter the matchmaking queue.
	MessageTypeFindMatch MessageType = "find_match"
	// MessageTypeLeaveQueue signals the user wants to leave the matchmaking queue.
	MessageTypeLeaveQueue MessageType = "leave_queue"
	// MessageTypeTyping signals the user is currently typing.
	MessageTypeTyping MessageType = "typing"
	// MessageTypeStopTyping signals the user has stopped typing.
	MessageTypeStopTyping MessageType = "stop_typing"
	// MessageTypeOffer is a WebRTC SDP offer message.
	MessageTypeOffer MessageType = "webrtc_offer"
	// MessageTypeAnswer is a WebRTC SDP answer message.
	MessageTypeAnswer MessageType = "webrtc_answer"
	// MessageTypeCandidate is a WebRTC ICE candidate message.
	MessageTypeCandidate MessageType = "webrtc_candidate"
	// MessageTypeMediaToggle notifies the peer of camera/microphone mute changes.
	MessageTypeMediaToggle MessageType = "media_toggle"
)

// MediaState indicates whether video/audio tracks are currently enabled.
type MediaState struct {
	Video bool `json:"video"`
	Audio bool `json:"audio"`
}

// Message is the WebSocket message envelope exchanged between client and server.
type Message struct {
	Type            MessageType `json:"type"`
	Content         string      `json:"content,omitempty"`
	Interests       []string    `json:"interests,omitempty"`
	CommonInterests []string    `json:"commonInterests,omitempty"`
	OnlineCount     int         `json:"onlineCount,omitempty"`
	Mode            string      `json:"mode,omitempty"`
	SDP             string      `json:"sdp,omitempty"`
	Candidate       any         `json:"candidate,omitempty"`
	IsInitiator     bool        `json:"isInitiator,omitempty"`
	MediaState      *MediaState `json:"mediaState,omitempty"`
}

// Session represents an active chat session between two matched users.
type Session struct {
	User1           string
	User2           string
	CommonInterests []string
	Mode            string
	Initiator       string

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
