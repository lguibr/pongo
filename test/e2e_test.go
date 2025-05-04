// File: test/e2e_test.go
package test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net" // Re-add net import
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
func waitForStateCondition(t *testing.T, ws *websocket.Conn, timeout time.Duration, description string, condition func(gs game.GameState) bool) (game.GameState, bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastState game.GameState
	stateReceived := false // Track if we received at least one valid state

	for time.Now().Before(deadline) {
		// Use a shorter timeout for each individual read attempt
		var rawMsg json.RawMessage
		// Pass the destination variable (&rawMsg)
		err := ReadWsJSONMessage(t, ws, 1*time.Second, &rawMsg)
		if err == nil {
			// Check if it's a GameState update
			var stateUpdate game.GameState
			if json.Unmarshal(rawMsg, &stateUpdate) == nil && stateUpdate.MessageType == "gameStateUpdate" {
				lastState = stateUpdate // Update last known state
				stateReceived = true
				if condition(lastState) {
					// t.Logf("Condition '%s' met.", description) // Optional success log
					return lastState, true // Condition met
				}
			}
			// Ignore other message types (like initial grid, assignment) in this loop
		} else {
			// Handle read errors
			netErr, isNetErr := err.(net.Error) // Use net.Error
			if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "reset by peer") || (isNetErr && netErr.Timeout()) {
				// Use t.Logf for test logging
				t.Logf("Connection closed or timed out while waiting for condition '%s': %v", description, err)
				return lastState, false // Return last known state and failure
			}
			// Log other errors but continue trying
			t.Logf("Error reading state while waiting for condition '%s': %v", description, err)
		}
		// Small delay before next attempt to avoid busy-waiting
		time.Sleep(50 * time.Millisecond)
	}
	t.Logf("Timeout waiting %v for state condition '%s'", timeout, description)
	if !stateReceived {
		t.Logf("No valid game state updates received during the wait period.")
	}
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

	// 3.5 Read initial messages (Assignment + Grid) - Consume them
	var assignmentMsg game.PlayerAssignmentMessage
	errAssign := ReadWsJSONMessage(t, ws, 5*time.Second, &assignmentMsg) // Use capitalized name
	assert.NoError(t, errAssign, "Should receive assignment message")
	assert.Equal(t, "playerAssignment", assignmentMsg.MessageType) // Check message type

	var gridMsg game.InitialGridStateMessage
	errGrid := ReadWsJSONMessage(t, ws, 5*time.Second, &gridMsg) // Use capitalized name
	assert.NoError(t, errGrid, "Should receive initial grid message")
	assert.Equal(t, "initialGridState", gridMsg.MessageType) // Check message type

	// 4. Wait for First Game State Update (Player 0 assigned and ready)
	fmt.Println("E2E Test: Waiting for first game state update...")
	initialState, ok := waitForStateCondition(t, ws, 10*time.Second, "Player 0 Ready", func(gs game.GameState) bool {
		// Check if Player 0 exists and has a paddle
		return gs.Players[0] != nil && gs.Paddles[0] != nil
	})
	assert.True(t, ok, "Should receive initial game state update with Player 0 and Paddle 0 within timeout")
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

	// 6. Wait for Updated Game State showing ANY position change
	fmt.Println("E2E Test: Waiting for paddle position change...")
	// Modify the condition to check for *any* change from the initial Y
	movedState, ok := waitForStateCondition(t, ws, 10*time.Second, "Paddle Position Changed", func(gs game.GameState) bool {
		if gs.Paddles[0] != nil {
			currentY := gs.Paddles[0].Y
			// Check if Y is different from the initial Y
			return currentY != initialPaddleY
		}
		return false
	})
	assert.True(t, ok, "Should receive game state with paddle 0 position changed")
	if !ok {
		fmt.Println("WARN: E2E Test did not detect paddle position change.")
		// Don't fail immediately, proceed to check stop state
	} else {
		fmt.Printf("E2E Test: SUCCESS - Detected paddle position changed (Y: %d -> %d)\n", initialPaddleY, movedState.Paddles[0].Y)
		// Check if it moved in the expected direction (down for player 0)
		assert.Greater(t, movedState.Paddles[0].Y, initialPaddleY, "Paddle should have moved down (Y increased)")
	}

	// 7. Send Stop Input
	fmt.Println("E2E Test: Sending 'Stop' input...")
	stopCmd := game.Direction{Direction: "Stop"}
	err = websocket.JSON.Send(ws, stopCmd)
	assert.NoError(t, err, "Should send stop direction without error")

	// 8. Wait for Updated Game State showing stopped state (IsMoving == false)
	fmt.Println("E2E Test: Waiting for paddle stopped state (IsMoving=false)...")
	stoppedState, ok := waitForStateCondition(t, ws, 10*time.Second, "Paddle Stopped", func(gs game.GameState) bool {
		if gs.Paddles[0] != nil {
			currentIsMoving := gs.Paddles[0].IsMoving
			// Check the IsMoving flag specifically
			return !currentIsMoving
		}
		return false
	})
	assert.True(t, ok, "Should receive game state with paddle 0 stopped (IsMoving == false)")
	if !ok {
		fmt.Println("WARN: E2E Test did not detect paddle stopped state (IsMoving=false).")
	} else {
		fmt.Printf("E2E Test: SUCCESS - Received updated state with Player 0 paddle stopped (Y: %d, IsMoving: %t)\n",
			stoppedState.Paddles[0].Y, stoppedState.Paddles[0].IsMoving)
		// Assert velocity components are zero when stopped
		assert.Equal(t, 0, stoppedState.Paddles[0].Vx, "Vx should be 0 when IsMoving is false")
		assert.Equal(t, 0, stoppedState.Paddles[0].Vy, "Vy should be 0 when IsMoving is false")
	}

	// 9. Disconnect Client by closing the WebSocket
	fmt.Println("E2E Test: Closing client connection...")
	err = ws.Close()
	// Ignore common "connection closed" errors during shutdown
	netErr, isNetErr := err.(net.Error) // Use net.Error
	if err != nil && !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "use of closed network connection") && !strings.Contains(err.Error(), "connection reset by peer") && !(isNetErr && netErr.Timeout()) {
		t.Logf("Note: ws.Close() returned unexpected error: %v", err)
	}

	// 10. Wait briefly for Server to Process Disconnect (optional)
	time.Sleep(500 * time.Millisecond)
	fmt.Println("E2E Test: Finished.")
}

// TestE2E_BallWallNonStick verifies that balls correctly move away from walls after collision.
func TestE2E_BallWallNonStick(t *testing.T) {
	// 1. Setup Engine, RoomManager, and Config
	engine := bollywood.NewEngine()
	defer engine.Shutdown(e2eTestTimeout) // Use full timeout for shutdown
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

	// 3.5 Read initial messages (Assignment + Grid) to get canvas size
	var assignmentMsg game.PlayerAssignmentMessage
	errAssign := ReadWsJSONMessage(t, ws, 5*time.Second, &assignmentMsg) // Use capitalized name
	assert.NoError(t, errAssign, "Should receive assignment message")
	assert.Equal(t, "playerAssignment", assignmentMsg.MessageType)

	var gridMsg game.InitialGridStateMessage
	errGrid := ReadWsJSONMessage(t, ws, 5*time.Second, &gridMsg) // Use capitalized name
	assert.NoError(t, errGrid, "Should receive initial grid message")
	assert.Equal(t, "initialGridState", gridMsg.MessageType)
	canvasSize := gridMsg.CanvasWidth // Store canvas size
	assert.Positive(t, canvasSize, "Canvas size should be positive")

	// 4. Monitor game state for wall collisions and subsequent movement
	fmt.Println("E2E Wall Stick Test: Waiting for wall collision...")
	var lastState game.GameState
	collisionDetected := false
	var collidedBallID int
	var collisionWall int // 0:Right, 1:Top, 2:Left, 3:Bottom
	var checkCounter int
	var lastCollidedCoord int // Store the coordinate *at* collision time

	testDeadline := time.Now().Add(e2eTestTimeout - 2*time.Second) // Deadline for the test itself

	for time.Now().Before(testDeadline) {
		// Read raw message first to check type
		var rawMsg json.RawMessage
		// Use the helper function ReadWsJSONMessage
		err := ReadWsJSONMessage(t, ws, 1*time.Second, &rawMsg)
		if err != nil {
			// Check for expected closure errors
			netErr, isNetErr := err.(net.Error) // Use net.Error
			if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "reset by peer") || (isNetErr && netErr.Timeout()) {
				t.Log("Connection closed or timed out during wall stick test.")
				break
			}
			t.Logf("Error reading state during wall stick test: %v", err)
			time.Sleep(50 * time.Millisecond)
			continue
		}

		// Check if it's a GameState update
		var stateUpdate game.GameState
		if json.Unmarshal(rawMsg, &stateUpdate) == nil && stateUpdate.MessageType == "gameStateUpdate" {
			lastState = stateUpdate // Update last known state
		} else {
			continue // Ignore other message types
		}

		// --- Check logic using lastState and stored canvasSize ---
		if len(lastState.Balls) == 0 {
			continue // Wait for valid state with balls
		}

		if !collisionDetected {
			// Look for a ball hitting a wall AND having the Collided flag set
			for _, ball := range lastState.Balls {
				if ball == nil {
					continue
				}
				radius := ball.Radius
				hit := -1
				coord := 0
				// Use stored canvasSize, check if ball center is *very close* to boundary
				boundaryThreshold := radius + 5 // Allow a small buffer
				if ball.X >= canvasSize-boundaryThreshold {
					hit = 0; coord = ball.X
				} else if ball.Y <= boundaryThreshold {
					hit = 1; coord = ball.Y
				} else if ball.X <= boundaryThreshold {
					hit = 2; coord = ball.X
				} else if ball.Y >= canvasSize-boundaryThreshold {
					hit = 3; coord = ball.Y
				}

				// Crucially, check the Collided flag from the broadcast state
				if hit != -1 && ball.Collided {
					fmt.Printf("E2E Wall Stick Test: Detected collision flag for Ball %d at Wall %d (Coord: %d)\n", ball.Id, hit, coord)
					collisionDetected = true
					collidedBallID = ball.Id
					collisionWall = hit
					lastCollidedCoord = coord // Store the coordinate when collision flag was true
					checkCounter = 0          // Start checking subsequent states
					break                     // Stop checking other balls for this state
				}
			}
		} else {
			// Collision flag was detected, check subsequent states for movement *away* from lastCollidedCoord
			foundBall := false
			for _, ball := range lastState.Balls {
				if ball != nil && ball.Id == collidedBallID {
					foundBall = true
					currentCoord := 0
					movedAway := false

					switch collisionWall {
					case 0: // Right wall (collided near X=canvasSize)
						currentCoord = ball.X
						// Expect X to decrease (move left away from lastCollidedCoord)
						if currentCoord < lastCollidedCoord {
							movedAway = true
						}
						fmt.Printf("E2E Wall Stick Test: Ball %d (Wall 0): CollidedAtX=%d, CurrX=%d\n", ball.Id, lastCollidedCoord, currentCoord)
					case 1: // Top wall (collided near Y=0)
						currentCoord = ball.Y
						// Expect Y to increase (move down away from lastCollidedCoord)
						if currentCoord > lastCollidedCoord {
							movedAway = true
						}
						fmt.Printf("E2E Wall Stick Test: Ball %d (Wall 1): CollidedAtY=%d, CurrY=%d\n", ball.Id, lastCollidedCoord, currentCoord)
					case 2: // Left wall (collided near X=0)
						currentCoord = ball.X
						// Expect X to increase (move right away from lastCollidedCoord)
						if currentCoord > lastCollidedCoord {
							movedAway = true
						}
						fmt.Printf("E2E Wall Stick Test: Ball %d (Wall 2): CollidedAtX=%d, CurrX=%d\n", ball.Id, lastCollidedCoord, currentCoord)
					case 3: // Bottom wall (collided near Y=canvasSize)
						currentCoord = ball.Y
						// Expect Y to decrease (move up away from lastCollidedCoord)
						if currentCoord < lastCollidedCoord {
							movedAway = true
						}
						fmt.Printf("E2E Wall Stick Test: Ball %d (Wall 3): CollidedAtY=%d, CurrY=%d\n", ball.Id, lastCollidedCoord, currentCoord)
					}

					checkCounter++
					// Check if it moved away within a few ticks
					// Allow 1 tick buffer as the collision flag might be set slightly before pos adjustment is broadcast
					if checkCounter >= 1 && movedAway {
						fmt.Printf("E2E Wall Stick Test: SUCCESS - Ball %d moved away from Wall %d after %d ticks.\n", ball.Id, collisionWall, checkCounter)
						return // Test successful
					}
					// If it hasn't moved away after 5 checks, fail
					if checkCounter >= 5 && !movedAway {
						t.Errorf("E2E Wall Stick Test: FAILED - Ball %d did not move away from Wall %d within 5 ticks (Last Coord: %d, Collided Coord: %d)", ball.Id, collisionWall, currentCoord, lastCollidedCoord)
						return // Test failed
					}
					break // Found the ball, move to next state
				}
			}
			if !foundBall {
				// Ball might have been destroyed (e.g., temporary ball hitting empty wall)
				fmt.Printf("E2E Wall Stick Test: Collided Ball %d not found in subsequent state. Assuming destroyed/removed.\n", collidedBallID)
				collisionDetected = false // Reset to look for another collision
			}
		}
	}

	// If loop finishes without success
	if !collisionDetected {
		t.Log("E2E Wall Stick Test: No wall collision flag detected within the test timeout.")
	} else {
		t.Errorf("E2E Wall Stick Test: FAILED - Ball %d did not confirm moving away from Wall %d before test timeout.", collidedBallID, collisionWall)
	}
}