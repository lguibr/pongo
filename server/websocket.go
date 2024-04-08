package server

import (
	"golang.org/x/net/websocket"
)

type Server struct {
	connections map[*websocket.Conn]bool
}

func New() *Server {
	return &Server{connections: make(map[*websocket.Conn]bool)}
}

func (s *Server) OpenConnection(ws *websocket.Conn) {
	s.connections[ws] = true
}

func (s *Server) CloseConnection(ws *websocket.Conn) {
	ws.Close()
	delete(s.connections, ws) // remove the connection from the map
}

func (s *Server) KeepConnection(ws *websocket.Conn) {
	for {
		if !ws.IsServerConn() {
			s.CloseConnection(ws)
		}
	}
}
