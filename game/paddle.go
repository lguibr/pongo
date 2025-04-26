// File: game/paddle.go
package game

import (
	"encoding/json"
	"fmt"
	// "runtime/debug" // No longer needed here if Engine removed
	// "time" // No longer needed here if Engine removed

	"github.com/lguibr/pongo/utils"
)

// PaddleMessage is the base interface for messages related to paddles
// (Kept for potential use by actors, e.g., GameActor receiving PaddlePositionMessage)
type PaddleMessage interface{}

// PaddlePositionMessage signals the paddle's current state.
// Typically sent *by* the PaddleActor.
type PaddlePositionMessage struct {
	Paddle *Paddle // Send a pointer to a copy of the state
}

// PaddleDirectionMessage signals a desired direction change.
// Typically sent *to* the PaddleActor.
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
	canvasSize int
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

	velocity := [2]int{0, paddle.Velocity}

	if paddle.Index%2 != 0 { // Horizontal paddles (1, 3) move along X
		velocity = utils.SwapVectorCoordinates(velocity)
	}

	velocityX, velocityY := velocity[0], velocity[1]

	if paddle.Direction == "left" { // "Left" means decreasing Y for vertical, decreasing X for horizontal
		if paddle.Index%2 == 0 { // Vertical paddle (Index 0, 2)
			if paddle.Y-velocityY < 0 {
				paddle.Y = 0 // Clamp to boundary
				return
			}
			paddle.Y -= velocityY
		} else { // Horizontal paddle (Index 1, 3)
			if paddle.X-velocityX < 0 {
				paddle.X = 0 // Clamp to boundary
				return
			}
			paddle.X -= velocityX
		}
	} else { // Moving "right" (relative to paddle orientation) - increasing Y for vertical, increasing X for horizontal
		if paddle.Index%2 == 0 { // Vertical paddle (Index 0, 2)
			if paddle.Y+paddle.Height+velocityY > paddle.canvasSize {
				paddle.Y = paddle.canvasSize - paddle.Height // Clamp to boundary
				return
			}
			paddle.Y += velocityY
		} else { // Horizontal paddle (Index 1, 3)
			if paddle.X+paddle.Width+velocityX > paddle.canvasSize {
				paddle.X = paddle.canvasSize - paddle.Width // Clamp to boundary
				return
			}
			paddle.X += velocityX
		}
	}
}

// --- Constructor ---

// NewPaddle creates a new Paddle data structure with initial position and dimensions.
func NewPaddle(canvasSize, index int) *Paddle {

	offSet := -utils.PaddleLength/2 + utils.PaddleWeight/2
	if index > 1 {
		offSet = -offSet
	}

	cardinalPosition := [2]int{canvasSize/2 - utils.PaddleWeight/2, offSet}
	rotateX, rotateY := utils.RotateVector(index, cardinalPosition[0], cardinalPosition[1], canvasSize, canvasSize)
	translatedVector := utils.SumVectors([2]int{rotateX, rotateY}, [2]int{canvasSize/2 - utils.PaddleWeight/2, canvasSize/2 - utils.PaddleWeight/2})
	x, y := translatedVector[0], translatedVector[1]

	indexOdd := index % 2
	var width, height int

	if indexOdd == 0 {
		height = utils.PaddleLength
		width = utils.PaddleWeight
	} else {
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
// This might be better placed in a shared types package if used elsewhere (e.g., by the actor).
type Direction struct {
	// The json tag ensures Go unmarshals the lowercase "direction" key from the frontend
	Direction string `json:"direction"`
}

// SetDirection updates the paddle's internal direction state based on the received message buffer.
// NOTE: This logic is now primarily handled within PaddleActor.Receive,
// but the function is kept here for reference or potential utility. It does NOT
// belong to the core state management of the Paddle struct anymore.
func (paddle *Paddle) SetDirection(buffer []byte) (Direction, error) {
	receivedDirection := Direction{}
	err := json.Unmarshal(buffer, &receivedDirection)
	if err != nil {
		// Avoid printing full buffer in production
		fmt.Printf("Error unmarshalling direction message for paddle %d: %v\n", paddle.Index, err)
		return receivedDirection, err
	}

	// Convert "ArrowLeft"/"ArrowRight" to internal "left"/"right" state
	newInternalDirection := utils.DirectionFromString(receivedDirection.Direction)

	// Update the paddle's internal state
	paddle.Direction = newInternalDirection
	// fmt.Printf("Paddle %d direction set to: %s\n", paddle.Index, paddle.Direction) // Debug log
	return receivedDirection, nil
}

// --- Deprecated Goroutine Logic ---
// func NewPaddleChannel()... removed
// func (paddle *Paddle) Engine()... removed
