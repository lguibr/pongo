package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/internal/actor"

	"golang.org/x/net/websocket"
)

// HandleSubscribe sets up the WebSocket connection and spawns a ConnectionHandlerActor.
func (s *Server) HandleSubscribe() func(ws *websocket.Conn) {
	return func(ws *websocket.Conn) {
		if s.activeConnections.Add(1) > 512 {
			s.activeConnections.Add(-1)
			_ = ws.Close()
			return
		}
		defer s.activeConnections.Add(-1)
		connectionAddr := ws.RemoteAddr().String()
		slog.Debug("websocket connected", "remote", connectionAddr)

		// Create a channel to signal when the handler actor is done
		handlerDone := make(chan struct{})

		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic in websocket handler", "remote", connectionAddr, "panic", r, "stack", string(debug.Stack()))
				// Ensure connection is closed on panic during setup
				_ = ws.Close()
				// Ensure the done channel is closed if panic happens before actor signals
				select {
				case <-handlerDone: // Already closed
				default:
					close(handlerDone)
				}
			}
		}()

		engine := s.GetEngine()
		managerPID := s.GetRoomManagerPID()
		if engine == nil || managerPID == nil {
			slog.Error("server not initialized; closing websocket", "remote", connectionAddr)
			_ = ws.Close()
			close(handlerDone) // Signal completion on error
			return
		}

		// Spawn a ConnectionHandlerActor for this connection, passing the done channel
		args := ConnectionHandlerArgs{
			Conn:           ws,
			Engine:         engine,
			RoomManagerPID: managerPID,
			Done:           handlerDone,
		}
		handlerProps := actor.NewProps(NewConnectionHandlerProducer(args))
		handlerPID := engine.Spawn(handlerProps)

		if handlerPID == nil {
			slog.Error("failed to spawn connection handler; closing websocket", "remote", connectionAddr)
			_ = ws.Close()
			close(handlerDone) // Signal completion on error
			return
		}

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
				slog.Error("panic in room list handler", "panic", rec, "stack", string(debug.Stack()))
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
			if errors.Is(err, actor.ErrTimeout) {
				slog.Warn("room list request timed out")
				http.Error(w, "Timeout querying game state", http.StatusGatewayTimeout)
			} else {
				slog.Error("room list request failed", "err", err)
				http.Error(w, "Error querying game state", http.StatusInternalServerError)
			}
			return
		}

		// Process the reply
		switch v := reply.(type) {
		case game.RoomListResponse:
			roomListData, marshalErr := json.Marshal(v)
			if marshalErr != nil {
				slog.Error("encoding room list failed", "err", marshalErr)
				http.Error(w, "Error generating room list", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(roomListData)
		case error: // Handle case where RoomManager replied with an error
			slog.Error("room manager replied with error", "err", v)
			http.Error(w, "Error retrieving game state", http.StatusInternalServerError)
		default:
			slog.Error("unexpected room list reply", "type", fmt.Sprintf("%T", v))
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
