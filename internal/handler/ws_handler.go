package handler

import (
	"log"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/sohaib_04/GOmegle/internal/service"
	ws "github.com/sohaib_04/GOmegle/internal/websocket"
)

// WSHandler handles incoming WebSocket connections.
type WSHandler struct {
	hub         *ws.Hub
	chatService *service.ChatService
}

// NewWSHandler creates a new WSHandler.
func NewWSHandler(hub *ws.Hub, chatService *service.ChatService) *WSHandler {
	return &WSHandler{
		hub:         hub,
		chatService: chatService,
	}
}

// UpgradeMiddleware returns Fiber middleware that checks if the request
// is a WebSocket upgrade request before passing it to the WebSocket handler.
func (h *WSHandler) UpgradeMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	}
}

// HandleWebSocket returns the WebSocket handler function.
// Each new connection creates a Client, registers it with the Hub,
// starts its read/write pumps, and enqueues it for matchmaking.
func (h *WSHandler) HandleWebSocket() fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		// Generate a unique ID for this connection.
		clientID := uuid.New().String()
		log.Printf("handler: new WebSocket connection: %s", clientID)

		// Create the client.
		client := ws.NewClient(clientID, c, h.hub)

		// Register with the Hub.
		h.hub.Register <- client

		// Start the write pump in a goroutine.
		go client.WritePump()

		// Enqueue for matchmaking.
		h.chatService.EnqueueUser(clientID)

		// ReadPump blocks until the connection closes.
		// This is intentional — the Fiber WebSocket handler expects
		// the function to block for the lifetime of the connection.
		client.ReadPump()
	})
}
