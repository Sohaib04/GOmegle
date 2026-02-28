package matchmaking

import (
	"log"
	"sync"
)

// Queue is a thread-safe matchmaking queue that pairs users for chat sessions.
//
// When a second user is enqueued, a pair is immediately formed and sent to the
// matched channel. The consumer (ChatService) listens on this channel to create
// sessions.
type Queue struct {
	mu      sync.Mutex
	queue   []string
	matched chan [2]string
}

// NewQueue creates a new matchmaking Queue.
func NewQueue() *Queue {
	return &Queue{
		queue:   make([]string, 0),
		matched: make(chan [2]string, 64),
	}
}

// Enqueue adds a user to the matchmaking queue.
// If another user is already waiting, they are immediately matched and
// the pair is sent to the matched channel.
func (q *Queue) Enqueue(userID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Don't add duplicates.
	for _, id := range q.queue {
		if id == userID {
			log.Printf("matchmaking: user %s already in queue, skipping", userID)
			return
		}
	}

	q.queue = append(q.queue, userID)
	log.Printf("matchmaking: user %s enqueued (queue size: %d)", userID, len(q.queue))

	// If we have at least 2 users, match them immediately.
	if len(q.queue) >= 2 {
		user1 := q.queue[0]
		user2 := q.queue[1]
		q.queue = q.queue[2:]
		log.Printf("matchmaking: matched %s with %s (queue size: %d)", user1, user2, len(q.queue))

		// Non-blocking send — the channel has a buffer.
		select {
		case q.matched <- [2]string{user1, user2}:
		default:
			log.Printf("matchmaking: matched channel full, re-enqueuing %s and %s", user1, user2)
			q.queue = append([]string{user1, user2}, q.queue...)
		}
	}
}

// Dequeue removes a user from the matchmaking queue (e.g., if they disconnect
// before being matched). Returns true if the user was found and removed.
func (q *Queue) Dequeue(userID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, id := range q.queue {
		if id == userID {
			q.queue = append(q.queue[:i], q.queue[i+1:]...)
			log.Printf("matchmaking: user %s dequeued (queue size: %d)", userID, len(q.queue))
			return true
		}
	}
	return false
}

// MatchedPairs returns a read-only channel that emits matched pairs.
// The ChatService should consume from this channel to create sessions.
func (q *Queue) MatchedPairs() <-chan [2]string {
	return q.matched
}
