// File: server/handlers.go
package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime/debug" // Import debug package
	"strings"

	"github.com/lguibr/pongo/game"

	"golang.org/x/net/websocket"
)

// HandleSubscribe sets up the WebSocket connection and forwards it to the GameActor.
func (s *Server) HandleSubscribe() func(ws *websocket.Conn) {
	return func(ws *websocket.Conn) {
		connectionAddr := ws.RemoteAddr().String()
		fmt.Printf("HandleSubscribe: New connection attempt from %s\n", connectionAddr)

		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("PANIC recovered in HandleSubscribe/readLoop for %s: %v\nStack trace:\n%s\n", connectionAddr, r, string(debug.Stack()))
			}
			// fmt.Printf("HandleSubscribe: Handler exiting for %s, ensuring connection is closed.\n", connectionAddr) // Reduce noise
			_ = ws.Close() // Ensure close happens
		}()

		engine := s.GetEngine()
		gameActorPID := s.GetGameActorPID()
		if engine == nil || gameActorPID == nil {
			fmt.Printf("HandleSubscribe: Server engine or GameActorPID is nil. Closing connection %s.\n", connectionAddr)
			return // Exit early if server components are nil
		}

		// Send connect request to GameActor immediately
		connectMsg := game.PlayerConnectRequest{WsConn: ws}
		engine.Send(gameActorPID, connectMsg, nil)

		// Run the read loop directly in the handler function.
		s.readLoop(ws)

		// fmt.Printf("HandleSubscribe: readLoop finished for %s.\n", connectionAddr) // Reduce noise
		// Disconnect signal is now sent via defer in readLoop
	}
}

// readLoop handles reading messages from a single WebSocket connection.
func (s *Server) readLoop(conn *websocket.Conn) {
	connectionAddr := conn.RemoteAddr().String()
	// fmt.Printf("ReadLoop: Starting for %s.\n", connectionAddr) // Reduce noise

	engine := s.GetEngine()
	gameActorPID := s.GetGameActorPID()
	if engine == nil || gameActorPID == nil {
		fmt.Printf("ReadLoop: Server engine or GameActorPID is nil. Aborting read loop for %s.\n", connectionAddr)
		return
	}

	var disconnectSent bool
	defer func() {
		// Ensure disconnect is sent *exactly once* when the loop exits
		if !disconnectSent {
			// fmt.Printf("ReadLoop: Exiting for %s. Sending disconnect signal.\n", connectionAddr) // Reduce noise
			disconnectMsg := game.PlayerDisconnect{PlayerIndex: -1, WsConn: conn}
			engine.Send(gameActorPID, disconnectMsg, nil)
			disconnectSent = true // Mark as sent
		}
		// fmt.Printf("ReadLoop: Finished for %s.\n", connectionAddr) // Reduce noise
	}()

	// Main Read Loop using websocket.JSON.Receive
	for {
		var dirMsg game.Direction
		err := websocket.JSON.Receive(conn, &dirMsg)

		if err != nil {
			isClosedErr := strings.Contains(err.Error(), "use of closed network connection") ||
				strings.Contains(err.Error(), "closed") ||
				err == io.EOF
			isTimeoutErr := false
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				isTimeoutErr = true
			}

			if isClosedErr {
				// Connection closed normally or by timeout handled by SetReadDeadline
				// fmt.Printf("ReadLoop: Connection %s closed gracefully or timed out: %v\n", connectionAddr, err) // Reduce noise
			} else if isTimeoutErr {
				// This case might be redundant if SetReadDeadline is used, but kept for safety
				fmt.Printf("ReadLoop: Explicit read timeout for %s. Assuming disconnect.\n", connectionAddr)
			} else {
				// Log other unexpected errors (e.g., invalid JSON format)
				fmt.Printf("ReadLoop: Error receiving from %s: %v\n", connectionAddr, err)
			}
			return // Exit loop, defer sends disconnect signal
		}

		// Process the successfully received and unmarshalled message
		// *** FIX: Always forward the valid message, including "Stop" ***

		// Re-marshal the original received message to send raw bytes
		// (PaddleActor expects the raw bytes containing the original JSON)
		directionPayload, marshalErr := json.Marshal(dirMsg)
		if marshalErr != nil {
			fmt.Printf("ReadLoop: Error re-marshalling direction from %s: %v\n", connectionAddr, marshalErr)
			continue // Skip sending if marshalling failed
		}

		// Create and send the forwarded message
		forwardMsg := game.ForwardedPaddleDirection{
			WsConn:    conn,
			Direction: directionPayload,
		}
		engine.Send(gameActorPID, forwardMsg, nil)

		// Optional: Log the forwarded action
		// fmt.Printf("ReadLoop: Forwarded direction '%s' from %s\n", dirMsg.Direction, connectionAddr)
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

		// Returning placeholder as direct query is complex/not implemented.
		gameState := []byte(`{"error": "Live state query not implemented via HTTP GET in actor model"}`)
		w.Header().Set("Content-Type", "application/json")
		if len(gameState) <= 2 {
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
