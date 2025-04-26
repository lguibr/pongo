// File: game/paddle.go
package game

import (
	"encoding/json"
	"fmt"
	"runtime/debug" // Import debug package
	"time"

	"github.com/lguibr/pongo/utils"
)

type PaddleMessage interface{}

type PaddlePositionMessage struct {
	Paddle *Paddle
}
type PaddleDirectionMessage struct {
	Direction []byte
}

type Paddle struct {
	X          int    `json:"x"`
	Y          int    `json:"y"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Index      int    `json:"index"`
	Direction  string `json:"direction"` // This is internal state, not the message key
	Velocity   int    `json:"velocity"`
	canvasSize int
	channel    chan PaddleMessage // This channel is internal, not used by frontend directly anymore
}

func (p *Paddle) GetX() int      { return p.X }
func (p *Paddle) GetY() int      { return p.Y }
func (p *Paddle) GetWidth() int  { return p.Width }
func (p *Paddle) GetHeight() int { return p.Height }

func NewPaddleChannel() chan PaddleMessage {
	// This function might become obsolete or change with actor refactoring
	return make(chan PaddleMessage, 10) // Added buffer
}

func (paddle *Paddle) Move() {
	if paddle.Direction != "left" && paddle.Direction != "right" {
		return
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

func NewPaddle(channel chan PaddleMessage, canvasSize, index int) *Paddle {

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
		channel:    channel,
	}
}

// Direction struct used for unmarshaling messages from the frontend
type Direction struct {
	// The json tag ensures Go unmarshals the lowercase "direction" key from the frontend
	Direction string `json:"direction"`
}

// SetDirection updates the paddle's internal direction state based on the received message.
func (paddle *Paddle) SetDirection(buffer []byte) (Direction, error) {
	receivedDirection := Direction{}
	err := json.Unmarshal(buffer, &receivedDirection)
	if err != nil {
		// Avoid printing full buffer in production, could contain sensitive info if format changes
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

// Engine simulates the paddle's independent movement loop.
// Added panic recovery.
func (paddle *Paddle) Engine() {
	// Panic Recovery
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC recovered in Paddle %d Engine: %v\nStack trace:\n%s\n", paddle.Index, r, string(debug.Stack()))
		}
		fmt.Printf("Paddle %d Engine loop stopped.\n", paddle.Index)
	}()

	fmt.Printf("Paddle %d Engine loop started.\n", paddle.Index)
	ticker := time.NewTicker(utils.Period)
	defer ticker.Stop()

	for range ticker.C {
		// A proper stop mechanism (e.g., listening on a stop channel) is better.
		// For now, rely on the channel check.
		if paddle.channel == nil {
			fmt.Printf("Paddle %d channel is nil, stopping engine.\n", paddle.Index)
			return // Exit loop if channel is nil (might happen during cleanup)
		}

		paddle.Move()

		// Send position update - non-blocking
		// In an actor model, this would be sending a message to the GameActor.
		select {
		case paddle.channel <- PaddlePositionMessage{Paddle: paddle}:
			// Position sent
		default:
			// Log if channel is full, but don't block or panic
			// fmt.Printf("Paddle %d channel full, could not send position.\n", paddle.Index)
		}
	}
}