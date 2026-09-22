package websocket

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/sohaib_04/GOmegle/internal/models"
)

const (
	// writeWait is the time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// pongWait is the time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// pingPeriod sends pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// maxMessageSize is the maximum message size allowed from peer (128 KB for WebRTC SDP payloads).
	maxMessageSize = 131072
)

// Client represents a single WebSocket connection.
type Client struct {
	// ID is a unique identifier for this client (UUID).
	ID string

	// Conn is the underlying WebSocket connection.
	Conn *websocket.Conn

	// Send is a buffered channel of outbound messages.
	Send chan []byte

	// Hub is the connection registry this client belongs to.
	Hub *Hub

	closed  bool
	closeMu sync.RWMutex
}

// NewClient creates a new Client instance.
func NewClient(id string, conn *websocket.Conn, hub *Hub) *Client {
	return &Client{
		ID:   id,
		Conn: conn,
		Send: make(chan []byte, 256),
		Hub:  hub,
	}
}

// SendMessage safely queues a message to the Send channel without panicking if closed.
func (c *Client) SendMessage(data []byte) bool {
	c.closeMu.RLock()
	defer c.closeMu.RUnlock()

	if c.closed {
		return false
	}

	select {
	case c.Send <- data:
		return true
	default:
		log.Printf("client %s send buffer full, dropping message", c.ID)
		return false
	}
}

// Close safely marks the client as closed and closes the Send channel.
func (c *Client) Close() {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()

	if !c.closed {
		c.closed = true
		close(c.Send)
	}
}

// ReadPump pumps messages from the WebSocket connection to the Hub.
//
// The application runs ReadPump in a per-connection goroutine. It ensures
// that there is at most one reader on a connection by executing all reads
// from this goroutine.
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, rawMsg, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("websocket read error for client %s: %v", c.ID, err)
			}
			break
		}

		var msg models.Message
		if err := json.Unmarshal(rawMsg, &msg); err != nil {
			log.Printf("invalid message from client %s: %v", c.ID, err)
			continue
		}

		c.Hub.Incoming <- &IncomingMessage{
			ClientID: c.ID,
			Message:  msg,
		}
	}
}

// WritePump pumps messages from the Send channel to the WebSocket connection.
//
// A goroutine running WritePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The Hub closed the channel.
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("websocket write error for client %s: %v", c.ID, err)
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
