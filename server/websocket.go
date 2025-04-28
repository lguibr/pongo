// File: server/websocket.go
package server

import (
	"fmt"

	"github.com/lguibr/bollywood"
)

// Server holds references needed for handling requests.
type Server struct {
	engine         *bollywood.Engine
	roomManagerPID *bollywood.PID // Changed from gameActorPID
}

// New creates a new Server instance.
func New(engine *bollywood.Engine, roomManagerPID *bollywood.PID) *Server { // Changed parameter name
	if engine == nil || roomManagerPID == nil {
		panic("Server requires a valid engine and roomManagerPID")
	}
	// fmt.Println("Creating new Server instance.") // Removed redundant log
	return &Server{
		engine:         engine,
		roomManagerPID: roomManagerPID, // Store RoomManager PID
	}
}

// GetRoomManagerPID returns the PID of the room manager actor.
func (s *Server) GetRoomManagerPID() *bollywood.PID {
	if s == nil {
		fmt.Println("ERROR: GetRoomManagerPID called on nil Server")
		return nil
	}
	return s.roomManagerPID
}

// GetEngine returns the Bollywood engine instance.
func (s *Server) GetEngine() *bollywood.Engine {
	if s == nil {
		fmt.Println("ERROR: GetEngine called on nil Server")
		return nil
	}
	return s.engine
}
