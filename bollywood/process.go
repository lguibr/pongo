package bollywood

import (
	"fmt"
	"runtime/debug"
	"sync/atomic"
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
	stopped atomic.Bool   // Use atomic bool for safer concurrent checks
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
	// Optimization: Don't bother sending user messages if already stopped/stopping
	// Allow system messages (like Stopping, Stopped) through.
	_, isStopping := message.(Stopping)
	_, isStopped := message.(Stopped)
	if p.stopped.Load() && !isStopping && !isStopped {
		// fmt.Printf("Actor %s already stopped, dropping user message %T\n", p.pid.ID, message)
		return
	}

	envelope := &messageEnvelope{
		Sender:  sender,
		Message: message,
	}

	// Use non-blocking send with a fallback to report if mailbox is full
	select {
	case p.mailbox <- envelope:
		// Message sent successfully
	default:
		fmt.Printf("Actor %s mailbox full, dropping message type %T\n", p.pid.ID, message)
	}
}

// run is the main loop for the actor process.
func (p *process) run() {
	// Defer cleanup and removal from engine
	defer func() {
		// Ensure actor is marked as stopped
		p.stopped.Store(true)
		// Send the final Stopped message if actor was initialized
		if p.actor != nil {
			// Use a panic-safe invoke here as well
			func() {
				defer func() {
					if r := recover(); r != nil {
						fmt.Printf("Actor %s panicked during Stopped processing: %v\n", p.pid.ID, r)
					}
				}()
				p.invokeReceive(Stopped{}, nil)
			}()
		}
		// Remove from engine *after* Stopped message is processed
		p.engine.remove(p.pid)
		// fmt.Printf("Actor %s goroutine exiting.\n", p.pid.ID) // Debug logging
	}()

	// Defer panic recovery for the main loop
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Actor %s panicked: %v\nStack trace:\n%s\n", p.pid.ID, r, string(debug.Stack()))
			p.stopped.Store(true) // Mark as stopped
			// Ensure stopCh is closed on panic (non-blocking)
			select {
			case <-p.stopCh: // Already closed
			default:
				close(p.stopCh)
			}
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
			// Stop signal received directly (e.g., from engine.Stop or panic recovery)
			// fmt.Printf("Actor %s received stop signal via stopCh.\n", p.pid.ID) // Debug logging
			if p.stopped.CompareAndSwap(false, true) {
				// If not already marked stopped (e.g., by Stopping message),
				// invoke Stopping handler now before exiting.
				// fmt.Printf("Actor %s invoking Stopping due to stopCh closure.\n", p.pid.ID)
				p.invokeReceive(Stopping{}, nil)
			}
			return // Exit the loop, deferred functions will run

		case envelope, ok := <-p.mailbox:
			if !ok {
				// Mailbox closed unexpectedly? Should not happen.
				fmt.Printf("Actor %s mailbox closed unexpectedly.\n", p.pid.ID)
				p.stopped.Store(true) // Mark as stopped
				select {
				case <-p.stopCh:
				default:
					close(p.stopCh)
				}
				return
			}

			// Check if stopped *after* receiving from mailbox,
			// but before processing, unless it's a system message.
			_, isStopping := envelope.Message.(Stopping)
			_, isStoppedMsg := envelope.Message.(Stopped) // Renamed to avoid conflict
			if p.stopped.Load() && !isStopping && !isStoppedMsg {
				// fmt.Printf("Actor %s is stopped, ignoring message type %T\n", p.pid.ID, envelope.Message)
				continue
			}

			// Handle system messages directly
			switch msg := envelope.Message.(type) {
			case Started:
				p.invokeReceive(msg, envelope.Sender)
			case Stopping:
				// fmt.Printf("Actor %s processing Stopping message.\n", p.pid.ID) // Debug logging
				if p.stopped.CompareAndSwap(false, true) { // Process only once
					p.invokeReceive(msg, envelope.Sender)
					// Signal the loop to stop *after* processing Stopping
					select {
					case <-p.stopCh: // Already closed by engine.Stop?
					default:
						close(p.stopCh)
					}
				}
			case Stopped:
				// Should be handled in defer, but log if received via mailbox
				fmt.Printf("Actor %s received unexpected Stopped message via mailbox.\n", p.pid.ID)
				if p.stopped.CompareAndSwap(false, true) {
					p.invokeReceive(msg, envelope.Sender)
					select {
					case <-p.stopCh:
					default:
						close(p.stopCh)
					}
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

	// Call the actor's Receive method, recovering from panics within it
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Actor %s panicked during Receive(%T): %v\nStack trace:\n%s\n", p.pid.ID, msg, r, string(debug.Stack()))
				// TODO: Notify supervisor?
				// For now, just log. The main loop's panic handler will ensure shutdown.
			}
		}()
		p.actor.Receive(ctx)
	}()
}
