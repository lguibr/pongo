package game

import "fmt"

func (g *Game) ReadBallChannel(index int, ballChannel chan BallMessage) {
	for message := range ballChannel {
		switch payload := message.(type) {
		case BallPositionMessage:
			if g.Players[index] == nil {
				return
			}
			ball := payload.Ball
			ball.CollidePaddles(g.Paddles)
			ball.CollideCells(g.Canvas.Grid, g.Canvas.CellSize)
			ball.CollideWalls()
		default:
			continue
		}
	}
}

func (playerPaddle *Paddle) ReadPaddleChannel(paddleChannel chan PaddleMessage) {
	for message := range paddleChannel {
		switch message.(type) {
		case PaddleDirectionMessage:
			direction := message.(PaddleDirectionMessage).Direction
			playerPaddle.SetDirection(direction)
		default:
			continue
		}
	}
}

func (g *Game) ReadPlayerChannel(
	index int,
	playerChannel chan PlayerMessage,
	paddle *Paddle,
	ball *Ball,
	close func(),
) {
	for message := range playerChannel {
		switch payload := message.(type) {
		case PlayerConnectMessage:
			player := message.(PlayerConnectMessage).PlayerPayload
			g.AddPlayer(index, player, paddle, ball)
			fmt.Println("Player Connect: ", payload)
		case PlayerDisconnectMessage:
			g.RemovePlayer(index)
			close()
			fmt.Println("Player Disconnected")

		default:
			continue
		}
	}
}
