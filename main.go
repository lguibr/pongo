// File: main.go
package main

import (
	"fmt"
	"net/http"
	"time" // Added for shutdown timeout

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/server"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

var port = ":3001"

func main() {
	// 0. Load Configuration
	cfg := utils.DefaultConfig()
	fmt.Println("Configuration loaded (using defaults).")
	fmt.Printf("Canvas Size: %d, Grid Size: %d, Tick Period: %v\n", cfg.CanvasSize, cfg.GridSize, cfg.GameTickPeriod)

	// 1. Initialize Bollywood Engine
	engine := bollywood.NewEngine()
	fmt.Println("Bollywood Engine created.")

	// 2. Spawn the RoomManagerActor, passing the config
	roomManagerProps := bollywood.NewProps(game.NewRoomManagerProducer(engine, cfg)) // Pass cfg
	roomManagerPID := engine.Spawn(roomManagerProps)
	if roomManagerPID == nil {
		panic("Failed to spawn RoomManagerActor")
	}
	fmt.Printf("RoomManagerActor spawned with PID: %s\n", roomManagerPID)

	// Allow RoomManagerActor to start
	time.Sleep(50 * time.Millisecond)

	// 3. Create the HTTP/WebSocket Server
	// Pass engine and roomManagerPID to the server
	websocketServer := server.New(engine, roomManagerPID) // Pass RoomManager PID
	fmt.Println("WebSocket Server created.")

	// 4. Setup Handlers (pass engine and roomManagerPID via server instance)
	http.HandleFunc("/", server.HandleHealthCheck())                                // Simple health check at root
	http.HandleFunc("/health-check/", server.HandleHealthCheck())                   // Explicit health check endpoint
	http.HandleFunc("/rooms/", websocketServer.HandleGetRooms())                    // Get room list
	http.Handle("/subscribe", websocket.Handler(websocketServer.HandleSubscribe())) // WebSocket connections

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
