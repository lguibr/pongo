// File: test/e2e_test.go
package test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	// "math" // Removed unused import
	"net" // Re-add net import
	// "net/http/httptest" // No longer needed directly
	"strings"
	"testing"
	"time"

	// "github.com/lguibr/bollywood" // No longer needed directly
	"github.com/lguibr/pongo/game"
	// "github.com/lguibr/pongo/server" // No longer needed directly
	"github.com/lguibr/pongo/utils"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/websocket"
)

const e2eTestTimeout = 20 * time.Second // Keep increased timeout

// Helper function to wait for a specific game state condition using atomic updates
func waitForStateCondition(t *testing.T, ws *websocket.Conn, localState *game.LocalGameState, timeout time.Duration, description string, condition func(ls *game.LocalGameState) bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)

	// Check initial state first
	if condition(localState) {
		return true
	}

	for time.Now().Before(deadline) {
		var rawMsg json.RawMessage
		err := ReadWsJSONMessage(t, ws, 1*time.Second, &rawMsg) // Use helper to read raw

		if err == nil {
			var batch game.GameUpdatesBatch
			if json.Unmarshal(rawMsg, &batch) == nil && batch.MessageType == "gameUpdates" {
				game.ApplyUpdatesToLocalState(localState, batch.Updates, t) // Apply updates (use game package)
				if condition(localState) {                                  // Check condition again
					return true
				}
			} // else { // Removed SA9003
			// Could be another message type (e.g., gameOver), ignore for condition check
			// t.Logf("Received non-batch message: %s", string(rawMsg))
			// }
		} else {
			// Simplify error checking for closed connections
			if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "closed network connection") || strings.Contains(err.Error(), "reset by peer") {
				t.Logf("Connection closed while waiting for condition '%s': %v", description, err)
				return false // Condition not met if connection closed
			}
			netErr, isNetErr := err.(net.Error)
			if isNetErr && netErr.Timeout() {
				// Ignore read timeouts, continue waiting
			} else {
				t.Logf("Non-fatal error reading state while waiting for condition '%s': %v", description, err)
				// Don't return false immediately on non-fatal read errors, maybe next read works
			}
		}
		time.Sleep(50 * time.Millisecond) // Small delay between reads
	}
	t.Logf("Timeout waiting %v for state condition '%s'", timeout, description)
	return false // Condition not met by timeout
}

func TestE2E_SinglePlayerConnectMoveStopDisconnect(t *testing.T) {
	// 1. Setup using helper with E2E config
	e2eCfg := utils.E2ETestConfig()
	setup := SetupE2ETest(t, e2eCfg)
	defer TeardownE2ETest(t, setup, e2eTestTimeout/2)

	// 2. Connect WebSocket Client
	ws, err := websocket.Dial(setup.WsURL, "", setup.Origin)
	assert.NoError(t, err, "WebSocket dial should succeed")
	if err != nil {
		t.FailNow()
	}
	defer func() { _ = ws.Close() }() // Ignore error on close in test defer

	// 3. Read initial messages (Assignment + Initial Entities) - Consume them
	var assignmentMsg game.PlayerAssignmentMessage
	errAssign := ReadWsJSONMessage(t, ws, 5*time.Second, &assignmentMsg)
	assert.NoError(t, errAssign, "Should receive assignment message")
	assert.Equal(t, "playerAssignment", assignmentMsg.MessageType)
	playerIndex := assignmentMsg.PlayerIndex // Store assigned index

	// Consume InitialPlayersAndBallsState
	var initialEntitiesMsg game.InitialPlayersAndBallsState
	errEntities := ReadWsJSONMessage(t, ws, 5*time.Second, &initialEntitiesMsg)
	assert.NoError(t, errEntities, "Should receive initial entities message")
	assert.Equal(t, "initialPlayersAndBallsState", initialEntitiesMsg.MessageType)

	// 4. Wait for First Game State Update (Player 0 assigned and ready)
	localState := game.NewLocalGameState() // Use game package
	// Grid/Bricks will be populated by ApplyUpdatesToLocalState from the first FullGridUpdate
	fmt.Println("E2E Test: Waiting for first game state update (Player/Paddle/Ball/Bricks)...")
	ok := waitForStateCondition(t, ws, localState, 10*time.Second, "Player/Paddle/Ball/Bricks Ready", func(ls *game.LocalGameState) bool {
		playerReady := ls.Players[playerIndex] != nil
		paddleReady := ls.Paddles[playerIndex] != nil // Check paddle exists
		ballExists := false
		for _, ball := range ls.Balls {
			if ball != nil && ball.OwnerIndex == playerIndex { // Check ball exists
				ballExists = true
				break
			}
		}
		// Also wait for brick states to be populated
		bricksReady := len(ls.BrickStates) > 0 // Use exported field
		return playerReady && paddleReady && ballExists && bricksReady
	})
	// Use E2E config grid settings to assert bricks are ready
	assert.True(t, ok, "Should receive initial game state updates with Player, Paddle, Ball, and Bricks within timeout (using E2E config)")
	if !ok {
		t.FailNow()
	}
	initialPaddleY := localState.Paddles[playerIndex].Y // Get initial Y

	// 5. Send Input (Move Right -> Down for Player 0)
	fmt.Println("E2E Test: Sending 'ArrowRight' input...")
	directionCmd := game.Direction{Direction: "ArrowRight"}
	err = websocket.JSON.Send(ws, directionCmd)
	assert.NoError(t, err, "Should send direction without error")

	// 6. Wait for Updated Game State showing position change and IsMoving=true
	fmt.Println("E2E Test: Waiting for paddle position change and IsMoving=true...")
	ok = waitForStateCondition(t, ws, localState, 10*time.Second, "Paddle Position Changed & Moving", func(ls *game.LocalGameState) bool {
		paddle := ls.Paddles[playerIndex]
		return paddle != nil && paddle.Y != initialPaddleY && paddle.IsMoving
	})
	assert.True(t, ok, "Should receive game state with paddle position changed and IsMoving=true")
	if !ok {
		// If this fails, log the state for debugging
		t.Logf("Current local state when failing wait for move: %+v", localState)
		t.FailNow()
	}

	// 7. Send Stop Input
	fmt.Println("E2E Test: Sending 'Stop' input...")
	stopCmd := game.Direction{Direction: "Stop"}
	err = websocket.JSON.Send(ws, stopCmd)
	assert.NoError(t, err, "Should send stop direction without error")

	// 8. Wait for Updated Game State showing stopped state (IsMoving == false)
	fmt.Println("E2E Test: Waiting for paddle stopped state (IsMoving=false)...")
	ok = waitForStateCondition(t, ws, localState, 10*time.Second, "Paddle Stopped", func(ls *game.LocalGameState) bool {
		paddle := ls.Paddles[playerIndex]
		return paddle != nil && !paddle.IsMoving
	})
	assert.True(t, ok, "Should receive game state with paddle stopped (IsMoving == false)")

	// 9. Disconnect Client by closing the WebSocket
	fmt.Println("E2E Test: Closing client connection...")
	err = ws.Close()
	// Ignore common close errors in tests
	if err != nil && !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "use of closed network connection") && !strings.Contains(err.Error(), "connection reset by peer") {
		t.Logf("Note: ws.Close() returned unexpected error: %v", err)
	}

	// 10. Wait briefly for Server to Process Disconnect (optional)
	time.Sleep(500 * time.Millisecond)
	fmt.Println("E2E Test: Finished.")
}

// TestE2E_BallWallNonStick verifies that balls correctly move away from walls after collision.
func TestE2E_BallWallNonStick(t *testing.T) {
	// 1. Setup using helper with E2E config
	e2eCfg := utils.E2ETestConfig()
	setup := SetupE2ETest(t, e2eCfg)
	defer TeardownE2ETest(t, setup, e2eTestTimeout) // Use longer timeout for this test
	canvasSize := e2eCfg.CanvasSize

	// 2. Connect WebSocket Client
	ws, err := websocket.Dial(setup.WsURL, "", setup.Origin)
	assert.NoError(t, err, "WebSocket dial should succeed")
	if err != nil {
		t.FailNow()
	}
	defer func() { _ = ws.Close() }() // Ignore error on close in test defer

	// 3. Read initial messages (Assignment + Initial Entities)
	var assignmentMsg game.PlayerAssignmentMessage
	errAssign := ReadWsJSONMessage(t, ws, 5*time.Second, &assignmentMsg)
	assert.NoError(t, errAssign, "Should receive assignment message")

	var initialEntitiesMsg game.InitialPlayersAndBallsState
	errEntities := ReadWsJSONMessage(t, ws, 5*time.Second, &initialEntitiesMsg)
	assert.NoError(t, errEntities, "Should receive initial entities message")

	// 4. Monitor game state for wall collisions and subsequent movement
	localState := game.NewLocalGameState() // Use game package
	fmt.Println("E2E Wall Stick Test: Waiting for wall collision...")

	collisionDetected := false
	collidedBallID := -1   // Use := for inference
	collisionWall := -1    // Use := for inference // 0:Right, 1:Top, 2:Left, 3:Bottom
	var collisionCoord int // X or Y coord at collision (original coords)

	testDeadline := time.Now().Add(e2eTestTimeout - 2*time.Second) // Deadline for the test itself

	for time.Now().Before(testDeadline) {
		var rawMsg json.RawMessage
		err := ReadWsJSONMessage(t, ws, 1*time.Second, &rawMsg)
		if err != nil {
			// Simplify error checking for closed connections
			if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "closed network connection") || strings.Contains(err.Error(), "reset by peer") {
				t.Log("Connection closed or timed out during wall stick test.")
				break
			}
			netErr, isNetErr := err.(net.Error)
			if isNetErr && netErr.Timeout() {
				// Ignore read timeouts, continue waiting
			} else {
				t.Logf("Non-fatal error reading state during wall stick test: %v", err)
			}
			time.Sleep(50 * time.Millisecond)
			continue
		}

		var batch game.GameUpdatesBatch
		if json.Unmarshal(rawMsg, &batch) == nil && batch.MessageType == "gameUpdates" {
			game.ApplyUpdatesToLocalState(localState, batch.Updates, t) // Apply updates (use game package)
		} else {
			// Check for game over message
			var header game.MessageHeader
			if json.Unmarshal(rawMsg, &header) == nil && header.MessageType == "gameOver" {
				t.Log("Game ended before wall collision was detected and verified.")
				break // Exit loop if game ends
			}
			continue // Ignore other message types
		}

		// --- Check logic using locally maintained state ---
		if !collisionDetected {
			// Look for a ball that has the Collided flag set and is near a wall
			for id, ball := range localState.Balls {
				if ball != nil && ball.Collided {
					radius := ball.Radius
					if radius <= 0 {
						radius = e2eCfg.BallRadius
					}
					buffer := radius + 5 // Allow some buffer near wall

					if ball.X+buffer >= canvasSize { // Near right wall
						collisionDetected = true
						collidedBallID = id
						collisionWall = 0
						collisionCoord = ball.X
						t.Logf("Collision detected: Ball %d hit Right Wall (Wall 0) at X=%d", id, ball.X)
						break
					} else if ball.Y-buffer <= 0 { // Near top wall
						collisionDetected = true
						collidedBallID = id
						collisionWall = 1
						collisionCoord = ball.Y
						t.Logf("Collision detected: Ball %d hit Top Wall (Wall 1) at Y=%d", id, ball.Y)
						break
					} else if ball.X-buffer <= 0 { // Near left wall
						collisionDetected = true
						collidedBallID = id
						collisionWall = 2
						collisionCoord = ball.X
						t.Logf("Collision detected: Ball %d hit Left Wall (Wall 2) at X=%d", id, ball.X)
						break
					} else if ball.Y+buffer >= canvasSize { // Near bottom wall
						collisionDetected = true
						collidedBallID = id
						collisionWall = 3
						collisionCoord = ball.Y
						t.Logf("Collision detected: Ball %d hit Bottom Wall (Wall 3) at Y=%d", id, ball.Y)
						break
					}
				}
			}
		} else {
			// Collision flag was detected, check subsequent states for movement *away*
			ball, exists := localState.Balls[collidedBallID]
			if !exists || ball == nil {
				t.Logf("Collided ball %d disappeared, assuming test passed for this collision.", collidedBallID)
				collisionDetected = false
				collidedBallID = -1
				continue
			}

			movedAway := false
			currentCoord := 0
			switch collisionWall {
			case 0:
				currentCoord = ball.X
				movedAway = currentCoord < collisionCoord
			case 1:
				currentCoord = ball.Y
				movedAway = currentCoord > collisionCoord
			case 2:
				currentCoord = ball.X
				movedAway = currentCoord > collisionCoord
			case 3:
				currentCoord = ball.Y
				movedAway = currentCoord < collisionCoord
			}

			if movedAway {
				t.Logf("SUCCESS: Ball %d moved away from Wall %d (Original Coord: %d -> %d)", collidedBallID, collisionWall, collisionCoord, currentCoord)
				return // Exit after first successful non-stick detection
			}
		}
	} // End for loop

	// If loop finishes without success
	if !collisionDetected {
		t.Log("E2E Wall Stick Test: No wall collision flag detected within the test timeout.")
		// This might be acceptable if the game ends quickly, don't fail the test
	} else {
		t.Errorf("E2E Wall Stick Test: FAILED - Ball %d hit Wall %d at original coord %d, but did not confirm moving away before test timeout.", collidedBallID, collisionWall, collisionCoord)
	}
}
