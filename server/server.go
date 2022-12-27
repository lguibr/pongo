package server

import (
	"fmt"
	"io"

	"golang.org/x/net/websocket"
)

type Server struct {
	conns map[*websocket.Conn]bool
}

func NewServer() *Server {
	return &Server{
		conns: make(map[*websocket.Conn]bool),
	}
}

func (s *Server) readLoop(ws *websocket.Conn, callback func(buffer []byte)) {

	buffer := make([]byte, 1024)
	for {

		size, err := ws.Read(buffer)

		if err != nil {
			fmt.Println("Error reading from client:", err)
			if err == io.EOF {
				fmt.Println("Connection closed by the client:", err)
				break
			}
			continue
		}

		message := buffer[:size]
		callback(message)
	}
}
