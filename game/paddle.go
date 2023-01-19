package game

import (
	"encoding/json"
	"fmt"

	"github.com/lguibr/pongo/utils"
)

type Paddle struct {
	X         int     `json:"x"`
	Y         int     `json:"y"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	Index     int     `json:"index"`
	Direction string  `json:"direction"`
	Velocity  int     `json:"velocity"`
	Canvas    *Canvas `json:"canvas"`
}

func (paddle *Paddle) Move() {
	if paddle.Direction != "left" && paddle.Direction != "right" {
		return
	}

	velocity := [2]int{0, paddle.Velocity}

	if paddle.Index%2 != 0 {
		velocity = utils.SwapVectorCoordinates(velocity)
	}

	velocityX, velocityY := velocity[0], velocity[1]
	if paddle.Direction == "left" {

		if paddle.X-velocityX < 0 || paddle.Y+velocityY < 0 {
			return
		}

		paddle.X -= velocityX
		paddle.Y -= velocityY
	} else {

		if paddle.X+paddle.Width+velocityX > utils.CanvasSize || paddle.Y+paddle.Height-velocityY > utils.CanvasSize {
			return
		}

		paddle.X += velocityX
		paddle.Y += velocityY
	}

}

func NewPaddle(canvas *Canvas, index int) *Paddle {

	offSet := -utils.PaddleLength/2 + utils.PaddleWeight/2
	if index > 1 {
		offSet = -offSet
	}

	cardinalPosition := [2]int{utils.CanvasSize/2 - utils.PaddleWeight/2, offSet}
	rotateX, rotateY := utils.RotateVector(index, cardinalPosition[0], cardinalPosition[1], utils.CanvasSize, utils.CanvasSize)
	translatedVector := utils.SumVectors([2]int{rotateX, rotateY}, [2]int{utils.CanvasSize/2 - utils.PaddleWeight/2, utils.CanvasSize/2 - utils.PaddleWeight/2})
	x, y := translatedVector[0], translatedVector[1]

	indexOdd := index % 2
	var width, height int

	if indexOdd == 0 {
		height = utils.PaddleLength
		width = utils.PaddleWeight
	} else {
		width = utils.PaddleLength
		height = utils.PaddleWeight
	}

	return &Paddle{
		X:         x,
		Y:         y,
		Index:     index,
		Width:     width,
		Height:    height,
		Direction: "",
		Velocity:  utils.MinVelocity * 2,
		Canvas:    canvas,
	}
}

type Direction struct {
	Direction string `json:"direction"`
}

func (paddle *Paddle) SetDirection(buffer []byte) {
	direction := Direction{}
	err := json.Unmarshal(buffer, &direction)
	if err != nil {
		fmt.Println("Error unmarshalling message:", err)
	}
	newDirection := utils.DirectionFromString(direction.Direction)
	paddle.Direction = newDirection
}
