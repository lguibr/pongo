package server

import (
	"golang.org/x/net/websocket"
)

type Server struct {
	conns map[*websocket.Conn]bool
}

func New() *Server {
	return &Server{conns: make(map[*websocket.Conn]bool)}
}

func (s *Server) OpenConnection(ws *websocket.Conn) {
	s.conns[ws] = true
}

func (s *Server) CloseConnection(ws *websocket.Conn) {
	ws.Close()
	delete(s.conns, ws) // remove the connection from the map
}
