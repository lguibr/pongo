// File: game/paddle_actor.go
package game

import (
	"encoding/json"
	"fmt"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
)

// PaddleActor implements the bollywood.Actor interface for managing a paddle.
// It updates its internal direction based on commands and sends a state update
// back to the GameActor.
type PaddleActor struct {
	state        *Paddle        // Use a pointer to the Paddle state
	cfg          utils.Config   // Store config
	gameActorPID *bollywood.PID // PID of the GameActor (parent)
	selfPID      *bollywood.PID // Store self PID for logging
}

// NewPaddleActorProducer creates a bollywood.Producer for PaddleActor.
func NewPaddleActorProducer(initialState Paddle, gameActorPID *bollywood.PID, cfg utils.Config) bollywood.Producer {
	return func() bollywood.Actor {
		// Create a copy of the initial state for this actor instance
		actorState := initialState
		return &PaddleActor{
			state:        &actorState, // Pass the address of the copy
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

	case PaddleDirectionMessage:
		var receivedDirection Direction
		err := json.Unmarshal(msg.Direction, &receivedDirection)
		directionChanged := false
		newInternalDirection := ""
		if err == nil {
			newInternalDirection = utils.DirectionFromString(receivedDirection.Direction)
			if a.state.Direction != newInternalDirection {
				a.state.Direction = newInternalDirection
				directionChanged = true
			}
		} else {
			fmt.Printf("PaddleActor %s (Index %d) failed to unmarshal direction: %v\n", pidStr, a.state.Index, err)
			// Ensure stopped state on error
			if a.state.Direction != "" {
				a.state.Direction = ""
				directionChanged = true // Direction changed to stop
			}
		}
		// Send state update back to GameActor if direction changed
		if directionChanged && a.gameActorPID != nil && a.selfPID != nil {
			updateMsg := PaddleStateUpdate{
				PID:       a.selfPID,
				Index:     a.state.Index,
				Direction: a.state.Direction,
			}
			ctx.Engine().Send(a.gameActorPID, updateMsg, a.selfPID)
		}

	// Removed GetInternalStateDebug handler case

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