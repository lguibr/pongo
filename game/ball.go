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
	angleOffset := math.Pi / 12
	angle := angleOffset + rand.Float64()*(math.Pi/2-2*angleOffset)

	switch ownerIndex {
	case 0:
		angle += math.Pi / 2
		if rand.Intn(2) == 0 {
			angle += math.Pi
		}
	case 1:
		angle += math.Pi
		if rand.Intn(2) == 0 {
			angle += math.Pi / 2
		}
	case 2:
		if rand.Intn(2) == 0 {
			angle += 3 * math.Pi / 2
		}
	case 3:
		angle += 3 * math.Pi / 2
		if rand.Intn(2) == 0 {
			angle += math.Pi / 2
		}
	}

	speed := float64(cfg.MinBallVelocity + rand.Intn(cfg.MaxBallVelocity-cfg.MinBallVelocity+1))

	vxFloat := speed * math.Cos(angle)
	vyFloat := speed * math.Sin(angle)
	vx := int(vxFloat)
	vy := int(vyFloat)

	if speed > 0 {
		if vx == 0 {
			vx = int(math.Copysign(1.0, vxFloat))
		}
		if vy == 0 {
			vy = int(math.Copysign(1.0, vyFloat))
		}
	}

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
		IsPermanent: isPermanent,
	}
}

// Move updates the ball's position based on velocity and clamps it within bounds. Called by BallActor.
func (ball *Ball) Move() {
	// Update position
	ball.X += ball.Vx
	ball.Y += ball.Vy

	// Clamp position to ensure the ball center stays within canvas boundaries,
	// leaving space for the radius.
	minCoord := ball.Radius
	maxCoord := ball.canvasSize - ball.Radius

	if ball.X < minCoord {
		ball.X = minCoord
		// Optional: Reflect velocity immediately if clamped (can sometimes help prevent sticking)
		// if ball.Vx < 0 { ball.Vx = -ball.Vx }
	} else if ball.X > maxCoord {
		ball.X = maxCoord
		// Optional: Reflect velocity immediately if clamped
		// if ball.Vx > 0 { ball.Vx = -ball.Vx }
	}

	if ball.Y < minCoord {
		ball.Y = minCoord
		// Optional: Reflect velocity immediately if clamped
		// if ball.Vy < 0 { ball.Vy = -ball.Vy }
	} else if ball.Y > maxCoord {
		ball.Y = maxCoord
		// Optional: Reflect velocity immediately if clamped
		// if ball.Vy > 0 { ball.Vy = -ball.Vy }
	}
}

// getCenterIndex calculates the grid cell indices for the ball's center.
func (ball *Ball) getCenterIndex(cfg utils.Config) (col, row int) {
	if ball.canvasSize <= 0 || cfg.GridSize <= 0 {
		fmt.Printf("WARN: getCenterIndex called with invalid canvasSize (%d) or GridSize (%d)\n", ball.canvasSize, cfg.GridSize)
		return 0, 0
	}
	cellSize := ball.canvasSize / cfg.GridSize
	if cellSize == 0 {
		fmt.Printf("WARN: getCenterIndex calculated cellSize = 0 (canvasSize=%d, gridSize=%d)\n", ball.canvasSize, cfg.GridSize)
		return 0, 0
	}
	gridSize := ball.canvasSize / cellSize

	col = ball.X / cellSize
	row = ball.Y / cellSize

	finalCol := utils.MaxInt(0, utils.MinInt(gridSize-1, col))
	finalRow := utils.MaxInt(0, utils.MinInt(gridSize-1, row))

	return finalCol, finalRow
}

// --- Velocity/State Modification Methods (Called by BallActor via messages) ---

// ReflectVelocity reverses the velocity along the specified axis, ensuring it doesn't become zero.
func (ball *Ball) ReflectVelocity(axis string) {
	if axis == "X" {
		originalVx := ball.Vx
		ball.Vx = -ball.Vx
		if ball.Vx == 0 && originalVx != 0 {
			ball.Vx = int(math.Copysign(1.0, float64(-originalVx)))
		}
	} else if axis == "Y" {
		originalVy := ball.Vy
		ball.Vy = -ball.Vy
		if ball.Vy == 0 && originalVy != 0 {
			ball.Vy = int(math.Copysign(1.0, float64(-originalVy)))
		}
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

// IncreaseMass increases the ball's mass and scales its radius slightly. Uses config.
func (ball *Ball) IncreaseMass(cfg utils.Config, additional int) {
	ball.Mass += additional
	ball.Radius += additional * cfg.PowerUpIncreaseMassSize
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
func (ball *Ball) InterceptsIndex(col, row, cellSize int) bool {
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
