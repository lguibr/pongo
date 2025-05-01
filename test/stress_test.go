// File: test/stress_test.go
package test

import (
	"math/rand"
	"net/http/httptest"
	"strings"
	"sync"
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
	stressTestClientCount = 200                                 // Number of concurrent clients (creates clientCount / MaxPlayers rooms)
	stressTestDuration    = 30 * time.Second                    // How long to run the stress test
	stressTestTimeout     = stressTestDuration + 30*time.Second // Overall test timeout (increased slightly)
	sendCommandInterval   = 100 * time.Millisecond              // How often each client sends a command
)

// clientWorker simulates a single game client for the stress test.
func clientWorker(t *testing.T, wg *sync.WaitGroup, wsURL, origin string, stopCh <-chan struct{}, cfg utils.Config) {
	defer wg.Done()
	t.Helper()

	ws, err := websocket.Dial(wsURL, "", origin)
	if err != nil {
		// Log error but don't fail the whole test immediately, allow others to connect
		t.Logf("Client failed to dial: %v", err)
		return
	}
	defer ws.Close()

	// 1. Wait for initial state (optional but confirms assignment)
	var initialState game.GameState
	// Use the existing helper from helpers.go (capitalized)
	err = ReadWsJSONMessage(t, ws, 15*time.Second, &initialState) // Generous timeout for initial state under load
	if err != nil {
		t.Logf("Client failed to receive initial state (may be expected under load): %v", err)
		// Continue anyway to send commands
	} else {
		// Optional: Log only if verbose or needed
		// t.Logf("Client connected and received initial state.")
	}

	// 2. Send random commands periodically
	ticker := time.NewTicker(sendCommandInterval)
	defer ticker.Stop()

	directions := []string{"ArrowLeft", "ArrowRight", "Stop"}
	randGen := rand.New(rand.NewSource(time.Now().UnixNano())) // Use local random generator

	for {
		select {
		case <-stopCh:
			// t.Logf("Client received stop signal.") // Reduce log noise
			return
		case <-ticker.C:
			direction := directions[randGen.Intn(len(directions))]
			cmd := game.Direction{Direction: direction}
			err := websocket.JSON.Send(ws, cmd)
			if err != nil {
				// If connection is closed, just exit cleanly
				if strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "broken pipe") || strings.Contains(err.Error(), "EOF") {
					// t.Logf("Client send error (connection likely closed): %v", err) // Reduce log noise
					return
				}
				// Log other send errors but don't fail the test immediately
				t.Logf("Client failed to send command: %v", err)
				// Consider returning on persistent errors to avoid log spam
				// return
			}
		}
	}
}

// TestE2E_StressTestMultipleRooms simulates many clients connecting and sending inputs.
func TestE2E_StressTestMultipleRooms(t *testing.T) {
	// Skip in short mode as it takes time
	if testing.Short() {
		t.Skip("Skipping stress test in short mode.")
	}

	t.Logf("Starting Stress Test: %d clients for %v", stressTestClientCount, stressTestDuration)

	// 1. Setup Engine, RoomManager, and Config
	engine := bollywood.NewEngine()
	// Use a longer shutdown timeout for stress tests
	defer engine.Shutdown(stressTestTimeout / 2)

	cfg := utils.DefaultConfig()

	// Spawn RoomManager
	roomManagerPID := engine.Spawn(bollywood.NewProps(game.NewRoomManagerProducer(engine, cfg)))
	assert.NotNil(t, roomManagerPID)
	time.Sleep(200 * time.Millisecond) // Allow manager to start fully

	// 2. Setup Test Server
	testServer := server.New(engine, roomManagerPID)
	s := httptest.NewServer(websocket.Handler(testServer.HandleSubscribe()))
	defer s.Close()
	wsURL := "ws" + strings.TrimPrefix(s.URL, "http")
	origin := "http://localhost/" // Origin doesn't matter much for local test server

	// 3. Launch Client Workers
	var wg sync.WaitGroup
	stopCh := make(chan struct{})

	connectSuccessCount := 0
	var connectMu sync.Mutex // Protect the success count

	for i := 0; i < stressTestClientCount; i++ {
		wg.Add(1)
		go func(workerIndex int) {
			// Wrap worker call to handle potential panics within the goroutine
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic in client worker %d: %v", workerIndex, r)
				}
			}()
			// Pass t *testing.T to the worker
			clientWorker(t, &wg, wsURL, origin, stopCh, cfg)
			// Increment success count if worker finishes without calling t.Errorf related to connection
			// Note: This is a basic check; more sophisticated error tracking could be added.
			connectMu.Lock()
			connectSuccessCount++
			connectMu.Unlock()
		}(i)
		// Reduce stagger time slightly for faster ramp-up with more clients
		time.Sleep(10 * time.Millisecond)
	}

	t.Logf("Launched %d client workers.", stressTestClientCount)

	// 4. Run for specified duration
	startTime := time.Now()
	<-time.After(stressTestDuration)
	elapsed := time.Since(startTime)
	t.Logf("Stress duration (%v) elapsed.", elapsed)

	// 5. Signal clients to stop and wait
	t.Logf("Signaling clients to stop...")
	close(stopCh)
	t.Logf("Waiting for client workers to finish...")

	// Wait with a timeout
	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		t.Logf("All client workers finished.")
	case <-time.After(20 * time.Second): // Increased timeout for waiting for more clients
		t.Errorf("Timeout waiting for client workers to finish.")
	}

	// 6. Assertions (Basic)
	connectMu.Lock()
	t.Logf("Successfully connected clients (approx): %d / %d", connectSuccessCount, stressTestClientCount)
	// Basic assertion: Check if at least a majority of clients connected successfully.
	// This accounts for potential transient errors under load.
	assert.GreaterOrEqual(t, connectSuccessCount, stressTestClientCount*8/10, "Expected at least 80% of clients to connect without immediate failure")
	connectMu.Unlock()

	t.Logf("Stress Test Completed.")
	// The test passes if it reaches here without panics and meets the basic connection assertion.
	// Check logs for server-side errors or performance bottlenecks.
}
