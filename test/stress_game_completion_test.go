package test

import (
	"fmt"
	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
	"sync"
	"testing"
	"time"
)

// Empty arenas end on their first physics tick. This deterministically tests
// concurrent final-message delivery and actor cleanup rather than random physics.
func TestE2E_StressTestGameCompletion(t *testing.T) {
	if testing.Short() {
		t.Skip("concurrent completion test")
	}
	cfg := utils.DefaultConfig()
	cfg.GridFillDensity = 0
	setup := SetupE2ETest(t, cfg)
	defer TeardownE2ETest(t, setup, 3*time.Second)
	const rooms = 40
	var clients []*websocket.Conn
	defer func() {
		for _, c := range clients {
			_ = c.Close()
		}
	}()
	for i := 0; i < rooms; i++ {
		c, err := websocket.Dial(setup.WsURL, "", setup.Origin)
		if err != nil {
			t.Fatal(err)
		}
		clients = append(clients, c)
		if err := websocket.JSON.Send(c, game.CreateRoomRequest{MessageType: "createRoom", SessionID: fmt.Sprintf("completion-%d", i)}); err != nil {
			t.Fatal(err)
		}
		var response game.RoomCreatedResponse
		if err := ReadWsJSONMessage(t, c, 3*time.Second, &response); err != nil || response.MessageType != "roomCreated" {
			t.Fatalf("create: %v %+v", err, response)
		}
		var assignment game.PlayerAssignmentMessage
		if err := ReadWsJSONMessage(t, c, 3*time.Second, &assignment); err != nil || assignment.MessageType != "playerAssignment" {
			t.Fatalf("assignment: %v", err)
		}
	}
	var wg sync.WaitGroup
	results := make(chan error, rooms)
	for _, c := range clients {
		wg.Add(1)
		go func(ws *websocket.Conn) {
			defer wg.Done()
			if err := websocket.JSON.Send(ws, game.PlayerReadyRequest{MessageType: "playerReady", IsReady: true}); err != nil {
				results <- err
				return
			}
			if err := ws.SetReadDeadline(time.Now().Add(6 * time.Second)); err != nil {
				results <- err
				return
			}
			for {
				var h game.GameOverMessage
				if err := websocket.JSON.Receive(ws, &h); err != nil {
					results <- err
					return
				}
				if h.MessageType == "gameOver" {
					results <- nil
					return
				}
			}
		}(c)
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Errorf("final message: %v", err)
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for setup.Engine.ActiveCount() != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if n := setup.Engine.ActiveCount(); n != 1 {
		t.Fatalf("actors remaining after %d room completions: %d (want only manager)", rooms, n)
	}
}
