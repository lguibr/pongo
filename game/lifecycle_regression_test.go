package game

import (
	"github.com/lguibr/pongo/internal/actor"
	"github.com/lguibr/pongo/internal/transport"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type localContext struct {
	e   *actor.Engine
	pid *actor.PID
	msg interface{}
}

func (c localContext) Engine() *actor.Engine { return c.e }
func (c localContext) Self() *actor.PID      { return c.pid }
func (c localContext) Sender() *actor.PID    { return nil }
func (c localContext) Message() interface{}  { return c.msg }
func (c localContext) RequestID() string     { return "" }
func (c localContext) Reply(interface{})     {}
func newLocalRoom(t *testing.T) (*GameActor, localContext) {
	e := actor.NewEngine()
	t.Cleanup(func() { e.Shutdown(time.Second) })
	a := NewGameActorProducer(e, utils.DefaultConfig(), nil)().(*GameActor)
	a.selfPID = &actor.PID{ID: "test-room"}
	return a, localContext{e: e, pid: a.selfPID}
}
func TestGameOverAlwaysCleansChildrenAndTimers(t *testing.T) {
	a, c := newLocalRoom(t)
	a.handleStart(c)
	a.startPhasingTimer(1)
	a.checkGameOver(c) // Empty grid -> gameOver sets isStopping before Stopping.
	if !a.isStopping {
		t.Fatal("did not end game")
	}
	a.handleStopping(c)
	a.handleStopping(c)
	deadline := time.Now().Add(time.Second)
	for a.engine.ActiveCount() != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if a.engine.ActiveCount() != 0 {
		t.Fatal("child actors leaked")
	}
	if len(a.phasingTimers) != 0 {
		t.Fatal("phasing timers leaked")
	}
}
func TestRoomEntitiesDoNotSpawnActors(t *testing.T) {
	a, c := newLocalRoom(t)
	a.handleInternalTestPlayerAdd(c, 0)
	a.spawnBall(c, 0, 300, 300, 0, true, false)
	if len(a.balls) != 1 || a.engine.ActiveCount() != 0 {
		t.Fatal("entities spawned actors")
	}
}
func TestForceStartWaitsForInitializedGrid(t *testing.T) {
	a, c := newLocalRoom(t)
	a.handleForceStartGame(c)
	if a.physicsTicker != nil || !a.forceStartPending || a.gameOver {
		t.Fatal("started uninitialized room")
	}
}
func TestStalePhasingExpiryIsIgnored(t *testing.T) {
	a, c := newLocalRoom(t)
	a.balls[1] = &Ball{Id: 1, Phasing: true}
	a.startPhasingTimer(1)
	a.startPhasingTimer(1)
	defer a.cleanupPhasingTimers()
	c.msg = stopPhasingTimerMsg{BallID: 1, Generation: 1}
	a.Receive(c)
	if !a.balls[1].Phasing {
		t.Fatal("old expiry cancelled refreshed powerup")
	}
	c.msg = stopPhasingTimerMsg{BallID: 1, Generation: 2}
	a.Receive(c)
	if a.balls[1].Phasing {
		t.Fatal("current expiry not applied")
	}
}
func TestFullGridCoordinatesAndUnchangedGridNotRepeated(t *testing.T) {
	a, c := newLocalRoom(t)
	mock := &MockBroadcasterActor{}
	a.broadcasterPID = a.engine.Spawn(actor.NewProps(func() actor.Actor { return mock }))
	a.gridDirty = true
	a.handleBroadcastTick(c)
	a.handleBroadcastTick(c)
	// The mailbox is FIFO, so the reply proves the broadcasts sent before it were handled.
	if _, err := a.engine.Ask(a.broadcasterPID, "barrier", time.Second); err != nil {
		t.Fatal(err)
	}
	grids := 0
	for _, m := range mock.GetMessages() {
		if batch, ok := m.(BroadcastUpdatesCommand); ok {
			for _, u := range batch.Updates {
				if grid, ok := u.(*FullGridUpdate); ok {
					grids++
					if grid.CellSize != 50 || len(grid.Bricks) != 324 || grid.Bricks[0].X != -425 {
						t.Fatal("incorrect grid snapshot")
					}
				}
			}
		}
	}
	if grids != 1 {
		t.Fatalf("grid snapshots=%d", grids)
	}
}

func TestRoomStopCancelsQueuedAdmissions(t *testing.T) {
	e := actor.NewEngine()
	defer e.Shutdown(time.Second)
	gate := make(chan struct{})
	started := make(chan struct{})
	room := e.Spawn(actor.NewProps(func() actor.Actor { return &gatedRoom{gate: gate, started: started} }))
	<-started
	sink := &MockBroadcasterActor{}
	reply := e.Spawn(actor.NewProps(func() actor.Actor { return sink }))
	manager := NewRoomManagerProducer(e, utils.DefaultConfig())().(*RoomManagerActor)
	manager.selfPID = &actor.PID{ID: "manager"}
	manager.pending = make(map[string]pendingAdmission)
	manager.rooms[room.ID] = &RoomInfo{PID: room, Code: "CODE", Sessions: make(map[string]bool)}
	manager.admit(reply, &websocket.Conn{}, &transport.Client{}, "reservation", "CODE", false, false, false)
	if len(manager.pending) != 1 {
		t.Fatal("admission not tracked")
	}
	e.Stop(room)
	close(gate)
	manager.Receive(localContext{e: e, pid: manager.selfPID, msg: GameRoomEmpty{RoomPID: room}})
	if len(manager.pending) != 0 {
		t.Fatal("reservation retained after room stop")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		for _, m := range sink.GetMessages() {
			if r, ok := m.(RoomJoinedResponse); ok && !r.Success {
				return
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("handler never notified that queued admission was cancelled")
}

type gatedRoom struct{ gate, started chan struct{} }

func (g *gatedRoom) Receive(c actor.Context) {
	if _, ok := c.Message().(actor.Started); ok {
		close(g.started)
		<-g.gate
	}
}

func TestConcurrentSessionAdmissionsCannotShareReservation(t *testing.T) {
	e := actor.NewEngine()
	defer e.Shutdown(time.Second)
	gate := make(chan struct{})
	started := make(chan struct{})
	room := e.Spawn(actor.NewProps(func() actor.Actor { return &gatedRoom{gate: gate, started: started} }))
	<-started
	defer close(gate)
	sink := &MockBroadcasterActor{}
	reply := e.Spawn(actor.NewProps(func() actor.Actor { return sink }))
	manager := NewRoomManagerProducer(e, utils.DefaultConfig())().(*RoomManagerActor)
	manager.selfPID = &actor.PID{ID: "manager"}
	manager.pending = make(map[string]pendingAdmission)
	manager.rooms[room.ID] = &RoomInfo{PID: room, Code: "CODE", Sessions: make(map[string]bool)}
	manager.admit(reply, &websocket.Conn{}, &transport.Client{}, "same", "CODE", false, false, false)
	manager.admit(&actor.PID{ID: "second"}, &websocket.Conn{}, &transport.Client{}, "same", "CODE", false, false, false)
	if len(manager.pending) != 1 || manager.rooms[room.ID].PlayerCount != 1 {
		t.Fatal("duplicate session was admitted without a reservation")
	}
	manager.Receive(localContext{e: e, pid: manager.selfPID, msg: AdmissionRejected{RoomPID: room, ReplyTo: reply, SessionID: "same", Reserved: true}})
	if len(manager.rooms) != 0 || len(manager.pending) != 0 {
		t.Fatal("failed pending admission did not release reservation")
	}
}

func gameSocket(t *testing.T) (*websocket.Conn, *transport.Client) {
	t.Helper()
	accepted := make(chan *websocket.Conn, 1)
	done := make(chan struct{})
	s := httptest.NewServer(websocket.Handler(func(c *websocket.Conn) { accepted <- c; <-done }))
	reader, err := websocket.Dial("ws"+strings.TrimPrefix(s.URL, "http"), "", s.URL)
	if err != nil {
		t.Fatal(err)
	}
	ws := <-accepted
	client := transport.New(ws)
	t.Cleanup(func() { client.Close(); _ = reader.Close(); close(done); s.Close() })
	return ws, client
}
func TestCommittedAdmissionFailureReleasesExactlyOnce(t *testing.T) {
	a, c := newLocalRoom(t)
	ws, client := gameSocket(t)
	defer a.performCleanup()
	sink := &MockBroadcasterActor{}
	a.roomManagerPID = a.engine.Spawn(actor.NewProps(func() actor.Actor { return sink }))
	a.handleAdmission(c, AssignPlayerToRoom{WsConn: ws, Client: client, SessionID: "failed-output", Reserved: true, ReplyTo: &actor.PID{ID: "stopped-handler"}})
	if a.players[0] == nil || a.players[0].IsConnected {
		t.Fatal("committed failed connection did not enter grace period")
	}
	a.handleStopReconnectTimerMsg(c, 0)
	a.handleStopReconnectTimerMsg(c, 0)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		accepted, rejected, left := 0, 0, 0
		for _, m := range sink.GetMessages() {
			switch m.(type) {
			case AdmissionAccepted:
				accepted++
			case AdmissionRejected:
				rejected++
			case PlayerLeftRoom:
				left++
			}
		}
		if left == 1 {
			if accepted != 1 || rejected != 0 {
				t.Fatalf("double rollback accepted=%d rejected=%d left=%d", accepted, rejected, left)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("no release notification")
}
func TestCleanupClosesClientBeforeBroadcasterRegistration(t *testing.T) {
	a, c := newLocalRoom(t)
	ws, client := gameSocket(t)
	gate := make(chan struct{})
	started := make(chan struct{})
	a.broadcasterPID = a.engine.Spawn(actor.NewProps(func() actor.Actor { return &gatedRoom{gate: gate, started: started} }))
	<-started
	defer close(gate)
	a.players[0] = &playerInfo{Ws: ws, Client: client, IsConnected: true}
	a.engine.Send(a.broadcasterPID, AddClient{Conn: ws, Client: client}, a.selfPID)
	a.handleStopping(c)
	select {
	case <-client.Done():
	default:
		t.Fatal("unregistered client survived room cleanup")
	}
}
