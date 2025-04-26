package game

import (
	"encoding/json"
	"fmt"

	// "runtime/debug" // No longer needed here
	// "time" // No longer needed here

	"github.com/lguibr/pongo/utils"
)

// PaddleMessage interface might still be useful if GameActor sends specific commands.
type PaddleMessage interface{}

// PaddlePositionMessage signals the paddle's current state. Sent BY PaddleActor.
type PaddlePositionMessage struct {
	Paddle *Paddle // Send a pointer to a copy of the state
}

// PaddleDirectionMessage signals a desired direction change. Sent TO PaddleActor.
// Payload is raw JSON bytes.
type PaddleDirectionMessage struct {
	Direction []byte // Expecting JSON: {"direction": "ArrowLeft"} or {"direction": "ArrowRight"}
}

// Paddle struct holds the state of a paddle. It's primarily data now.
type Paddle struct {
	X          int    `json:"x"`
	Y          int    `json:"y"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Index      int    `json:"index"`
	Direction  string `json:"direction"` // Internal state ("left", "right", "")
	Velocity   int    `json:"velocity"`
	canvasSize int    // Keep internal canvas size for boundary checks
	// channel field removed
}

// --- Getters ---

func (p *Paddle) GetX() int      { return p.X }
func (p *Paddle) GetY() int      { return p.Y }
func (p *Paddle) GetWidth() int  { return p.Width }
func (p *Paddle) GetHeight() int { return p.Height }

// --- Movement Logic (Used by PaddleActor) ---

// Move updates the paddle's position based on its current direction and velocity.
// This logic is called by the PaddleActor internally.
func (paddle *Paddle) Move() {
	if paddle.Direction != "left" && paddle.Direction != "right" {
		return // No movement if direction is not set
	}

	// Calculate base velocity vector (vertical movement for index 0, 2)
	velocity := [2]int{0, paddle.Velocity}

	// Swap for horizontal paddles (index 1, 3)
	if paddle.Index%2 != 0 {
		velocity = utils.SwapVectorCoordinates(velocity)
	}

	velocityX, velocityY := velocity[0], velocity[1]

	// Apply movement based on direction and orientation
	if paddle.Direction == "left" { // "Left" relative to paddle orientation
		if paddle.Index%2 == 0 { // Vertical paddle (Index 0, 2) - Move Up (decrease Y)
			newY := paddle.Y - velocityY
			if newY < 0 {
				paddle.Y = 0 // Clamp to top boundary
			} else {
				paddle.Y = newY
			}
		} else { // Horizontal paddle (Index 1, 3) - Move Left (decrease X)
			newX := paddle.X - velocityX
			if newX < 0 {
				paddle.X = 0 // Clamp to left boundary
			} else {
				paddle.X = newX
			}
		}
	} else { // Moving "right" relative to paddle orientation
		if paddle.Index%2 == 0 { // Vertical paddle (Index 0, 2) - Move Down (increase Y)
			newY := paddle.Y + velocityY
			if newY+paddle.Height > paddle.canvasSize {
				paddle.Y = paddle.canvasSize - paddle.Height // Clamp to bottom boundary
			} else {
				paddle.Y = newY
			}
		} else { // Horizontal paddle (Index 1, 3) - Move Right (increase X)
			newX := paddle.X + velocityX
			if newX+paddle.Width > paddle.canvasSize {
				paddle.X = paddle.canvasSize - paddle.Width // Clamp to right boundary
			} else {
				paddle.X = newX
			}
		}
	}
}

// --- Constructor ---

// NewPaddle creates a new Paddle data structure with initial position and dimensions.
func NewPaddle(canvasSize, index int) *Paddle {

	// offSet := utils.PaddleLength / 2 // Offset from center line
	var x, y int

	// Determine position based on index (0: right, 1: top, 2: left, 3: bottom)
	switch index {
	case 0: // Right wall
		x = canvasSize - utils.PaddleWeight
		y = canvasSize/2 - utils.PaddleLength/2
	case 1: // Top wall
		x = canvasSize/2 - utils.PaddleLength/2
		y = 0
	case 2: // Left wall
		x = 0
		y = canvasSize/2 - utils.PaddleLength/2
	case 3: // Bottom wall
		x = canvasSize/2 - utils.PaddleLength/2
		y = canvasSize - utils.PaddleWeight
	default:
		// Default to player 0 position if index is invalid
		x = canvasSize - utils.PaddleWeight
		y = canvasSize/2 - utils.PaddleLength/2
		fmt.Printf("Warning: Invalid paddle index %d, defaulting to 0.\n", index)
	}

	// Determine width/height based on orientation
	indexOdd := index % 2
	var width, height int
	if indexOdd == 0 { // Vertical (index 0, 2)
		width = utils.PaddleWeight
		height = utils.PaddleLength
	} else { // Horizontal (index 1, 3)
		width = utils.PaddleLength
		height = utils.PaddleWeight
	}

	return &Paddle{
		X:          x,
		Y:          y,
		Index:      index,
		Width:      width,
		Height:     height,
		Direction:  "", // Initial internal direction state
		Velocity:   utils.MinVelocity * 2,
		canvasSize: canvasSize,
		// channel field removed
	}
}

// --- Input Handling (Reference, Logic moved to Actor) ---

// Direction struct used for unmarshaling messages from the frontend
type Direction struct {
	Direction string `json:"direction"`
}

// SetDirection is DEPRECATED. Logic handled by PaddleActor.
func (paddle *Paddle) SetDirection(buffer []byte) (Direction, error) {
	fmt.Println("WARNING: paddle.SetDirection() is deprecated. Logic moved to PaddleActor.")
	receivedDirection := Direction{}
	err := json.Unmarshal(buffer, &receivedDirection)
	if err != nil {
		return receivedDirection, err
	}
	paddle.Direction = utils.DirectionFromString(receivedDirection.Direction)
	return receivedDirection, nil
}

// --- Deprecated Goroutine Logic ---
// func NewPaddleChannel()... removed
// func (paddle *Paddle) Engine()... removed
