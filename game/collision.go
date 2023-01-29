package game

import (
	"math"

	"github.com/lguibr/pongo/utils"
)

func (ball *Ball) CollidesTopWall() bool {
	return ball.Y-ball.Radius <= 0
}

func (ball *Ball) CollidesBottomWall() bool {
	return ball.Y+ball.Radius >= ball.canvasSize
}

func (ball *Ball) CollidesRightWall() bool {
	return ball.X+ball.Radius >= ball.canvasSize
}

func (ball *Ball) CollidesLeftWall() bool {
	return ball.X-ball.Radius <= 0
}

func (ball *Ball) CollidePaddle(paddle *Paddle) {
	if paddle == nil {
		return
	}

	collisionDetected := ball.BallInterceptPaddles(paddle)
	if collisionDetected {
		ball.OwnerIndex = paddle.Index
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

func (ball *Ball) CollideCells(grid Grid, cellSize int) {
	gridSize := len(grid)
	row, col := ball.getCenterIndex(grid)
	if row < 0 || row > gridSize-1 || col < 0 || col > gridSize-1 {
		return
	}

	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			surroundingRow, surroundingCol := row+i, col+j
			if surroundingRow < 0 || surroundingRow > gridSize-1 || surroundingCol < 0 || surroundingCol > gridSize-1 {
				continue
			}

			ballInterceptsCell := ball.InterceptsIndex(surroundingRow, surroundingCol, cellSize)
			if ballInterceptsCell {
				t := grid[surroundingRow][surroundingCol].Data.Type
				if t == utils.Cells.Brick {
					ball.handleCollideBrick([2]int{row, col}, [2]int{surroundingRow, surroundingCol}, grid)
					return
				}
				if t == utils.Cells.Block {
					ball.handleCollideBlock([2]int{row, col}, [2]int{surroundingRow, surroundingCol})
					return
				}
			}
		}
	}
}

func (ball *Ball) CollideWalls() {
	if ball.CollidesBottomWall() {
		ball.HandleCollideBottom()
	}
	if ball.CollidesLeftWall() {
		ball.HandleCollideLeft()
	}
	if ball.CollidesTopWall() {
		ball.HandleCollideTop()
	}
	if ball.CollidesRightWall() {
		ball.HandleCollideRight()
	}
}

func (ball *Ball) CollidePaddles(paddles [4]*Paddle) {
	for _, paddle := range paddles {
		if paddle == nil {
			continue
		}
		ball.CollidePaddle(paddle)
	}
}

func (ball *Ball) handleCollideBrick(oldIndices, newIndices [2]int, grid Grid) {
	ball.handleCollideBlock(oldIndices, newIndices)
	grid[newIndices[0]][newIndices[1]].Data.Life -= 1
	if grid[newIndices[0]][newIndices[1]].Data.Life == 0 {
		grid[newIndices[0]][newIndices[1]].Data.Type = utils.Cells.Empty
	}
}

func (ball *Ball) handleCollideBlock(oldIndices, newIndices [2]int) {
	velocityReflector := utils.SubtractVectors(oldIndices, newIndices)

	if velocityReflector[0] != 0 {
		ball.ReflectVelocityX()
	}
	if velocityReflector[1] != 0 {
		ball.ReflectVelocityY()
	}

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

func (ball *Ball) InterceptsIndex(x, y, cellSize int) bool {
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
