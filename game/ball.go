package game

import (
	"fmt"
	"math"
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

	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			surroundingRow, surroundingCol := row+i, col+j

			if surroundingRow < 0 || surroundingRow > b.Canvas.GridSize-1 || surroundingCol < 0 || surroundingCol > b.Canvas.GridSize-1 {
				continue
			}

			ballInterceptsCell := b.BallInterceptCellIndex(surroundingRow, surroundingCol)

			if ballInterceptsCell {
				t := b.Canvas.Grid[surroundingRow][surroundingCol].Data.Type
				if t == utils.CellTypes["Brick"] {
					b.handleCollideBrick([2]int{row, col}, [2]int{surroundingRow, surroundingCol})
				}
				if t == utils.CellTypes["Block"] {
					b.handleCollideBlock([2]int{row, col}, [2]int{surroundingRow, surroundingCol})
				}
			}
		}
	}

	newIntersectedCell := [2]int{row, col}

	if newIntersectedCell != b.interceptingCell {
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
func (ball *Ball) BallInterceptCellIndex(x, y int) bool {
	cellSize := ball.Canvas.CellSize

	leftTopX := x * cellSize
	leftTopY := y * cellSize

	// calculate the position of the bottom right corner of the grid cell
	bottomRightX := leftTopX + cellSize
	bottomRightY := leftTopY + cellSize

	// check if the center of the ball is within the boundaries of the grid cell
	if ball.X > leftTopX &&
		ball.X < bottomRightX &&
		ball.Y > leftTopY &&
		ball.Y < bottomRightY {
		return true
	}

	// check if the distance between the center of the ball and the closest point on the grid cell boundary is less than the radius of the ball
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

	// calculate the position of the top left corner of the grid cell
	paddleTopLeftX := paddle.X
	paddleTopLeftY := paddle.Y

	// calculate the position of the bottom right corner of the grid cell
	paddleBottomRightX := paddleTopLeftX + paddle.Width
	paddleBottomRightY := paddleTopLeftY + paddle.Height

	// check if the center of the ball is within the boundaries of the grid cell
	if ball.X > paddleTopLeftX &&
		ball.X < paddleBottomRightX &&
		ball.Y > paddleTopLeftY &&
		ball.Y < paddleBottomRightY {
		return true
	}

	// check if the distance between the center of the ball and the closest point on the grid cell boundary is less than the radius of the ball
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
