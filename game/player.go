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
	Paddle  *Paddle `json:"paddle"`
	Balls   []*Ball `json:"balls"`
	channel chan PlayerMessage
}

func NewPlayer(canvas *Canvas, index int, channel chan PlayerMessage, initialBall *Ball) *Player {

	return &Player{
		Index:   index,
		Id:      "player" + fmt.Sprint(index),
		Canvas:  canvas,
		Color:   utils.NewRandomColor(),
		Paddle:  NewPaddle(canvas.CanvasSize, index),
		Balls:   []*Ball{initialBall},
		channel: channel,
	}
}

func (player *Player) Subscribe() {
	defer func() { fmt.Println("Player subscribed: ", player) }()

	go player.Paddle.Engine()
	go player.Balls[0].Engine()
	fmt.Println("Player dependencies engined: ", player)
	player.channel <- PlayerConnectMessage{PlayerPayload: player}
	fmt.Println("Player Subscribed: ", player)

}

func (player *Player) Unsubscribe() {
	fmt.Println("UnSubscribePlayer of index: ", player.Index)
	player.channel <- PlayerDisconnectMessage{}
}

func (player *Player) ReadInput(ws *websocket.Conn) {
	defer func() {
		player.Unsubscribe()
	}()

	buffer := make([]byte, 1024)
	for {
		size, err := ws.Read(buffer)
		if err != nil {
			fmt.Println("Error reading from client:", err)
			if err == io.EOF {
				fmt.Println("Connection closed by the client:", err)
				return
			}
			continue
		}
		player.Paddle.SetDirection(buffer[:size])
	}
}
