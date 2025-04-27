// File: game/ball.go
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
	X  int `json:"x"`
	Y  int `json:"y"`
	Vx int `json:"vx"`
	Vy int `json:"vy"`
	// Ax         int  `json:"ax"` // Acceleration - removed
	// Ay         int  `json:"ay"` // Acceleration - removed
	Radius      int  `json:"radius"`
	Id          int  `json:"id"`         // Unique ID (e.g., timestamp + index)
	OwnerIndex  int  `json:"ownerIndex"` // Index of the player who last hit it
	Phasing     bool `json:"phasing"`    // Is the ball currently phasing? (Managed by BallActor)
	Mass        int  `json:"mass"`
	IsPermanent bool `json:"isPermanent"` // True if this is the player's initial, non-expiring ball
	canvasSize  int  // Keep for boundary checks within Move or getters if needed
}

func (b *Ball) GetX() int      { return b.X }
func (b *Ball) GetY() int      { return b.Y }
func (b *Ball) GetRadius() int { return b.Radius }

// NewBall creates the initial state data structure for a ball.
// Added isPermanent flag. Uses config for defaults.
func NewBall(cfg utils.Config, x, y, ownerIndex, index int, isPermanent bool) *Ball {
	// Determine initial position if not provided
	if x == 0 && y == 0 {
		paddleOffset := cfg.PaddleWidth * 2
		switch ownerIndex {
		case 0: // Right
			x = cfg.CanvasSize - paddleOffset - cfg.BallRadius
			y = cfg.CanvasSize / 2
		case 1: // Top
			x = cfg.CanvasSize / 2
			y = paddleOffset + cfg.BallRadius
		case 2: // Left
			x = paddleOffset + cfg.BallRadius
			y = cfg.CanvasSize / 2
		case 3: // Bottom
			x = cfg.CanvasSize / 2
			y = cfg.CanvasSize - paddleOffset - cfg.BallRadius
		default: // Center as fallback
			x = cfg.CanvasSize / 2
			y = cfg.CanvasSize / 2
		}
	}

	mass := cfg.BallMass
	radius := cfg.BallRadius

	// --- New Velocity Calculation ---
	// Generate a random angle in radians (avoiding angles too close to horizontal/vertical)
	angleOffset := math.Pi / 12                                     // ~15 degrees offset from axes
	angle := angleOffset + rand.Float64()*(math.Pi/2-2*angleOffset) // Angle within the first quadrant section

	// Randomly assign quadrant based on owner index (roughly towards center)
	switch ownerIndex {
	case 0: // Right player -> towards left (Quadrant 2 or 3)
		angle += math.Pi / 2
		if rand.Intn(2) == 0 {
			angle += math.Pi
		}
	case 1: // Top player -> towards bottom (Quadrant 3 or 4)
		angle += math.Pi
		if rand.Intn(2) == 0 {
			angle += math.Pi / 2
		}
	case 2: // Left player -> towards right (Quadrant 1 or 4)
		if rand.Intn(2) == 0 {
			angle += 3 * math.Pi / 2
		}
	case 3: // Bottom player -> towards top (Quadrant 1 or 2)
		angle += 3 * math.Pi / 2
		if rand.Intn(2) == 0 {
			angle += math.Pi / 2
		}
	}

	// Generate a random speed within the defined range from config
	speed := float64(cfg.MinBallVelocity + rand.Intn(cfg.MaxBallVelocity-cfg.MinBallVelocity+1))

	// Calculate Vx and Vy based on angle and speed
	vxFloat := speed * math.Cos(angle)
	vyFloat := speed * math.Sin(angle)
	vx := int(vxFloat)
	vy := int(vyFloat)

	// Ensure velocity components are not zero if speed is non-zero
	if speed > 0 {
		if vx == 0 {
			vx = int(math.Copysign(1.0, vxFloat)) // Set to +/- 1 based on original float sign
		}
		if vy == 0 {
			vy = int(math.Copysign(1.0, vyFloat)) // Set to +/- 1 based on original float sign
		}
	}
	// --- End New Velocity Calculation ---

	return &Ball{
		X:           x,
		Y:           y,
		Vx:          vx,
		Vy:          vy,
		Radius:      radius,
		Id:          index,
		OwnerIndex:  ownerIndex,
		canvasSize:  cfg.CanvasSize, // Store canvasSize
		Mass:        mass,
		Phasing:     false,
		IsPermanent: isPermanent, // Set the flag
	}
}

// Move updates the ball's position based on velocity. Called by BallActor.
func (ball *Ball) Move() {
	ball.X += ball.Vx
	ball.Y += ball.Vy
}

// getCenterIndex calculates the grid cell indices for the ball's center.
// Used by GameActor for collision checks. Uses config.
func (ball *Ball) getCenterIndex(cfg utils.Config) (col, row int) { // Return col, row
	if ball.canvasSize <= 0 || cfg.GridSize <= 0 {
		fmt.Printf("WARN: getCenterIndex called with invalid canvasSize (%d) or GridSize (%d)\n", ball.canvasSize, cfg.GridSize)
		return 0, 0
	}
	cellSize := ball.canvasSize / cfg.GridSize
	if cellSize == 0 {
		fmt.Printf("WARN: getCenterIndex calculated cellSize = 0 (canvasSize=%d, gridSize=%d)\n", ball.canvasSize, cfg.GridSize)
		return 0, 0
	}
	gridSize := ball.canvasSize / cellSize // Recalculate based on actual cell size

	col = ball.X / cellSize
	row = ball.Y / cellSize

	finalCol := utils.MaxInt(0, utils.MinInt(gridSize-1, col))
	finalRow := utils.MaxInt(0, utils.MinInt(gridSize-1, row))

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
	// Prevent velocity from becoming zero if it wasn't already
	if ball.Vx != 0 && newVx == 0 {
		newVx = int(math.Copysign(1, float64(ball.Vx)))
	}
	if ball.Vy != 0 && newVy == 0 {
		newVy = int(math.Copysign(1, float64(ball.Vy)))
	}
	ball.Vx = newVx
	ball.Vy = newVy
}

// IncreaseMass increases the ball's mass and scales its radius slightly. Uses config.
func (ball *Ball) IncreaseMass(cfg utils.Config, additional int) {
	ball.Mass += additional
	// Increase radius proportionally, ensure minimum radius
	ball.Radius += additional * cfg.PowerUpIncreaseMassSize // Use config for scaling
	if ball.Radius <= 0 {
		ball.Radius = 1 // Ensure radius is always positive
	}
}

// --- Geometric Intersection Checks (Used by GameActor) ---

// BallInterceptPaddles checks for intersection with a paddle.
func (ball *Ball) BallInterceptPaddles(paddle *Paddle) bool {
	if paddle == nil {
		return false
	}
	// Find the closest point on the paddle rectangle to the ball's center
	closestX := float64(utils.MaxInt(paddle.X, utils.MinInt(ball.X, paddle.X+paddle.Width)))
	closestY := float64(utils.MaxInt(paddle.Y, utils.MinInt(ball.Y, paddle.Y+paddle.Height)))

	// Calculate the distance between the ball's center and this closest point
	distanceX := float64(ball.X) - closestX
	distanceY := float64(ball.Y) - closestY

	// If the distance is less than the ball's radius, an intersection occurs
	distanceSquared := (distanceX * distanceX) + (distanceY * distanceY)
	return distanceSquared < float64(ball.Radius*ball.Radius)
}

// InterceptsIndex checks if the ball circle intersects with a grid cell rectangle.
func (ball *Ball) InterceptsIndex(col, row, cellSize int) bool { // Use col, row consistent with getCenterIndex
	if cellSize <= 0 {
		return false
	}
	// Cell boundaries
	cellLeft := col * cellSize
	cellTop := row * cellSize
	cellRight := cellLeft + cellSize
	cellBottom := cellTop + cellSize

	// Find the closest point on the cell rectangle to the ball's center
	closestX := float64(utils.MaxInt(cellLeft, utils.MinInt(ball.X, cellRight)))
	closestY := float64(utils.MaxInt(cellTop, utils.MinInt(ball.Y, cellBottom)))

	// Calculate the distance between the ball's center and this closest point
	distanceX := float64(ball.X) - closestX
	distanceY := float64(ball.Y) - closestY

	// If the distance is less than the ball's radius, an intersection occurs
	distanceSquared := (distanceX * distanceX) + (distanceY * distanceY)
	return distanceSquared < float64(ball.Radius*ball.Radius)
}
