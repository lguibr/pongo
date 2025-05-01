// File: test/e2e_test.go
package test

import (
	"errors"
	"fmt"
	"io"

	// "net" // No longer needed directly here
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/server"
	"github.com/lguibr/pongo/utils"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/websocket"
)

const e2eTestTimeout = 20 * time.Second // Keep increased timeout

// Helper function to wait for a specific game state condition
func waitForStateCondition(t *testing.T, ws *websocket.Conn, timeout time.Duration, condition func(gs game.GameState) bool) (game.GameState, bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastState game.GameState
	for time.Now().Before(deadline) {
		// Use a shorter timeout for each individual read attempt
		err := ReadWsJSONMessage(t, ws, 1*time.Second, &lastState) // Use capitalized name
		if err == nil {
			if condition(lastState) {
				return lastState, true // Condition met
			}
			// Condition not met, continue loop
		} else {
			// Handle read errors
			if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "reset by peer") || strings.Contains(err.Error(), "timeout") {
				t.Logf("Connection closed or timed out while waiting for state condition: %v", err)
				return lastState, false // Return last known state and failure
			}
			// Log other errors but continue trying
			t.Logf("Error reading state while waiting for condition: %v", err)
		}
		// Small delay before next attempt to avoid busy-waiting
		time.Sleep(50 * time.Millisecond)
	}
	t.Logf("Timeout waiting for state condition after %v", timeout)
	return lastState, false // Timeout reached
}

func TestE2E_SinglePlayerConnectMoveStopDisconnect(t *testing.T) {
	// 1. Setup Engine, RoomManager, and Config
	engine := bollywood.NewEngine()
	defer engine.Shutdown(e2eTestTimeout / 2)
	cfg := utils.DefaultConfig()
	roomManagerPID := engine.Spawn(bollywood.NewProps(game.NewRoomManagerProducer(engine, cfg)))
	assert.NotNil(t, roomManagerPID)
	time.Sleep(100 * time.Millisecond) // Allow manager to start

	// 2. Setup Test Server
	testServer := server.New(engine, roomManagerPID)
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

	// 4. Wait for Initial Game State (Player 0 assigned and ready)
	fmt.Println("E2E Test: Waiting for initial game state...")
	initialState, ok := waitForStateCondition(t, ws, 10*time.Second, func(gs game.GameState) bool {
		// Check if Player 0 exists and has a paddle
		return gs.Players[0] != nil && gs.Paddles[0] != nil
	})
	assert.True(t, ok, "Should receive initial game state with Player 0 and Paddle 0 within timeout")
	if !ok {
		t.FailNow() // Cannot proceed without initial state
	}
	fmt.Printf("E2E Test: Received initial state with Player 0 (Score: %d, PaddleY: %d)\n", initialState.Players[0].Score, initialState.Paddles[0].Y)
	initialPaddleY := initialState.Paddles[0].Y

	// 5. Send Input (Move Right -> Down for Player 0)
	fmt.Println("E2E Test: Sending 'ArrowRight' input...")
	directionCmd := game.Direction{Direction: "ArrowRight"}
	err = websocket.JSON.Send(ws, directionCmd)
	assert.NoError(t, err, "Should send direction without error")

	// 6. Wait for Updated Game State showing movement
	fmt.Println("E2E Test: Waiting for updated game state after move...")
	movedState, ok := waitForStateCondition(t, ws, 10*time.Second, func(gs game.GameState) bool {
		if gs.Paddles[0] != nil {
			currentY := gs.Paddles[0].Y
			currentIsMoving := gs.Paddles[0].IsMoving
			// fmt.Printf("E2E Test (Move Check): Received state - Paddle Y: %d, IsMoving: %t\n", currentY, currentIsMoving) // Reduce log noise
			// Check if paddle actually moved DOWN (Y increases for player 0) and IsMoving is true
			return currentY > initialPaddleY && currentIsMoving
		}
		return false
	})
	assert.True(t, ok, "Should receive game state with paddle 0 moved down and IsMoving=true")
	if !ok {
		fmt.Println("WARN: E2E Test did not detect paddle movement.")
	} else {
		fmt.Printf("E2E Test: SUCCESS - Detected paddle moved down (Y: %d -> %d) and IsMoving=true\n", initialPaddleY, movedState.Paddles[0].Y)
	}

	// 7. Send Stop Input
	fmt.Println("E2E Test: Sending 'Stop' input...")
	stopCmd := game.Direction{Direction: "Stop"}
	err = websocket.JSON.Send(ws, stopCmd)
	assert.NoError(t, err, "Should send stop direction without error")

	// 8. Wait for Updated Game State showing stopped state (IsMoving == false)
	fmt.Println("E2E Test: Waiting for updated game state after stop...")
	stoppedState, ok := waitForStateCondition(t, ws, 10*time.Second, func(gs game.GameState) bool {
		if gs.Paddles[0] != nil {
			currentIsMoving := gs.Paddles[0].IsMoving
			// fmt.Printf("E2E Test (Stop Check): Received state - Paddle Y: %d, IsMoving: %t\n", gs.Paddles[0].Y, currentIsMoving) // Reduce log noise
			// Check the IsMoving flag specifically
			return !currentIsMoving
		}
		return false
	})
	assert.True(t, ok, "Should receive game state with paddle 0 stopped (IsMoving == false)")
	if !ok {
		fmt.Println("WARN: E2E Test did not detect paddle stopped state.")
	} else {
		fmt.Printf("E2E Test: SUCCESS - Received updated state with Player 0 paddle stopped (Y: %d, IsMoving: %t)\n",
			stoppedState.Paddles[0].Y, stoppedState.Paddles[0].IsMoving)
		assert.Equal(t, 0, stoppedState.Paddles[0].Vx, "Vx should be 0 when IsMoving is false")
		assert.Equal(t, 0, stoppedState.Paddles[0].Vy, "Vy should be 0 when IsMoving is false")
	}

	// 9. Disconnect Client by closing the WebSocket
	fmt.Println("E2E Test: Closing client connection...")
	err = ws.Close()
	if err != nil && !strings.Contains(err.Error(), "use of closed network connection") && !strings.Contains(err.Error(), "connection reset by peer") {
		t.Logf("Note: ws.Close() returned error: %v", err)
	}

	// 10. Wait briefly for Server to Process Disconnect (optional)
	time.Sleep(500 * time.Millisecond)
	fmt.Println("E2E Test: Finished.")
}
