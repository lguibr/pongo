package actor

import (
	"sync"
	"testing"
	"time"
)

type receiveFunc func(Context)

func (f receiveFunc) Receive(c Context) { f(c) }
func TestReliableInternalQueueAndBoundedIngress(t *testing.T) {
	e := NewEngine()
	defer e.Shutdown(time.Second)
	gate := make(chan struct{})
	started := make(chan struct{})
	seen := make(chan int, 2048)
	p := e.Spawn(NewProps(func() Actor {
		return receiveFunc(func(c Context) {
			switch m := c.Message().(type) {
			case Started:
				close(started)
				<-gate
			case int:
				seen <- m
			}
		})
	}))
	<-started
	for i := 0; i < 2048; i++ {
		if !e.Send(p, i, nil) {
			t.Fatal("internal send lost")
		}
	}
	if e.TrySend(p, 3000, nil) {
		t.Fatal("ingress should reject a full queue")
	}
	close(gate)
	for i := 0; i < 2048; i++ {
		select {
		case got := <-seen:
			if got != i {
				t.Fatalf("order: %d != %d", got, i)
			}
		case <-time.After(time.Second):
			t.Fatal("message lost")
		}
	}
}
func TestConcurrentStopAndShutdown(t *testing.T) {
	for n := 0; n < 100; n++ {
		e := NewEngine()
		p := e.Spawn(NewProps(func() Actor { return receiveFunc(func(Context) {}) }))
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); e.Stop(p) }()
		}
		wg.Add(1)
		go func() { defer wg.Done(); e.Shutdown(time.Second) }()
		wg.Wait()
		if e.ActiveCount() != 0 {
			t.Fatal("actor leaked")
		}
		if e.Send(p, 1, nil) {
			t.Fatal("send to stopped actor accepted")
		}
	}
}
func TestSpawnShutdownAndAskStop(t *testing.T) {
	e := NewEngine()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); e.Spawn(NewProps(func() Actor { return receiveFunc(func(Context) {}) })) }()
	}
	e.Shutdown(time.Second)
	wg.Wait()
	if e.ActiveCount() != 0 {
		t.Fatal("spawn escaped shutdown")
	}
	e = NewEngine()
	p := e.Spawn(NewProps(func() Actor {
		return receiveFunc(func(c Context) {
			if _, ok := c.Message().(int); ok {
				c.Engine().Stop(c.Self())
			}
		})
	}))
	if _, err := e.Ask(p, 1, time.Second); err != ErrStopped {
		t.Fatalf("ask stop: %v", err)
	}
}
func TestPanicAfterReplyDoesNotBlockCleanup(t *testing.T) {
	e := NewEngine()
	p := e.Spawn(NewProps(func() Actor {
		return receiveFunc(func(c Context) {
			if _, ok := c.Message().(int); ok {
				c.Reply(1)
				panic("after reply")
			}
		})
	}))
	e.Ask(p, 1, time.Second)
	e.Shutdown(time.Second)
	if e.ActiveCount() != 0 {
		t.Fatal("panic cleanup blocked")
	}
}
