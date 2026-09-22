package matchmaking

import (
	"log"
	"sync"
)

// QueueEntry represents a user waiting in the matchmaking queue.
type QueueEntry struct {
	UserID    string
	Interests []string
	Mode      string
}

// MatchedPair represents two users that have been matched.
type MatchedPair struct {
	User1           string
	User2           string
	CommonInterests []string
	Mode            string
}

// Queue is a thread-safe matchmaking queue that pairs users for chat sessions.
//
// When a user is enqueued and another user is already waiting in the same mode
// (text or video), the queue attempts to find the best match based on shared
// interests. If no interest overlap exists, it falls back to FIFO matching
// within that mode.
type Queue struct {
	mu      sync.Mutex
	queue   []QueueEntry
	matched chan MatchedPair
}

// NewQueue creates a new matchmaking Queue.
func NewQueue() *Queue {
	return &Queue{
		queue:   make([]QueueEntry, 0),
		matched: make(chan MatchedPair, 64),
	}
}

// Enqueue adds a user to the matchmaking queue with optional interests and mode.
// If a compatible match with the same mode is found, the pair is immediately
// sent to the matched channel.
func (q *Queue) Enqueue(userID string, interests []string, mode string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if mode == "" {
		mode = "text"
	}

	// Don't add duplicates.
	for _, entry := range q.queue {
		if entry.UserID == userID {
			log.Printf("matchmaking: user %s already in queue, skipping", userID)
			return
		}
	}

	// Try to find the best match with the same mode.
	bestIdx := -1
	bestOverlap := 0

	for i, entry := range q.queue {
		if entry.Mode != mode || entry.UserID == userID {
			continue
		}

		overlap := countOverlap(interests, entry.Interests)
		if overlap > bestOverlap {
			bestOverlap = overlap
			bestIdx = i
		}
	}

	// If no interest match found within the same mode, fall back to FIFO for this mode.
	if bestIdx == -1 {
		for i, entry := range q.queue {
			if entry.Mode == mode && entry.UserID != userID {
				bestIdx = i
				break
			}
		}
	}

	// If no match found in this mode, enqueue the user.
	if bestIdx == -1 {
		q.queue = append(q.queue, QueueEntry{UserID: userID, Interests: interests, Mode: mode})
		log.Printf("matchmaking: user %s enqueued for mode %s (queue size: %d)", userID, mode, len(q.queue))
		return
	}

	// Match the two users.
	partner := q.queue[bestIdx]
	q.queue = append(q.queue[:bestIdx], q.queue[bestIdx+1:]...)
	common := findCommonInterests(interests, partner.Interests)

	log.Printf("matchmaking: matched %s with %s in mode %s (common: %v, queue size: %d)",
		userID, partner.UserID, mode, common, len(q.queue))

	pair := MatchedPair{
		User1:           partner.UserID,
		User2:           userID,
		CommonInterests: common,
		Mode:            mode,
	}

	// Non-blocking send — the channel has a buffer.
	select {
	case q.matched <- pair:
	default:
		log.Printf("matchmaking: matched channel full, re-enqueuing %s and %s",
			partner.UserID, userID)
		q.queue = append(q.queue, partner, QueueEntry{UserID: userID, Interests: interests, Mode: mode})
	}
}

// Dequeue removes a user from the matchmaking queue (e.g., if they disconnect
// before being matched). Returns true if the user was found and removed.
func (q *Queue) Dequeue(userID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, entry := range q.queue {
		if entry.UserID == userID {
			q.queue = append(q.queue[:i], q.queue[i+1:]...)
			log.Printf("matchmaking: user %s dequeued (queue size: %d)", userID, len(q.queue))
			return true
		}
	}
	return false
}

// MatchedPairs returns a read-only channel that emits matched pairs.
// The ChatService should consume from this channel to create sessions.
func (q *Queue) MatchedPairs() <-chan MatchedPair {
	return q.matched
}

// QueueSize returns the current number of users waiting in the queue.
func (q *Queue) QueueSize() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.queue)
}

// countOverlap returns the number of common elements between two string slices.
func countOverlap(a, b []string) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	set := make(map[string]bool, len(a))
	for _, s := range a {
		set[s] = true
	}
	count := 0
	for _, s := range b {
		if set[s] {
			count++
		}
	}
	return count
}

// findCommonInterests returns the intersection of two string slices.
func findCommonInterests(a, b []string) []string {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	set := make(map[string]bool, len(a))
	for _, s := range a {
		set[s] = true
	}
	var common []string
	for _, s := range b {
		if set[s] {
			common = append(common, s)
		}
	}
	return common
}
