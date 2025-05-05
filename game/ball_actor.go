// File: game/ball_actor.go
package game

import (
	"fmt"
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
)

// --- Ball Actor ---

// BallActor implements the bollywood.Actor interface for managing a ball.
// It updates its internal state based on commands and sends state updates
// back to the GameActor when relevant state changes.
type BallActor struct {
	state        *Ball        // Use a pointer to the Ball state
	cfg          utils.Config // Store config
	gameActorPID *bollywood.PID // PID of the GameActor (parent)
	phasingTimer *time.Timer    // Timer for phasing effect
	selfPID      *bollywood.PID // Store self PID

	// Tracks bricks damaged by this ball during its current phasing period
	bricksDamagedThisPhase map[[2]int]bool // map{[col,row]}bool
}

// NewBallActorProducer creates a Producer for BallActor.
func NewBallActorProducer(initialState Ball, gameActorPID *bollywood.PID, cfg utils.Config) bollywood.Producer {
	return func() bollywood.Actor {
		// Create a copy of the initial state for this actor instance
		stateCopy := initialState
		return &BallActor{
			state:        &stateCopy, // Pass address of the copy
			cfg:          cfg,
			gameActorPID: gameActorPID,
			// bricksDamagedThisPhase is initialized when phasing starts
		}
	}
}

// --- Messages Specific to BallActor ---

// stopPhasingCommand internal message from timer.
type stopPhasingCommand struct{}

// --- Receive Method ---

func (a *BallActor) Receive(ctx bollywood.Context) {
	if a.selfPID == nil {
		a.selfPID = ctx.Self()
	}

	stateChanged := false // Flag to track if state relevant to GameActor changed

	switch msg := ctx.Message().(type) {
	case bollywood.Started:
		// Actor started

	case ReflectVelocityCommand:
		a.state.ReflectVelocity(msg.Axis)
		stateChanged = true
	case SetVelocityCommand:
		a.state.SetVelocity(msg.Vx, msg.Vy)
		stateChanged = true
	case SetPhasingCommand:
		if !a.state.Phasing { // Only change state if not already phasing
			a.state.Phasing = true
			// Initialize the damage tracker when phasing starts
			a.bricksDamagedThisPhase = make(map[[2]int]bool)
			stateChanged = true
			if a.phasingTimer != nil {
				a.phasingTimer.Stop() // Stop existing timer if any
			}
			// Use config for phasing time
			a.phasingTimer = time.AfterFunc(a.cfg.BallPhasingTime, func() {
				engine := ctx.Engine()
				selfPID := ctx.Self()
				if engine != nil && selfPID != nil {
					engine.Send(selfPID, stopPhasingCommand{}, nil)
				} else {
					fmt.Printf("ERROR: BallActor %d phasing timer fired but engine or selfPID is nil.\n", a.state.Id)
				}
			})
		}
	case stopPhasingCommand:
		if a.state.Phasing { // Only change state if currently phasing
			a.state.Phasing = false
			// Clear the damage tracker when phasing ends
			a.bricksDamagedThisPhase = nil
			stateChanged = true
		}
		a.phasingTimer = nil // Clear the timer reference
	case IncreaseVelocityCommand:
		a.state.IncreaseVelocity(msg.Ratio)
		stateChanged = true
	case IncreaseMassCommand:
		a.state.IncreaseMass(a.cfg, msg.Additional)
		stateChanged = true // Mass and Radius changed
	case DestroyBallCommand:
		ctx.Engine().Stop(ctx.Self()) // Initiate stop process

	case DamageBrickCommand:
		// Check if phasing and if brick hasn't been damaged this phase
		if a.state.Phasing && a.bricksDamagedThisPhase != nil {
			if !a.bricksDamagedThisPhase[msg.Coord] {
				// Mark as damaged this phase
				a.bricksDamagedThisPhase[msg.Coord] = true
				// Tell GameActor to apply the damage
				if a.gameActorPID != nil {
					ctx.Engine().Send(a.gameActorPID, ApplyBrickDamage{
						BallID:     a.state.Id,
						Coord:      msg.Coord,
						BallX:      a.state.X, // Pass current ball state for context
						BallY:      a.state.Y,
						OwnerIndex: a.state.OwnerIndex,
					}, a.selfPID)
				}
			}
			// If already damaged this phase, do nothing.
		}

	case bollywood.Stopping:
		if a.phasingTimer != nil {
			a.phasingTimer.Stop()
			a.phasingTimer = nil
		}
		a.bricksDamagedThisPhase = nil // Clear map on stop

	case bollywood.Stopped:
		// Actor stopped

	default:
		fmt.Printf("BallActor %d received unknown message: %T\n", a.state.Id, msg)
		if ctx.RequestID() != "" {
			ctx.Reply(fmt.Errorf("ball actor received unknown message type: %T", msg))
		}
	}

	// Send state update back to GameActor if relevant state changed
	if stateChanged && a.gameActorPID != nil && a.selfPID != nil {
		updateMsg := BallStateUpdate{
			PID:     a.selfPID,
			ID:      a.state.Id,
			Vx:      a.state.Vx,
			Vy:      a.state.Vy,
			Radius:  a.state.Radius,
			Mass:    a.state.Mass,
			Phasing: a.state.Phasing,
		}
		ctx.Engine().Send(a.gameActorPID, updateMsg, a.selfPID)
	}
}