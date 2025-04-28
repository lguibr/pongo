// File: pongo/game/paddle_actor.go
package game

import (
	"encoding/json"
	"fmt"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
)

// PaddleActor implements the bollywood.Actor interface for managing a paddle.
type PaddleActor struct {
	state        *Paddle        // Use a pointer to the Paddle state
	cfg          utils.Config   // Store config
	gameActorPID *bollywood.PID // PID of the GameActor (parent)
	selfPID      *bollywood.PID // Store self PID for logging
}

// NewPaddleActorProducer creates a bollywood.Producer for PaddleActor.
func NewPaddleActorProducer(initialState Paddle, gameActorPID *bollywood.PID, cfg utils.Config) bollywood.Producer {
	return func() bollywood.Actor {
		actorState := initialState
		return &PaddleActor{
			state:        &actorState,
			cfg:          cfg,
			gameActorPID: gameActorPID,
		}
	}
}

// Receive handles incoming messages for the PaddleActor.
func (a *PaddleActor) Receive(ctx bollywood.Context) {
	if a.selfPID == nil {
		a.selfPID = ctx.Self()
	}
	pidStr := "unknown"
	if a.selfPID != nil {
		pidStr = a.selfPID.String()
	}

	switch msg := ctx.Message().(type) {
	case bollywood.Started:
		// Actor started

	case UpdatePositionCommand:
		// fmt.Printf("PaddleActor %s (Index %d): Received UpdatePositionCommand, calling Move()\n", pidStr, a.state.Index) // Optional log
		a.state.Move() // Move calculates Vx/Vy/IsMoving based on Direction

	case GetPositionRequest:
		// Reply immediately with current state using ctx.Reply if it's an Ask request
		if ctx.RequestID() != "" {
			response := PositionResponse{
				X:        a.state.X,
				Y:        a.state.Y,
				Vx:       a.state.Vx,
				Vy:       a.state.Vy,
				Width:    a.state.Width,
				Height:   a.state.Height,
				IsMoving: a.state.IsMoving,
			}
			ctx.Reply(response)
		} else {
			// This case should ideally not happen if GameActor always uses Ask for GetPositionRequest
			fmt.Printf("WARN: PaddleActor %s (Index %d) received GetPositionRequest not via Ask.\n", pidStr, a.state.Index)
		}

	case PaddleDirectionMessage:
		var receivedDirection Direction
		err := json.Unmarshal(msg.Direction, &receivedDirection)
		if err == nil {
			newInternalDirection := utils.DirectionFromString(receivedDirection.Direction)
			fmt.Printf("PaddleActor %s (Index %d): Received direction '%s', internal: '%s'\n", pidStr, a.state.Index, receivedDirection.Direction, newInternalDirection) // Log direction

			// Update state only if direction actually changed
			if a.state.Direction != newInternalDirection {
				fmt.Printf("PaddleActor %s (Index %d): Direction changed from '%s' to '%s'\n", pidStr, a.state.Index, a.state.Direction, newInternalDirection) // Log change
				a.state.Direction = newInternalDirection
				a.state.IsMoving = (newInternalDirection != "") // Update IsMoving flag

				// If stopping, immediately reset velocity components
				if newInternalDirection == "" {
					a.state.Vx = 0
					a.state.Vy = 0
				}
			}
		} else {
			fmt.Printf("PaddleActor %s (Index %d) failed to unmarshal direction: %v\n", pidStr, a.state.Index, err)
			// Ensure stopped state on error
			if a.state.Direction != "" {
				a.state.Direction = ""
				a.state.Vx = 0
				a.state.Vy = 0
				a.state.IsMoving = false
			}
		}

	case bollywood.Stopping:
		// Actor stopping

	case bollywood.Stopped:
		// Actor stopped

	default:
		fmt.Printf("PaddleActor %s (Index %d) received unknown message: %T\n", pidStr, a.state.Index, msg)
		// If it was an Ask, reply with error
		if ctx.RequestID() != "" {
			ctx.Reply(fmt.Errorf("paddle actor received unknown message type: %T", msg))
		}
	}
}
