package game

import (
	"math"
	"math/rand"
	"time"

	"github.com/lguibr/pongo/utils"
)

type BallMessage interface{}

type BallPositionMessage struct {
	Ball *Ball
}

type Ball struct {
	X          int              `json:"x"`
	Y          int              `json:"y"`
	Vx         int              `json:"vx"`
	Vy         int              `json:"vy"`
	Ax         int              `json:"ax"`
	Ay         int              `json:"ay"`
	Radius     int              `json:"radius"`
	Id         int              `json:"id"`
	OwnerIndex int              `json:"ownerIndex"`
	Phasing    bool             `json:"phasing"`
	Mass       int              `json:"mass"`
	Channel    chan BallMessage `json:"-"`
	canvasSize int
	open       bool
}

func (b *Ball) GetX() int      { return b.X }
func (b *Ball) GetY() int      { return b.Y }
func (b *Ball) GetRadius() int { return b.Radius }

func NewBallChannel() chan BallMessage {
	return make(chan BallMessage, 1)

}

func NewBall(channel chan BallMessage, x, y, radius, canvasSize, ownerIndex, index int) *Ball {
	if x == 0 && y == 0 {
		cardinalPosition := [2]int{canvasSize/2 - utils.CellSize*1.5, 0}

		rotateX, rotateY := utils.RotateVector(
			ownerIndex,
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

	mass := utils.BallMass

	if radius == 0 {
		radius = utils.BallSize
	}

	maxVelocity := utils.MaxVelocity
	minVelocity := utils.MinVelocity

	cardinalVX := minVelocity + rand.Intn(maxVelocity-minVelocity)
	cardinalVY := utils.RandomNumberN(maxVelocity)

	vx, vy := utils.RotateVector(ownerIndex, -cardinalVX, cardinalVY, 1, 1)
	return &Ball{
		X:          x,
		Y:          y,
		Vx:         vx,
		Vy:         vy,
		Radius:     radius,
		Id:         index,
		OwnerIndex: ownerIndex,
		canvasSize: canvasSize,
		Channel:    channel,
		open:       true,
		Mass:       mass,
	}
}

func (ball *Ball) Engine() {
	for {
		if !ball.open {
			return
		}
		ball.Move()
		ball.Channel <- BallPositionMessage{ball}
		time.Sleep(utils.Period)
	}
}

func (ball *Ball) Move() {
	ball.X += ball.Vx + ball.Ax/2
	ball.Y += ball.Vy + ball.Ay/2

	ball.Vx += ball.Ax
	ball.Vy += ball.Ay
}

func (ball *Ball) getCenterIndex() (x, y int) {
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

func (ball *Ball) IncreaseVelocity(ratio float64) {
	ball.Vx = int(math.Floor(float64(ball.Vx) * ratio))
	ball.Vy = int(math.Floor(float64(ball.Vy) * ratio))
}

func (ball *Ball) IncreaseMass(additional int) {
	ball.Mass += additional
	ball.Radius += additional * 2
}
func (ball *Ball) SetBallPhasing(expiresIn int) {
	ball.Phasing = true
	go time.AfterFunc(time.Duration(expiresIn)*time.Second, func() {
		ball.Phasing = false
	})

}
