package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/sohaib_04/GOmegle/internal/matchmaking"
	"github.com/sohaib_04/GOmegle/internal/models"
	"github.com/sohaib_04/GOmegle/internal/websocket"
)

func TestChatServiceSignalingRelay(t *testing.T) {
	hub := websocket.NewHub()
	go hub.Run()

	queue := matchmaking.NewQueue()
	cs := NewChatService(hub, queue)
	cs.Start()

	// Register two mock clients with channels
	client1 := &websocket.Client{
		ID:   "peer1",
		Send: make(chan []byte, 10),
		Hub:  hub,
	}
	client2 := &websocket.Client{
		ID:   "peer2",
		Send: make(chan []byte, 10),
		Hub:  hub,
	}

	hub.Register <- client1
	hub.Register <- client2
	time.Sleep(20 * time.Millisecond)

	// Enqueue both in video mode
	cs.EnqueueUser("peer1", []string{"art"}, "video")
	cs.EnqueueUser("peer2", []string{"art"}, "video")

	// Both should receive matched system message
	var msg1, msg2 models.Message
	select {
	case raw := <-client1.Send:
		if err := json.Unmarshal(raw, &msg1); err != nil {
			t.Fatalf("Failed to unmarshal client1 message: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for client1 matched message")
	}

	select {
	case raw := <-client2.Send:
		if err := json.Unmarshal(raw, &msg2); err != nil {
			t.Fatalf("Failed to unmarshal client2 message: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for client2 matched message")
	}

	// Verify one is initiator and the other is not
	if msg1.IsInitiator == msg2.IsInitiator {
		t.Fatalf("Expected one initiator and one non-initiator, got %v and %v", msg1.IsInitiator, msg2.IsInitiator)
	}

	// Verify mode is video
	if msg1.Mode != "video" || msg2.Mode != "video" {
		t.Fatalf("Expected mode 'video', got %s and %s", msg1.Mode, msg2.Mode)
	}

	// Test WebRTC Offer relay from peer1 to peer2
	offerMsg := models.Message{
		Type: models.MessageTypeOffer,
		SDP:  "v=0\r\no=- 12345 2 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n",
	}
	cs.HandleMessage("peer1", offerMsg)

	select {
	case raw := <-client2.Send:
		var rcv models.Message
		if err := json.Unmarshal(raw, &rcv); err != nil {
			t.Fatalf("Failed to unmarshal offer message: %v", err)
		}
		if rcv.Type != models.MessageTypeOffer || rcv.SDP != offerMsg.SDP {
			t.Fatalf("Unexpected offer relay content: %+v", rcv)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("Timeout waiting for offer relay to peer2")
	}

	// Test ICE candidate relay from peer2 to peer1
	candidateMsg := models.Message{
		Type:      models.MessageTypeCandidate,
		Candidate: map[string]any{"candidate": "candidate:1 1 UDP 2122252543 ...", "sdpMLineIndex": 0},
	}
	cs.HandleMessage("peer2", candidateMsg)

	select {
	case raw := <-client1.Send:
		var rcv models.Message
		if err := json.Unmarshal(raw, &rcv); err != nil {
			t.Fatalf("Failed to unmarshal candidate message: %v", err)
		}
		if rcv.Type != models.MessageTypeCandidate {
			t.Fatalf("Unexpected candidate relay type: %s", rcv.Type)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("Timeout waiting for candidate relay to peer1")
	}
}

func TestChatServiceTextMessageRelay(t *testing.T) {
	hub := websocket.NewHub()
	go hub.Run()

	queue := matchmaking.NewQueue()
	cs := NewChatService(hub, queue)
	cs.Start()

	client1 := websocket.NewClient("user_a", nil, hub)
	client2 := websocket.NewClient("user_b", nil, hub)

	hub.Register <- client1
	hub.Register <- client2
	time.Sleep(20 * time.Millisecond)

	cs.EnqueueUser("user_a", nil, "text")
	cs.EnqueueUser("user_b", nil, "text")

	// Drain matched system messages
	<-client1.Send
	<-client2.Send

	// User A sends chat message
	cs.HandleMessage("user_a", models.Message{
		Type:    models.MessageTypeChat,
		Content: "Hello from user A",
	})

	select {
	case raw := <-client2.Send:
		var rcv models.Message
		if err := json.Unmarshal(raw, &rcv); err != nil {
			t.Fatalf("Failed to unmarshal message: %v", err)
		}
		if rcv.Type != models.MessageTypeChat || rcv.Content != "Hello from user A" {
			t.Fatalf("Unexpected chat message received: %+v", rcv)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("Timeout waiting for chat message to reach user_b")
	}
}

func TestChatServiceDisconnectAndReMatch(t *testing.T) {
	hub := websocket.NewHub()
	go hub.Run()

	queue := matchmaking.NewQueue()
	cs := NewChatService(hub, queue)
	cs.Start()

	client1 := websocket.NewClient("user1", nil, hub)
	client2 := websocket.NewClient("user2", nil, hub)

	hub.Register <- client1
	hub.Register <- client2
	time.Sleep(20 * time.Millisecond)

	// Initial match
	cs.EnqueueUser("user1", nil, "text")
	cs.EnqueueUser("user2", nil, "text")

	<-client1.Send
	<-client2.Send

	// User 1 disconnects cleanly
	cs.HandleMessage("user1", models.Message{Type: models.MessageTypeDisconnect})

	// User 2 should receive "Stranger has disconnected."
	select {
	case raw := <-client2.Send:
		var rcv models.Message
		if err := json.Unmarshal(raw, &rcv); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}
		if rcv.Type != models.MessageTypeSystem || rcv.Content != "Stranger has disconnected." {
			t.Fatalf("Expected stranger disconnected notice, got %+v", rcv)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("Timeout waiting for disconnect notice to user2")
	}

	// Verify neither user is stuck in a session
	cs.mu.RLock()
	s1 := cs.sessions["user1"]
	s2 := cs.sessions["user2"]
	cs.mu.RUnlock()
	if s1 != nil || s2 != nil {
		t.Fatalf("Expected nil sessions after disconnect, got s1=%v, s2=%v", s1, s2)
	}

	// Now User 1 and User 2 request a new match via find_match
	cs.HandleMessage("user1", models.Message{Type: models.MessageTypeFindMatch})
	cs.HandleMessage("user2", models.Message{Type: models.MessageTypeFindMatch})

	// Both should receive new matched messages
	select {
	case raw := <-client1.Send:
		var rcv models.Message
		json.Unmarshal(raw, &rcv)
		if rcv.Type != models.MessageTypeSystem {
			t.Fatalf("Expected system matched message, got %+v", rcv)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for user1 re-match")
	}

	select {
	case raw := <-client2.Send:
		var rcv models.Message
		json.Unmarshal(raw, &rcv)
		if rcv.Type != models.MessageTypeSystem {
			t.Fatalf("Expected system matched message, got %+v", rcv)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for user2 re-match")
	}
}

func TestChatServicePreSessionDisconnect(t *testing.T) {
	hub := websocket.NewHub()
	go hub.Run()

	queue := matchmaking.NewQueue()
	cs := NewChatService(hub, queue)
	cs.Start()

	// Only register client2 in the Hub. client1 is disconnected.
	client2 := websocket.NewClient("surviving_user", nil, hub)
	hub.Register <- client2
	time.Sleep(20 * time.Millisecond)

	// createSession called with a disconnected user1 and connected user2
	cs.createSession("ghost_user", "surviving_user", nil, "text")

	// surviving_user should NOT have an active session with ghost_user
	cs.mu.RLock()
	s := cs.sessions["surviving_user"]
	cs.mu.RUnlock()
	if s != nil {
		t.Fatalf("Expected no session created with disconnected ghost user, got %+v", s)
	}

	// surviving_user should have been safely re-enqueued
	if queue.QueueSize() != 1 {
		t.Fatalf("Expected surviving user to be re-enqueued (queue size 1), got %d", queue.QueueSize())
	}
}
