// File: game/paddle.go
package game

import (
	"fmt"

	"github.com/lguibr/pongo/utils"
)

// --- Message Types for Paddle Communication ---

// PaddlePositionMessage signals the paddle's current state (sent by PaddleActor).
type PaddlePositionMessage struct {
	Paddle *Paddle // Pointer to a state snapshot
}

// PaddleDirectionMessage carries direction input (sent to PaddleActor).
type PaddleDirectionMessage struct {
	Direction []byte // Raw JSON bytes {"direction": "ArrowLeft/ArrowRight/Stop"}
}

// Direction struct for unmarshalling JSON from frontend
type Direction struct {
	Direction string `json:"direction"` // "ArrowLeft", "ArrowRight", "Stop"
}

// --- Paddle Struct (State Holder) ---

type Paddle struct {
	X          int    `json:"x"`
	Y          int    `json:"y"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Index      int    `json:"index"`     // Player index (0-3)
	Direction  string `json:"direction"` // Internal: "left", "right", or "" (stop)
	Velocity   int    `json:"-"`         // Base velocity from config, not marshalled
	Vx         int    `json:"vx"`        // Current horizontal velocity (for physics)
	Vy         int    `json:"vy"`        // Current vertical velocity (for physics)
	canvasSize int    // Store canvas size for boundary checks
}

func (p *Paddle) GetX() int      { return p.X }
func (p *Paddle) GetY() int      { return p.Y }
func (p *Paddle) GetWidth() int  { return p.Width }
func (p *Paddle) GetHeight() int { return p.Height }

// NewPaddle creates the initial state data structure for a paddle. Uses config.
func NewPaddle(cfg utils.Config, index int) *Paddle {
	paddle := &Paddle{
		Index:      index,
		Velocity:   cfg.PaddleVelocity, // Use config
		canvasSize: cfg.CanvasSize,     // Store canvas size
		Direction:  "",                 // Start stopped
		Vx:         0,
		Vy:         0,
	}

	// Set dimensions and initial position based on index
	switch index {
	case 0: // Right edge, vertical
		paddle.Width = cfg.PaddleWidth
		paddle.Height = cfg.PaddleLength
		paddle.X = cfg.CanvasSize - paddle.Width
		paddle.Y = (cfg.CanvasSize - paddle.Height) / 2
	case 1: // Top edge, horizontal
		paddle.Width = cfg.PaddleLength
		paddle.Height = cfg.PaddleWidth
		paddle.X = (cfg.CanvasSize - paddle.Width) / 2
		paddle.Y = 0
	case 2: // Left edge, vertical
		paddle.Width = cfg.PaddleWidth
		paddle.Height = cfg.PaddleLength
		paddle.X = 0
		paddle.Y = (cfg.CanvasSize - paddle.Height) / 2
	case 3: // Bottom edge, horizontal
		paddle.Width = cfg.PaddleLength
		paddle.Height = cfg.PaddleWidth
		paddle.X = (cfg.CanvasSize - paddle.Width) / 2
		paddle.Y = cfg.CanvasSize - paddle.Height
	default:
		// Should not happen with MaxPlayers check
		fmt.Printf("Warning: Invalid paddle index %d\n", index)
		paddle.X, paddle.Y, paddle.Width, paddle.Height = 0, 0, 10, 10 // Default fallback
	}

	return paddle
}

// Move updates the paddle's position based on its direction and velocity.
// Handles stopping when direction is empty. Called by PaddleActor.
func (paddle *Paddle) Move() {
	// Reset velocity before applying movement
	paddle.Vx = 0
	paddle.Vy = 0

	switch paddle.Index {
	case 0, 2: // Vertical paddles (Right, Left)
		switch paddle.Direction {
		case "left": // Move Up
			paddle.Vy = -paddle.Velocity
			paddle.Y = utils.MaxInt(0, paddle.Y-paddle.Velocity)
		case "right": // Move Down
			paddle.Vy = paddle.Velocity
			paddle.Y = utils.MinInt(paddle.canvasSize-paddle.Height, paddle.Y+paddle.Velocity)
		case "": // Stop
			// Vx, Vy already 0
		default:
			// Unknown direction, stop
			paddle.Direction = ""
		}
	case 1, 3: // Horizontal paddles (Top, Bottom)
		switch paddle.Direction {
		case "left": // Move Left
			paddle.Vx = -paddle.Velocity
			paddle.X = utils.MaxInt(0, paddle.X-paddle.Velocity)
		case "right": // Move Right
			paddle.Vx = paddle.Velocity
			paddle.X = utils.MinInt(paddle.canvasSize-paddle.Width, paddle.X+paddle.Velocity)
		case "": // Stop
			// Vx, Vy already 0
		default:
			// Unknown direction, stop
			paddle.Direction = ""
		}
	}
}
