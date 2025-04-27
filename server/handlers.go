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

	// "time" // No longer needed

	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/utils"

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
			fmt.Printf("HandleSubscribe: Handler exiting for %s, ensuring connection is closed.\n", connectionAddr)
			_ = ws.Close()
		}()

		engine := s.GetEngine()
		gameActorPID := s.GetGameActorPID()
		if engine == nil || gameActorPID == nil {
			fmt.Printf("HandleSubscribe: Server engine or GameActorPID is nil. Closing connection %s.\n", connectionAddr)
			return
		}

		// Send connect request to GameActor immediately
		connectMsg := game.PlayerConnectRequest{WsConn: ws} // Pass concrete type
		engine.Send(gameActorPID, connectMsg, nil)

		// Run the read loop directly in the handler function.
		s.readLoop(ws) // Pass concrete type

		fmt.Printf("HandleSubscribe: readLoop finished for %s.\n", connectionAddr)
	}
}

// readLoop handles reading messages from a single WebSocket connection.
// Now uses websocket.JSON.Receive
func (s *Server) readLoop(conn *websocket.Conn) { // Use concrete type
	connectionAddr := conn.RemoteAddr().String()
	fmt.Printf("ReadLoop: Starting for %s.\n", connectionAddr)

	engine := s.GetEngine()
	gameActorPID := s.GetGameActorPID()
	if engine == nil || gameActorPID == nil {
		fmt.Printf("ReadLoop: Server engine or GameActorPID is nil. Aborting read loop for %s.\n", connectionAddr)
		return
	}

	var disconnectSent bool
	defer func() {
		if !disconnectSent {
			fmt.Printf("ReadLoop: Exiting for %s. Sending disconnect signal.\n", connectionAddr)
			disconnectMsg := game.PlayerDisconnect{PlayerIndex: -1, WsConn: conn} // Pass concrete type
			engine.Send(gameActorPID, disconnectMsg, nil)
			disconnectSent = true
		}
		fmt.Printf("ReadLoop: Finished for %s.\n", connectionAddr)
	}()

	// Main Read Loop using websocket.JSON.Receive
	for {
		var dirMsg game.Direction
		err := websocket.JSON.Receive(conn, &dirMsg) // Receive directly into struct

		if err != nil {
			// Handle errors (EOF, closed connection, etc.)
			isClosedErr := strings.Contains(err.Error(), "use of closed network connection") ||
				strings.Contains(err.Error(), "closed") ||
				err == io.EOF
			isTimeoutErr := false
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				isTimeoutErr = true
			}

			if isClosedErr {
				// fmt.Printf("ReadLoop: Connection %s closed gracefully or already closed: %v\n", connectionAddr, err) // Reduce noise
			} else if isTimeoutErr {
				// This shouldn't happen often with blocking Receive unless a deadline is set elsewhere
				fmt.Printf("ReadLoop: Read timeout for %s. Assuming disconnect.\n", connectionAddr)
			} else {
				// Log other errors (e.g., invalid JSON format from client)
				fmt.Printf("ReadLoop: Error receiving from %s: %v\n", connectionAddr, err)
			}
			return // Exit loop, defer sends disconnect signal
		}

		// Process the successfully received and unmarshalled message
		internalDir := utils.DirectionFromString(dirMsg.Direction)
		if internalDir != "" {
			// Need to re-marshal to send raw bytes if PaddleActor expects that
			// Alternatively, change PaddleActor to accept the parsed direction string
			// Let's re-marshal for now to minimize changes to PaddleActor
			directionPayload, marshalErr := json.Marshal(dirMsg)
			if marshalErr != nil {
				fmt.Printf("ReadLoop: Error re-marshalling direction from %s: %v\n", connectionAddr, marshalErr)
				continue // Skip sending if marshalling failed
			}

			forwardMsg := game.ForwardedPaddleDirection{
				WsConn:    conn, // Pass concrete type
				Direction: directionPayload,
			}
			engine.Send(gameActorPID, forwardMsg, nil)
		} else {
			// Log if direction is not ArrowLeft/ArrowRight but JSON is valid
			// fmt.Printf("ReadLoop: Received valid JSON but unknown direction from %s: %s\n", connectionAddr, dirMsg.Direction)
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
