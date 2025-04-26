
package bollywood

import (
	"fmt"
	"runtime/debug"
)

const defaultMailboxSize = 1024

// process represents the running instance of an actor, including its state and mailbox.
type process struct {
	engine  *Engine
	pid     *PID
	actor   Actor
	mailbox chan *messageEnvelope
	props   *Props
	stopCh  chan struct{} // Signal to stop the run loop
	stopped bool          // Indicates if the actor is stopped or stopping
}

func newProcess(engine *Engine, pid *PID, props *Props) *process {
	return &process{
		engine:  engine,
		pid:     pid,
		props:   props,
		mailbox: make(chan *messageEnvelope, defaultMailboxSize),
		stopCh:  make(chan struct{}),
	}
}

// sendMessage sends a message to the actor's mailbox.
func (p *process) sendMessage(message interface{}, sender *PID) {
	envelope := &messageEnvelope{
		Sender:  sender,
		Message: message,
	}

	// Use non-blocking send with a fallback to report if mailbox is full
	select {
	case p.mailbox <- envelope:
		// Message sent successfully
	default:
		// TODO: Handle mailbox full scenario (e.g., drop, log, deadletter)
		fmt.Printf("Actor %s mailbox full, dropping message type %T\n", p.pid.ID, message)
	}
}

// run is the main loop for the actor process.
func (p *process) run() {
	// Defer cleanup and removal from engine
	defer func() {
		// Ensure actor is marked as stopped
		p.stopped = true
		// Send the final Stopped message
		p.invokeReceive(Stopped{}, nil)
		// Remove from engine *after* Stopped message is processed
		p.engine.remove(p.pid)
		// fmt.Printf("Actor %s goroutine exiting.\n", p.pid.ID) // Debug logging
	}()

	// Defer panic recovery
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Actor %s panicked: %v\nStack trace:\n%s\n", p.pid.ID, r, string(debug.Stack()))
			// TODO: Implement supervisor strategy (restart, stop, escalate)
			// For now, just log and stop the actor.
			p.stopped = true // Mark as stopped to prevent further processing
		}
	}()

	// Create the actor instance
	p.actor = p.props.Produce()
	if p.actor == nil {
		panic(fmt.Sprintf("Actor %s producer returned nil actor", p.pid.ID))
	}

	// fmt.Printf("Actor %s goroutine started.\n", p.pid.ID) // Debug logging

	// Main message processing loop
	for {
		select {
		case <-p.stopCh:
			// Stop signal received, exit loop after cleanup
			// fmt.Printf("Actor %s received stop signal.\n", p.pid.ID) // Debug logging
			return // Exit the loop, deferred functions will run

		case envelope := <-p.mailbox:
			if p.stopped {
				// If already stopping/stopped, ignore further messages except system ones handled below
				// fmt.Printf("Actor %s is stopped, ignoring message type %T\n", p.pid.ID, envelope.Message) // Debug logging
				continue
			}

			// Handle system messages directly
			switch msg := envelope.Message.(type) {
			case Started:
				p.invokeReceive(msg, envelope.Sender)
			case Stopping:
				// fmt.Printf("Actor %s processing Stopping message.\n", p.pid.ID) // Debug logging
				p.stopped = true // Mark as stopping
				p.invokeReceive(msg, envelope.Sender)
				// Signal the loop to stop *after* processing Stopping
				close(p.stopCh)
			case Stopped:
				// This case should ideally not be hit via mailbox, but handled in defer.
				// If it arrives here, it means something sent it manually.
				fmt.Printf("Actor %s received unexpected Stopped message via mailbox.\n", p.pid.ID)
				p.stopped = true
				p.invokeReceive(msg, envelope.Sender)
				// Ensure stopCh is closed if not already
				select {
				case <-p.stopCh: // Already closed
				default:
					close(p.stopCh)
				}
			default:
				// Process regular user message
				p.invokeReceive(envelope.Message, envelope.Sender)
			}
		}
	}
}

// invokeReceive calls the actor's Receive method within a protected context.
func (p *process) invokeReceive(msg interface{}, sender *PID) {
	// Create context for this message
	ctx := &context{
		engine:  p.engine,
		self:    p.pid,
		sender:  sender,
		message: msg,
	}

	// Call the actor's Receive method
	// Panic recovery is handled in the run loop's defer
	p.actor.Receive(ctx)
}