package matchmaking

import (
	"testing"
	"time"
)

func TestQueueModeIsolation(t *testing.T) {
	q := NewQueue()

	// Enqueue a video user and a text user
	q.Enqueue("user_text", []string{"coding", "music"}, "text")
	q.Enqueue("user_video", []string{"coding", "music"}, "video")

	// Neither should have been matched because modes differ despite shared interests
	select {
	case pair := <-q.MatchedPairs():
		t.Fatalf("Unexpected match between different modes: %+v", pair)
	case <-time.After(50 * time.Millisecond):
		// Expected
	}

	if q.QueueSize() != 2 {
		t.Fatalf("Expected queue size 2, got %d", q.QueueSize())
	}

	// Now enqueue a second video user
	q.Enqueue("user_video_2", []string{"gaming"}, "video")

	select {
	case pair := <-q.MatchedPairs():
		if pair.Mode != "video" {
			t.Fatalf("Expected matched pair mode 'video', got %s", pair.Mode)
		}
		if (pair.User1 != "user_video" && pair.User1 != "user_video_2") ||
			(pair.User2 != "user_video" && pair.User2 != "user_video_2") {
			t.Fatalf("Unexpected matched pair users: %+v", pair)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected video match timed out")
	}

	// Text user should still be in the queue
	if q.QueueSize() != 1 {
		t.Fatalf("Expected queue size 1 after matching video users, got %d", q.QueueSize())
	}

	// Enqueue second text user
	q.Enqueue("user_text_2", []string{"music"}, "text")

	select {
	case pair := <-q.MatchedPairs():
		if pair.Mode != "text" {
			t.Fatalf("Expected matched pair mode 'text', got %s", pair.Mode)
		}
		if len(pair.CommonInterests) != 1 || pair.CommonInterests[0] != "music" {
			t.Fatalf("Expected common interest 'music', got %v", pair.CommonInterests)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected text match timed out")
	}
}

func TestQueueDequeue(t *testing.T) {
	q := NewQueue()
	q.Enqueue("u1", nil, "video")
	if q.QueueSize() != 1 {
		t.Fatalf("Expected queue size 1, got %d", q.QueueSize())
	}

	removed := q.Dequeue("u1")
	if !removed {
		t.Fatal("Expected Dequeue to return true for existing user")
	}
	if q.QueueSize() != 0 {
		t.Fatalf("Expected queue size 0 after dequeue, got %d", q.QueueSize())
	}
}
