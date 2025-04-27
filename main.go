package main

import (
	"fmt"
	"net/http"
	"time" // Added for shutdown timeout

	"github.com/lguibr/bollywood" // Import bollywood
	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/server"
	"github.com/lguibr/pongo/utils" // Import utils
	"golang.org/x/net/websocket"
)

var port = ":3001"

func main() {
	// 0. Load Configuration
	// For now, using default config. Later, could load from file.
	cfg := utils.DefaultConfig()
	fmt.Println("Configuration loaded (using defaults).")
	fmt.Printf("Canvas Size: %d, Grid Size: %d, Tick Period: %v\n", cfg.CanvasSize, cfg.GridSize, cfg.GameTickPeriod)

	// 1. Initialize Bollywood Engine
	engine := bollywood.NewEngine()
	fmt.Println("Bollywood Engine created.")

	// 2. Spawn the GameActor, passing the config
	gameActorProps := bollywood.NewProps(game.NewGameActorProducer(engine, cfg)) // Pass cfg
	gameActorPID := engine.Spawn(gameActorProps)
	if gameActorPID == nil {
		panic("Failed to spawn GameActor")
	}
	fmt.Printf("GameActor spawned with PID: %s\n", gameActorPID)

	// Allow GameActor to start its ticker etc.
	time.Sleep(50 * time.Millisecond)

	// 3. Create the HTTP/WebSocket Server
	// Pass engine and gameActorPID to the server or handlers
	websocketServer := server.New(engine, gameActorPID) // Modify server.New
	fmt.Println("WebSocket Server created.")

	// 4. Setup Handlers (pass engine and gameActorPID)
	http.HandleFunc("/", websocketServer.HandleGetSit())                            // Modify HandleGetSit
	http.Handle("/subscribe", websocket.Handler(websocketServer.HandleSubscribe())) // Modify HandleSubscribe

	// 5. Start Server
	fmt.Println("Server starting on port", port)
	err := http.ListenAndServe(port, nil)
	if err != nil {
		// Handle shutdown gracefully
		fmt.Println("Server stopped:", err)
		fmt.Println("Shutting down engine...")
		engine.Shutdown(5 * time.Second) // Allow actors time to stop
		fmt.Println("Engine shutdown complete.")
	}
}
