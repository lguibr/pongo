package game

import "golang.org/x/net/websocket"

func (game *Game) LifeCycle(ws *websocket.Conn, close func()) {
	//INFO Start the WebSocket connection
	playerIndex := game.GetNextIndex()
	currentBallIndex := len(game.Balls)
	if currentBallIndex < 0 {
		currentBallIndex = 0
	}

	//INFO Initiate a new game if there is no player
	if !game.HasPlayer() {
		game.Canvas.Grid.Fill(0, 0, 0, 0)
	}
	//INFO Initiating channels
	playerChannel := make(chan PlayerMessage)
	ballChannel := make(chan BallMessage)
	paddleChannel := make(chan PaddleMessage)
	// INFO Initiate the player and player's dependencies

	player := NewPlayer(game.Canvas, playerIndex, playerChannel)
	playerPaddle := NewPaddle(paddleChannel, game.Canvas.CanvasSize, playerIndex)
	initialPlayerBall := NewBall(ballChannel, 0, 0, 0, game.Canvas.CanvasSize, playerIndex, currentBallIndex)
	//INFO Start reading from game's entities channels
	go game.ReadPlayerChannel(playerIndex, playerChannel, playerPaddle, initialPlayerBall, close)
	go game.ReadBallChannel(playerIndex, ballChannel)
	go playerPaddle.ReadPaddleChannel(paddleChannel)
	//INFO Connect the player
	player.Connect()
	//INFO Start reading input from player and writing game state to player
	go player.ReadInput(ws, paddleChannel)
	go game.WriteGameState(ws)
}
