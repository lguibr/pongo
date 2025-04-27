// File: test/e2e_test.go
package test

import (
	"fmt"
	"io" // Import io for EOF check
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	// Adjust import paths
	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/server"
	"github.com/lguibr/pongo/utils"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/websocket"
)

const e2eTestTimeout = 10 * time.Second // Generous timeout for E2E test

// Helper to read JSON messages with timeout
func readWsJSONMessage(t *testing.T, ws *websocket.Conn, timeout time.Duration, v interface{}) error {
	t.Helper()
	readDone := make(chan error, 1)

	go func() {
		_ = ws.SetReadDeadline(time.Now().Add(timeout))
		err := websocket.JSON.Receive(ws, v)
		_ = ws.SetReadDeadline(time.Time{})
		readDone <- err
	}()

	select {
	case err := <-readDone:
		return err
	case <-time.After(timeout + 100*time.Millisecond):
		return fmt.Errorf("websocket read timeout after %v", timeout)
	}
}

// Renamed back to original name, includes move check
func TestE2E_SinglePlayerConnectMoveDisconnect(t *testing.T) {
	// 1. Setup Engine and GameActor
	engine := bollywood.NewEngine()
	defer engine.Shutdown(e2eTestTimeout / 2)

	cfg := utils.DefaultConfig()

	gameActorPID := engine.Spawn(bollywood.NewProps(game.NewGameActorProducer(engine, cfg)))
	assert.NotNil(t, gameActorPID)
	time.Sleep(100 * time.Millisecond)

	// 2. Setup Test Server
	testServer := server.New(engine, gameActorPID)
	s := httptest.NewServer(websocket.Handler(testServer.HandleSubscribe()))
	defer s.Close()
	wsURL := "ws" + strings.TrimPrefix(s.URL, "http")

	// 3. Connect WebSocket Client
	origin := "http://localhost/"
	ws, err := websocket.Dial(wsURL, "", origin)
	assert.NoError(t, err, "WebSocket dial should succeed")
	if err != nil {
		t.FailNow()
	}
	defer ws.Close()

	// 4. Wait for Initial Game State (Player 0 and Paddle 0 ready)
	fmt.Println("E2E Test: Waiting for initial game state...")
	initialStateReceived := false
	var initialState game.GameState
	initialReadTimeout := 4 * time.Second
	overallDeadline := time.Now().Add(initialReadTimeout)

	for time.Now().Before(overallDeadline) && !initialStateReceived {
		err := readWsJSONMessage(t, ws, initialReadTimeout/2, &initialState)

		if err == nil {
			if initialState.Players[0] != nil && initialState.Paddles[0] != nil {
				fmt.Printf("E2E Test: Received initial state with Player 0 (Score: %d, PaddleY: %d)\n", initialState.Players[0].Score, initialState.Paddles[0].Y)
				initialStateReceived = true
				break
			}
		} else {
			fmt.Printf("E2E Test: Error reading/parsing initial state: %v\n", err)
			if err == io.EOF || strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "reset by peer") {
				t.Logf("Connection closed while waiting for initial state.")
				break
			}
		}
		if !initialStateReceived {
			time.Sleep(cfg.GameTickPeriod * 2)
		}
	}
	assert.True(t, initialStateReceived, "Should receive initial game state with Player 0 and Paddle 0")
	if !initialStateReceived {
		t.FailNow()
	}
	initialPaddleY := initialState.Paddles[0].Y

	// 5. Send Input (Move Right -> Down for Player 0)
	fmt.Println("E2E Test: Sending 'ArrowRight' input...")
	directionCmd := game.Direction{Direction: "ArrowRight"}
	err = websocket.JSON.Send(ws, directionCmd)
	assert.NoError(t, err, "Should send direction without error")

	// 6. Wait for Updated Game State showing movement
	fmt.Println("E2E Test: Waiting for updated game state after move...")
	moveStateReceived := false
	var movedState game.GameState
	moveDeadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(moveDeadline) {
		err := readWsJSONMessage(t, ws, 500*time.Millisecond, &movedState)

		if err == nil {
			if movedState.Paddles[0] != nil && movedState.Paddles[0].Y > initialPaddleY {
				fmt.Printf("E2E Test: Received updated state with Player 0 paddle moved (Y: %d -> %d)\n", initialPaddleY, movedState.Paddles[0].Y)
				moveStateReceived = true
				break
			}
		} else {
			fmt.Printf("E2E Test: Error reading state while waiting for move: %v\n", err)
			if err == io.EOF || strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "reset by peer") {
				t.Logf("Connection closed while waiting for move state.")
				break
			}
		}
		time.Sleep(cfg.GameTickPeriod * 2)
	}
	assert.True(t, moveStateReceived, "Should receive game state with paddle 0 moved down")
	if !moveStateReceived {
		t.FailNow()
	}

	// 7. Send Stop Input
	fmt.Println("E2E Test: Sending 'Stop' input...")
	stopCmd := game.Direction{Direction: "Stop"}
	err = websocket.JSON.Send(ws, stopCmd)
	assert.NoError(t, err, "Should send stop direction without error")

	// 8. Wait for Updated Game State showing stopped state (IsMoving == false)
	fmt.Println("E2E Test: Waiting for updated game state after stop...")
	stopStateReceived := false
	var stoppedState game.GameState
	stopDeadline := time.Now().Add(4 * time.Second)

	for time.Now().Before(stopDeadline) {
		err := readWsJSONMessage(t, ws, 500*time.Millisecond, &stoppedState)

		if err == nil {
			if stoppedState.Paddles[0] != nil {
				// *** ADD LOGGING ***
				fmt.Printf("E2E Test: Checking received state after stop: PaddleY=%d, Dir='%s', Vx=%d, Vy=%d, IsMoving=%t\n",
					stoppedState.Paddles[0].Y, stoppedState.Paddles[0].Direction, stoppedState.Paddles[0].Vx, stoppedState.Paddles[0].Vy, stoppedState.Paddles[0].IsMoving)

				if !stoppedState.Paddles[0].IsMoving {
					fmt.Printf("E2E Test: Received updated state with Player 0 paddle stopped (Y: %d, IsMoving: %t)\n",
						stoppedState.Paddles[0].Y, stoppedState.Paddles[0].IsMoving)
					assert.Equal(t, 0, stoppedState.Paddles[0].Vx, "Vx should be 0 when IsMoving is false")
					assert.Equal(t, 0, stoppedState.Paddles[0].Vy, "Vy should be 0 when IsMoving is false")
					stopStateReceived = true
					break
				}
			} else {
				fmt.Println("E2E Test: Checking received state after stop: Paddle 0 is nil")
			}
		} else {
			fmt.Printf("E2E Test: Error reading state while waiting for stop: %v\n", err)
			if err == io.EOF || strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "reset by peer") {
				t.Logf("Connection closed while waiting for stop state.")
				break
			}
		}
		time.Sleep(cfg.GameTickPeriod)
	}
	assert.True(t, stopStateReceived, "Should receive game state with paddle 0 stopped (IsMoving == false)")

	// 9. Disconnect Client
	fmt.Println("E2E Test: Closing client connection...")
	err = ws.Close()
	if err != nil {
		t.Logf("Note: ws.Close() returned error (expected if server closed first): %v", err)
	}

	// 10. Wait briefly for Server to Process Disconnect (allow logs to flush)
	time.Sleep(200 * time.Millisecond)
	fmt.Println("E2E Test: Finished.")
}
