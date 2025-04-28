// File: test/e2e_test.go
package test

import (
	// Import json
	"errors" // Import errors
	"fmt"
	"io"
	"net"
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

const e2eTestTimeout = 20 * time.Second // Further increased timeout

// Helper to read JSON messages with timeout
func readWsJSONMessage(t *testing.T, ws *websocket.Conn, timeout time.Duration, v interface{}) error {
	t.Helper()
	readDone := make(chan error, 1)
	var readErr error

	go func() {
		// It's crucial to set deadline *before* Receive
		setReadErr := ws.SetReadDeadline(time.Now().Add(timeout))
		if setReadErr != nil {
			// Check if the error is due to closed connection, which might be expected during shutdown
			if !errors.Is(setReadErr, net.ErrClosed) && !strings.Contains(setReadErr.Error(), "use of closed network connection") {
				readDone <- fmt.Errorf("failed to set read deadline: %w", setReadErr)
				return
			}
			// If connection is already closed, signal EOF or similar
			readDone <- io.EOF
			return
		}
		err := websocket.JSON.Receive(ws, v)
		// Clear deadline immediately after Receive returns
		_ = ws.SetReadDeadline(time.Time{})
		readDone <- err
	}()

	select {
	case readErr = <-readDone:
		return readErr // Return error from Receive (can be nil, io.EOF, etc.)
	case <-time.After(timeout + 500*time.Millisecond): // Slightly longer overall timeout
		// If the select times out, it means the Receive call is blocked indefinitely.
		_ = ws.Close() // Attempt to close to unblock
		return fmt.Errorf("websocket read timeout after %v (Receive call blocked)", timeout)
	}
}

func TestE2E_SinglePlayerConnectMoveStopDisconnect(t *testing.T) {
	// 1. Setup Engine, RoomManager, and Config
	engine := bollywood.NewEngine()
	defer engine.Shutdown(e2eTestTimeout / 2)

	cfg := utils.DefaultConfig()

	// Spawn RoomManager
	roomManagerPID := engine.Spawn(bollywood.NewProps(game.NewRoomManagerProducer(engine, cfg)))
	assert.NotNil(t, roomManagerPID)
	time.Sleep(100 * time.Millisecond) // Allow manager to start

	// 2. Setup Test Server (pointing to RoomManager)
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
	defer ws.Close() // Ensure close happens eventually

	// 4. Wait for Initial Game State (Player 0 assigned and ready)
	fmt.Println("E2E Test: Waiting for initial game state...")
	initialStateReceived := false
	var initialState game.GameState
	initialReadTimeout := 7 * time.Second // Increased timeout
	overallDeadline := time.Now().Add(initialReadTimeout)

	for time.Now().Before(overallDeadline) && !initialStateReceived {
		// Use a shorter timeout per read attempt
		err := readWsJSONMessage(t, ws, 1*time.Second, &initialState)

		if err == nil {
			// Check if Player 0 exists and has a paddle
			if initialState.Players[0] != nil && initialState.Paddles[0] != nil {
				fmt.Printf("E2E Test: Received initial state with Player 0 (Score: %d, PaddleY: %d)\n", initialState.Players[0].Score, initialState.Paddles[0].Y)
				initialStateReceived = true
				break // Exit loop on success
			} else {
				fmt.Println("E2E Test: Received state, but Player 0 or Paddle 0 is nil. Waiting...")
			}
		} else {
			// Handle read errors
			if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "reset by peer") || strings.Contains(err.Error(), "timeout") {
				t.Logf("Connection closed or timed out while waiting for initial state: %v. Exiting wait loop.", err)
				break // Exit loop if connection closed prematurely
			} else {
				fmt.Printf("E2E Test: Error reading/parsing initial state: %v\n", err)
			}
			// Check if deadline exceeded even with errors
			if !time.Now().Before(overallDeadline) {
				break
			}
		}
		// Wait before next read attempt only if no error occurred but state wasn't ready
		if err == nil && !initialStateReceived {
			time.Sleep(cfg.GameTickPeriod * 20) // Increased wait between checks
		} else if err != nil {
			time.Sleep(300 * time.Millisecond) // Shorter wait after error before retry
		}
	}
	assert.True(t, initialStateReceived, "Should receive initial game state with Player 0 and Paddle 0 within timeout")
	if !initialStateReceived {
		t.FailNow() // Cannot proceed without initial state
	}
	initialPaddleY := initialState.Paddles[0].Y

	// 5. Send Input (Move Right -> Down for Player 0)
	fmt.Println("E2E Test: Sending 'ArrowRight' input...")
	directionCmd := game.Direction{Direction: "ArrowRight"}
	err = websocket.JSON.Send(ws, directionCmd)
	assert.NoError(t, err, "Should send direction without error")
	time.Sleep(cfg.GameTickPeriod * 10) // Add longer delay after sending command

	// 6. Wait for Updated Game State showing movement
	fmt.Println("E2E Test: Waiting for updated game state after move...")
	moveStateReceived := false
	var movedState game.GameState
	moveDeadline := time.Now().Add(7 * time.Second) // Increased timeout

	for time.Now().Before(moveDeadline) {
		err := readWsJSONMessage(t, ws, 500*time.Millisecond, &movedState)

		if err == nil {
			if movedState.Paddles[0] != nil {
				currentY := movedState.Paddles[0].Y
				currentIsMoving := movedState.Paddles[0].IsMoving
				fmt.Printf("E2E Test (Move Check): Received state - Paddle Y: %d, IsMoving: %t\n", currentY, currentIsMoving) // Log received state

				// Check if paddle actually moved DOWN (Y increases for player 0) and IsMoving is true
				if currentY > initialPaddleY && currentIsMoving {
					fmt.Printf("E2E Test: SUCCESS - Detected paddle moved down (Y: %d -> %d) and IsMoving=true\n", initialPaddleY, currentY)
					moveStateReceived = true
					break
				}
				// Update last known position (variables removed as they were unused)
			} else {
				fmt.Println("E2E Test (Move Check): Received state, but Paddle 0 is nil.")
			}
		} else {
			fmt.Printf("E2E Test (Move Check): Error reading state: %v\n", err)
			if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "reset by peer") || strings.Contains(err.Error(), "timeout") {
				t.Logf("Connection closed or timed out while waiting for move state.")
				break
			}
		}
		// Wait before next read attempt only if no error occurred but state wasn't ready
		if err == nil && !moveStateReceived {
			time.Sleep(cfg.GameTickPeriod * 15) // Increased wait
		} else if err != nil {
			time.Sleep(200 * time.Millisecond)
		}
	}
	assert.True(t, moveStateReceived, "Should receive game state with paddle 0 moved down and IsMoving=true")
	if !moveStateReceived {
		fmt.Println("WARN: E2E Test did not detect paddle movement.")
	}

	// 7. Send Stop Input
	fmt.Println("E2E Test: Sending 'Stop' input...")
	stopCmd := game.Direction{Direction: "Stop"}
	err = websocket.JSON.Send(ws, stopCmd)
	assert.NoError(t, err, "Should send stop direction without error")
	time.Sleep(cfg.GameTickPeriod * 10) // Add longer delay after sending command

	// 8. Wait for Updated Game State showing stopped state (IsMoving == false)
	fmt.Println("E2E Test: Waiting for updated game state after stop...")
	stopStateReceived := false
	var stoppedState game.GameState
	stopDeadline := time.Now().Add(7 * time.Second) // Increased timeout

	for time.Now().Before(stopDeadline) {
		err := readWsJSONMessage(t, ws, 500*time.Millisecond, &stoppedState)

		if err == nil {
			if stoppedState.Paddles[0] != nil {
				currentY := stoppedState.Paddles[0].Y
				currentIsMoving := stoppedState.Paddles[0].IsMoving
				fmt.Printf("E2E Test (Stop Check): Received state - Paddle Y: %d, IsMoving: %t\n", currentY, currentIsMoving) // Log received state

				// Check the IsMoving flag specifically
				if !currentIsMoving {
					fmt.Printf("E2E Test: SUCCESS - Received updated state with Player 0 paddle stopped (Y: %d, IsMoving: %t)\n",
						currentY, currentIsMoving)
					assert.Equal(t, 0, stoppedState.Paddles[0].Vx, "Vx should be 0 when IsMoving is false")
					assert.Equal(t, 0, stoppedState.Paddles[0].Vy, "Vy should be 0 when IsMoving is false")
					assert.Equal(t, "", stoppedState.Paddles[0].Direction, "Direction should be empty when stopped")
					stopStateReceived = true
					break
				}
			} else {
				fmt.Println("E2E Test (Stop Check): Received state, but Paddle 0 is nil.")
			}
		} else {
			fmt.Printf("E2E Test (Stop Check): Error reading state: %v\n", err)
			if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "reset by peer") || strings.Contains(err.Error(), "timeout") {
				t.Logf("Connection closed or timed out while waiting for stop state.")
				break
			}
		}
		// Wait before next read attempt only if no error occurred but state wasn't ready
		if err == nil && !stopStateReceived {
			time.Sleep(cfg.GameTickPeriod * 15) // Increased wait
		} else if err != nil {
			time.Sleep(200 * time.Millisecond)
		}
	}
	assert.True(t, stopStateReceived, "Should receive game state with paddle 0 stopped (IsMoving == false)")
	if !stopStateReceived {
		fmt.Println("WARN: E2E Test did not detect paddle stopped state.")
	}

	// 9. Disconnect Client by closing the WebSocket
	fmt.Println("E2E Test: Closing client connection...")
	err = ws.Close()
	if err != nil && !strings.Contains(err.Error(), "use of closed network connection") && !strings.Contains(err.Error(), "connection reset by peer") { // Ignore specific errors on close
		t.Logf("Note: ws.Close() returned error: %v", err)
	}

	// 10. Wait briefly for Server (RoomManager/GameActor) to Process Disconnect
	time.Sleep(1 * time.Second) // Increased wait slightly more
	fmt.Println("E2E Test: Finished.")
}
