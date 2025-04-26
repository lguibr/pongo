package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime/debug" // Import debug package
	"time"

	// "errors" // No longer needed for websocket error checking here

	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/utils"

	"golang.org/x/net/websocket"
)

// HandleSubscribe sets up the WebSocket connection and manages its read loop.
func (s *Server) HandleSubscribe() func(ws *websocket.Conn) {
	return func(ws *websocket.Conn) {
		connectionAddr := ws.RemoteAddr().String() // Get address for logging
		fmt.Printf("HandleSubscribe: New connection attempt from %s\n", connectionAddr)

		// Add panic recovery for the entire handler
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("PANIC recovered in HandleSubscribe for %s: %v\nStack trace:\n%s\n", connectionAddr, r, string(debug.Stack()))
				s.CloseConnection(ws) // Ensure cleanup via server method
			}
		}()

		// Track the connection and get the stop channel for its reader
		stopReadCh := s.OpenConnection(ws)

		// Send connect request to GameActor
		// Pass the concrete *websocket.Conn type which satisfies the PlayerConnection interface
		connectMsg := game.PlayerConnectRequest{WsConn: ws}
		s.GetEngine().Send(s.GetGameActorPID(), connectMsg, nil) // Sender is nil (system)

		// Start the read loop for this connection in a separate goroutine
		go s.readLoop(ws, stopReadCh)

		fmt.Printf("HandleSubscribe: Setup complete for %s. Read loop started.\n", connectionAddr)
	}
}

// readLoop handles reading messages from a single WebSocket connection.
func (s *Server) readLoop(ws *websocket.Conn, stopCh <-chan struct{}) {
	connectionAddr := ws.RemoteAddr().String()
	playerIndex := -1 // Will be assigned by GameActor

	// Defer cleanup for this specific connection when the loop exits
	defer func() {
		fmt.Printf("ReadLoop: Exiting for %s (Player %d). Triggering disconnect.\n", connectionAddr, playerIndex)
		// Ensure connection is closed and GameActor is notified
		closedPlayerIndex := s.CloseConnection(ws)
		if closedPlayerIndex != -1 { // Only send disconnect if index was assigned
			disconnectMsg := game.PlayerDisconnect{PlayerIndex: closedPlayerIndex}
			s.GetEngine().Send(s.GetGameActorPID(), disconnectMsg, nil)
		}
		fmt.Printf("ReadLoop: Cleanup finished for %s (Player %d).\n", connectionAddr, playerIndex)
	}()

	// Temporary loop to wait for player index assignment (or rejection)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	assignmentTimeout := time.After(5 * time.Second) // Timeout for assignment
AssignLoop:
	for {
		select {
		case <-ticker.C:
			s.mu.RLock()
			info, ok := s.connections[ws]
			s.mu.RUnlock()
			if ok && info.playerIndex != -1 {
				playerIndex = info.playerIndex
				fmt.Printf("ReadLoop: Assigned player index %d for %s.\n", playerIndex, connectionAddr)
				break AssignLoop // Exit assignment loop
			}
			if !ok {
				fmt.Printf("ReadLoop: Connection %s removed before index assignment. Exiting.\n", connectionAddr)
				return // Connection closed before assignment
			}
		case <-assignmentTimeout:
			fmt.Printf("ReadLoop: Timeout waiting for player index assignment for %s. Closing.\n", connectionAddr)
			return // Exit loop, defer handles cleanup
		case <-stopCh:
			fmt.Printf("ReadLoop: Stop signal received during index assignment for %s. Exiting.\n", connectionAddr)
			return // Exit loop
		}
	}

	// --- Main Read Loop ---
	fmt.Printf("ReadLoop: Starting main read loop for player %d (%s).\n", playerIndex, connectionAddr)
	for {
		select {
		case <-stopCh:
			fmt.Printf("ReadLoop: Stop signal received for player %d (%s). Exiting.\n", playerIndex, connectionAddr)
			return // Exit loop cleanly
		default:
			// Non-blocking check for stop signal before reading
		}

		buffer := make([]byte, 512)
		size, err := ws.Read(buffer)

		if err != nil {
			// Simplified error checking for golang.org/x/net/websocket
			if err == io.EOF {
				fmt.Printf("ReadLoop: Player %d (%s) disconnected (EOF).\n", playerIndex, connectionAddr)
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// This check might still be relevant if deadlines are set
				fmt.Printf("ReadLoop: Read timeout for player %d (%s). Closing.\n", playerIndex, connectionAddr)
			} else {
				// Assume any other error means the connection is broken or closed
				fmt.Printf("ReadLoop: Error/Connection closed for player %d (%s): %v\n", playerIndex, connectionAddr, err)
			}
			return // Exit loop, defer handles cleanup
		}

		// Process the received message (Paddle Direction)
		receivedData := buffer[:size]

		var dirMsg game.Direction
		if err := json.Unmarshal(receivedData, &dirMsg); err == nil {
			internalDir := utils.DirectionFromString(dirMsg.Direction)
			if internalDir != "" {
				forwardMsg := game.ForwardedPaddleDirection{
					PlayerIndex: playerIndex,
					Direction:   receivedData,
				}
				s.GetEngine().Send(s.GetGameActorPID(), forwardMsg, nil)
			} else {
				// fmt.Printf("ReadLoop: Player %d sent invalid direction string: %s\n", playerIndex, dirMsg.Direction)
			}
		} else {
			// fmt.Printf("ReadLoop: Player %d sent invalid JSON: %s\n", playerIndex, string(receivedData))
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

		// Placeholder implementation
		fmt.Println("HandleGetSit: Returning placeholder state (GameActor query not implemented)")
		gameState := []byte(`{"error": "Live state query not implemented via HTTP GET in actor model"}`)

		w.Header().Set("Content-Type", "application/json")
		if len(gameState) <= 2 {
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
