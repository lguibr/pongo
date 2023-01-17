package game

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/lguibr/pongo/utils"
)

type Ball struct {
	X      int     `json:"x"`
	Y      int     `json:"y"`
	Vx     int     `json:"vx"`
	Vy     int     `json:"vy"`
	Ax     int     `json:"ax"`
	Ay     int     `json:"ay"`
	Radius int     `json:"radius"`
	Canvas *Canvas `json:"canvas"`
	Index  int     `json:"index"`
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
	}
}

func (ball *Ball) Move() {

	ball.X += ball.Vx + ball.Ax/2
	ball.Y += ball.Vy + ball.Ay/2

	ball.Vx += ball.Ax
	ball.Vy += ball.Ay

}

func (ball *Ball) CollidePaddle(paddle *Paddle) {

	collisionDetected := ball.BallInterceptPaddles(paddle)

	if collisionDetected {
		handlers := [4]func(){
			ball.HandleCollideRight,
			ball.HandleCollideTop,
			ball.HandleCollideLeft,
			ball.HandleCollideBottom,
		}

		handlerCollision := handlers[paddle.Index]
		handlerCollision()
	}
}

func (ball *Ball) CollideCells() {

	row, col := ball.GetIntersectedIndices(ball.Canvas.Grid)

	if row < 0 || row > ball.Canvas.GridSize-1 || col < 0 || col > ball.Canvas.GridSize-1 {
		return
	}

	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			surroundingRow, surroundingCol := row+i, col+j

			if surroundingRow < 0 || surroundingRow > ball.Canvas.GridSize-1 || surroundingCol < 0 || surroundingCol > ball.Canvas.GridSize-1 {
				continue
			}

			ballInterceptsCell := ball.BallInterceptCellIndex(surroundingRow, surroundingCol)

			if ballInterceptsCell {
				t := ball.Canvas.Grid[surroundingRow][surroundingCol].Data.Type
				if t == utils.CellTypes["Brick"] {
					ball.handleCollideBrick([2]int{row, col}, [2]int{surroundingRow, surroundingCol})
				}
				if t == utils.CellTypes["Block"] {
					ball.handleCollideBlock([2]int{row, col}, [2]int{surroundingRow, surroundingCol})
				}
			}
		}
	}
}

func (ball *Ball) CollideWalls() {
	if ball.CollideBottomWall() {
		fmt.Println("Collide bottom wall")
		ball.HandleCollideBottom()
	}
	if ball.CollideTopWall() {
		fmt.Println("Collide top wall")
		ball.HandleCollideTop()
	}
	if ball.CollideLeftWall() {
		fmt.Println("Collide left wall")
		ball.HandleCollideLeft()
	}
	if ball.CollideRightWall() {
		fmt.Println("Collide right wall")
		ball.HandleCollideRight()
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

func (ball *Ball) handleCollideBrick(oldIndices, newIndices [2]int) {
	ball.handleCollideBlock(oldIndices, newIndices)
	ball.Canvas.Grid[newIndices[0]][newIndices[1]].Data.Life -= 1
	if ball.Canvas.Grid[newIndices[0]][newIndices[1]].Data.Life == 0 {
		ball.Canvas.Grid[newIndices[0]][newIndices[1]].Data.Type = utils.CellTypes["Empty"]
	}
}

func (ball *Ball) handleCollideBlock(oldIndices, newIndices [2]int) {
	velocityReflector := utils.SubtractVectors(oldIndices, newIndices)

	if velocityReflector[0] != 0 {
		ball.Vx = -ball.Vx
	}
	if velocityReflector[1] != 0 {
		ball.Vy = -ball.Vy
	}

}

func (ball *Ball) GetIntersectedIndices(grid Grid) (x, y int) {
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

func (ball *Ball) BallInterceptCellIndex(x, y int) bool {
	cellSize := ball.Canvas.CellSize

	leftTopX := x * cellSize
	leftTopY := y * cellSize

	bottomRightX := leftTopX + cellSize
	bottomRightY := leftTopY + cellSize

	if ball.X > leftTopX &&
		ball.X < bottomRightX &&
		ball.Y > leftTopY &&
		ball.Y < bottomRightY {
		return true
	}

	closestX := math.Min(
		math.Max(float64(ball.X), float64(leftTopX)),
		float64(bottomRightX),
	)

	closestY := math.Min(
		math.Max(float64(ball.Y), float64(leftTopY)),
		float64(bottomRightY),
	)

	distance := utils.Distance(
		ball.X,
		ball.Y,
		int(closestX),
		int(closestY),
	)

	return distance < float64(ball.Radius)
}

func (ball *Ball) BallInterceptPaddles(paddle *Paddle) bool {

	paddleTopLeftX := paddle.X
	paddleTopLeftY := paddle.Y

	paddleBottomRightX := paddleTopLeftX + paddle.Width
	paddleBottomRightY := paddleTopLeftY + paddle.Height

	if ball.X > paddleTopLeftX &&
		ball.X < paddleBottomRightX &&
		ball.Y > paddleTopLeftY &&
		ball.Y < paddleBottomRightY {
		return true
	}

	closestX := math.Min(
		math.Max(float64(ball.X), float64(paddleTopLeftX)),
		float64(paddleBottomRightX),
	)

	closestY := math.Min(
		math.Max(float64(ball.Y), float64(paddleTopLeftY)),
		float64(paddleBottomRightY),
	)

	distance := utils.Distance(
		ball.X,
		ball.Y,
		int(closestX),
		int(closestY),
	)

	return distance < float64(ball.Radius)
}
