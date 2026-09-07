package transport

import (
	"golang.org/x/net/websocket"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func socketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	accepted := make(chan *websocket.Conn, 1)
	done := make(chan struct{})
	s := httptest.NewServer(websocket.Handler(func(c *websocket.Conn) { accepted <- c; <-done }))
	ws, err := websocket.Dial("ws"+strings.TrimPrefix(s.URL, "http"), "", s.URL)
	if err != nil {
		t.Fatal(err)
	}
	server := <-accepted
	t.Cleanup(func() { server.Close(); ws.Close(); close(done); s.Close() })
	return server, ws
}
func TestQueueOverflowClosesConnection(t *testing.T) {
	s, _ := socketPair(t)
	c := &Client{Conn: s, queue: make(chan frame, QueueSize), done: make(chan struct{})}
	for i := 0; i < QueueSize; i++ {
		if c.SendBytes([]byte(`{}`)) != nil {
			t.Fatal("early overflow")
		}
	}
	if c.SendBytes([]byte(`{}`)) == nil {
		t.Fatal("overflow silently accepted")
	}
	select {
	case <-c.Done():
	default:
		t.Fatal("slow client not closed")
	}
}
func TestSlowWriterDoesNotDelayOtherClient(t *testing.T) {
	s, _ := socketPair(t)
	slow := New(s)
	defer slow.Close()
	if err := slow.SendBytes([]byte(strings.Repeat("x", 16<<20))); err != nil {
		t.Fatal(err)
	}
	fastServer, fastReader := socketPair(t)
	fast := New(fastServer)
	defer fast.Close()
	fast.SendBytes([]byte(`{"ok":true}`))
	fastReader.SetReadDeadline(time.Now().Add(time.Second))
	var raw string
	if err := websocket.Message.Receive(fastReader, &raw); err != nil {
		t.Fatal(err)
	}
	if raw != `{"ok":true}` {
		t.Fatal("invalid frame")
	}
	select {
	case <-slow.Done():
	case <-time.After(WriteTimeout + time.Second):
		t.Fatal("write deadline did not close slow client")
	}
}
func TestFinishPreservesOrderThenCloses(t *testing.T) {
	s, r := socketPair(t)
	c := New(s)
	defer c.Close()
	c.SendBytes([]byte(`{"first":1}`))
	c.Finish(map[string]int{"last": 2})
	r.SetReadDeadline(time.Now().Add(time.Second))
	for _, expected := range []string{`{"first":1}`, `{"last":2}`} {
		var raw string
		if err := websocket.Message.Receive(r, &raw); err != nil || raw != expected {
			t.Fatalf("got %q err=%v", raw, err)
		}
	}
	select {
	case <-c.Done():
	case <-time.After(time.Second):
		t.Fatal("final write did not close")
	}
}

func TestOverflowDuringBlockedWriteReturnsPromptly(t *testing.T) {
	s, _ := socketPair(t)
	c := New(s)
	defer c.Close()
	payload := []byte(strings.Repeat("x", 16<<20))
	if err := c.SendBytes(payload); err != nil {
		t.Fatal(err)
	}
	// Give the writer a chance to acquire the socket's write mutex.
	time.Sleep(20 * time.Millisecond)
	start := time.Now()
	rejected := false
	for i := 0; i < QueueSize+2; i++ {
		if c.SendBytes([]byte(`{}`)) != nil {
			rejected = true
			break
		}
	}
	if !rejected {
		t.Fatal("did not fill queue")
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("overflow blocked broadcaster for %v", elapsed)
	}
}
