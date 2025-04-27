// File: server/handlers.go
package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime/debug" // Import debug package

	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/utils"

	"golang.org/x/net/websocket"
)

// HandleSubscribe sets up the WebSocket connection and forwards it to the GameActor.
func (s *Server) HandleSubscribe() func(ws *websocket.Conn) {
	return func(ws *websocket.Conn) {
		connectionAddr := ws.RemoteAddr().String() // Get address for logging
		fmt.Printf("HandleSubscribe: New connection attempt from %s\n", connectionAddr)

		// Add panic recovery for the entire handler
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("PANIC recovered in HandleSubscribe for %s: %v\nStack trace:\n%s\n", connectionAddr, r, string(debug.Stack()))
				// Attempt to close the raw connection on panic here, as GameActor might not know about it yet.
				_ = ws.Close()
			}
		}()

		// Ensure server components are valid before proceeding
		engine := s.GetEngine()
		gameActorPID := s.GetGameActorPID()
		if engine == nil || gameActorPID == nil {
			fmt.Printf("HandleSubscribe: Server engine or GameActorPID is nil. Closing connection %s.\n", connectionAddr)
			_ = ws.Close()
			return
		}

		// Send connect request to GameActor immediately
		// Pass the concrete *websocket.Conn type which satisfies the PlayerConnection interface
		connectMsg := game.PlayerConnectRequest{WsConn: ws}
		engine.Send(gameActorPID, connectMsg, nil) // Sender is nil (system)

		// Start the read loop for this connection in a separate goroutine
		// The read loop now only signals disconnects, GameActor handles cleanup.
		go s.readLoop(ws)

		fmt.Printf("HandleSubscribe: Setup complete for %s. Read loop started.\n", connectionAddr)
	}
}

// readLoop handles reading messages from a single WebSocket connection.
// It only sends disconnect signals to the GameActor upon error or EOF.
func (s *Server) readLoop(ws *websocket.Conn) {
	connectionAddr := ws.RemoteAddr().String()
	fmt.Printf("ReadLoop: Starting for %s.\n", connectionAddr)

	// Ensure server components are valid before starting loop
	engine := s.GetEngine()
	gameActorPID := s.GetGameActorPID()
	if engine == nil || gameActorPID == nil {
		fmt.Printf("ReadLoop: Server engine or GameActorPID is nil. Aborting read loop for %s.\n", connectionAddr)
		// Don't close ws here, let HandleSubscribe potentially handle it if it panics early.
		// If HandleSubscribe succeeded, GameActor will eventually clean up via timeout or other means.
		return
	}

	// Defer sending disconnect message ONCE when loop exits for any reason.
	var disconnectSent bool
	defer func() {
		if !disconnectSent {
			fmt.Printf("ReadLoop: Exiting for %s. Sending disconnect signal.\n", connectionAddr)
			// Send disconnect message with the connection object
			disconnectMsg := game.PlayerDisconnect{PlayerIndex: -1, WsConn: ws} // Index is unknown here
			engine.Send(gameActorPID, disconnectMsg, nil)
			disconnectSent = true // Mark as sent
		}
		fmt.Printf("ReadLoop: Finished for %s.\n", connectionAddr)
	}()

	// Main Read Loop
	for {
		buffer := make([]byte, 512)
		size, err := ws.Read(buffer)

		if err != nil {
			// Simplified error checking for golang.org/x/net/websocket
			if err == io.EOF {
				fmt.Printf("ReadLoop: Connection %s disconnected (EOF).\n", connectionAddr)
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				fmt.Printf("ReadLoop: Read timeout for %s.\n", connectionAddr)
			} else {
				// Assume any other error means the connection is broken or closed
				fmt.Printf("ReadLoop: Error/Connection closed for %s: %v\n", connectionAddr, err)
			}
			return // Exit loop, defer sends disconnect signal
		}

		// Process the received message (Paddle Direction)
		receivedData := buffer[:size]

		var dirMsg game.Direction
		if err := json.Unmarshal(receivedData, &dirMsg); err == nil {
			internalDir := utils.DirectionFromString(dirMsg.Direction)
			if internalDir != "" {
				// Send the message with the connection object
				forwardMsg := game.ForwardedPaddleDirection{
					WsConn:    ws, // Pass the connection
					Direction: receivedData,
				}
				engine.Send(gameActorPID, forwardMsg, nil)
			}
		}
	}
}

// HandleGetSit provides the current game state via HTTP GET by querying the GameActor.
func (s *Server) HandleGetSit() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				fmt.Printf("PANIC recovered in HandleGetSit: %v\nStack trace:\n%s\n", rec, string(debug.Stack()))
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		// Get GameActor state (assuming GameActor has a method to expose this)
		// This requires accessing the GameActor instance, which isn't directly
		// available here without modification. A common pattern is to send a
		// message to the GameActor and wait for a response, but that adds complexity.
		// For simplicity, we'll assume the GameActor updates an atomic value
		// that the Server can access (needs modification in GameActor).

		// Placeholder: Query GameActor (Needs GameActor modification)
		// For now, we keep the placeholder. A real implementation would involve:
		// 1. Add a method to GameActor like `GetGameStateJSON() []byte` that reads its atomic value.
		// 2. The Server needs access to the GameActor instance or its PID to call this.
		//    Since we only have the PID, direct access isn't possible without changing Server struct
		//    or using message passing (which is complex for HTTP GET).
		// Let's assume GameActor exposes state via an atomic value accessible through a (hypothetical) method on the engine or server.
		// gameState := s.GetGameStateFromActor() // Hypothetical

		// Using the previous placeholder as direct query isn't straightforward with actors
		fmt.Println("HandleGetSit: Returning placeholder state (GameActor query not implemented)")
		gameState := []byte(`{"error": "Live state query not implemented via HTTP GET in actor model"}`)

		w.Header().Set("Content-Type", "application/json")
		if len(gameState) <= 2 { // Basic check for empty/error JSON
			http.Error(w, "Error generating game state", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(gameState)
		if err != nil {
			fmt.Println("Error writing HTTP game state:", err)
		}
	}
}
