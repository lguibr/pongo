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
		return nil
	}

	pid := e.nextPID()
	proc := newProcess(e, pid, props)

	e.mu.Lock()
	e.actors[pid.ID] = proc
	e.mu.Unlock()

	go proc.run()

	e.Send(pid, Started{}, nil)

	return pid
}

// Send delivers a message to the actor identified by the PID.
func (e *Engine) Send(pid *PID, message interface{}, sender *PID) {
	// Allow system messages during shutdown for cleanup
	_, isStopping := message.(Stopping)
	_, isStopped := message.(Stopped)
	isSystemMsg := isStopping || isStopped || (message == Started{}) // Add Started

	if e.stopping.Load() && !isSystemMsg {
		// fmt.Printf("Engine is stopping, dropping user message for %s\n", pid.ID) // Reduce noise
		return
	}

	e.mu.RLock()
	proc, ok := e.actors[pid.ID]
	e.mu.RUnlock()

	if ok {
		proc.sendMessage(message, sender)
	} else {
		// Avoid logging dropped messages during shutdown tests if actor was already stopped
		// if !e.stopping.Load() {
		// 	fmt.Printf("Actor %s not found, dropping message %T\n", pid.ID, message)
		// }
	}
}

// Stop requests an actor to stop processing messages and shut down.
// It sends the Stopping message and also directly signals the actor's stop channel.
func (e *Engine) Stop(pid *PID) {
	e.mu.RLock()
	proc, ok := e.actors[pid.ID]
	e.mu.RUnlock()

	if ok {
		// Send Stopping message first to allow graceful cleanup within the actor's context
		e.Send(pid, Stopping{}, nil)

		// Directly signal the stop channel to ensure termination even if mailbox is full
		// Use non-blocking close
		select {
		case <-proc.stopCh: // Already closed
		default:
			close(proc.stopCh)
			// fmt.Printf("Engine directly closed stopCh for %s\n", pid.ID) // Debug log
		}
	}
}

// remove removes an actor process from the engine's tracking.
func (e *Engine) remove(pid *PID) {
	e.mu.Lock()
	delete(e.actors, pid.ID)
	e.mu.Unlock()
	// fmt.Printf("Actor %s removed from engine\n", pid.ID)
}

// Shutdown stops all actors and waits for them to terminate gracefully.
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
		e.Stop(pid) // Stop now signals stopCh directly too
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		e.mu.RLock()
		remaining := len(e.actors)
		e.mu.RUnlock()
		if remaining == 0 {
			fmt.Println("All actors stopped.")
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	e.mu.RLock()
	remainingCount := len(e.actors)
	if remainingCount > 0 {
		fmt.Printf("Engine shutdown timeout: %d actors did not stop gracefully.\n", remainingCount)
		remainingActors := []string{}
		for pidStr := range e.actors {
			remainingActors = append(remainingActors, pidStr)
		}
		fmt.Printf("Remaining actors: %v\n", remainingActors)
		e.mu.RUnlock()
		e.mu.Lock()
		e.actors = make(map[string]*process)
		e.mu.Unlock()
	} else {
		e.mu.RUnlock()
	}

	fmt.Println("Engine shutdown complete.")
}
