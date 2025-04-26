
package bollywood

// Actor is the interface that defines actor behavior.
// Actors process messages sequentially received from their mailbox.
type Actor interface {
	// Receive processes incoming messages. The actor can use the context
	// to interact with the system (e.g., get self PID, sender PID, spawn children).
	Receive(ctx Context)
}