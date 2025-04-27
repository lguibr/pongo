// File: server/websocket.go
package server

import (
	"fmt"

	"github.com/lguibr/pongo/bollywood" // Import bollywood
	// "golang.org/x/net/websocket" // No longer needed here
)

// Server holds references needed for handling requests.
type Server struct {
	engine       *bollywood.Engine
	gameActorPID *bollywood.PID
	// connections map removed
	// mu removed (no longer managing shared map)
}

// connectionInfo removed

// New creates a new Server instance.
func New(engine *bollywood.Engine, gameActorPID *bollywood.PID) *Server {
	if engine == nil || gameActorPID == nil {
		panic("Server requires a valid engine and gameActorPID")
	}
	fmt.Println("Creating new Server instance.")
	return &Server{
		engine:       engine,
		gameActorPID: gameActorPID,
	}
}

// OpenConnection REMOVED - No longer managed by Server struct.

// CloseConnection REMOVED - Cleanup handled by GameActor.

// GetGameActorPID returns the PID of the main game actor.
func (s *Server) GetGameActorPID() *bollywood.PID {
	// Add nil check for safety, although New should prevent this
	if s == nil {
		fmt.Println("ERROR: GetGameActorPID called on nil Server")
		return nil
	}
	return s.gameActorPID
}

// GetEngine returns the Bollywood engine instance.
func (s *Server) GetEngine() *bollywood.Engine {
	// Add nil check for safety
	if s == nil {
		fmt.Println("ERROR: GetEngine called on nil Server")
		return nil
	}
	return s.engine
}
