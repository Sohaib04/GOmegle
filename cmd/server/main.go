package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/sohaib_04/GOmegle/internal/handler"
	"github.com/sohaib_04/GOmegle/internal/matchmaking"
	"github.com/sohaib_04/GOmegle/internal/service"
	"github.com/sohaib_04/GOmegle/internal/websocket"
)

func main() {
	// --- 1. Initialize core components ---
	hub := websocket.NewHub()
	queue := matchmaking.NewQueue()
	chatService := service.NewChatService(hub, queue)
	wsHandler := handler.NewWSHandler(hub, chatService)

	// --- 2. Start background goroutines ---
	go hub.Run()
	chatService.Start()

	// --- 3. Create Fiber app with middleware ---
	app := fiber.New(fiber.Config{
		AppName: "GOmegle v2.0.0",
	})

	app.Use(logger.New())
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	// --- 4. API routes ---
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "GOmegle",
			"version": "2.0.0",
		})
	})

	app.Get("/api/stats", wsHandler.HandleStats())

	// --- 5. WebSocket route ---
	app.Use("/ws", wsHandler.UpgradeMiddleware())
	app.Get("/ws", wsHandler.HandleWebSocket())

	// --- 6. Serve static frontend files ---
	app.Static("/", "./web", fiber.Static{
		Index:  "index.html",
		Browse: false,
	})

	// --- 7. Graceful shutdown ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("shutting down server...")
		if err := app.Shutdown(); err != nil {
			log.Fatalf("server shutdown error: %v", err)
		}
	}()

	// --- 8. Start listening ---
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("GOmegle server starting on :%s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
