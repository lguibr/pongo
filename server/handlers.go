
package server

import (
	"encoding/json"
	"errors" // Import errors
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/game"

	"golang.org/x/net/websocket"
)

// HandleSubscribe sets up the WebSocket connection and spawns a ConnectionHandlerActor.
func (s *Server) HandleSubscribe() func(ws *websocket.Conn) {
	return func(ws *websocket.Conn) {
		connectionAddr := ws.RemoteAddr().String()
		fmt.Printf("HandleSubscribe: New connection attempt from %s\n", connectionAddr)

		// Create a channel to signal when the handler actor is done
		handlerDone := make(chan struct{})

		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("PANIC recovered in HandleSubscribe for %s: %v\nStack trace:\n%s\n", connectionAddr, r, string(debug.Stack()))
				// Ensure connection is closed on panic during setup
				_ = ws.Close()
				// Ensure the done channel is closed if panic happens before actor signals
				select {
				case <-handlerDone: // Already closed
				default:
					close(handlerDone)
				}
			}
			// fmt.Printf("HandleSubscribe: Handler finished for %s\n", connectionAddr) // Removed log
		}()

		engine := s.GetEngine()
		managerPID := s.GetRoomManagerPID()
		if engine == nil || managerPID == nil {
			fmt.Printf("HandleSubscribe: Server engine or RoomManagerPID is nil. Closing connection %s.\n", connectionAddr)
			_ = ws.Close()
			close(handlerDone) // Signal completion on error
			return
		}

		// Spawn a ConnectionHandlerActor for this connection, passing the done channel
		args := ConnectionHandlerArgs{
			Conn:           ws,
			Engine:         engine,
			RoomManagerPID: managerPID,
			Done:           handlerDone, // Pass the channel
		}
		handlerProps := bollywood.NewProps(NewConnectionHandlerProducer(args))
		handlerPID := engine.Spawn(handlerProps)

		if handlerPID == nil {
			fmt.Printf("HandleSubscribe: Failed to spawn ConnectionHandlerActor for %s. Closing connection.\n", connectionAddr)
			_ = ws.Close()
			close(handlerDone) // Signal completion on error
			return
		}

		// fmt.Printf("HandleSubscribe: Spawned ConnectionHandlerActor %s for %s. Waiting for completion...\n", handlerPID, connectionAddr) // Removed log

		// Wait here until the ConnectionHandlerActor signals it's done
		<-handlerDone

		// Now the handler can return, connection management is complete.
	}
}

// HandleGetRooms provides room list information via HTTP GET by querying the RoomManager using Ask.
func (s *Server) HandleGetRooms() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				fmt.Printf("PANIC recovered in HandleGetRooms: %v\nStack trace:\n%s\n", rec, string(debug.Stack()))
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		engine := s.GetEngine()
		managerPID := s.GetRoomManagerPID()
		if engine == nil || managerPID == nil {
			http.Error(w, "Server not properly initialized", http.StatusInternalServerError)
			return
		}

		// Use engine.Ask to query the RoomManager
		askTimeout := 2 * time.Second
		reply, err := engine.Ask(managerPID, game.GetRoomListRequest{}, askTimeout)

		if err != nil {
			if errors.Is(err, bollywood.ErrTimeout) {
				fmt.Println("Timeout waiting for RoomManager response in HandleGetRooms")
				http.Error(w, "Timeout querying game state", http.StatusGatewayTimeout)
			} else {
				fmt.Printf("Error asking RoomManager: %v\n", err)
				http.Error(w, "Error querying game state", http.StatusInternalServerError)
			}
			return
		}

		// Process the reply
		switch v := reply.(type) {
		case game.RoomListResponse:
			roomListData, marshalErr := json.Marshal(v)
			if marshalErr != nil {
				fmt.Println("Error marshalling room list data:", marshalErr)
				http.Error(w, "Error generating room list", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(roomListData)
		case error: // Handle case where RoomManager replied with an error
			fmt.Printf("RoomManager replied with error: %v\n", v)
			http.Error(w, "Error retrieving game state", http.StatusInternalServerError)
		default:
			fmt.Printf("Received unexpected reply type from RoomManager via Ask: %T\n", v)
			http.Error(w, "Internal server error processing reply", http.StatusInternalServerError)
		}
	}
}

// HandleHealthCheck provides a simple health check endpoint.
func HandleHealthCheck() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Simple JSON response indicating success
		_, _ = w.Write([]byte(`{"status": "ok"}`))
	}
}