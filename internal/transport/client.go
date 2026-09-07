// Package transport owns bounded, ordered WebSocket output per connection.
package transport

import (
	"encoding/json"
	"errors"
	"golang.org/x/net/websocket"
	"sync"
	"time"
)

const QueueSize = 64
const WriteTimeout = 2 * time.Second

var ErrClosed = errors.New("connection closed or output queue full")

type frame struct {
	data  string
	final bool
}
type Client struct {
	Conn    *websocket.Conn
	queue   chan frame
	done    chan struct{}
	once    sync.Once
	mu      sync.Mutex
	closing bool
}

func New(conn *websocket.Conn) *Client {
	c := &Client{Conn: conn, queue: make(chan frame, QueueSize), done: make(chan struct{})}
	go c.writeLoop()
	return c
}
func (c *Client) Send(v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.SendBytes(b)
}

// SendBytes takes immutable JSON bytes shared by all clients in one room.
func (c *Client) SendBytes(b []byte) error { return c.SendText(string(b)) }

// SendText shares already-encoded immutable JSON across clients.
func (c *Client) SendText(text string) error { return c.enqueue(frame{data: text}) }
func (c *Client) Finish(v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	err = c.enqueue(frame{data: string(b), final: true})
	if err == nil {
		time.AfterFunc(WriteTimeout, c.Close)
	}
	return err
}
func (c *Client) enqueue(f frame) error {
	c.mu.Lock()
	if c.closing {
		c.mu.Unlock()
		return ErrClosed
	}
	select {
	case <-c.done:
		c.mu.Unlock()
		return ErrClosed
	default:
	}
	if f.final {
		c.closing = true
	}
	select {
	case c.queue <- f:
		c.mu.Unlock()
		return nil
	default:
		c.mu.Unlock()
		c.Close()
		return ErrClosed
	}
}
func (c *Client) Done() <-chan struct{} { return c.done }
func (c *Client) Close() {
	c.once.Do(func() {
		c.mu.Lock()
		close(c.done)
		// Abort blocked IO before the protocol close attempts to acquire its writer lock.
		_ = c.Conn.SetDeadline(time.Now())
		c.mu.Unlock()
		go func() { _ = c.Conn.Close() }()
	})
}
func (c *Client) writeLoop() {
	defer c.Close()
	for {
		select {
		case <-c.done:
			return
		case f := <-c.queue:
			c.mu.Lock()
			select {
			case <-c.done:
				c.mu.Unlock()
				return
			default:
			}
			_ = c.Conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
			c.mu.Unlock()
			if err := websocket.Message.Send(c.Conn, f.data); err != nil {
				return
			}
			if f.final {
				return
			}
		}
	}
}
