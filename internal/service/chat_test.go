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
