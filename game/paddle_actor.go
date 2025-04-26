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
// It takes the initial state *by value* to ensure the actor owns its state copy.
func NewPaddleActorProducer(initialState Paddle, gameActorPID *bollywood.PID) bollywood.Producer {
	return func() bollywood.Actor {
		// Create a new Paddle instance for the actor, copying initialState.
		actorState := initialState
		return &PaddleActor{
			state:        &actorState, // Store pointer to the owned state
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
		// Start the internal ticker loop
		a.ticker = time.NewTicker(utils.Period)
		go a.runTicker(ctx)

	case *internalTick:
		// Message from internal ticker
		a.state.Move() // Update the actor's internal state
		// Send position update to GameActor (if PID is known)
		if a.gameActorPID != nil {
			// Send a pointer to the current state. The GameActor should
			// ideally copy this state upon receipt if it needs to store it,
			// to avoid race conditions if the PaddleActor modifies it later.
			// For simplicity here, we send the pointer directly.
			// Create a snapshot copy to send
            snapshot := *a.state
			ctx.Engine().Send(a.gameActorPID, PaddlePositionMessage{Paddle: &snapshot}, ctx.Self())
		} else {
			// Log infrequently or remove in production
			// fmt.Printf("PaddleActor %d: No gameActorPID to send position update to.\n", a.state.Index)
		}

	case PaddleDirectionMessage: // Message from websocket handler (via Engine.Send)
		// Replicate SetDirection logic here
		var receivedDirection Direction
		err := json.Unmarshal(msg.Direction, &receivedDirection)
		if err == nil {
			newInternalDirection := utils.DirectionFromString(receivedDirection.Direction)
			if newInternalDirection != "" {
				a.state.Direction = newInternalDirection // Update internal state
				// fmt.Printf("PaddleActor %d internal direction set to: %s\n", a.state.Index, a.state.Direction)
			} // else: ignore invalid directions from frontend
		} else {
			fmt.Printf("PaddleActor %d failed to unmarshal direction: %v\n", a.state.Index, err)
		}

	case bollywood.Stopping:
		fmt.Printf("PaddleActor %d stopping...\n", a.state.Index)
		// Stop the internal ticker
		if a.ticker != nil {
			a.ticker.Stop()
			// Use non-blocking close for safety
			select {
			case <-a.stopTickerCh:
				// Already closed
			default:
				close(a.stopTickerCh)
			}
		}

	case bollywood.Stopped:
		fmt.Printf("PaddleActor %d stopped.\n", a.state.Index)
		// Final cleanup if any

	default:
		fmt.Printf("PaddleActor %d received unknown message: %T\n", a.state.Index, msg)
	}
}

// internalTick is a message sent by the ticker goroutine to the actor's mailbox.
type internalTick struct{}

// runTicker is the internal loop that sends tick messages to the actor's mailbox.
func (a *PaddleActor) runTicker(ctx bollywood.Context) {
	fmt.Printf("PaddleActor %d ticker started.\n", a.state.Index)
	defer fmt.Printf("PaddleActor %d ticker stopped.\n", a.state.Index)

	selfPID := ctx.Self()
	engine := ctx.Engine()

	for {
		select {
		case <-a.ticker.C:
			// Send tick message to self (non-blocking)
			// Use Engine.Send to ensure it goes through the mailbox
			// Use a pointer for internalTick as it's likely more efficient.
			engine.Send(selfPID, &internalTick{}, nil) // Sender is nil as it's internal
		case <-a.stopTickerCh:
			return // Exit goroutine
		}
	}
}

// Direction struct used for unmarshaling messages from the frontend
// This is duplicated from paddle.go - consider creating a shared types package
// or moving paddle.go's version if it's only used for this.
/*
type Direction struct {
	Direction string `json:"direction"`
}
*/
