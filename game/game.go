package game

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/lguibr/asciiring/render"
	"github.com/lguibr/asciiring/types"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

type GameMessage interface{}

type AddBall struct {
	BallPayload *Ball
	ExpireIn    int
}
type RemoveBall struct {
	Id int
}

type IncreaseBallVelocity struct {
	BallPayload *Ball
	Ratio       float64
}
type IncreaseBallMass struct {
	BallPayload *Ball
	Additional  int
}
type BallPhasing struct {
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

func StartGame() *Game {
	rand.Seed(time.Now().UnixNano())

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
	timestamp := time.Now().Format("20060102150405") // YYYYMMDDHHMMSS
	frame := 0
	for {
		time.Sleep(utils.Period)
		rgbaGrid := game.Canvas.DrawGameOnRGBGrid(game.Paddles, game.Balls)
		dirPath := fmt.Sprintf("./data/%s", timestamp)

		// Create directory if it doesn't exist
		err := os.MkdirAll(dirPath, 0755)
		if err != nil {
			fmt.Println("Error creating directory: ", err)
			return
		}

		filePath := fmt.Sprintf("%s/%d.json", dirPath, frame)
		utils.JsonLogger(filePath, game)
		color := types.RGBPixel{R: 255, G: 255, B: 255}
		ascii := render.RenderToASCII(rgbaGrid, 64, &color)
		fmt.Println(ascii)
		_, err = ws.Write([]byte(ascii))

		if err != nil {
			fmt.Println("Error writing to client: ", err)
			return
		}
		frame++
	}
}

func (game *Game) RemovePlayer(playerIndex int) {
	game.Players[playerIndex] = nil
	game.Paddles[playerIndex] = nil
	for _, ball := range game.Balls {
		if ball.OwnerIndex != playerIndex {
			continue
		}

		game.channel <- RemoveBall{Id: ball.Id}
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
				game.channel <- RemoveBall{Id: ball.Id}
			}
		}
	}()
}

func (game *Game) RemoveBall(id int) {
	for index, ball := range game.Balls {
		if ball.Id != id {
			continue
		}
		// fmt.Println("Removing ball", ball.Id)
		ball.open = false
		if index < len(game.Balls)-1 {
			game.Balls = append(game.Balls[:index], game.Balls[index+1:]...)
		} else {
			game.Balls = game.Balls[:index]
		}
	}
}
