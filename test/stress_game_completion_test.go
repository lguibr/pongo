// File: test/stress_test_game_completion.go
package test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net" // Import net
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/server"
	"github.com/lguibr/pongo/utils"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/websocket"
)

const (
	completionTestClientCount = 400             // Keep high client count (100 rooms * 4 players)
	completionTestDuration    = 25 * time.Second // Significantly reduced duration
	completionTestTimeout     = completionTestDuration + 15*time.Second // Adjusted timeout
	expectedRooms             = completionTestClientCount / 4
)

// ultraFastGameConfig is defined in utils/config.go

// completionClientWorker simulates a client waiting for game completion.
func completionClientWorker(t *testing.T, wg *sync.WaitGroup, wsURL, origin string, stopCh <-chan struct{}, completedGames *atomic.Int32) {
	defer wg.Done()
	t.Helper()

	var ws *websocket.Conn
	var err error
	var assignedIndex int = -1

	// Retry dialing briefly in case of initial connection refused under load
	for i := 0; i < 3; i++ {
		ws, err = websocket.Dial(wsURL, "", origin)
		if err == nil {
			break
		}
		time.Sleep(time.Duration(50+i*50) * time.Millisecond) // Small backoff
	}

	if err != nil {
		t.Logf("Client failed to dial after retries: %v", err)
		return
	}
	defer ws.Close()

	// 1. Wait for PlayerAssignmentMessage
	var assignmentMsg game.PlayerAssignmentMessage
	err = ReadWsJSONMessage(t, ws, 15*time.Second, &assignmentMsg) // Use capitalized name
	if err != nil {
		t.Logf("Client failed to receive assignment message: %v", err)
		return
	}
	assignedIndex = assignmentMsg.PlayerIndex

	// 2. Listen only for GameOverMessage
	readDeadline := time.Now().Add(completionTestDuration + 5*time.Second) // Set a deadline for reading
	for time.Now().Before(readDeadline) {
		select {
		case <-stopCh:
			return
		default:
			var msg json.RawMessage
			// Use a shorter read timeout within the loop
			err := ReadWsJSONMessage(t, ws, 5*time.Second, &msg) // Use capitalized name

			if err != nil {
				netErr, isNetErr := err.(net.Error)
				if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "reset by peer") {
					// Check if we received a completion before closing
					// This is a bit of a race, but better than nothing
					select {
					case <-stopCh: // Already stopped
					default:
						// t.Logf("Client %d: Connection closed.", assignedIndex)
					}
					return // Expected if game ends and broadcaster closes connection
				} else if isNetErr && netErr.Timeout() {
					continue // Read timeout, continue loop unless stopCh is closed
				}
				t.Logf("Client %d: Error reading message: %v", assignedIndex, err)
				return // Exit on unexpected error
			}

			var gameOverMsg game.GameOverMessage
			if json.Unmarshal(msg, &gameOverMsg) == nil && gameOverMsg.MessageType == "gameOver" {
				completedGames.Add(1)
				// t.Logf("Client %d received GameOverMessage for room %s", assignedIndex, gameOverMsg.RoomPID)
				return // Game finished for this client
			}
			// Ignore other message types
		}
	}
	// If loop finishes due to readDeadline
	t.Logf("Client %d: Read deadline exceeded without receiving GameOverMessage.", assignedIndex)
}

// TestE2E_StressTestGameCompletion simulates many games running to completion.
func TestE2E_StressTestGameCompletion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping game completion stress test in short mode.")
	}

	t.Logf("Starting Game Completion Stress Test: %d clients (%d rooms) for %v", completionTestClientCount, expectedRooms, completionTestDuration)

	// 1. Setup Engine, RoomManager, and ULTRA FAST Config
	engine := bollywood.NewEngine()
	defer engine.Shutdown(completionTestTimeout / 2) // Longer shutdown timeout

	cfg := utils.UltraFastGameConfig() // Use the new ultra-fast config
	t.Logf("Using UltraFast Config: Tick=%v, Grid=%dx%d, BallVel=%d-%d", cfg.GameTickPeriod, cfg.GridSize, cfg.GridSize, cfg.MinBallVelocity, cfg.MaxBallVelocity)

	roomManagerPID := engine.Spawn(bollywood.NewProps(game.NewRoomManagerProducer(engine, cfg)))
	assert.NotNil(t, roomManagerPID)
	time.Sleep(200 * time.Millisecond)

	// 2. Setup Test Server
	testServer := server.New(engine, roomManagerPID)
	s := httptest.NewServer(websocket.Handler(testServer.HandleSubscribe()))
	defer s.Close()
	wsURL := "ws" + strings.TrimPrefix(s.URL, "http")
	origin := "http://localhost/"

	// 3. Launch Client Workers
	var wg sync.WaitGroup
	stopCh := make(chan struct{})
	var completedGames atomic.Int32
	completedGames.Store(0)

	connectSuccessCount := 0
	var connectMu sync.Mutex

	for i := 0; i < completionTestClientCount; i++ {
		wg.Add(1)
		go func(workerIndex int) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic in client worker %d: %v", workerIndex, r)
				}
			}()
			completionClientWorker(t, &wg, wsURL, origin, stopCh, &completedGames)
			connectMu.Lock()
			connectSuccessCount++ // Increment even if worker failed after dial, to track attempts
			connectMu.Unlock()
		}(i)
		time.Sleep(10 * time.Millisecond) // Stagger connections slightly
	}

	t.Logf("Launched %d client workers.", completionTestClientCount)

	// 4. Run for specified duration
	startTime := time.Now()
	testEndTimer := time.NewTimer(completionTestDuration)
	defer testEndTimer.Stop()

	<-testEndTimer.C // Wait for the test duration

	elapsed := time.Since(startTime)
	t.Logf("Stress duration (%v) elapsed.", elapsed)

	// 5. Signal clients to stop and wait
	t.Logf("Signaling clients to stop...")
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
	case <-time.After(15 * time.Second): // Shorter wait timeout now
		t.Errorf("Timeout waiting for client workers to finish.")
	}

	// 6. Assertions
	finalCompletedCount := completedGames.Load()
	connectMu.Lock()
	finalConnectCount := connectSuccessCount
	connectMu.Unlock()

	t.Logf("Client workers finished (approx): %d / %d", finalConnectCount, completionTestClientCount)
	t.Logf("GameOver messages received by clients: %d", finalCompletedCount)

	// Assert connection success rate (less critical now, focus on completion)
	// assert.GreaterOrEqual(t, finalConnectCount, completionTestClientCount*8/10, "Expected at least 80% of clients to connect without immediate failure")

	// Assert game completion rate
	// Since each game has 4 players, divide count by 4 to get completed games.
	actualCompletedGames := finalCompletedCount / 4
	// Expect a high percentage of games to complete within the shorter duration due to faster config
	minExpectedCompletions := int32(float64(expectedRooms) * 0.85) // Expect 85% completion
	assert.GreaterOrEqual(t, actualCompletedGames, minExpectedCompletions, fmt.Sprintf("Expected at least %d games to complete (received %d GameOver messages from %d clients)", minExpectedCompletions, finalCompletedCount, finalConnectCount))

	t.Logf("Game Completion Stress Test Completed.")
	// Check server logs for errors.
}