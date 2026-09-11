// Package actor implements the in-process room runtime. Internal messages are
// reliable; network ingress uses TrySend to bound queued client work.
package actor

import (
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

var ErrTimeout = errors.New("actor: ask timeout")
var ErrStopped = errors.New("actor: stopped")
var ErrMailboxFull = errors.New("actor: mailbox full")

const IngressLimit = 256

type PID struct{ ID string }

func (p *PID) String() string {
	if p == nil {
		return "<nil>"
	}
	return p.ID
}

type Actor interface{ Receive(Context) }
type Producer func() Actor
type Props struct{ producer Producer }

func NewProps(p Producer) *Props {
	if p == nil {
		panic("nil producer")
	}
	return &Props{p}
}
func (p *Props) Produce() Actor { return p.producer() }

type Started struct{}
type Stopping struct{}
type Stopped struct{}
type Context interface {
	Engine() *Engine
	Self() *PID
	Sender() *PID
	Message() interface{}
	RequestID() string
	Reply(interface{})
}
type envelope struct {
	message interface{}
	sender  *PID
	reply   chan interface{}
}
type context struct {
	e   *Engine
	p   *process
	env envelope
}

func (c *context) Engine() *Engine      { return c.e }
func (c *context) Self() *PID           { return c.p.pid }
func (c *context) Sender() *PID         { return c.env.sender }
func (c *context) Message() interface{} { return c.env.message }
func (c *context) RequestID() string {
	if c.env.reply != nil {
		return "ask"
	}
	return ""
}
func (c *context) Reply(v interface{}) {
	if c.env.reply != nil {
		select {
		case c.env.reply <- v:
		default:
		}
	}
}

type Engine struct {
	mu       sync.RWMutex
	actors   map[string]*process
	next     uint64
	stopping bool
}
type process struct {
	e        *Engine
	pid      *PID
	props    *Props
	mu       sync.Mutex
	queue    []envelope
	head     int
	stopping bool
	wake     chan struct{}
	done     chan struct{}
}

func NewEngine() *Engine { return &Engine{actors: make(map[string]*process)} }
func (e *Engine) Spawn(props *Props) *PID {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stopping || props == nil {
		return nil
	}
	e.next++
	p := &process{e: e, pid: &PID{fmt.Sprintf("actor-%d", e.next)}, props: props, wake: make(chan struct{}, 1), done: make(chan struct{})}
	e.actors[p.pid.ID] = p
	go p.run()
	return p.pid
}
func (e *Engine) find(pid *PID) *process {
	if pid == nil {
		return nil
	}
	e.mu.RLock()
	p := e.actors[pid.ID]
	e.mu.RUnlock()
	return p
}
func (p *process) enqueue(env envelope, limit int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopping || (limit > 0 && len(p.queue)-p.head >= limit) {
		return false
	}
	p.queue = append(p.queue, env)
	select {
	case p.wake <- struct{}{}:
	default:
	}
	return true
}

// Send never waits for another actor. False means the target has stopped.
func (e *Engine) Send(pid *PID, msg interface{}, sender *PID) bool {
	p := e.find(pid)
	return p != nil && p.enqueue(envelope{message: msg, sender: sender}, 0)
}

// TrySend is for untrusted ingress. Callers must close/reject on overload.
func (e *Engine) TrySend(pid *PID, msg interface{}, sender *PID) bool {
	p := e.find(pid)
	return p != nil && p.enqueue(envelope{message: msg, sender: sender}, IngressLimit)
}
func (e *Engine) Ask(pid *PID, msg interface{}, timeout time.Duration) (interface{}, error) {
	p := e.find(pid)
	if p == nil {
		return nil, ErrStopped
	}
	ch := make(chan interface{}, 1)
	if !p.enqueue(envelope{message: msg, reply: ch}, IngressLimit) {
		return nil, ErrMailboxFull
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case v := <-ch:
		return v, nil
	case <-p.done:
		return nil, ErrStopped
	case <-timer.C:
		return nil, ErrTimeout
	}
}
func (e *Engine) Stop(pid *PID) {
	if p := e.find(pid); p != nil {
		p.mu.Lock()
		p.stopping = true
		p.mu.Unlock()
		select {
		case p.wake <- struct{}{}:
		default:
		}
	}
}
func (e *Engine) ActiveCount() int { e.mu.RLock(); defer e.mu.RUnlock(); return len(e.actors) }

// QueueLen reports how many messages wait for the actor; 0 if it has stopped.
func (e *Engine) QueueLen(pid *PID) int {
	p := e.find(pid)
	if p == nil {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.queue) - p.head
}
func (e *Engine) Shutdown(timeout time.Duration) {
	e.mu.Lock()
	e.stopping = true
	ps := make([]*process, 0, len(e.actors))
	for _, p := range e.actors {
		ps = append(ps, p)
	}
	e.mu.Unlock()
	for _, p := range ps {
		e.Stop(p.pid)
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for _, p := range ps {
		select {
		case <-p.done:
		case <-timer.C:
			return
		}
	}
}
func (p *process) run() {
	var a Actor
	invoke := func(env envelope) (panicked bool) {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
				slog.Error("actor panic", "pid", p.pid, "panic", r, "stack", string(debug.Stack()))
				if env.reply != nil {
					select {
					case env.reply <- fmt.Errorf("actor panic: %v", r):
					default:
					}
				}
			}
		}()
		a.Receive(&context{e: p.e, p: p, env: env})
		return
	}
	defer func() {
		if r := recover(); r != nil {
			slog.Error("actor initialization panic", "pid", p.pid, "panic", r)
		}
		p.mu.Lock()
		p.stopping = true
		p.queue = nil
		p.mu.Unlock()
		if a != nil {
			invoke(envelope{message: Stopping{}})
			invoke(envelope{message: Stopped{}})
		}
		p.e.mu.Lock()
		delete(p.e.actors, p.pid.ID)
		p.e.mu.Unlock()
		close(p.done)
	}()
	a = p.props.Produce()
	if a == nil {
		return
	}
	if invoke(envelope{message: Started{}}) {
		return
	}
	for {
		p.mu.Lock()
		if p.stopping {
			p.mu.Unlock()
			return
		}
		if p.head == len(p.queue) {
			p.queue = p.queue[:0]
			p.head = 0
			p.mu.Unlock()
			<-p.wake
			continue
		}
		env := p.queue[p.head]
		p.queue[p.head] = envelope{}
		p.head++
		// Compact consumed storage before a busy mailbox grows indefinitely.
		if p.head >= 256 && p.head*2 >= len(p.queue) {
			n := copy(p.queue, p.queue[p.head:])
			for i := n; i < len(p.queue); i++ {
				p.queue[i] = envelope{}
			}
			p.queue = p.queue[:n]
			p.head = 0
		}
		p.mu.Unlock()
		if invoke(env) {
			return
		}
	}
}

// PendingTick coalesces timer notifications until the room consumes one.
type PendingTick struct{ pending atomic.Bool }

func (t *PendingTick) Send(e *Engine, p *PID, msg interface{}) {
	if t.pending.CompareAndSwap(false, true) {
		if !e.Send(p, msg, nil) {
			t.pending.Store(false)
		}
	}
}
func (t *PendingTick) Clear() { t.pending.Store(false) }
