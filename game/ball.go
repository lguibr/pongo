package game

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/lguibr/pongo/utils"
)

type BallMessage interface{}

type PositionPayload struct {
	X      int
	Y      int
	Radius int
}

type BallPositionMessage struct {
	Ball *Ball
}

type Ball struct {
	X          int `json:"x"`
	Y          int `json:"y"`
	Vx         int `json:"vx"`
	Vy         int `json:"vy"`
	Ax         int `json:"ax"`
	Ay         int `json:"ay"`
	Radius     int `json:"radius"`
	Index      int `json:"index"`
	channel    chan BallMessage
	canvasSize int
}

func NewBall(channel chan BallMessage, x, y, radius, canvasSize, index int) *Ball {

	if x == 0 && y == 0 {
		cardinalPosition := [2]int{canvasSize/2 - utils.CellSize*1.5, 0}

		rotateX, rotateY := utils.RotateVector(
			index,
			cardinalPosition[0],
			cardinalPosition[1],
			canvasSize,
			canvasSize,
		)

		translatedVector := utils.SumVectors(
			[2]int{rotateX, rotateY},
			[2]int{canvasSize / 2, canvasSize / 2},
		)

		x, y = translatedVector[0], translatedVector[1]
	}

	if radius == 0 {
		radius = utils.BallSize
	}

	maxVelocity := utils.MaxVelocity
	minVelocity := utils.MinVelocity

	cardinalVX := minVelocity + rand.Intn(maxVelocity-minVelocity)
	cardinalVY := utils.RandomNumberN(maxVelocity)

	vx, vy := utils.RotateVector(index, -cardinalVX, cardinalVY, 1, 1)

	return &Ball{
		X:          x,
		Y:          y,
		Vx:         vx,
		Vy:         vy,
		Radius:     radius,
		Index:      index,
		canvasSize: canvasSize,
		channel:    channel,
	}
}

func (ball *Ball) Engine() {
	for {
		if ball == nil {
			fmt.Println("player ball", ball.Index, "disconnected")
			return
		}

		ball.Move()
		ball.channel <- BallPositionMessage{ball}
		time.Sleep(utils.Period)
	}
}

func (ball *Ball) Move() {

	ball.X += ball.Vx + ball.Ax/2
	ball.Y += ball.Vy + ball.Ay/2

	ball.Vx += ball.Ax
	ball.Vy += ball.Ay

}

func (ball *Ball) getCenterIndex(grid Grid) (x, y int) {
	cellSize := utils.CellSize
	row := ball.X / cellSize
	col := ball.Y / cellSize
	return row, col
}

func (ball *Ball) HandleCollideRight() {
	ball.Vx = -utils.Abs(ball.Vx)
}

func (ball *Ball) HandleCollideLeft() {
	ball.Vx = utils.Abs(ball.Vx)
}

func (ball *Ball) HandleCollideTop() {
	ball.Vy = utils.Abs(ball.Vy)
}

func (ball *Ball) HandleCollideBottom() {
	ball.Vy = -utils.Abs(ball.Vy)
}

func (ball *Ball) ReflectVelocityX() {
	ball.Vx = -ball.Vx
}

func (ball *Ball) ReflectVelocityY() {
	ball.Vy = -ball.Vy
}
