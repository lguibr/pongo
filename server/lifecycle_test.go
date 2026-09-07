package server

import (
	"encoding/json"
	"fmt"
	"github.com/lguibr/pongo/game"
	bollywood "github.com/lguibr/pongo/internal/actor"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newLiveServer(t *testing.T) (*bollywood.Engine, *bollywood.PID, func() *websocket.Conn) {
	t.Helper()
	e := bollywood.NewEngine()
	cfg := utils.DefaultConfig()
	cfg.PowerUpChance = 0
	cfg.GridBrickMinLife = 10000
	cfg.GridBrickMaxLife = 10000
	p := e.Spawn(bollywood.NewProps(game.NewRoomManagerProducer(e, cfg)))
	s := httptest.NewServer(websocket.Handler(New(e, p).HandleSubscribe()))
	var conns []*websocket.Conn
	t.Cleanup(func() {
		for _, c := range conns {
			c.Close()
		}
		e.Shutdown(3 * time.Second)
		s.Close()
		if e.ActiveCount() != 0 {
			t.Errorf("leaked %d actors", e.ActiveCount())
		}
	})
	return e, p, func() *websocket.Conn {
		c, err := websocket.Dial("ws"+strings.TrimPrefix(s.URL, "http"), "", s.URL)
		if err != nil {
			t.Fatal(err)
		}
		conns = append(conns, c)
		return c
	}
}
func sendJSON(t *testing.T, c *websocket.Conn, v interface{}) {
	t.Helper()
	if err := websocket.JSON.Send(c, v); err != nil {
		t.Fatal(err)
	}
}
func readUntil(t *testing.T, c *websocket.Conn, kind string) map[string]interface{} {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	defer c.SetReadDeadline(time.Time{})
	for {
		var m map[string]interface{}
		if err := websocket.JSON.Receive(c, &m); err != nil {
			t.Fatalf("waiting for %s: %v", kind, err)
		}
		if m["messageType"] == kind {
			return m
		}
	}
}
func roomCount(t *testing.T, e *bollywood.Engine, p *bollywood.PID) (int, int) {
	t.Helper()
	v, err := e.Ask(p, game.GetRoomListRequest{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	rooms := v.(game.RoomListResponse).Rooms
	players := 0
	for _, n := range rooms {
		players += n
	}
	return len(rooms), players
}
func TestQuickPlayStartsAndRepeatedAdmissionIsIdempotent(t *testing.T) {
	e, p, dial := newLiveServer(t)
	c := dial()
	sendJSON(t, c, game.QuickPlayRequest{MessageType: "quickPlay", SessionID: "one"})
	joined := readUntil(t, c, "roomJoined")
	if joined["success"] != true {
		t.Fatal(joined)
	}
	assigned := readUntil(t, c, "playerAssignment")
	if assigned["phase"] != "playing" {
		t.Fatal("quickplay not started", assigned)
	}
	for i := 0; i < 20; i++ {
		sendJSON(t, c, game.CreateRoomRequest{MessageType: "createRoom", SessionID: "one"})
	}
	// Later input traverses the same ordered connection mailbox.
	sendJSON(t, c, map[string]string{"messageType": "direction", "direction": "ArrowRight"})
	for i := 0; i < 10; i++ {
		batch := readUntil(t, c, "gameUpdates")
		b, _ := json.Marshal(batch)
		if strings.Contains(string(b), `"isMoving":true`) {
			break
		}
		if i == 9 {
			t.Fatal("input did not move paddle")
		}
	}
	if rooms, players := roomCount(t, e, p); rooms != 1 || players != 1 {
		t.Fatalf("repeated admission changed counts: %d/%d", rooms, players)
	}
}
func TestRoomAdmissionCapacityAndReconnect(t *testing.T) {
	e, p, dial := newLiveServer(t)
	first := dial()
	sendJSON(t, first, game.CreateRoomRequest{MessageType: "createRoom", SessionID: "p0"})
	code := readUntil(t, first, "roomCreated")["code"].(string)
	assignment := readUntil(t, first, "playerAssignment")
	for i := 1; i < 4; i++ {
		c := dial()
		sendJSON(t, c, game.JoinRoomRequest{MessageType: "joinRoom", Code: code, SessionID: fmt.Sprintf("p%d", i)})
		readUntil(t, c, "playerAssignment")
	}
	extra := dial()
	sendJSON(t, extra, game.JoinRoomRequest{MessageType: "joinRoom", Code: code, SessionID: "extra"})
	r := readUntil(t, extra, "roomJoined")
	if r["success"] != false {
		t.Fatal("fifth player admitted")
	}
	first.Close()
	time.Sleep(30 * time.Millisecond)
	reconnect := dial()
	sendJSON(t, reconnect, game.JoinRoomRequest{MessageType: "joinRoom", Code: code, SessionID: "p0"})
	r = readUntil(t, reconnect, "playerAssignment")
	if r["playerIndex"] != assignment["playerIndex"] {
		t.Fatal("reconnect changed player slot")
	}
	if rooms, players := roomCount(t, e, p); rooms != 1 || players != 4 {
		t.Fatalf("incorrect reconnect count %d/%d", rooms, players)
	}
}
func TestOversizedInputClosesConnection(t *testing.T) {
	_, _, dial := newLiveServer(t)
	c := dial()
	websocket.Message.Send(c, strings.Repeat("x", maxInputBytes+1))
	c.SetReadDeadline(time.Now().Add(time.Second))
	var raw []byte
	if err := websocket.Message.Receive(c, &raw); err == nil {
		t.Fatal("oversized input accepted")
	}
}

func TestPendingAdmissionTimeoutClosesActiveConnection(t *testing.T) {
	e := bollywood.NewEngine()
	defer e.Shutdown(time.Second)
	mock := &MockRoomManager{}
	p := e.Spawn(bollywood.NewProps(func() bollywood.Actor { return mock }))
	s := httptest.NewServer(websocket.Handler(New(e, p).HandleSubscribe()))
	defer s.Close()
	c, err := websocket.Dial("ws"+strings.TrimPrefix(s.URL, "http"), "", s.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	sendJSON(t, c, game.QuickPlayRequest{MessageType: "quickPlay", SessionID: "timeout"})
	done := make(chan struct{})
	defer close(done)
	go func() {
		tick := time.NewTicker(20 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-tick.C:
				if websocket.JSON.Send(c, map[string]string{"messageType": "direction", "direction": "Stop"}) != nil {
					return
				}
			}
		}
	}()
	c.SetReadDeadline(time.Now().Add(7 * time.Second))
	var raw []byte
	start := time.Now()
	if err := websocket.Message.Receive(c, &raw); err == nil {
		t.Fatal("pending request stayed open")
	}
	if time.Since(start) > 6*time.Second {
		t.Fatal("server did not close admission timeout")
	}
}
