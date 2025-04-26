// File: server/websocket.go
package server

import (
	"fmt"
	"sync"

	"golang.org/x/net/websocket"
)

// Server manages active WebSocket connections.
type Server struct {
	connections map[*websocket.Conn]bool
	mu          sync.RWMutex // Protect the connections map
}

// New creates a new Server instance.
func New() *Server {
	return &Server{
		connections: make(map[*websocket.Conn]bool),
	}
}

// OpenConnection adds a new WebSocket connection to the tracking map.
func (s *Server) OpenConnection(ws *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connections[ws] = true
	fmt.Printf("Connection opened: %s. Total connections: %d\n", ws.RemoteAddr(), len(s.connections))
}

// CloseConnection closes the WebSocket connection and removes it from the tracking map.
func (s *Server) CloseConnection(ws *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if connection still exists before closing/deleting
	if _, ok := s.connections[ws]; ok {
		fmt.Printf("Closing connection: %s\n", ws.RemoteAddr())
		err := ws.Close() // Attempt to close gracefully
        if err != nil {
            fmt.Printf("Error closing websocket for %s: %v\n", ws.RemoteAddr(), err)
        }
		delete(s.connections, ws) // Remove from the map
		fmt.Printf("Connection removed: %s. Total connections: %d\n", ws.RemoteAddr(), len(s.connections))
	} else {
		fmt.Printf("Attempted to close already removed connection: %s\n", ws.RemoteAddr())
	}
}

