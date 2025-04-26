// File: game/ball.go
package game

import (
	"fmt"
	"math"
	"math/rand" // Needed for NewBall velocity
	// "runtime/debug" // No longer needed as Engine removed
	// "time"          // No longer needed here

	"github.com/lguibr/pongo/utils"
)

// --- Message Types for Ball Communication (Sent BY BallActor or TO BallActor) ---

// BallMessage is the base interface for messages sent FROM the ball actor
// (Currently only BallPositionMessage)
type BallMessage interface{}

// BallPositionMessage signals the ball's current state (sent by BallActor).
type BallPositionMessage struct {
	Ball *Ball // Pointer to a state snapshot
}

// WallCollisionMessage signals a collision with a canvas boundary (sent by GameActor).
// No longer sent by Ball itself.
type WallCollisionMessage struct {
	Index int   // Which wall was hit (0: Right, 1: Top, 2: Left, 3: Bottom)
	Ball  *Ball // Reference to the ball that hit the wall
}

// BreakBrickMessage signals that a brick was broken (sent by GameActor).
// No longer sent by Ball itself.
type BreakBrickMessage struct {
	BallPayload *Ball // Reference to the ball that broke the brick
	Level       int   // Level of the broken brick (for scoring)
}

// --- Ball Struct (State Holder) ---

type Ball struct {
	X          int  `json:"x"`
	Y          int  `json:"y"`
	Vx         int  `json:"vx"`
	Vy         int  `json:"vy"`
	Ax         int  `json:"ax"` // Acceleration - currently unused?
	Ay         int  `json:"ay"` // Acceleration - currently unused?
	Radius     int  `json:"radius"`
	Id         int  `json:"id"` // Unique ID (e.g., timestamp)
	OwnerIndex int  `json:"ownerIndex"` // Index of the player who last hit it
	Phasing    bool `json:"phasing"`    // Is the ball currently phasing?
	Mass       int  `json:"mass"`
	canvasSize int  // Keep for boundary checks within Move or getters if needed
	open       bool // Flag used by old engine loop, potentially reusable for actor state? (Set false on stop)
	// Channel    chan BallMessage `json:"-"` // Removed
}

func (b *Ball) GetX() int      { return b.X }
func (b *Ball) GetY() int      { return b.Y }
func (b *Ball) GetRadius() int { return b.Radius }

// NewBallChannel removed

// NewBall creates the initial state data structure for a ball.
func NewBall(x, y, radius, canvasSize, ownerIndex, index int) *Ball {
	// If position is zero, calculate initial position based on owner
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

	// Calculate initial velocity based on owner index
	maxVelocity := utils.MaxVelocity
	minVelocity := utils.MinVelocity

	cardinalVX := minVelocity + rand.Intn(maxVelocity-minVelocity+1) // +1 to include maxVelocity
	if cardinalVX == 0 {
		cardinalVX = minVelocity // Ensure non-zero initial velocity component
	}
	cardinalVY := utils.RandomNumberN(maxVelocity) // Already ensures non-zero

	vx, vy := utils.RotateVector(ownerIndex, -cardinalVX, cardinalVY, 1, 1)

	return &Ball{
		X:          x,
		Y:          y,
		Vx:         vx,
		Vy:         vy,
		Radius:     radius,
		Id:         index, // Use provided index (timestamp) as ID
		OwnerIndex: ownerIndex,
		canvasSize: canvasSize,
		// Channel:    channel, // Removed
		open:       true, // Ball starts active
		Mass:       mass,
	}
}

// Engine removed

// Move updates the ball's position based on velocity and acceleration.
// Called by BallActor.
func (ball *Ball) Move() {
    // Basic physics update
	ball.X += ball.Vx // + ball.Ax/2 // Acceleration not used currently
	ball.Y += ball.Vy // + ball.Ay/2 // Acceleration not used currently

    // Update velocity from acceleration (if used)
	// ball.Vx += ball.Ax
	// ball.Vy += ball.Ay

    // TODO: Add optional friction or velocity damping here?
    // ball.Vx = int(float64(ball.Vx) * 0.99)
    // ball.Vy = int(float64(ball.Vy) * 0.99)
}

// getCenterIndex calculates the grid cell indices for the ball's center.
func (ball *Ball) getCenterIndex() (row, col int) {
	// Might be used by GameActor for collision checks.
	if utils.CellSize == 0 { // Avoid division by zero
		fmt.Println("WARN: getCenterIndex called with utils.CellSize = 0")
		return 0, 0
	}
	row = ball.X / utils.CellSize
	col = ball.Y / utils.CellSize
	return row, col
}

// --- Velocity/State Modification Methods (Called by BallActor via messages) ---

// ReflectVelocityX reverses the X velocity.
func (ball *Ball) ReflectVelocityX() {
	ball.Vx = -ball.Vx
}

// ReflectVelocityY reverses the Y velocity.
func (ball *Ball) ReflectVelocityY() {
	ball.Vy = -ball.Vy
}

// HandleCollideRight adjusts velocity for hitting the right side of something.
func (ball *Ball) HandleCollideRight() {
	ball.Vx = -utils.Abs(ball.Vx) // Ensure velocity moves left
}

// HandleCollideLeft adjusts velocity for hitting the left side of something.
func (ball *Ball) HandleCollideLeft() {
	ball.Vx = utils.Abs(ball.Vx) // Ensure velocity moves right
}

// HandleCollideTop adjusts velocity for hitting the top side of something.
func (ball *Ball) HandleCollideTop() {
	ball.Vy = utils.Abs(ball.Vy) // Ensure velocity moves down
}

// HandleCollideBottom adjusts velocity for hitting the bottom side of something.
func (ball *Ball) HandleCollideBottom() {
	ball.Vy = -utils.Abs(ball.Vy) // Ensure velocity moves up
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

// IncreaseMass increases the ball's mass and scales its radius slightly.
func (ball *Ball) IncreaseMass(additional int) {
	ball.Mass += additional
	// Simple radius scaling based on mass, adjust if needed
	// Might be better to calculate radius based on mass: R = k * Mass^(1/3) ?
	ball.Radius += additional * 2 // Example scaling
	if ball.Radius <= 0 { // Prevent negative/zero radius
		ball.Radius = 1
	}
}

// SetBallPhasing removed (handled by BallActor state and timer)

// --- Collision Logic (REMOVED - To be handled by GameActor) ---
// func (ball *Ball) CollidesTopWall()... removed
// func (ball *Ball) CollidesBottomWall()... removed
// func (ball *Ball) CollidesRightWall()... removed
// func (ball *Ball) CollidesLeftWall()... removed
// func (ball *Ball) CollidePaddle()... removed
// func (ball *Ball) CollideCells()... removed
// func (ball *Ball) CollideWalls()... removed
// func (ball *Ball) CollidePaddles()... removed
// func (ball *Ball) handleCollideBrick()... removed
// func (ball *Ball) handleCollideBlock()... removed

// --- Geometric Intersection Checks (May be useful for GameActor) ---

// BallInterceptPaddles checks for intersection using distance to closest point on paddle rectangle.
// Note: Takes a single paddle, GameActor would iterate.
func (ball *Ball) BallInterceptPaddles(paddle *Paddle) bool {
    if paddle == nil { return false }
	closestX := math.Max(float64(paddle.X), math.Min(float64(ball.X), float64(paddle.X+paddle.Width)))
	closestY := math.Max(float64(paddle.Y), math.Min(float64(ball.Y), float64(paddle.Y+paddle.Height)))
	distance := utils.Distance(ball.X, ball.Y, int(closestX), int(closestY))
	return distance < float64(ball.Radius)
}

// InterceptsIndex checks if the ball circle intersects with a grid cell rectangle.
func (ball *Ball) InterceptsIndex(x, y, cellSize int) bool {
    if cellSize <= 0 { return false } // Avoid issues with zero cell size
	cellLeft := x * cellSize
	cellTop := y * cellSize
	cellRight := cellLeft + cellSize
	cellBottom := cellTop + cellSize
	// Find the closest point on the cell rectangle to the center of the ball
	closestX := math.Max(float64(cellLeft), math.Min(float64(ball.X), float64(cellRight)))
	closestY := math.Max(float64(cellTop), math.Min(float64(ball.Y), float64(cellBottom)))
	// Calculate the distance between the ball's center and this closest point
	distance := utils.Distance(ball.X, ball.Y, int(closestX), int(closestY))
	// If the distance is less than the ball's radius, an intersection occurs
	return distance < float64(ball.Radius)
}

