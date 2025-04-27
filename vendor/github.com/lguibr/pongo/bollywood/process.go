// File: bollywood/process.go
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
// (No changes needed here)
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
		// Avoid logging during shutdown spam
		// if !p.engine.stopping.Load() {
		// 	fmt.Printf("Actor %s mailbox full, dropping message type %T\n", p.pid.ID, message)
		// }
	}
}

// run is the main loop for the actor process.
func (p *process) run() {
	var stoppingInvoked bool // Track if Stopping handler has been called

	// Defer final cleanup and Stopped message
	defer func() {
		// Ensure actor is marked as stopped
		p.stopped.Store(true)

		// Recover from panic during Stopped processing
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Actor %s panicked during final cleanup/Stopped processing: %v\n", p.pid.ID, r)
			}
			// Remove from engine *after* all cleanup attempts
			p.engine.remove(p.pid)
			// fmt.Printf("Actor %s goroutine exiting.\n", p.pid.ID) // Debug logging
		}()

		// Send the final Stopped message if actor was initialized and Stopping was invoked
		if p.actor != nil && stoppingInvoked {
			// fmt.Printf("Actor %s invoking final Stopped handler.\n", p.pid.ID) // Debug log
			p.invokeReceive(Stopped{}, nil) // Call Stopped handler
		} else if p.actor != nil && !stoppingInvoked {
			// This case might happen if the actor panicked *before* Stopping could be called.
			// We might still want to log Stopped, but it indicates an unusual exit.
			fmt.Printf("WARN: Actor %s stopped without Stopping handler being invoked (likely due to early panic).\n", p.pid.ID)
			p.invokeReceive(Stopped{}, nil) // Call Stopped handler anyway? Or just log? Let's call it.
		}

	}()

	// Defer panic recovery for the main loop and actor initialization
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Actor %s panicked: %v\nStack trace:\n%s\n", p.pid.ID, r, string(debug.Stack()))
			// Ensure stopCh is closed on panic (non-blocking)
			// and mark as stopped immediately
			if p.stopped.CompareAndSwap(false, true) {
				select {
				case <-p.stopCh: // Already closed
				default:
					close(p.stopCh)
				}
				// Attempt to invoke Stopping handler on panic if not already invoked
				if p.actor != nil && !stoppingInvoked {
					// fmt.Printf("Actor %s invoking Stopping due to panic.\n", p.pid.ID)
					p.invokeReceive(Stopping{}, nil)
					stoppingInvoked = true
				}
			}
		}
	}()

	// Create the actor instance
	p.actor = p.props.Produce()
	if p.actor == nil {
		panic(fmt.Sprintf("Actor %s producer returned nil actor", p.pid.ID))
	}
	// Send Started message *after* actor is created
	p.invokeReceive(Started{}, nil)

	// fmt.Printf("Actor %s goroutine started and processing messages.\n", p.pid.ID) // Debug logging

	// Main message processing loop
	for {
		select {
		case <-p.stopCh:
			// Stop signal received directly (e.g., from engine.Stop or panic recovery)
			// fmt.Printf("Actor %s received stop signal via stopCh.\n", p.pid.ID) // Debug logging
			if p.stopped.CompareAndSwap(false, true) {
				// If not already marked stopped (e.g., by Stopping message),
				// invoke Stopping handler now before exiting.
				if !stoppingInvoked {
					// fmt.Printf("Actor %s invoking Stopping due to stopCh closure.\n", p.pid.ID)
					p.invokeReceive(Stopping{}, nil)
					stoppingInvoked = true
				}
			}
			return // Exit the loop, deferred functions will run

		case envelope, ok := <-p.mailbox:
			if !ok {
				// Mailbox closed unexpectedly? Should not happen with current design.
				fmt.Printf("Actor %s mailbox closed unexpectedly.\n", p.pid.ID)
				if p.stopped.CompareAndSwap(false, true) {
					select {
					case <-p.stopCh:
					default:
						close(p.stopCh)
					}
					if !stoppingInvoked {
						// fmt.Printf("Actor %s invoking Stopping due to unexpected mailbox closure.\n", p.pid.ID)
						p.invokeReceive(Stopping{}, nil)
						stoppingInvoked = true
					}
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
			case Stopping:
				// fmt.Printf("Actor %s processing Stopping message from mailbox.\n", p.pid.ID) // Debug logging
				if p.stopped.CompareAndSwap(false, true) { // Process only once
					if !stoppingInvoked {
						p.invokeReceive(msg, envelope.Sender)
						stoppingInvoked = true
					}
					// Signal the loop to stop *after* processing Stopping
					select {
					case <-p.stopCh: // Already closed by engine.Stop?
					default:
						close(p.stopCh)
					}
				}
			case Stopped:
				// Should be handled in defer, but log if received via mailbox.
				// This indicates a potential logic error elsewhere.
				fmt.Printf("WARN: Actor %s received unexpected Stopped message via mailbox.\n", p.pid.ID)
				if p.stopped.CompareAndSwap(false, true) {
					if !stoppingInvoked {
						// If Stopping wasn't called, call it now before Stopped
						p.invokeReceive(Stopping{}, nil)
						stoppingInvoked = true
					}
					p.invokeReceive(msg, envelope.Sender) // Call the received Stopped handler
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
// (No changes needed here)
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
				// Ensure stopCh is closed on panic within Receive
				if p.stopped.CompareAndSwap(false, true) {
					select {
					case <-p.stopCh:
					default:
						close(p.stopCh)
					}
					// Attempt to invoke Stopping handler on panic if not already invoked
					if !p.stopped.Load() { // Check stopped flag again inside recover
						// fmt.Printf("Actor %s invoking Stopping due to panic in Receive.\n", p.pid.ID)
						p.invokeReceive(Stopping{}, nil) // Call stopping handler
						// stoppingInvoked = true // Cannot set this here, need to pass it back or handle differently
					}
				}
			}
		}()
		p.actor.Receive(ctx)
	}()
}
