// File: game/ball_actor.go
package game

import (
	"fmt"
	"time"

	"github.com/lguibr/pongo/bollywood"
	"github.com/lguibr/pongo/utils"
)

// --- Ball Actor ---

// BallActor implements the bollywood.Actor interface for managing a ball.
type BallActor struct {
	state *Ball // Use a pointer to the Ball state

	gameActorPID *bollywood.PID // PID of the GameActor to send position updates
	ticker       *time.Ticker
	stopTickerCh chan struct{}
	phasingTimer *time.Timer // Timer for phasing effect
}

// NewBallActorProducer creates a Producer for BallActor.
func NewBallActorProducer(initialState Ball, gameActorPID *bollywood.PID) bollywood.Producer {
	return func() bollywood.Actor {
		stateCopy := initialState // Make a copy for the actor
		return &BallActor{
			state:        &stateCopy,
			stopTickerCh: make(chan struct{}), // Initialize the channel
			gameActorPID: gameActorPID,        // Store GameActor PID
		}
	}
}

// --- Messages Specific to BallActor ---
// Using messages defined in messages.go and ball.go

// stopPhasingCommand internal message from timer.
type stopPhasingCommand struct{}

// --- Receive Method ---

func (a *BallActor) Receive(ctx bollywood.Context) {
	switch msg := ctx.Message().(type) {
	case bollywood.Started:
		fmt.Printf("BallActor %d (owner %d) started.\n", a.state.Id, a.state.OwnerIndex)
		a.ticker = time.NewTicker(utils.Period)
		go a.runTicker(ctx) // Start ticker goroutine
		if a.gameActorPID != nil {
			snapshot := *a.state
			ctx.Engine().Send(a.gameActorPID, BallPositionMessage{Ball: &snapshot}, ctx.Self())
		}

	case *internalTick:
		a.state.Move()
		if a.gameActorPID != nil {
			snapshot := *a.state
			ctx.Engine().Send(a.gameActorPID, BallPositionMessage{Ball: &snapshot}, ctx.Self())
		}

	case ReflectVelocityCommand:
		a.state.ReflectVelocity(msg.Axis)
	case SetVelocityCommand:
		a.state.SetVelocity(msg.Vx, msg.Vy)
	case SetPhasingCommand:
		a.state.Phasing = true
		if a.phasingTimer != nil {
			a.phasingTimer.Stop()
		}
		a.phasingTimer = time.AfterFunc(msg.ExpireIn, func() {
			engine := ctx.Engine()
			selfPID := ctx.Self()
			if engine != nil && selfPID != nil {
				// Check if actor is still running before sending
				// (This requires access to the process state, which isn't directly available here.
				// The engine handles dropping messages to stopped actors, so this is generally safe.)
				engine.Send(selfPID, stopPhasingCommand{}, nil)
			} else {
				fmt.Printf("ERROR: BallActor %d phasing timer fired but engine/selfPID invalid.\n", a.state.Id)
			}
		})
	case stopPhasingCommand:
		a.state.Phasing = false
		a.phasingTimer = nil
	case IncreaseVelocityCommand:
		a.state.IncreaseVelocity(msg.Ratio)
	case IncreaseMassCommand:
		a.state.IncreaseMass(msg.Additional)
	case DestroyBallCommand:
		fmt.Printf("BallActor %d received DestroyBallCommand. Stopping.\n", a.state.Id)
		// Let the Stopping message handle the actual cleanup
	case bollywood.Stopping:
		fmt.Printf("BallActor %d stopping...\n", a.state.Id)
		if a.ticker != nil {
			a.ticker.Stop() // Stop the ticker first
		}
		// Close channel non-blockingly to signal goroutine
		select {
		case <-a.stopTickerCh: // Already closed
		default:
			close(a.stopTickerCh) // Signal ticker loop to stop
		}
		if a.phasingTimer != nil {
			a.phasingTimer.Stop()
			a.phasingTimer = nil
		}
	case bollywood.Stopped:
		fmt.Printf("BallActor %d stopped.\n", a.state.Id)
	default:
		fmt.Printf("BallActor %d received unknown message: %T\n", a.state.Id, msg)
	}
}

// --- Ticker Goroutine ---

// runTicker is the internal loop that sends tick messages to the actor's mailbox.
func (a *BallActor) runTicker(ctx bollywood.Context) {
	fmt.Printf("BallActor %d ticker started.\n", a.state.Id)
	defer fmt.Printf("BallActor %d ticker stopped.\n", a.state.Id)

	engine := ctx.Engine()
	selfPID := ctx.Self()

	if engine == nil || selfPID == nil {
		fmt.Printf("ERROR: BallActor %d ticker cannot start, invalid engine/PID.\n", a.state.Id)
		return
	}

	tickMsg := &internalTick{}

	for {
		select {
		case <-a.stopTickerCh: // Prioritize stop signal
			return
		case <-a.ticker.C:
			// Check stop signal again before sending
			select {
			case <-a.stopTickerCh:
				return // Stop immediately if signaled
			default:
				// Send tick message to self
				engine.Send(selfPID, tickMsg, nil)
			}
		}
	}
}
