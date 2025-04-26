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

	// Game Context (Needed for Collision Checks)
	gameActorPID *bollywood.PID // PID of the GameActor to send collision/state messages
	// TODO: How does the BallActor get grid/paddle info for collisions?
	// Option 1: GameActor sends it periodically/on change (complex state sync).
	// Option 2: BallActor sends its position, GameActor does collisions (simpler BallActor).
	// Option 3: BallActor requests info from GameActor (blocking/async complexity).
	// --> Let's try Option 2 for now: BallActor moves and sends position, GameActor handles collisions.
	// This means CollidePaddles, CollideCells, CollideWalls logic moves *out* of BallActor.

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
			stopTickerCh: make(chan struct{}),
			gameActorPID: gameActorPID,
		}
	}
}

// --- Messages Specific to BallActor ---

// SetPhasingCommand tells the ball actor to start phasing.
type SetPhasingCommand struct {
	ExpireIn time.Duration // Duration, not int seconds
}

// StopPhasingCommand internal message from timer.
type stopPhasingCommand struct{}

// IncreaseVelocityCommand tells the ball to increase velocity.
type IncreaseVelocityCommand struct {
	Ratio float64
}

// IncreaseMassCommand tells the ball to increase mass.
type IncreaseMassCommand struct {
	Additional int
}

// --- Receive Method ---

func (a *BallActor) Receive(ctx bollywood.Context) {
	switch msg := ctx.Message().(type) {
	case bollywood.Started:
		fmt.Printf("BallActor %d (owner %d) started.\n", a.state.Id, a.state.OwnerIndex)
		a.ticker = time.NewTicker(utils.Period)
		go a.runTicker(ctx)
		// Send initial position
		if a.gameActorPID != nil {
            snapshot := *a.state
			ctx.Engine().Send(a.gameActorPID, BallPositionMessage{Ball: &snapshot}, ctx.Self())
		}


	case *internalTick: // Message from internal ticker
		if a.state.Phasing && a.phasingTimer == nil {
             // This shouldn't happen if phasing is set correctly via messages, but handle defensively.
             fmt.Printf("WARN: BallActor %d phasing flag is true but timer is nil. Resetting flag.\n", a.state.Id)
             a.state.Phasing = false
        }
		
		a.state.Move() // Update position

		// Send new position to GameActor. GameActor will handle collisions.
		if a.gameActorPID != nil {
            snapshot := *a.state
			ctx.Engine().Send(a.gameActorPID, BallPositionMessage{Ball: &snapshot}, ctx.Self())
		}

	// --- Commands from GameActor ---
	case SetPhasingCommand:
		fmt.Printf("BallActor %d received SetPhasingCommand (%v)\n", a.state.Id, msg.ExpireIn)
		a.state.Phasing = true
        // Stop existing timer if any
        if a.phasingTimer != nil {
            a.phasingTimer.Stop()
        }
		// Start a timer to send stopPhasingCommand back to self
		a.phasingTimer = time.AfterFunc(msg.ExpireIn, func() {
            // Need engine and self PID to send message back
            // Capture them carefully. A reference to ctx might be invalid later.
            engine := ctx.Engine()
            selfPID := ctx.Self()
            if engine != nil && selfPID != nil {
			    engine.Send(selfPID, stopPhasingCommand{}, nil)
            } else {
                 fmt.Printf("ERROR: BallActor %d phasing timer fired but engine/selfPID invalid.\n", a.state.Id)
            }
		})

	case stopPhasingCommand: // Internal command from timer
         fmt.Printf("BallActor %d received stopPhasingCommand\n", a.state.Id)
		a.state.Phasing = false
        a.phasingTimer = nil // Clear timer reference

	case IncreaseVelocityCommand:
        fmt.Printf("BallActor %d received IncreaseVelocityCommand (%.2f)\n", a.state.Id, msg.Ratio)
		a.state.IncreaseVelocity(msg.Ratio)

	case IncreaseMassCommand:
        fmt.Printf("BallActor %d received IncreaseMassCommand (+%d)\n", a.state.Id, msg.Additional)
		a.state.IncreaseMass(msg.Additional)

	case bollywood.Stopping:
		fmt.Printf("BallActor %d stopping...\n", a.state.Id)
		if a.ticker != nil {
			a.ticker.Stop()
			select {
			case <-a.stopTickerCh: // Already closed
			default: close(a.stopTickerCh)
			}
		}
        if a.phasingTimer != nil {
            a.phasingTimer.Stop() // Stop phasing timer if active
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

	// Capture engine and self PID from the initial context
	// This is safer than passing the context itself into the goroutine
	engine := ctx.Engine()
	selfPID := ctx.Self()

    if engine == nil || selfPID == nil {
        fmt.Printf("ERROR: BallActor %d ticker cannot start, invalid engine/PID.\n", a.state.Id)
        return
    }

	for {
		select {
		case <-a.ticker.C:
			// Send tick message to self (non-blocking)
			engine.Send(selfPID, &internalTick{}, nil)
		case <-a.stopTickerCh:
			return // Exit goroutine
		}
	}
}

