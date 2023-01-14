package game

import (
	"encoding/json"
	"fmt"
	"math/rand"

	"github.com/lguibr/pongo/utils"
)

type Ball struct {
	X                int     `json:"x"`
	Y                int     `json:"y"`
	Vx               int     `json:"vx"`
	Vy               int     `json:"vy"`
	Ax               int     `json:"ax"`
	Ay               int     `json:"ay"`
	Radius           int     `json:"radius"`
	Canvas           *Canvas `json:"canvas"`
	Index            int     `json:"index"`
	interceptingCell [2]int
}

func (b *Ball) Move() {

	b.X += b.Vx + b.Ax/2
	b.Y += b.Vy + b.Ay/2

	b.Vx += b.Ax
	b.Vy += b.Ay

}

func (b *Ball) CollideCells() {

	row, col := b.GetIntersectedIndices(b.Canvas.Grid)

	if row < 0 || row > b.Canvas.GridSize-1 || col < 0 || col > b.Canvas.GridSize-1 {
		return
	}
	newIntersectedCell := [2]int{row, col}
	if newIntersectedCell != b.interceptingCell {
		t := b.Canvas.Grid[row][col].Data.Type
		if t == utils.CellTypes["Brick"] {
			b.handleCollideBrick(b.interceptingCell, newIntersectedCell)
		}
		if t == utils.CellTypes["Block"] {
			b.handleCollideBlock(b.interceptingCell, newIntersectedCell)
		}
		b.interceptingCell = newIntersectedCell
	}
}

func (b *Ball) handleCollideBrick(oldIndices, newIndices [2]int) {
	b.handleCollideBlock(oldIndices, newIndices)
	b.Canvas.Grid[newIndices[0]][newIndices[1]].Data.Life -= 1
	if b.Canvas.Grid[newIndices[0]][newIndices[1]].Data.Life == 0 {
		b.Canvas.Grid[newIndices[0]][newIndices[1]].Data.Type = utils.CellTypes["Empty"]
	}
}

func (b *Ball) handleCollideBlock(oldIndices, newIndices [2]int) {
	velocityReflector := utils.SubtractVectors(oldIndices, newIndices)

	if velocityReflector[0] != 0 {
		b.Vx = -b.Vx
	}
	if velocityReflector[1] != 0 {
		b.Vy = -b.Vy
	}

}

func (b *Ball) CollideWalls() {
	if b.CollideBottomWall() {
		fmt.Println("Collide bottom wall")
		b.HandleCollideBottom()
	}
	if b.CollideTopWall() {
		fmt.Println("Collide top wall")
		b.HandleCollideTop()
	}
	if b.CollideLeftWall() {
		fmt.Println("Collide left wall")
		b.HandleCollideLeft()
	}
	if b.CollideRightWall() {
		fmt.Println("Collide right wall")
		b.HandleCollideRight()
	}
}

func (ball *Ball) CollidePaddles(players [4]*Player) {
	for _, player := range players {
		if player == nil {
			continue
		}
		ball.CollidePaddle(player.Paddle)
	}
}

func (ball *Ball) GetIntersectedIndices(grid Grid) (x, y int) {
	cellSize := utils.CellSize
	row := ball.X / cellSize
	col := ball.Y / cellSize
	return row, col
}

func (ball *Ball) CollidePaddle(paddle *Paddle) {
	collisionDetectors := [4]func(*Paddle) bool{
		ball.CollideOnRightPaddle,
		ball.CollideOnBottomPaddle,
		ball.CollideOnLeftPaddle,
		ball.CollideOnTopPaddle,
	}
	collisionDetector := collisionDetectors[paddle.Index]
	collisionDetected := collisionDetector(paddle)

	if collisionDetected {
		handlers := [4]func(){
			ball.HandleCollideRight,
			ball.HandleCollideBottom,
			ball.HandleCollideLeft,
			ball.HandleCollideTop,
		}
		handlerCollision := handlers[paddle.Index]
		handlerCollision()
	}
}

func (ball *Ball) CollideOnTopPaddle(paddle *Paddle) bool {
	collides := ball.Y-ball.Radius <= paddle.Y+paddle.Height &&
		ball.Y-ball.Radius >= paddle.Y && ball.X >= paddle.X &&
		ball.X <= paddle.X+paddle.Width

	if collides {
		fmt.Println("Collide on top paddle")
	}

	return collides

}

func (ball *Ball) CollideOnBottomPaddle(paddle *Paddle) bool {
	collides := ball.X-ball.Radius <= paddle.X+paddle.Width &&
		ball.X-ball.Radius >= paddle.X &&
		ball.Y >= paddle.Y && ball.Y <= paddle.Y+paddle.Height

	if collides {
		fmt.Println("Collide on bottom paddle")
	}

	return collides
}

func (ball *Ball) CollideOnLeftPaddle(paddle *Paddle) bool {
	collides := ball.X-ball.Radius <= paddle.X+paddle.Width/2 &&
		ball.Y+ball.Radius >= paddle.Y-paddle.Height/2 &&
		ball.Y-ball.Radius <= paddle.Y+paddle.Height/2

	if collides {
		fmt.Println("Collide on left paddle")
	}

	return collides
}

func (ball *Ball) CollideOnRightPaddle(paddle *Paddle) bool {
	collides := ball.X+ball.Radius >= paddle.X &&
		ball.X+ball.Radius <= paddle.X+paddle.Width &&
		ball.Y >= paddle.Y &&
		ball.Y <= paddle.Y+paddle.Height

	if collides {

		fmt.Println("Collide on right paddle")
	}
	return collides
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

func (ball *Ball) CollideTopWall() bool {
	return ball.Y-ball.Radius <= 0
}

func (ball *Ball) CollideBottomWall() bool {
	return ball.Y+ball.Radius >= ball.Canvas.Height
}

func (ball *Ball) CollideRightWall() bool {
	return ball.X+ball.Radius >= ball.Canvas.Width
}

func (ball *Ball) CollideLeftWall() bool {
	return ball.X-ball.Radius <= 0
}

func NewBall(canvas *Canvas, x, y, radius, index int) *Ball {

	if x == 0 && y == 0 {
		cardinalPosition := [2]int{utils.CanvasSize/2 - utils.CellSize*1.5, 0}

		rotateX, rotateY := utils.RotateVector(
			index,
			cardinalPosition[0],
			cardinalPosition[1],
			utils.CanvasSize,
			utils.CanvasSize,
		)

		translatedVector := utils.SumVectors(
			[2]int{rotateX, rotateY},
			[2]int{utils.CanvasSize / 2, utils.CanvasSize / 2},
		)

		x, y = translatedVector[0], translatedVector[1]
	}

	if radius == 0 {
		radius = utils.BallSize
	}

	maxVelocity := utils.MaxVelocity
	minVelocity := utils.MinVelocity

	cardinalVX := minVelocity + rand.Intn(maxVelocity-minVelocity)

	cardinalVY := minVelocity + rand.Intn(maxVelocity-minVelocity)
	vx, vy := utils.RotateVector(index, -cardinalVX, cardinalVY, 1, 1)

	return &Ball{
		X:      x,
		Y:      y,
		Vx:     vx,
		Vy:     vy,
		Radius: radius,
		Canvas: canvas,
		Index:  index,
		interceptingCell: [2]int{
			x / utils.CellSize,
			y / utils.CellSize,
		},
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
