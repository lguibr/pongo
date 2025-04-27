// File: test/e2e_test.go
package test

import (
	"encoding/json"
	"fmt"
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

	// --- Send an initial dummy message ---
	fmt.Println("E2E Test: Sending initial dummy message...")
	dummyPayload, _ := json.Marshal(map[string]string{"action": "ping"})
	_, err = ws.Write(dummyPayload)
	assert.NoError(t, err, "Should send dummy message without error")

	// 4. Wait for Initial Game State
	fmt.Println("E2E Test: Waiting for initial game state...")
	initialStateReceived := false
	var initialState game.GameState
	initialReadTimeout := 4 * time.Second
	readDeadline := time.Now().Add(initialReadTimeout * 2)

	for time.Now().Before(readDeadline) && !initialStateReceived {
		err := readWsJSONMessage(t, ws, initialReadTimeout, &initialState)

		if err == nil {
			if initialState.Canvas != nil && initialState.Players[0] != nil && initialState.Paddles[0] != nil { // Ensure paddle exists too
				fmt.Printf("E2E Test: Received initial state with Player 0 (Score: %d, PaddleY: %d)\n", initialState.Players[0].Score, initialState.Paddles[0].Y)
				initialStateReceived = true
			} else {
				fmt.Printf("E2E Test: Received valid JSON, but Player 0 or Paddle 0 not ready yet.\n")
			}
		} else {
			fmt.Printf("E2E Test: Error reading/parsing initial state: %v\n", err)
			if err.Error() == "EOF" || strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "reset by peer") {
				break
			}
		}
		if !initialStateReceived {
			time.Sleep(cfg.GameTickPeriod * 10)
		}
	}
	assert.True(t, initialStateReceived, "Should receive initial game state with Player 0 and Paddle 0")
	if !initialStateReceived {
		t.FailNow()
	}
	initialPaddleY := initialState.Paddles[0].Y

	// 5. Send Input (Move Right -> Down for Player 0)
	fmt.Println("E2E Test: Sending 'ArrowRight' input...")
	directionPayload, _ := json.Marshal(game.Direction{Direction: "ArrowRight"})
	_, err = ws.Write(directionPayload)
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
			if err.Error() == "EOF" || strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "reset by peer") {
				break
			}
		}
		time.Sleep(cfg.GameTickPeriod * 2)
	}
	assert.True(t, moveStateReceived, "Should receive game state with paddle 0 moved down")
	if !moveStateReceived {
		t.FailNow() // Fail if movement wasn't detected
	}

	// 7. Send Stop Input
	fmt.Println("E2E Test: Sending 'Stop' input...")
	stopPayload, _ := json.Marshal(game.Direction{Direction: "Stop"})
	_, err = ws.Write(stopPayload)
	assert.NoError(t, err, "Should send stop direction without error")

	// 8. Wait for Updated Game State showing stopped state
	fmt.Println("E2E Test: Waiting for updated game state after stop...")
	stopStateReceived := false
	var stoppedState game.GameState
	// Increase timeout slightly and wait longer between checks
	stopDeadline := time.Now().Add(5 * time.Second)
	lastMovedY := movedState.Paddles[0].Y

	for time.Now().Before(stopDeadline) {
		err := readWsJSONMessage(t, ws, 1000*time.Millisecond, &stoppedState) // Longer read timeout
		if err == nil {
			// Log the received state for debugging
			if stoppedState.Paddles[0] != nil {
				fmt.Printf("E2E Test: Checking received state after stop: PaddleY=%d, Dir='%s', Vx=%d, Vy=%d\n",
					stoppedState.Paddles[0].Y, stoppedState.Paddles[0].Direction, stoppedState.Paddles[0].Vx, stoppedState.Paddles[0].Vy)
			} else {
				fmt.Println("E2E Test: Checking received state after stop: Paddle 0 is nil")
			}

			// Check if paddle exists and its velocity is zero (more reliable than direction string sometimes)
			if stoppedState.Paddles[0] != nil && stoppedState.Paddles[0].Vx == 0 && stoppedState.Paddles[0].Vy == 0 {
				// Optionally, also check Y hasn't changed drastically from last known moving position
				if stoppedState.Paddles[0].Y == lastMovedY {
					fmt.Printf("E2E Test: Received updated state with Player 0 paddle stopped (Y: %d, Vx: %d, Vy: %d)\n",
						stoppedState.Paddles[0].Y, stoppedState.Paddles[0].Vx, stoppedState.Paddles[0].Vy)
					stopStateReceived = true
					break
				} else {
					fmt.Printf("E2E Test: Paddle stopped (Vx/Vy=0) but Y changed (%d -> %d). Waiting for stable state.\n", lastMovedY, stoppedState.Paddles[0].Y)
					lastMovedY = stoppedState.Paddles[0].Y // Update last known Y
				}
			}
		} else {
			fmt.Printf("E2E Test: Error reading state while waiting for stop: %v\n", err)
			if err.Error() == "EOF" || strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "reset by peer") {
				break
			}
		}
		// Wait longer between checks
		time.Sleep(cfg.GameTickPeriod * 5)
	}
	// Use assert.True here - the loop should break when condition is met
	assert.True(t, stopStateReceived, "Should receive game state with paddle 0 stopped (Vx=0, Vy=0)")

	// 9. Disconnect Client
	fmt.Println("E2E Test: Closing client connection...")
	err = ws.Close()
	// Ignore close errors as the connection might already be closed by the server side disconnect logic triggered by read errors
	// assert.NoError(t, err, "Should close client connection without error")

	// 10. Wait for Server to Process Disconnect
	time.Sleep(500 * time.Millisecond)
	fmt.Println("E2E Test: Finished.")
}
