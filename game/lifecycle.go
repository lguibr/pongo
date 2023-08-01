package game

import (
	"fmt"
	"time"

	"golang.org/x/net/websocket"
)

func (game *Game) LifeCycle(ws *websocket.Conn, close func()) {
	fmt.Println("LifeCycle")

	//INFO Start the WebSocket connection
	playerIndex := game.GetNextIndex()

	//INFO Initiate a new game if there is no player
	if !game.HasPlayer() {
		game.Canvas.Grid.Fill(0, 0, 0, 0)
	}
	//INFO Initiating channels
	playerChannel := NewPlayerChannel()
	//INFO Ball channel is buffered in one because ballBreakingBrick and ballMove could be simultaneously
	paddleChannel := NewPaddleChannel()
	// INFO Initiate the player and player's dependencies

	player := NewPlayer(game.Canvas, playerIndex, playerChannel)
	playerPaddle := NewPaddle(paddleChannel, game.Canvas.CanvasSize, playerIndex)
	initialPlayerBall := NewBall(
		NewBallChannel(),
		0,
		0,
		0,
		game.Canvas.CanvasSize,
		playerIndex,
		time.Now().Nanosecond(),
	)
	//INFO Start reading from game's entities channels
	go game.ReadPlayerChannel(playerIndex, playerChannel, playerPaddle, initialPlayerBall, close)
	go playerPaddle.ReadPaddleChannel(paddleChannel)
	go game.ReadBallChannel(playerIndex, initialPlayerBall)
	//INFO Connect the player
	player.Connect()
	//INFO Start reading input from player and writing game state to player
	go player.ReadInput(ws, paddleChannel)
	go game.WriteGameState(ws)
}
