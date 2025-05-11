// File: test/e2e_brick_collision_test.go
package test

import (
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/utils"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/websocket"
)

const (
	brickTestClientCount    = 4 // Number of concurrent clients (1 room)
	brickTestDuration       = 20 * time.Second
	brickTestOverallTimeout = brickTestDuration + 10*time.Second
	brickTestCmdInterval    = 200 * time.Millisecond
)

// brickCollisionTestWorker simulates a client for the brick penetration test.
func brickCollisionTestWorker(
	t *testing.T,
	wg *sync.WaitGroup,
	wsURL, origin string,
	stopCh <-chan struct{},
	cfg utils.Config,
	errorReported *atomic.Bool, // To signal if any worker found an error
) {
	defer wg.Done()
	t.Helper()

	ws, err := websocket.Dial(wsURL, "", origin)
	if err != nil {
		t.Logf("Client failed to dial: %v", err)
		return
	}
	defer func() { _ = ws.Close() }()

	localState := game.NewLocalGameState()
	var assignedIndex int = -1

	// Consume initial messages
	var assignmentMsg game.PlayerAssignmentMessage
	if err := ReadWsJSONMessage(t, ws, 10*time.Second, &assignmentMsg); err != nil {
		t.Logf("Client failed to receive assignment: %v", err)
		return
	}
	assignedIndex = assignmentMsg.PlayerIndex

	var initialEntities game.InitialPlayersAndBallsState
	if err := ReadWsJSONMessage(t, ws, 10*time.Second, &initialEntities); err != nil {
		t.Logf("Client %d failed to receive initial entities: %v", assignedIndex, err)
		return
	}
	// Apply initial state (especially for ball radii and initial grid)
	// This part is tricky as InitialPlayersAndBallsState has a different structure
	// For simplicity, we'll rely on the first GameUpdatesBatch to populate LocalGameState fully.

	cmdTicker := time.NewTicker(brickTestCmdInterval)
	defer cmdTicker.Stop()
	randGen := rand.New(rand.NewSource(time.Now().UnixNano() + int64(assignedIndex)))
	directions := []string{"ArrowLeft", "ArrowRight", "Stop"}

	for {
		select {
		case <-stopCh:
			return
		case <-cmdTicker.C:
			if assignedIndex != -1 { // Only send if assigned
				direction := directions[randGen.Intn(len(directions))]
				cmd := game.Direction{Direction: direction}
				if sendErr := websocket.JSON.Send(ws, cmd); sendErr != nil {
					if errors.Is(sendErr, io.EOF) || strings.Contains(sendErr.Error(), "closed") {
						return
					}
					t.Logf("Client %d: Error sending command: %v", assignedIndex, sendErr)
				}
			}
		default:
			// Non-blocking read attempt
			var rawMsg json.RawMessage
			readErr := ReadWsJSONMessage(t, ws, 200*time.Millisecond, &rawMsg)

			if readErr == nil {
				var batch game.GameUpdatesBatch
				if json.Unmarshal(rawMsg, &batch) == nil && batch.MessageType == "gameUpdates" {
					game.ApplyUpdatesToLocalState(localState, batch.Updates, t)

					// Perform the penetration check
					for _, updateInterface := range batch.Updates {
						updateBytes, _ := json.Marshal(updateInterface)
						var header game.MessageHeader
						_ = json.Unmarshal(updateBytes, &header)

						if header.MessageType == "ballPositionUpdate" {
							var bpu game.BallPositionUpdate
							_ = json.Unmarshal(updateBytes, &bpu)

							// Only check for penetration if the ball is NOT phasing AND
							// its Collided flag is false (meaning it wasn't reported as just colliding).
							if !bpu.Phasing && !bpu.Collided {
								ballData, ballExists := localState.Balls[bpu.ID]
								if !ballExists || ballData == nil {
									continue // Ball info not yet in local state or removed
								}
								ballRadius := float64(ballData.Radius)
								cellSize := float64(localState.CellSize) // Use exported field

								if cellSize == 0 {
									continue
								} // Grid not yet received

								for _, brick := range localState.BrickStates {
									if brick.Type == utils.Cells.Brick && brick.Life > 0 {
										brickMinX := brick.X - cellSize/2.0
										brickMaxX := brick.X + cellSize/2.0
										brickMinY := brick.Y - cellSize/2.0
										brickMaxY := brick.Y + cellSize/2.0

										// Check if ball's CENTER is inside the brick's bounds
										if bpu.R3fX >= brickMinX && bpu.R3fX <= brickMaxX &&
											bpu.R3fY >= brickMinY && bpu.R3fY <= brickMaxY {

											// Log detailed info for the failure
											t.Errorf("Client %d: Non-phasing Ball %d (Radius: %.2f, Collided: %t, Vx: %d, Vy: %d) CENTER (%.2f, %.2f) reported INSIDE Brick (CellSize: %.2f) at R3F bounds X[%.2f, %.2f], Y[%.2f, %.2f]. Brick State: Type=%v, Life=%d",
												assignedIndex, bpu.ID, ballRadius, bpu.Collided, bpu.Vx, bpu.Vy, bpu.R3fX, bpu.R3fY, cellSize, brickMinX, brickMaxX, brickMinY, brickMaxY, brick.Type, brick.Life)
											errorReported.Store(true)
										}
									}
								}
							}
						}
					}
				} else {
					// Could be gameOver or other messages, ignore for this check
				}
			} else {
				netErr, isNetErr := readErr.(net.Error)
				if errors.Is(readErr, io.EOF) || strings.Contains(readErr.Error(), "closed") {
					return // Connection closed, worker exits
				} else if isNetErr && netErr.Timeout() {
					// Read timeout is expected, continue loop
				} else {
					t.Logf("Client %d: Error reading message: %v", assignedIndex, readErr)
					return // Exit on other errors
				}
			}
		}
	}
}

// TestE2E_NonPhasingBallBrickPenetration checks if non-phasing balls get their centers reported inside bricks.
func TestE2E_NonPhasingBallBrickPenetration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping brick penetration stress test in short mode.")
	}

	t.Logf("Starting Non-Phasing Ball Brick Penetration Test: %d clients for %v", brickTestClientCount, brickTestDuration)

	cfg := utils.BrickCollisionTestConfig()
	setup := SetupE2ETest(t, cfg)
	defer TeardownE2ETest(t, setup, brickTestOverallTimeout/2)

	var wg sync.WaitGroup
	stopCh := make(chan struct{})
	var errorReportedAtomic atomic.Bool // Use atomic bool

	for i := 0; i < brickTestClientCount; i++ {
		wg.Add(1)
		go func(workerIndex int) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic in client worker %d: %v", workerIndex, r)
					errorReportedAtomic.Store(true)
				}
			}()
			brickCollisionTestWorker(t, &wg, setup.WsURL, setup.Origin, stopCh, cfg, &errorReportedAtomic)
		}(i)
		time.Sleep(50 * time.Millisecond) // Stagger connections
	}

	t.Logf("Launched %d client workers for brick penetration test.", brickTestClientCount)

	testRunTimer := time.NewTimer(brickTestDuration)
	defer testRunTimer.Stop()

	<-testRunTimer.C // Wait for the test duration or until an error is reported

	if errorReportedAtomic.Load() {
		t.Logf("Error reported by a client worker. Signaling stop early.")
	} else {
		t.Logf("Test duration (%v) elapsed.", brickTestDuration)
	}

	close(stopCh)
	t.Logf("Waiting for client workers to finish...")

	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		t.Logf("All client workers finished.")
	case <-time.After(10 * time.Second):
		t.Errorf("Timeout waiting for client workers to finish after stop signal.")
	}

	assert.False(t, errorReportedAtomic.Load(), "Test failed: Non-phasing ball penetration detected by one or more clients.")
	t.Logf("Brick Penetration Test Completed.")
}
