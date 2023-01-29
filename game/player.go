package game

import (
	"fmt"
	"io"

	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

type PlayerMessage interface{}

type PlayerConnectMessage struct {
	PlayerPayload *Player
}
type PlayerDisconnectMessage struct{}

type Player struct {
	Index   int     `json:"index"`
	Id      string  `json:"id"`
	Canvas  *Canvas `json:"canvas"`
	Color   [3]int  `json:"color"`
	channel chan PlayerMessage
}

func NewPlayer(canvas *Canvas, index int, channel chan PlayerMessage) *Player {
	return &Player{
		Index:   index,
		Id:      "player" + fmt.Sprint(index),
		Canvas:  canvas,
		Color:   utils.NewRandomColor(),
		channel: channel,
	}
}

func (player *Player) Connect() {
	player.channel <- PlayerConnectMessage{PlayerPayload: player}
}

func (player *Player) Disconnect() {
	player.channel <- PlayerDisconnectMessage{}
}

func (player *Player) ReadInput(ws *websocket.Conn, paddleChannel chan PaddleMessage) {
	defer func() {
		player.Disconnect()
	}()

	for {
		buffer := make([]byte, 1024)
		size, err := ws.Read(buffer)
		if err != nil {
			fmt.Println("EOFError reading from client:", err)
			if err == io.EOF {
				fmt.Println("Connection closed by the client:", err)
				return
			}
			continue
		}
		//Send I/O message to change the paddle direction
		newDirection := buffer[:size]
		paddleChannel <- PaddleDirectionMessage{Direction: newDirection}
	}
}
