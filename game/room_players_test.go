package game

import (
	"testing"
	"time"

	"github.com/lguibr/pongo/internal/actor"
)

func TestFindSlotPrefersHeldSessionThenFreeSlot(t *testing.T) {
	a, _ := newLocalRoom(t)
	a.players[0] = &playerInfo{Index: 0, SessionID: "a", IsConnected: true}
	a.players[1] = &playerInfo{Index: 1, SessionID: "b"}
	if got := a.findSlot("b"); got != 1 {
		t.Fatalf("returning session got slot %d, want 1", got)
	}
	if got := a.findSlot("c"); got != 2 {
		t.Fatalf("new session got slot %d, want 2", got)
	}
	a.players[2] = &playerInfo{Index: 2, IsConnected: true}
	a.players[3] = &playerInfo{Index: 3, IsConnected: true}
	if got := a.findSlot("c"); got != -1 {
		t.Fatalf("full room gave slot %d", got)
	}
	if got := a.findSlot("b"); got != 1 {
		t.Fatalf("held slot not returned in a full room: %d", got)
	}
}

func TestJoiningScoreAveragesConnectedPlayers(t *testing.T) {
	a, _ := newLocalRoom(t)
	if got := a.joiningScore(0); got != int32(a.cfg.InitialScore) {
		t.Fatalf("empty room score %d", got)
	}
	a.players[0] = &playerInfo{Index: 0, IsConnected: true, Score: 10}
	a.players[1] = &playerInfo{Index: 1, IsConnected: true, Score: 20}
	a.players[2] = &playerInfo{Index: 2, Score: 100} // disconnected players do not count
	if got := a.joiningScore(3); got != 15 {
		t.Fatalf("joining score %d, want 15", got)
	}
}

func TestLobbyStateListsConnectedPlayers(t *testing.T) {
	a, _ := newLocalRoom(t)
	a.players[0] = &playerInfo{Index: 0, IsConnected: true, IsReady: true}
	a.players[2] = &playerInfo{Index: 2}
	state := a.lobbyState()
	if len(state.Players) != 1 || state.Players[0].Index != 0 || !state.Players[0].IsReady {
		t.Fatalf("lobby state %+v", state.Players)
	}
}

func TestDisconnectHoldsSlotUntilGraceExpires(t *testing.T) {
	a, c := newLocalRoom(t)
	defer a.performCleanup()
	manager := &MockBroadcasterActor{}
	a.roomManagerPID = a.engine.Spawn(actor.NewProps(func() actor.Actor { return manager }))
	ws, client := gameSocket(t)
	a.ensureGrid(c)
	a.attachPlayer(0, ws, "keeper", client, 0)
	a.spawnBall(c, 0, 300, 300, 0, true, false)
	a.spawnBall(c, 0, 400, 400, time.Minute, false, false)

	a.handlePlayerDisconnect(c, ws)
	if a.players[0] == nil || a.players[0].IsConnected || a.reconnectTimers[0] == nil {
		t.Fatal("disconnect did not hold the slot for reconnection")
	}
	if _, still := a.connToIndex[ws]; still {
		t.Fatal("disconnected connection still mapped")
	}
	if got := a.findSlot("keeper"); got != 0 {
		t.Fatalf("held slot not offered back to the session: %d", got)
	}

	a.handleStopReconnectTimerMsg(c, 0)
	if a.players[0] != nil || a.paddles[0] != nil {
		t.Fatal("expired grace did not free the slot")
	}
	if len(a.balls) != 1 {
		t.Fatalf("balls after expiry = %d, want only the permanent one", len(a.balls))
	}
	for _, b := range a.balls {
		if !b.IsPermanent || b.OwnerIndex != -1 {
			t.Fatalf("permanent ball not released: %+v", b)
		}
	}
	if a.roomCleanupTimer == nil {
		t.Fatal("empty room did not schedule its close")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		for _, m := range manager.GetMessages() {
			if left, ok := m.(PlayerLeftRoom); ok && left.SessionID == "keeper" {
				return
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("room manager was not told the player left")
}
