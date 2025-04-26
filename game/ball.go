package game

import (
	"fmt"
	"math"
	"math/rand" // Needed for NewBall velocity

	"github.com/lguibr/pongo/utils"
)

// --- Message Types for Ball Communication ---

// BallPositionMessage signals the ball's current state (sent by BallActor).
type BallPositionMessage struct {
	Ball *Ball // Pointer to a state snapshot
}

// --- Ball Struct (State Holder) ---

type Ball struct {
	X          int  `json:"x"`
	Y          int  `json:"y"`
	Vx         int  `json:"vx"`
	Vy         int  `json:"vy"`
	Ax         int  `json:"ax"` // Acceleration - currently unused
	Ay         int  `json:"ay"` // Acceleration - currently unused
	Radius     int  `json:"radius"`
	Id         int  `json:"id"`         // Unique ID (e.g., timestamp)
	OwnerIndex int  `json:"ownerIndex"` // Index of the player who last hit it
	Phasing    bool `json:"phasing"`    // Is the ball currently phasing? (Managed by BallActor)
	Mass       int  `json:"mass"`
	canvasSize int  // Keep for boundary checks within Move or getters if needed
}

func (b *Ball) GetX() int      { return b.X }
func (b *Ball) GetY() int      { return b.Y }
func (b *Ball) GetRadius() int { return b.Radius }

// NewBall creates the initial state data structure for a ball.
func NewBall(x, y, radius, canvasSize, ownerIndex, index int) *Ball {
	if x == 0 && y == 0 {
		paddleOffset := utils.PaddleWeight * 2
		switch ownerIndex {
		case 0:
			x = canvasSize - paddleOffset - utils.BallSize
			y = canvasSize / 2
		case 1:
			x = canvasSize / 2
			y = paddleOffset + utils.BallSize
		case 2:
			x = paddleOffset + utils.BallSize
			y = canvasSize / 2
		case 3:
			x = canvasSize / 2
			y = canvasSize - paddleOffset - utils.BallSize
		default:
			x = canvasSize / 2
			y = canvasSize / 2
		}
	}

	mass := utils.BallMass
	if radius == 0 {
		radius = utils.BallSize
	}

	targetX, targetY := canvasSize/2, canvasSize/2
	dx := targetX - x
	dy := targetY - y
	dist := math.Sqrt(float64(dx*dx + dy*dy))

	baseSpeed := float64(utils.MinVelocity + rand.Intn(utils.MaxVelocity-utils.MinVelocity+1))
	var vx, vy int
	if dist > 0 {
		vx = int(baseSpeed * float64(dx) / dist)
		vy = int(baseSpeed * float64(dy) / dist)
	} else {
		vx = utils.RandomNumberN(utils.MaxVelocity)
		vy = utils.RandomNumberN(utils.MaxVelocity)
	}
	if vx == 0 && vy == 0 {
		vx = utils.MinVelocity
	}

	return &Ball{
		X:          x,
		Y:          y,
		Vx:         vx,
		Vy:         vy,
		Radius:     radius,
		Id:         index,
		OwnerIndex: ownerIndex,
		canvasSize: canvasSize, // Store canvasSize
		Mass:       mass,
		Phasing:    false,
	}
}

// Move updates the ball's position based on velocity. Called by BallActor.
func (ball *Ball) Move() {
	ball.X += ball.Vx
	ball.Y += ball.Vy
}

// getCenterIndex calculates the grid cell indices for the ball's center.
// Used by GameActor for collision checks.
func (ball *Ball) getCenterIndex() (col, row int) { // Return col, row
	// Ensure canvasSize is valid before division
	if ball.canvasSize <= 0 || utils.GridSize <= 0 {
		fmt.Printf("WARN: getCenterIndex called with invalid canvasSize (%d) or GridSize (%d)\n", ball.canvasSize, utils.GridSize)
		return 0, 0
	}
	cellSize := ball.canvasSize / utils.GridSize // Calculate cellSize internally
	if cellSize == 0 {
		fmt.Printf("WARN: getCenterIndex calculated cellSize = 0 (canvasSize=%d, gridSize=%d)\n", ball.canvasSize, utils.GridSize)
		return 0, 0
	}
	gridSize := ball.canvasSize / cellSize // Recalculate gridSize based on actual cellSize

	// Calculate indices based on integer division
	col = ball.X / cellSize
	row = ball.Y / cellSize

	// --- DEBUG PRINT ---
	// fmt.Printf("DEBUG getCenterIndex: Ball(%d,%d) canvas=%d grid=%d cell=%d -> Raw Index (col=%d, row=%d)\n",
	// 	ball.X, ball.Y, ball.canvasSize, gridSize, cellSize, col, row)
	// --- END DEBUG ---

	// Clamp indices to grid boundaries [0, gridSize-1]
	finalCol := utils.MaxInt(0, utils.MinInt(gridSize-1, col))
	finalRow := utils.MaxInt(0, utils.MinInt(gridSize-1, row))

	// --- DEBUG PRINT ---
	// if finalCol != col || finalRow != row {
	// 	fmt.Printf("DEBUG getCenterIndex: Clamped Index (col=%d, row=%d)\n", finalCol, finalRow)
	// }
	// --- END DEBUG ---

	return finalCol, finalRow
}

// --- Velocity/State Modification Methods (Called by BallActor via messages) ---

// ReflectVelocity reverses the velocity along the specified axis.
func (ball *Ball) ReflectVelocity(axis string) {
	if axis == "X" {
		ball.Vx = -ball.Vx
	} else if axis == "Y" {
		ball.Vy = -ball.Vy
	}
}

// SetVelocity directly sets the ball's velocity components.
func (ball *Ball) SetVelocity(vx, vy int) {
	ball.Vx = vx
	ball.Vy = vy
}

// IncreaseVelocity scales the ball's velocity components.
func (ball *Ball) IncreaseVelocity(ratio float64) {
	newVx := int(math.Floor(float64(ball.Vx) * ratio))
	newVy := int(math.Floor(float64(ball.Vy) * ratio))
	if ball.Vx != 0 && newVx == 0 {
		newVx = int(math.Copysign(1, float64(ball.Vx)))
	}
	if ball.Vy != 0 && newVy == 0 {
		newVy = int(math.Copysign(1, float64(ball.Vy)))
	}
	ball.Vx = newVx
	ball.Vy = newVy
}

// IncreaseMass increases the ball's mass and scales its radius slightly.
func (ball *Ball) IncreaseMass(additional int) {
	ball.Mass += additional
	ball.Radius += additional * 2
	if ball.Radius <= 0 {
		ball.Radius = 1
	}
}

// --- Geometric Intersection Checks (Used by GameActor) ---

// BallInterceptPaddles checks for intersection with a paddle.
func (ball *Ball) BallInterceptPaddles(paddle *Paddle) bool {
	if paddle == nil {
		return false
	}
	closestX := float64(utils.MaxInt(paddle.X, utils.MinInt(ball.X, paddle.X+paddle.Width)))
	closestY := float64(utils.MaxInt(paddle.Y, utils.MinInt(ball.Y, paddle.Y+paddle.Height)))
	distanceX := float64(ball.X) - closestX
	distanceY := float64(ball.Y) - closestY
	distanceSquared := (distanceX * distanceX) + (distanceY * distanceY)
	return distanceSquared < float64(ball.Radius*ball.Radius)
}

// InterceptsIndex checks if the ball circle intersects with a grid cell rectangle.
func (ball *Ball) InterceptsIndex(col, row, cellSize int) bool { // Use col, row consistent with getCenterIndex
	if cellSize <= 0 {
		return false
	}
	cellLeft := col * cellSize
	cellTop := row * cellSize
	cellRight := cellLeft + cellSize
	cellBottom := cellTop + cellSize
	closestX := float64(utils.MaxInt(cellLeft, utils.MinInt(ball.X, cellRight)))
	closestY := float64(utils.MaxInt(cellTop, utils.MinInt(ball.Y, cellBottom)))
	distanceX := float64(ball.X) - closestX
	distanceY := float64(ball.Y) - closestY
	distanceSquared := (distanceX * distanceX) + (distanceY * distanceY)
	return distanceSquared < float64(ball.Radius*ball.Radius)
}
