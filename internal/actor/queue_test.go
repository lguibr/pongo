package actor

import (
	"testing"
	"time"
)

func TestQueueLenCountsWaitingMessages(t *testing.T) {
	e := NewEngine()
	defer e.Shutdown(time.Second)
	gate, started := make(chan struct{}), make(chan struct{})
	p := e.Spawn(NewProps(func() Actor {
		return receiveFunc(func(c Context) {
			if _, ok := c.Message().(Started); ok {
				close(started)
				<-gate
			}
		})
	}))
	<-started
	for i := 0; i < 3; i++ {
		e.Send(p, i, nil)
	}
	if n := e.QueueLen(p); n != 3 {
		t.Fatalf("QueueLen = %d, want 3", n)
	}
	close(gate)
	if n := e.QueueLen(&PID{ID: "missing"}); n != 0 {
		t.Fatalf("QueueLen of unknown actor = %d, want 0", n)
	}
}
