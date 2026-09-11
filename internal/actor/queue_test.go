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

func TestQueueLenIsZeroWhileStopping(t *testing.T) {
	e := NewEngine()
	defer e.Shutdown(time.Second)
	gate, started := make(chan struct{}), make(chan struct{})
	seen := make(chan int, 1)
	p := e.Spawn(NewProps(func() Actor {
		return receiveFunc(func(c Context) {
			switch c.Message().(type) {
			case int:
				close(started)
				<-gate
			case Stopping:
				seen <- c.Engine().QueueLen(c.Self())
			}
		})
	}))
	for i := 0; i < 3; i++ {
		e.Send(p, i, nil)
	}
	<-started // one message consumed, two still queued
	e.Stop(p)
	close(gate)
	select {
	case n := <-seen:
		if n != 0 {
			t.Fatalf("QueueLen while stopping = %d, want 0", n)
		}
	case <-time.After(time.Second):
		t.Fatal("Stopping was not delivered")
	}
}
