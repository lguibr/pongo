package game

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

type GameMessage interface{}

type AddBallMsg struct {
	BallPayload *Ball
	ExpireIn    int
}
type RemoveBallMsg struct {
	Id int
}

type IncreaseBallVelocityMsg struct {
	BallPayload *Ball
	Ratio       float64
}
type IncreaseBallMassMsg struct {
	BallPayload *Ball
	Additional  int
}
type BallPhasingMsg struct {
	BallPayload *Ball
	ExpireIn    int
}

type Game struct {
	Canvas  *Canvas    `json:"canvas"`
	Players [4]*Player `json:"players"`
	Paddles [4]*Paddle `json:"paddles"`
	Balls   []*Ball    `json:"balls"`
	channel chan GameMessage
}

func NewGame() *Game {
	canvas := NewCanvas(0, 0)
	canvas.Grid.Fill(0, 0, 0, 0)
	players := [4]*Player{}

	game := Game{
		Canvas:  canvas,
		Players: players,
		channel: make(chan GameMessage),
	}

	return &game
}

func (game *Game) ToJson() []byte {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered from panic:", r)
		}
	}()

	gameBytes, err := json.Marshal(game)
	if err != nil {
		fmt.Println("Error Marshaling the game state", err)
		return []byte{}
	}
	return gameBytes
}

func (game *Game) GetNextIndex() int {
	for i, player := range game.Players {
		if player == nil {
			return i
		}
	}
	return 0
}

func (game *Game) HasPlayer() bool {
	for _, player := range game.Players {
		if player != nil {
			return true
		}
	}
	return false
}

func (game *Game) WriteGameState(ws *websocket.Conn) {
	for {
		time.Sleep(utils.Period)
		_, err := ws.Write(game.ToJson())
		if err != nil {
			fmt.Println("Error writing to client: ", err)
			return
		}
	}
}

func (game *Game) RemovePlayer(playerIndex int) {
	game.Players[playerIndex] = nil
	game.Paddles[playerIndex] = nil
	for _, ball := range game.Balls {
		if ball.OwnerIndex != playerIndex {
			continue
		}

		game.channel <- RemoveBallMsg{Id: ball.Id}
	}
}

func (g *Game) AddPlayer(index int, player *Player, playerPaddle *Paddle) {
	g.Players[index] = player
	g.Paddles[index] = playerPaddle
	go playerPaddle.Engine()

}

func (game *Game) AddBall(ball *Ball, expire int) {
	game.Balls = append(game.Balls, ball)
	go game.ReadBallChannel(ball.OwnerIndex, ball)
	go ball.Engine()

	go func() {
		if expire == 0 {
			return
		}
		time.Sleep(time.Duration(expire) * time.Second)
		for _, b := range game.Balls {
			if b.Id == ball.Id {
				game.channel <- RemoveBallMsg{Id: ball.Id}
			}
		}
	}()
}

func (game *Game) RemoveBall(id int) {
	for index, ball := range game.Balls {
		if ball.Id != id {
			continue
		}
		ball.open = false
		if index < len(game.Balls)-1 {
			game.Balls = append(game.Balls[:index], game.Balls[index+1:]...)
		} else {
			game.Balls = game.Balls[:index]
		}
	}
}

func (game *Game) ReadChannel() {
	for message := range game.channel {
		switch message := message.(type) {
		case AddBallMsg:
			ball := message.BallPayload
			expire := message.ExpireIn
			game.AddBall(ball, expire)
		case RemoveBallMsg:
			id := message.Id
			game.RemoveBall(id)
		case IncreaseBallVelocityMsg:
			ball := message.BallPayload
			ratio := message.Ratio
			ball.IncreaseVelocity(ratio)
		case IncreaseBallMassMsg:
			ball := message.BallPayload
			additional := message.Additional
			ball.IncreaseMass(additional)
		case BallPhasingMsg:
			ball := message.BallPayload
			expireIn := message.ExpireIn
			ball.SetBallPhasing(expireIn)
		default:
			continue
		}
	}
}
