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
type BallActor struct {
	state *Ball        // Use a pointer to the Ball state
	cfg   utils.Config // Store config

	gameActorPID *bollywood.PID // PID of the GameActor (parent)
	phasingTimer *time.Timer    // Timer for phasing effect
}

// NewBallActorProducer creates a Producer for BallActor.
func NewBallActorProducer(initialState Ball, gameActorPID *bollywood.PID, cfg utils.Config) bollywood.Producer {
	return func() bollywood.Actor {
		stateCopy := initialState // Make a copy for the actor
		return &BallActor{
			state:        &stateCopy,
			cfg:          cfg,
			gameActorPID: gameActorPID,
		}
	}
}

// --- Messages Specific to BallActor ---

// stopPhasingCommand internal message from timer.
type stopPhasingCommand struct{}

// --- Receive Method ---

func (a *BallActor) Receive(ctx bollywood.Context) {
	switch msg := ctx.Message().(type) {
	case bollywood.Started:
		// Actor started

	case UpdatePositionCommand:
		a.state.Move()

	case GetPositionRequest:
		// Reply immediately with current state using ctx.Reply if it's an Ask request
		if ctx.RequestID() != "" {
			response := PositionResponse{
				X:       a.state.X,
				Y:       a.state.Y,
				Vx:      a.state.Vx,
				Vy:      a.state.Vy,
				Radius:  a.state.Radius,
				Phasing: a.state.Phasing,
				// Include other fields if needed, e.g., Mass, IsPermanent
			}
			ctx.Reply(response)
		} else {
			// This case should ideally not happen if GameActor always uses Ask for GetPositionRequest
			fmt.Printf("WARN: BallActor %d received GetPositionRequest not via Ask.\n", a.state.Id)
		}

	case ReflectVelocityCommand:
		a.state.ReflectVelocity(msg.Axis)
	case SetVelocityCommand:
		a.state.SetVelocity(msg.Vx, msg.Vy)
	case SetPhasingCommand:
		a.state.Phasing = true
		if a.phasingTimer != nil {
			a.phasingTimer.Stop() // Stop existing timer if any
		}
		// Use config for phasing time
		a.phasingTimer = time.AfterFunc(a.cfg.BallPhasingTime, func() {
			// Need engine and self PID to send message back to self
			engine := ctx.Engine() // Capture engine from context
			selfPID := ctx.Self()  // Capture self PID from context
			if engine != nil && selfPID != nil {
				// Send message back to the actor's own mailbox
				engine.Send(selfPID, stopPhasingCommand{}, nil)
			} else {
				// This case should be rare but log if it happens
				fmt.Printf("ERROR: BallActor %d phasing timer fired but engine or selfPID is nil.\n", a.state.Id)
			}
		})
	case stopPhasingCommand:
		a.state.Phasing = false
		a.phasingTimer = nil // Clear the timer reference
	case IncreaseVelocityCommand:
		a.state.IncreaseVelocity(msg.Ratio) // Ratio comes from GameActor physics now
	case IncreaseMassCommand:
		a.state.IncreaseMass(a.cfg, msg.Additional) // Pass config
	case DestroyBallCommand:
		// Let the Stopping message handle the actual cleanup
		ctx.Engine().Stop(ctx.Self()) // Initiate stop process

	case bollywood.Stopping:
		// Stop the phasing timer if it's running
		if a.phasingTimer != nil {
			a.phasingTimer.Stop()
			a.phasingTimer = nil
		}

	case bollywood.Stopped:
		// Actor stopped

	default:
		fmt.Printf("BallActor %d received unknown message: %T\n", a.state.Id, msg)
		// If it was an Ask, reply with error
		if ctx.RequestID() != "" {
			ctx.Reply(fmt.Errorf("ball actor received unknown message type: %T", msg))
		}
	}
}
