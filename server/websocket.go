package server

import (
	"fmt"
	"sync"

	"github.com/lguibr/pongo/bollywood" // Import bollywood
	"golang.org/x/net/websocket"
)

// Server manages active WebSocket connections and holds engine references.
type Server struct {
	connections map[*websocket.Conn]*connectionInfo
	mu          sync.RWMutex // Protect the connections map
	engine      *bollywood.Engine
	gameActorPID *bollywood.PID
}

// connectionInfo holds info about a connection, including its associated player index.
type connectionInfo struct {
	ws          *websocket.Conn
	playerIndex int
	stopReadCh  chan struct{} // Channel to stop the reader goroutine for this connection
}

// New creates a new Server instance.
func New(engine *bollywood.Engine, gameActorPID *bollywood.PID) *Server {
	return &Server{
		connections: make(map[*websocket.Conn]*connectionInfo),
		engine:      engine,
		gameActorPID: gameActorPID,
	}
}

// OpenConnection adds a new WebSocket connection to the tracking map.
// Now returns the stop channel for the reader goroutine.
func (s *Server) OpenConnection(ws *websocket.Conn) chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	stopCh := make(chan struct{})
	s.connections[ws] = &connectionInfo{
		ws:          ws,
		playerIndex: -1, // Index assigned later by GameActor
		stopReadCh:  stopCh,
	}
	fmt.Printf("Connection opened: %s. Total connections: %d\n", ws.RemoteAddr(), len(s.connections))
	return stopCh
}

// AssignPlayerIndex updates the player index for a connection.
func (s *Server) AssignPlayerIndex(ws *websocket.Conn, index int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if info, ok := s.connections[ws]; ok {
		info.playerIndex = index
		fmt.Printf("Assigned player index %d to connection %s\n", index, ws.RemoteAddr())
	}
}

// CloseConnection closes the WebSocket connection and removes it from the tracking map.
func (s *Server) CloseConnection(ws *websocket.Conn) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	playerIndex := -1
	// Check if connection still exists before closing/deleting
	if info, ok := s.connections[ws]; ok {
		playerIndex = info.playerIndex // Get index before deleting
		fmt.Printf("Closing connection for player %d: %s\n", playerIndex, ws.RemoteAddr())

		// Signal the reader goroutine to stop (non-blocking)
		select {
		case <-info.stopReadCh: // Already closed
		default:
			close(info.stopReadCh)
		}

		err := ws.Close() // Attempt to close gracefully
		if err != nil {
			// Log error but continue cleanup
			// fmt.Printf("Error closing websocket for %s: %v\n", ws.RemoteAddr(), err)
		}
		delete(s.connections, ws) // Remove from the map
		fmt.Printf("Connection removed: %s. Total connections: %d\n", ws.RemoteAddr(), len(s.connections))
	} else {
		fmt.Printf("Attempted to close already removed connection: %s\n", ws.RemoteAddr())
	}
	return playerIndex // Return the index of the closed player
}

// GetGameActorPID returns the PID of the main game actor.
func (s *Server) GetGameActorPID() *bollywood.PID {
	return s.gameActorPID
}

// GetEngine returns the Bollywood engine instance.
func (s *Server) GetEngine() *bollywood.Engine {
	return s.engine
}