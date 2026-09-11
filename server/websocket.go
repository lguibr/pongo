package server

import (
	"log/slog"
	"sync/atomic"

	"github.com/lguibr/pongo/internal/actor"
)

// Server holds references needed for handling requests.
type Server struct {
	activeConnections atomic.Int64
	engine            *actor.Engine
	roomManagerPID    *actor.PID
}

// New creates a new Server instance.
func New(engine *actor.Engine, roomManagerPID *actor.PID) *Server {
	if engine == nil || roomManagerPID == nil {
		panic("Server requires a valid engine and roomManagerPID")
	}
	return &Server{
		engine:         engine,
		roomManagerPID: roomManagerPID, // Store RoomManager PID
	}
}

// GetRoomManagerPID returns the PID of the room manager actor.
func (s *Server) GetRoomManagerPID() *actor.PID {
	if s == nil {
		slog.Error("GetRoomManagerPID called on nil Server")
		return nil
	}
	return s.roomManagerPID
}

// GetEngine returns the Bollywood engine instance.
func (s *Server) GetEngine() *actor.Engine {
	if s == nil {
		slog.Error("GetEngine called on nil Server")
		return nil
	}
	return s.engine
}
