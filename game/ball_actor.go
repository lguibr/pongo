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

	gameActorPID *bollywood.PID // PID of the GameActor to send position updates
	ticker       *time.Ticker
	stopTickerCh chan struct{}
	phasingTimer *time.Timer // Timer for phasing effect
}

// NewBallActorProducer creates a Producer for BallActor.
func NewBallActorProducer(initialState Ball, gameActorPID *bollywood.PID, cfg utils.Config) bollywood.Producer {
	return func() bollywood.Actor {
		stateCopy := initialState // Make a copy for the actor
		return &BallActor{
			state:        &stateCopy,
			cfg:          cfg,                 // Store config
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
		// fmt.Printf("BallActor %d (owner %d) started.\n", a.state.Id, a.state.OwnerIndex) // Reduce noise
		a.ticker = time.NewTicker(a.cfg.GameTickPeriod) // Use config for period
		go a.runTicker(ctx)                             // Start ticker goroutine
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
		// Use config for phasing time
		a.phasingTimer = time.AfterFunc(a.cfg.BallPhasingTime, func() {
			engine := ctx.Engine()
			selfPID := ctx.Self()
			if engine != nil && selfPID != nil {
				engine.Send(selfPID, stopPhasingCommand{}, nil)
			}
		})
	case stopPhasingCommand:
		a.state.Phasing = false
		a.phasingTimer = nil
	case IncreaseVelocityCommand:
		a.state.IncreaseVelocity(msg.Ratio) // Ratio comes from GameActor physics now
	case IncreaseMassCommand:
		a.state.IncreaseMass(a.cfg, msg.Additional) // Pass config
	case DestroyBallCommand:
		// Let the Stopping message handle the actual cleanup
	case bollywood.Stopping:
		if a.ticker != nil {
			a.ticker.Stop()
		}
		select {
		case <-a.stopTickerCh:
		default:
			close(a.stopTickerCh)
		}
		if a.phasingTimer != nil {
			a.phasingTimer.Stop()
			a.phasingTimer = nil
		}
	case bollywood.Stopped:
		// fmt.Printf("BallActor %d stopped.\n", a.state.Id) // Reduce noise
	default:
		fmt.Printf("BallActor %d received unknown message: %T\n", a.state.Id, msg)
	}
}

// --- Ticker Goroutine ---

// runTicker is the internal loop that sends tick messages to the actor's mailbox.
func (a *BallActor) runTicker(ctx bollywood.Context) {
	engine := ctx.Engine()
	selfPID := ctx.Self()

	if engine == nil || selfPID == nil {
		fmt.Printf("ERROR: BallActor %d ticker cannot start, invalid engine/PID.\n", a.state.Id)
		return
	}

	tickMsg := &internalTick{}

	for {
		select {
		case <-a.stopTickerCh:
			return
		case <-a.ticker.C:
			select {
			case <-a.stopTickerCh:
				return
			default:
				engine.Send(selfPID, tickMsg, nil)
			}
		}
	}
}
