
package bollywood

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Engine manages the lifecycle and message dispatching for actors.
type Engine struct {
	pidCounter uint64
	actors     map[string]*process
	mu         sync.RWMutex // Protects the actors map
	stopping   atomic.Bool  // Indicates if the engine is shutting down
}

// NewEngine creates a new actor engine.
func NewEngine() *Engine {
	return &Engine{
		actors: make(map[string]*process),
	}
}

// nextPID generates a unique process ID.
func (e *Engine) nextPID() *PID {
	id := atomic.AddUint64(&e.pidCounter, 1)
	return &PID{ID: fmt.Sprintf("actor-%d", id)}
}

// Spawn creates and starts a new actor based on the provided Props.
// It returns the PID of the newly created actor.
func (e *Engine) Spawn(props *Props) *PID {
	if e.stopping.Load() {
		fmt.Println("Engine is stopping, cannot spawn new actors")
		return nil // Or return an error
	}

	pid := e.nextPID()
	proc := newProcess(e, pid, props)

	e.mu.Lock()
	e.actors[pid.ID] = proc
	e.mu.Unlock()

	go proc.run() // Start the actor's message loop

	// Send the Started message *after* the process is running
	e.Send(pid, Started{}, nil)

	return pid
}

// Send delivers a message to the actor identified by the PID.
// sender can be nil if the message originates from outside the actor system.
func (e *Engine) Send(pid *PID, message interface{}, sender *PID) {
	if e.stopping.Load() {
		fmt.Printf("Engine is stopping, dropping message for %s\n", pid.ID)
		return
	}

	e.mu.RLock()
	proc, ok := e.actors[pid.ID]
	e.mu.RUnlock()

	if ok {
		proc.sendMessage(message, sender)
	} else {
		// TODO: Implement dead letter queue?
		fmt.Printf("Actor %s not found, dropping message\n", pid.ID)
	}
}

// Stop requests an actor to stop processing messages and shut down.
// The actor will process a Stopping message, followed by a Stopped message
// after its goroutine exits.
func (e *Engine) Stop(pid *PID) {
	e.mu.RLock()
	_, ok := e.actors[pid.ID] // Use _ if proc is not needed when ok is true
	e.mu.RUnlock()

	if ok {
		e.Send(pid, Stopping{}, nil) // Send Stopping first
	}
}

// remove removes an actor process from the engine's tracking.
// This is called internally by the process when it fully stops.
func (e *Engine) remove(pid *PID) {
	e.mu.Lock()
	delete(e.actors, pid.ID)
	e.mu.Unlock()
	// fmt.Printf("Actor %s removed from engine\n", pid.ID) // Debug logging
}

// Shutdown stops all actors and waits for them to terminate gracefully.
// Note: Simple shutdown, might need refinement for complex dependencies.
func (e *Engine) Shutdown(timeout time.Duration) {
	if !e.stopping.CompareAndSwap(false, true) {
		fmt.Println("Engine already shutting down")
		return
	}
	fmt.Println("Engine shutdown initiated...")

	e.mu.RLock()
	pidsToStop := make([]*PID, 0, len(e.actors))
	for _, proc := range e.actors {
		pidsToStop = append(pidsToStop, proc.pid)
	}
	e.mu.RUnlock()

	fmt.Printf("Stopping %d actors...\n", len(pidsToStop))
	for _, pid := range pidsToStop {
		e.Stop(pid)
	}

	// Wait for actors to be removed (simple polling)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		e.mu.RLock()
		remaining := len(e.actors)
		e.mu.RUnlock()
		if remaining == 0 {
			fmt.Println("All actors stopped.")
			break
		}
		// fmt.Printf("%d actors remaining...\n", remaining) // Debug logging
		time.Sleep(50 * time.Millisecond)
	}

	e.mu.RLock()
	if len(e.actors) > 0 {
		fmt.Printf("Engine shutdown timeout: %d actors did not stop gracefully.\n", len(e.actors))
		// Force remove remaining actors (might leak resources if actors are stuck)
		e.mu.RUnlock()
		e.mu.Lock()
		e.actors = make(map[string]*process) // Clear map forcefully
		e.mu.Unlock()
	} else {
		e.mu.RUnlock()
	}

	fmt.Println("Engine shutdown complete.")
}