
package bollywood

// Context provides information and capabilities to an Actor during message processing.
type Context interface {
	// Engine returns the Actor Engine managing this actor.
	Engine() *Engine
	// Self returns the PID of the actor processing the message.
	Self() *PID
	// Sender returns the PID of the actor that sent the message, if available.
	Sender() *PID
	// Message returns the actual message being processed.
	Message() interface{}
}

// context implements the Context interface.
type context struct {
	engine  *Engine
	self    *PID
	sender  *PID
	message interface{}
}

func (c *context) Engine() *Engine    { return c.engine }
func (c *context) Self() *PID         { return c.self }
func (c *context) Sender() *PID       { return c.sender }
func (c *context) Message() interface{} { return c.message }