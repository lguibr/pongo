package game

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/lguibr/pongo/bollywood"
	"github.com/lguibr/pongo/utils"
)

// PaddleActor implements the bollywood.Actor interface for managing a paddle.
type PaddleActor struct {
	state        *Paddle // Use a pointer to the Paddle state
	ticker       *time.Ticker
	stopTickerCh chan struct{}
	gameActorPID *bollywood.PID // PID of the GameActor to send position updates
}

// NewPaddleActorProducer creates a bollywood.Producer for PaddleActor.
func NewPaddleActorProducer(initialState Paddle, gameActorPID *bollywood.PID) bollywood.Producer {
	return func() bollywood.Actor {
		actorState := initialState
		return &PaddleActor{
			state:        &actorState,
			stopTickerCh: make(chan struct{}),
			gameActorPID: gameActorPID,
		}
	}
}

// Receive handles incoming messages for the PaddleActor.
func (a *PaddleActor) Receive(ctx bollywood.Context) {
	switch msg := ctx.Message().(type) {
	case bollywood.Started:
		fmt.Printf("PaddleActor %d started.\n", a.state.Index)
		a.ticker = time.NewTicker(utils.Period)
		go a.runTicker(ctx)
		if a.gameActorPID != nil {
			snapshot := *a.state
			ctx.Engine().Send(a.gameActorPID, PaddlePositionMessage{Paddle: &snapshot}, ctx.Self())
		}

	case *internalTick:
		a.state.Move()
		if a.gameActorPID != nil {
			snapshot := *a.state
			ctx.Engine().Send(a.gameActorPID, PaddlePositionMessage{Paddle: &snapshot}, ctx.Self())
		} else {
			// fmt.Printf("PaddleActor %d: No gameActorPID to send position update to.\n", a.state.Index)
		}

	case PaddleDirectionMessage:
		var receivedDirection Direction
		err := json.Unmarshal(msg.Direction, &receivedDirection)
		if err == nil {
			newInternalDirection := utils.DirectionFromString(receivedDirection.Direction)
			if newInternalDirection != "" {
				if a.state.Direction != newInternalDirection {
					a.state.Direction = newInternalDirection
				}
			} else {
				a.state.Direction = ""
			}
		} else {
			fmt.Printf("PaddleActor %d failed to unmarshal direction: %v\n", a.state.Index, err)
			a.state.Direction = ""
		}

	case bollywood.Stopping:
		fmt.Printf("PaddleActor %d stopping...\n", a.state.Index)
		if a.ticker != nil {
			a.ticker.Stop() // Stop the ticker first
		}
		// Close channel non-blockingly to signal goroutine
		select {
		case <-a.stopTickerCh: // Already closed
		default:
			close(a.stopTickerCh)
		}

	case bollywood.Stopped:
		fmt.Printf("PaddleActor %d stopped.\n", a.state.Index)

	default:
		fmt.Printf("PaddleActor %d received unknown message: %T\n", a.state.Index, msg)
	}
}

// runTicker is the internal loop that sends tick messages to the actor's mailbox.
func (a *PaddleActor) runTicker(ctx bollywood.Context) {
	fmt.Printf("PaddleActor %d ticker started.\n", a.state.Index)
	defer fmt.Printf("PaddleActor %d ticker stopped.\n", a.state.Index)

	engine := ctx.Engine()
	selfPID := ctx.Self()

	if engine == nil || selfPID == nil {
		fmt.Printf("ERROR: PaddleActor %d ticker cannot start, invalid engine/PID.\n", a.state.Index)
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
				return
			default:
				// Send tick message to self
				engine.Send(selfPID, tickMsg, nil)
			}
		}
	}
}

// Direction struct defined in paddle.go is used here for unmarshalling.
