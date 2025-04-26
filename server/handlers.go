// File: server/handlers.go
package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime/debug" // Import debug package

	"github.com/lguibr/pongo/bollywood" // Import bollywood
	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/utils"

	"golang.org/x/net/websocket"
)

// HandleSubscribe sets up the WebSocket connection and manages its read loop.
func (s *Server) HandleSubscribe(g *game.Game) func(ws *websocket.Conn) {
	return func(ws *websocket.Conn) {
		// Add panic recovery for the entire handler
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("PANIC recovered in HandleSubscribe for %s: %v\nStack trace:\n%s\n", ws.RemoteAddr(), r, string(debug.Stack()))
				// Ensure connection is closed if a panic occurred
				s.CloseConnection(ws) // Use the server's method for cleanup
			}
		}()

		fmt.Println("WebSocket connection established from:", ws.RemoteAddr())
		s.OpenConnection(ws) // Track the connection

		// --- Setup ---
		playerIndex := g.GetNextIndex() // Needs locking in Game
		if playerIndex == -1 {
			fmt.Println("Server full, rejecting connection from:", ws.RemoteAddr())
			_ = ws.Close()        // Try to close gracefully
			s.CloseConnection(ws) // Ensure it's removed from tracking
			return
		}
		fmt.Printf("Assigning player index %d to %s\n", playerIndex, ws.RemoteAddr())

		stopCh := make(chan struct{}) // Channel to signal writer to stop
		var player *game.Player       // Declare player reference
		var paddlePID *bollywood.PID // PID for the PaddleActor

		// Coordinated close function
		coordinatedClose := func() {
			fmt.Printf("Coordinated close triggered for player %d (%s)\n", playerIndex, ws.RemoteAddr())
			// Use non-blocking close for stopCh
			select {
			case <-stopCh: // Already closed
				fmt.Printf("Stop channel already closed for player %d\n", playerIndex)
			default:
				close(stopCh)
				fmt.Printf("Stop channel closed for player %d\n", playerIndex)
			}
			s.CloseConnection(ws) // Close the actual websocket via server method
		}

		// Start game logic components (player state, paddle actor, ball, writer)
		fmt.Printf("Calling LifeCycle for player %d (%s)...\n", playerIndex, ws.RemoteAddr()) // Log before call
		lifecycleData, err := g.LifeCycle(ws, playerIndex, stopCh, coordinatedClose)
		if err != nil {
			fmt.Printf("Error during LifeCycle setup for %s: %v\n", ws.RemoteAddr(), err)
			coordinatedClose() // Ensure cleanup happens
			return
		}
		// Assign values after successful LifeCycle call
		paddlePID = lifecycleData.PaddlePID // Get PID from result
		player = lifecycleData.Player
		fmt.Printf("LifeCycle call successful for player %d (%s). Paddle PID: %s\n", playerIndex, ws.RemoteAddr(), paddlePID)

		// --- Read Loop ---
		fmt.Printf("Starting read loop for player %d (%s).\n", playerIndex, ws.RemoteAddr()) // Log before loop
		defer func() {
			fmt.Printf("Exiting read loop for player %d (%s). Triggering cleanup.\n", playerIndex, ws.RemoteAddr())
			if player != nil {
				player.Disconnect() // Signal game logic about disconnect (e.g., to GameActor)
			}
			// TODO: Need to stop the PaddleActor using engine.Stop(paddlePID)
			fmt.Printf("WARNING: PaddleActor %s for player %d needs to be stopped explicitly.\n", paddlePID, playerIndex)
			coordinatedClose() // Close connection and signal writer
			fmt.Printf("Cleanup finished for player %d (%s)\n", playerIndex, ws.RemoteAddr())
		}()

		for {
			buffer := make([]byte, 1024)
			// Set read deadline? ws.SetReadDeadline(...)
			size, err := ws.Read(buffer)

			if err != nil {
				// Log specific error before returning
				if err == io.EOF {
					fmt.Printf("Read loop: Player %d (%s) disconnected (EOF).\n", playerIndex, ws.RemoteAddr())
				} else {
					fmt.Printf("Read loop: Error reading from player %d (%s): %v\n", playerIndex, ws.RemoteAddr(), err)
				}
				return // Exit loop, defer handles cleanup
			}

			// Process the received message (Paddle Direction)
			receivedData := buffer[:size]
			
			// TODO: Need reference to the main bollywood.Engine instance (e.g., s.Engine)
			// engine := s.Engine // Assuming server 's' holds the engine
			
			var dirMsg game.Direction // Parse to validate message format
			if err := json.Unmarshal(receivedData, &dirMsg); err == nil {
				internalDir := utils.DirectionFromString(dirMsg.Direction)
				if internalDir != "" { // Check if direction is valid ("left" or "right")
					// Send the raw JSON bytes as the message payload to the specific PaddleActor
					// The actor's Receive method will handle the unmarshaling again.
					// Example: engine.Send(paddlePID, game.PaddleDirectionMessage{Direction: receivedData}, nil)
					fmt.Printf("[TODO: Engine Needed] Read loop: Player %d would send direction '%s' (%d bytes) to PaddleActor %s\n", playerIndex, internalDir, len(receivedData), paddlePID)
					// *** Replace above fmt.Printf with actual engine.Send call below when engine is available ***
					// engine.Send(paddlePID, game.PaddleDirectionMessage{Direction: receivedData}, nil) // Sender is nil (from outside actor system)
					
				} // Ignore unknown directions
			} // Ignore invalid JSON
		}
		// --- End Read Loop ---
	}
}

// HandleGetSit provides the current game state via HTTP GET.
func (s *Server) HandleGetSit(g *game.Game) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Add panic recovery
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("PANIC recovered in HandleGetSit: %v\nStack trace:\n%s\n", r, string(debug.Stack()))
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		gameState := g.ToJson() // ToJson now includes locking
		if len(gameState) <= 2 { // Check for empty "{}" from marshalling error
			http.Error(w, "Error generating game state", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, err := io.WriteString(w, string(gameState))
		if err != nil {
			fmt.Println("Error writing HTTP game state:", err)
		}
	}
}
