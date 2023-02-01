package game

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/lguibr/pongo/utils"
)

func (g *Game) ReadBallChannel(ownerIndex int, ball *Ball) {
	ballChannel := ball.Channel

	for {
		message, ok := <-ballChannel

		if !ok {

			return
		}

		switch payload := message.(type) {
		case BallPositionMessage:

			ball := payload.Ball
			ball.CollidePaddles(g.Paddles)
			ball.CollideCells(g.Canvas.Grid, g.Canvas.CellSize)
			ball.CollideWalls()
		case WallCollisionMessage:
			ball := payload.Ball
			index := payload.Index
			if index == ball.OwnerIndex || g.Players[index] == nil {
				continue
			}
			g.Players[index].channel <- PlayerScore{-1}
			if g.Players[ball.OwnerIndex] != nil {
				g.Players[ball.OwnerIndex].channel <- PlayerScore{1}
			}
		case BreakBrickMessage:
			level := payload.Level
			ball := payload.BallPayload
			playerIndex := ball.OwnerIndex
			if g.Players[playerIndex] == nil {
				break
			}
			g.Players[playerIndex].channel <- PlayerScore{level}
			random := rand.Intn(4)
			fmt.Println("Reward random numb:", random)
			if random == 0 {
				g.channel <- AddBall{
					NewBall(
						NewBallChannel(),
						ball.X,
						ball.Y,
						utils.BallSize,
						utils.CanvasSize,
						playerIndex,
						time.Now().Nanosecond(),
					),
					rand.Intn(2) + 1,
				}
			} else if random == 1 {
				g.channel <- IncreaseBallMass{ball, 1}
			} else if random == 2 {
				g.channel <- IncreaseBallVelocity{ball, 1.1}
			} else {
				g.channel <- BallPhasing{ball, 1}
			}
		default:
			continue
		}

	}
}

func (playerPaddle *Paddle) ReadPaddleChannel(paddleChannel chan PaddleMessage) {
	for message := range paddleChannel {
		switch message := message.(type) {
		case PaddleDirectionMessage:
			direction := message.Direction
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
	callback func(),
) {
	for message := range playerChannel {
		switch payload := message.(type) {
		case PlayerConnectMessage:
			player := message.(PlayerConnectMessage).PlayerPayload
			g.channel <- AddBall{ball, 0}
			g.AddPlayer(index, player, paddle)
		case PlayerDisconnectMessage:
			g.RemovePlayer(index)
			callback()
		case PlayerScore:
			score := payload.Score
			g.Players[index].Score += score
		default:
			continue
		}
	}
}

func (g *Game) ReadGameChannel() {
	for message := range g.channel {
		switch message := message.(type) {
		case AddBall:
			ball := message.BallPayload
			expire := message.ExpireIn
			g.AddBall(ball, expire)
		case RemoveBall:
			id := message.Id
			g.RemoveBall(id)
		case IncreaseBallVelocity:
			ball := message.BallPayload
			ratio := message.Ratio
			ball.IncreaseVelocity(ratio)
		case IncreaseBallMass:
			ball := message.BallPayload
			additional := message.Additional
			ball.IncreaseMass(additional)
		case BallPhasing:
			ball := message.BallPayload
			expireIn := message.ExpireIn
			ball.SetBallPhasing(expireIn)
		default:
			continue
		}
	}
}
