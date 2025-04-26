// File: game/ball.go
package game

import (
	"fmt" // <-- Import fmt
	"math"
	"math/rand"
	"runtime/debug"
	"time"

	"github.com/lguibr/pongo/utils"
)

// --- Message Types for Ball Channel ---

// BallMessage is the base interface for messages sent FROM the ball's channel
type BallMessage interface{}

// BallPositionMessage signals the ball's current state.
type BallPositionMessage struct {
	Ball *Ball
}

// WallCollisionMessage signals a collision with a canvas boundary.
type WallCollisionMessage struct { // <-- Definition
	Index int   // Which wall was hit (0: Right, 1: Top, 2: Left, 3: Bottom)
	Ball  *Ball // Reference to the ball that hit the wall
}

// BreakBrickMessage signals that a brick was broken.
type BreakBrickMessage struct { // <-- Definition
	BallPayload *Ball // Reference to the ball that broke the brick
	Level       int   // Level of the broken brick (for scoring)
}

// --- Ball Struct and Methods ---

type Ball struct {
	X          int              `json:"x"`
	Y          int              `json:"y"`
	Vx         int              `json:"vx"`
	Vy         int              `json:"vy"`
	Ax         int              `json:"ax"`
	Ay         int              `json:"ay"`
	Radius     int              `json:"radius"`
	Id         int              `json:"id"`
	OwnerIndex int              `json:"ownerIndex"`
	Phasing    bool             `json:"phasing"`
	Mass       int              `json:"mass"`
	Channel    chan BallMessage `json:"-"` // Internal channel for this ball's events
	canvasSize int
	open       bool // Flag to signal the engine loop to stop
}

func (b *Ball) GetX() int      { return b.X }
func (b *Ball) GetY() int      { return b.Y }
func (b *Ball) GetRadius() int { return b.Radius }

func NewBallChannel() chan BallMessage {
	return make(chan BallMessage, 10) // Added buffer
}

func NewBall(channel chan BallMessage, x, y, radius, canvasSize, ownerIndex, index int) *Ball {
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
		Channel:    channel,
		open:       true, // Ball starts active
		Mass:       mass,
	}
}

// Engine runs the ball's movement and update loop.
// Added panic recovery.
func (ball *Ball) Engine() {
	// Panic Recovery
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC recovered in Ball %d Engine: %v\nStack trace:\n%s\n", ball.Id, r, string(debug.Stack()))
		}
		// Ensure channel is closed if this goroutine exits unexpectedly
		if ball.Channel != nil {
			// Check if channel is already closed before trying to close it
			select {
			case _, ok := <-ball.Channel:
				if !ok {
					// Already closed
				} else {
					// Received a message unexpectedly? Put it back? Or just close.
					// For simplicity, just attempt to close if it wasn't already.
					close(ball.Channel)
				}
			default:
				// Channel is empty, safe to close
				close(ball.Channel)
			}
			ball.Channel = nil // Prevent further use after closing attempt
		}
		fmt.Printf("Ball %d Engine loop stopped.\n", ball.Id)
	}()

	fmt.Printf("Ball %d Engine loop started.\n", ball.Id)
	ticker := time.NewTicker(utils.Period)
	defer ticker.Stop()

	for {
		// Use select for non-blocking check on ball.open and ticker
		select {
		case <-ticker.C:
			if !ball.open {
				fmt.Printf("Ball %d is not open, stopping engine.\n", ball.Id)
				return // Exit loop if ball is marked closed
			}

			if ball.Channel == nil {
				fmt.Printf("Ball %d channel is nil, stopping engine.\n", ball.Id)
				return // Exit if channel is somehow nil
			}

			ball.Move()

			// Send position update - non-blocking
			// In actor model, send message to GameActor.
			select {
			case ball.Channel <- BallPositionMessage{Ball: ball}:
				// Position sent
			default:
				// Log if channel is full, but don't block or panic
				// fmt.Printf("Ball %d channel full, could not send position.\n", ball.Id)
			}
		// Optional: Add a case here to listen on a dedicated stop channel for the ball
		// case <-ball.stopCh:
		//     fmt.Printf("Ball %d received stop signal.\n", ball.Id)
		//     return
		}
	}
}

func (ball *Ball) Move() {
	ball.X += ball.Vx + ball.Ax/2
	ball.Y += ball.Vy + ball.Ay/2

	ball.Vx += ball.Ax
	ball.Vy += ball.Ay
}

func (ball *Ball) getCenterIndex() (x, y int) {
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

func (ball *Ball) ReflectVelocityX() {
	ball.Vx = -ball.Vx
}

func (ball *Ball) ReflectVelocityY() {
	ball.Vy = -ball.Vy
}

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

func (ball *Ball) IncreaseMass(additional int) {
	ball.Mass += additional
	ball.Radius += additional * 2 // Simple scaling, adjust if needed
}

func (ball *Ball) SetBallPhasing(expiresIn int) {
	ball.Phasing = true
	go time.AfterFunc(time.Duration(expiresIn)*time.Second, func() {
		// TODO: This needs to be thread-safe if accessed concurrently.
		// In actor model, send a message back to the ball actor to turn phasing off.
		ball.Phasing = false
	})
}

// --- Collision Logic (belongs in Ball or GameActor) ---

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

// CollidePaddle checks and handles collision with a single paddle.
// Sends messages back via the ball's channel if collision occurs.
func (ball *Ball) CollidePaddle(paddle *Paddle) {
	if paddle == nil {
		return
	}

	collisionDetected := ball.BallInterceptPaddles(paddle)
	if collisionDetected {
		fmt.Printf("Ball %d collided with Paddle %d\n", ball.Id, paddle.Index)
		ball.OwnerIndex = paddle.Index // Ball now belongs to this player
		handlers := [4]func(){
			ball.HandleCollideRight, // Index 0 (Right wall relative to player 0)
			ball.HandleCollideTop,   // Index 1 (Top wall relative to player 1)
			ball.HandleCollideLeft,  // Index 2 (Left wall relative to player 2)
			ball.HandleCollideBottom,// Index 3 (Bottom wall relative to player 3)
		}

		// Apply reflection based on which paddle was hit
		handlerCollision := handlers[paddle.Index]
		handlerCollision()
		// Optionally add some velocity based on paddle movement?
	}
}

// CollideCells checks and handles collision with grid cells.
// Sends messages back via the ball's channel if collision occurs.
func (ball *Ball) CollideCells(grid Grid, cellSize int) {
	gridSize := len(grid)
	if gridSize == 0 {
		return
	}
	row, col := ball.getCenterIndex()

	// Check a 3x3 area around the ball's center cell index
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			checkRow, checkCol := row+i, col+j

			// Ensure indices are within grid bounds
			if checkRow < 0 || checkRow >= gridSize || checkCol < 0 || checkCol >= len(grid[checkRow]) {
				continue
			}

			// Check if the ball physically intercepts this cell
			if ball.InterceptsIndex(checkRow, checkCol, cellSize) {
				cell := &grid[checkRow][checkCol] // Get pointer to modify cell directly (needs locking later)
				cellType := cell.Data.Type

				if cellType == utils.Cells.Brick && cell.Data.Life > 0 {
					// fmt.Printf("Ball %d checking collision with Brick at [%d,%d]\n", ball.Id, checkRow, checkCol)
					ball.handleCollideBrick([2]int{row, col}, [2]int{checkRow, checkCol}, cell) // Pass cell pointer
					return // Handle only one collision per step for simplicity
				}
				if cellType == utils.Cells.Block {
					// fmt.Printf("Ball %d checking collision with Block at [%d,%d]\n", ball.Id, checkRow, checkCol)
					ball.handleCollideBlock([2]int{row, col}, [2]int{checkRow, checkCol})
					return // Handle only one collision per step
				}
			}
		}
	}
}

// WallCollision struct definition moved into ball.go
type WallCollision struct {
	Collides func() bool
	Handle   func()
}

// CollideWalls checks and handles collisions with outer canvas boundaries.
// Sends messages back via the ball's channel if collision occurs.
func (ball *Ball) CollideWalls() {
	wallsCollision := [4]*WallCollision{
		{ball.CollidesRightWall, ball.HandleCollideRight}, // Index 0
		{ball.CollidesTopWall, ball.HandleCollideTop},     // Index 1
		{ball.CollidesLeftWall, ball.HandleCollideLeft},   // Index 2
		{ball.CollidesBottomWall, ball.HandleCollideBottom}, // Index 3
	}

	for index, wallCollision := range wallsCollision {
		if wallCollision.Collides() {
			wallCollision.Handle()
			// Send message indicating which wall was hit
			select {
			case ball.Channel <- WallCollisionMessage{Index: index, Ball: ball}: // Use defined struct
				// Message sent
			default:
				// fmt.Printf("Ball %d channel full, could not send WallCollisionMessage.\n", ball.Id)
			}
			return // Handle only one wall collision per step
		}
	}
}

// CollidePaddles iterates through paddles and calls CollidePaddle.
// This is typically called by the GameActor or the main game loop.
func (ball *Ball) CollidePaddles(paddles [4]*Paddle) {
	for _, paddle := range paddles {
		ball.CollidePaddle(paddle) // CollidePaddle handles nil check
	}
}

// handleCollideBrick handles the logic when a ball hits a destructible brick.
// It modifies the cell directly (needs locking or actor message later).
func (ball *Ball) handleCollideBrick(oldIndices, newIndices [2]int, cell *Cell) {
	if ball.Phasing {
		return // Phasing balls pass through bricks
	}

	ball.handleCollideBlock(oldIndices, newIndices) // Reflect velocity first

	// Decrease brick life (needs thread safety)
	cell.Data.Life -= ball.Mass
	fmt.Printf("Brick [%d,%d] life reduced to %d by ball %d\n", newIndices[0], newIndices[1], cell.Data.Life, ball.Id)

	if cell.Data.Life <= 0 {
		level := cell.Data.Level // Get level before changing type
		cell.Data.Type = utils.Cells.Empty
		cell.Data.Life = 0 // Ensure life is 0
		fmt.Printf("Brick [%d,%d] broken by ball %d\n", newIndices[0], newIndices[1], ball.Id)

		// Send message about the brick break
		select {
		case ball.Channel <- BreakBrickMessage{Level: level, BallPayload: ball}: // Use defined struct
			// Message sent
		default:
			// fmt.Printf("Ball %d channel full, could not send BreakBrickMessage.\n", ball.Id)
		}
	}
}

// handleCollideBlock handles reflection logic for hitting any solid object (brick or block).
func (ball *Ball) handleCollideBlock(oldIndices, newIndices [2]int) {
	if ball.Phasing {
		return // Phasing balls pass through blocks too
	}

	// Determine reflection direction based on relative positions
	dx := newIndices[0] - oldIndices[0]
	dy := newIndices[1] - oldIndices[1]

	// Simple reflection logic (can be improved)
	if dx != 0 && dy == 0 { // Horizontal collision
		ball.ReflectVelocityX()
	} else if dy != 0 && dx == 0 { // Vertical collision
		ball.ReflectVelocityY()
	} else { // Diagonal collision - reflect both for simplicity
		ball.ReflectVelocityX()
		ball.ReflectVelocityY()
	}
}

// BallInterceptPaddles checks for intersection using distance to closest point.
func (ball *Ball) BallInterceptPaddles(paddle *Paddle) bool {
	closestX := math.Max(float64(paddle.X), math.Min(float64(ball.X), float64(paddle.X+paddle.Width)))
	closestY := math.Max(float64(paddle.Y), math.Min(float64(ball.Y), float64(paddle.Y+paddle.Height)))
	distance := utils.Distance(ball.X, ball.Y, int(closestX), int(closestY))
	return distance < float64(ball.Radius)
}

// InterceptsIndex checks if the ball circle intersects with a grid cell rectangle.
func (ball *Ball) InterceptsIndex(x, y, cellSize int) bool {
	cellLeft := x * cellSize
	cellTop := y * cellSize
	cellRight := cellLeft + cellSize
	cellBottom := cellTop + cellSize
	closestX := math.Max(float64(cellLeft), math.Min(float64(ball.X), float64(cellRight)))
	closestY := math.Max(float64(cellTop), math.Min(float64(ball.Y), float64(cellBottom)))
	distance := utils.Distance(ball.X, ball.Y, int(closestX), int(closestY))
	return distance < float64(ball.Radius)
}