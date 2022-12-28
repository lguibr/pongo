package game

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/lguibr/pongo/utils"
)

type Ball struct {
	X       int     `json:"x"`
	Y       int     `json:"y"`
	Vx      int     `json:"vx"`
	Vy      int     `json:"vy"`
	Ax      int     `json:"ax"`
	Ay      int     `json:"ay"`
	Radius  int     `json:"radius"`
	OwnerId string  `json:"ownerId"`
	Canvas  *Canvas `json:"canvas"`
}

func (b *Ball) Move() {
	b.X += b.Vx + b.Ax/2
	b.Y += b.Vy + b.Ay/2

	b.Vx += b.Ax
	b.Vy += b.Ay
}

func (b *Ball) Bounce() {
	if b.X-b.Radius < 0 || b.X+b.Radius > b.Canvas.Width {
		b.Vx = -b.Vx
	}
	if b.Y-b.Radius < 0 || b.Y+b.Radius > b.Canvas.Height {
		b.Vy = -b.Vy
	}
}

func CreateBall(ownerId string, canvas *Canvas, x, y, radius int) *Ball {

	if x == 0 && y == 0 {
		x = canvas.Width / 2
		y = canvas.Height / 2
	}

	if radius == 0 {
		radius = 10
	}

	maxVelocity := utils.MaxVelocity
	minVelocity := utils.MinVelocity

	rand.Seed(time.Now().UnixNano())

	vx := rand.Intn(maxVelocity-minVelocity+1) + minVelocity
	vy := rand.Intn(maxVelocity-minVelocity+1) + minVelocity

	return &Ball{
		X:       x,
		Y:       y,
		Vx:      vx,
		Vy:      vy,
		Radius:  radius,
		OwnerId: ownerId,
		Canvas:  canvas,
	}
}

func (b *Ball) ToJson() []byte {
	ball, err := json.Marshal(b)
	if err != nil {
		fmt.Println(err)
		return []byte{}
	}
	return ball
}
