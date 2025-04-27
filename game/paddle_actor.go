// File: game/paddle_actor.go
package game

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
)

// PaddleActor implements the bollywood.Actor interface for managing a paddle.
type PaddleActor struct {
	state        *Paddle      // Use a pointer to the Paddle state
	cfg          utils.Config // Store config
	ticker       *time.Ticker
	stopTickerCh chan struct{}
	gameActorPID *bollywood.PID // PID of the GameActor to send position updates
}

// NewPaddleActorProducer creates a bollywood.Producer for PaddleActor.
func NewPaddleActorProducer(initialState Paddle, gameActorPID *bollywood.PID, cfg utils.Config) bollywood.Producer { // Accept config
	return func() bollywood.Actor {
		actorState := initialState
		return &PaddleActor{
			state:        &actorState,
			cfg:          cfg,                 // Store config
			stopTickerCh: make(chan struct{}), // Initialize the channel
			gameActorPID: gameActorPID,
		}
	}
}

// Receive handles incoming messages for the PaddleActor.
func (a *PaddleActor) Receive(ctx bollywood.Context) {
	switch msg := ctx.Message().(type) {
	case bollywood.Started:
		a.ticker = time.NewTicker(a.cfg.GameTickPeriod)
		go a.runTicker(ctx)
		if a.gameActorPID != nil {
			snapshot := *a.state // Send initial state
			ctx.Engine().Send(a.gameActorPID, PaddlePositionMessage{Paddle: &snapshot}, ctx.Self())
		}

	case *internalTick:
		a.state.Move() // Move calculates Vx/Vy/IsMoving based on Direction
		if a.gameActorPID != nil {
			snapshot := *a.state // Send state after move
			ctx.Engine().Send(a.gameActorPID, PaddlePositionMessage{Paddle: &snapshot}, ctx.Self())
		}

	case PaddleDirectionMessage:
		var receivedDirection Direction
		err := json.Unmarshal(msg.Direction, &receivedDirection)
		if err == nil {
			newInternalDirection := utils.DirectionFromString(receivedDirection.Direction)

			if a.state.Direction != newInternalDirection {
				a.state.Direction = newInternalDirection
				a.state.IsMoving = (newInternalDirection != "")

				if newInternalDirection == "" {
					a.state.Vx = 0
					a.state.Vy = 0
					if a.gameActorPID != nil {
						snapshot := *a.state
						// *** ADD LOGGING ***
						fmt.Printf("PaddleActor %d: Received STOP. Setting IsMoving=false. Sending update.\n", a.state.Index)
						ctx.Engine().Send(a.gameActorPID, PaddlePositionMessage{Paddle: &snapshot}, ctx.Self())
					}
				} else {
					// fmt.Printf("PaddleActor %d: Set direction to '%s' (IsMoving: %t)\n", a.state.Index, newInternalDirection, a.state.IsMoving)
				}
			}
		} else {
			fmt.Printf("PaddleActor %d failed to unmarshal direction: %v\n", a.state.Index, err)
			if a.state.Direction != "" {
				a.state.Direction = ""
				a.state.Vx = 0
				a.state.Vy = 0
				a.state.IsMoving = false
				if a.gameActorPID != nil {
					snapshot := *a.state
					// *** ADD LOGGING ***
					fmt.Printf("PaddleActor %d: Error unmarshalling. Setting IsMoving=false. Sending update.\n", a.state.Index)
					ctx.Engine().Send(a.gameActorPID, PaddlePositionMessage{Paddle: &snapshot}, ctx.Self())
				}
			}
		}

	case bollywood.Stopping:
		if a.ticker != nil {
			a.ticker.Stop()
		}
		select {
		case <-a.stopTickerCh:
		default:
			close(a.stopTickerCh)
		}

	case bollywood.Stopped:
		// fmt.Printf("PaddleActor %d stopped.\n", a.state.Index)

	default:
		fmt.Printf("PaddleActor %d received unknown message: %T\n", a.state.Index, msg)
	}
}

// runTicker is the internal loop that sends tick messages to the actor's mailbox.
func (a *PaddleActor) runTicker(ctx bollywood.Context) {
	engine := ctx.Engine()
	selfPID := ctx.Self()

	if engine == nil || selfPID == nil {
		fmt.Printf("ERROR: PaddleActor %d ticker cannot start, invalid engine/PID.\n", a.state.Index)
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
