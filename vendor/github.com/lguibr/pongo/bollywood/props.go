
package bollywood

// Producer is a function that creates a new instance of an Actor.
type Producer func() Actor

// Props is a configuration object used to create actors.
type Props struct {
	producer Producer
	// We could add mailbox configuration, supervisor strategy, etc. here later
}

// NewProps creates a new Props object with the given actor producer.
func NewProps(producer Producer) *Props {
	if producer == nil {
		panic("bollywood: producer cannot be nil")
	}
	return &Props{
		producer: producer,
	}
}

// Produce creates a new actor instance using the configured producer.
func (p *Props) Produce() Actor {
	return p.producer()
}