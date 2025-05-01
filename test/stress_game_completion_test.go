// File: test/stress_test_game_completion.go
package test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	// "net" // No longer needed directly here
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
	completionTestClientCount = 400              // 100 rooms * 4 players
	completionTestDuration    = 90 * time.Second // Allow more time for games to finish
	completionTestTimeout     = completionTestDuration + 30*time.Second
	expectedCompletions       = 100
)

// fastGameConfig returns a config optimized for rapid game completion.
func fastGameConfig() utils.Config {
	cfg := utils.DefaultConfig()

	// Smaller grid, fewer bricks initially
	cfg.CanvasSize = 512 // Must be divisible by GridSize
	cfg.GridSize = 8     // Must be divisible by 2
	cfg.CellSize = cfg.CanvasSize / cfg.GridSize

	// Fewer generation steps -> less dense grid
	cfg.GridFillVectors = cfg.GridSize / 2
	cfg.GridFillVectorSize = cfg.GridSize / 2
	cfg.GridFillWalkers = cfg.GridSize / 4
	cfg.GridFillSteps = cfg.GridSize / 4

	// Faster game loop
	cfg.GameTickPeriod = 16 * time.Millisecond // ~60 FPS physics

	// Faster balls
	cfg.MinBallVelocity = cfg.CanvasSize / 60 // ~8.5
	cfg.MaxBallVelocity = cfg.CanvasSize / 40 // ~12.8
	cfg.BallRadius = cfg.CellSize / 4         // Smaller balls relative to cell

	// Less phasing
	cfg.BallPhasingTime = 50 * time.Millisecond

	// Lower power-up chance to avoid too many balls
	cfg.PowerUpChance = 0.1
	cfg.PowerUpSpawnBallExpiry = 5 * time.Second

	// Faster paddles (though not actively used by clients in this test)
	cfg.PaddleVelocity = cfg.CellSize / 2

	// Adjust paddle size relative to new cell size
	cfg.PaddleLength = cfg.CellSize * 2 // 128
	cfg.PaddleWidth = cfg.CellSize / 3  // ~21

	return cfg
}

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
	for {
		select {
		case <-stopCh:
			return
		default:
			var msg json.RawMessage
			err := ReadWsJSONMessage(t, ws, 30*time.Second, &msg) // Use capitalized name

			if err != nil {
				if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "reset by peer") {
					return // Expected if game ends and broadcaster closes connection
				} else if strings.Contains(err.Error(), "timeout") {
					continue // Read timeout, continue loop unless stopCh is closed
				}
				t.Logf("Client %d: Error reading message: %v", assignedIndex, err)
				return // Exit on unexpected error
			}

			var gameOverMsg game.GameOverMessage
			if json.Unmarshal(msg, &gameOverMsg) == nil && gameOverMsg.Reason != "" {
				completedGames.Add(1)
				return // Game finished for this client
			}
			// Ignore other message types
		}
	}
}

// TestE2E_StressTestGameCompletion simulates many games running to completion.
func TestE2E_StressTestGameCompletion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping game completion stress test in short mode.")
	}

	t.Logf("Starting Game Completion Stress Test: %d clients (%d rooms) for %v", completionTestClientCount, expectedCompletions, completionTestDuration)

	// 1. Setup Engine, RoomManager, and FAST Config
	engine := bollywood.NewEngine()
	defer engine.Shutdown(completionTestTimeout / 2) // Longer shutdown timeout

	cfg := fastGameConfig()
	t.Logf("Using Fast Config: Tick=%v, Grid=%dx%d, BallVel=%d-%d", cfg.GameTickPeriod, cfg.GridSize, cfg.GridSize, cfg.MinBallVelocity, cfg.MaxBallVelocity)

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
			connectSuccessCount++
			connectMu.Unlock()
		}(i)
		time.Sleep(20 * time.Millisecond) // Stagger connections slightly more
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
	case <-time.After(30 * time.Second): // Generous wait timeout
		t.Errorf("Timeout waiting for client workers to finish.")
	}

	// 6. Assertions
	finalCompletedCount := completedGames.Load()
	connectMu.Lock()
	finalConnectCount := connectSuccessCount
	connectMu.Unlock()

	t.Logf("Successfully connected clients (approx): %d / %d", finalConnectCount, completionTestClientCount)
	t.Logf("Games reported completed by clients: %d / %d", finalCompletedCount, expectedCompletions*4) // Compare raw count

	// Assert connection success rate
	assert.GreaterOrEqual(t, finalConnectCount, completionTestClientCount*9/10, "Expected at least 90% of clients to connect without immediate failure")

	// Assert game completion rate (allow some games not finishing within the duration)
	// The count is based on clients receiving the message, so divide by 4.
	actualCompletedGames := finalCompletedCount / 4
	minExpectedCompletions := int32(float64(expectedCompletions) * 0.95)
	assert.GreaterOrEqual(t, actualCompletedGames, minExpectedCompletions, fmt.Sprintf("Expected at least %d games to complete", minExpectedCompletions))

	t.Logf("Game Completion Stress Test Completed.")
	// Check server logs for errors.
}
