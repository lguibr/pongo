// File: game/game_actor_test.go
package game

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/lguibr/pongo/bollywood"
	"github.com/lguibr/pongo/utils"
	"github.com/stretchr/testify/assert"
)

// --- Mock WebSocket Conn ---
// MockWebSocket implements PlayerConnection
type MockWebSocket struct {
	mu       sync.Mutex
	Written  [][]byte
	Closed   bool
	Remote   string
	ReadChan chan []byte
	ErrChan  chan error
	closeSig chan struct{}
}

func NewMockWebSocket(remoteAddr string) *MockWebSocket {
	return &MockWebSocket{
		Remote:   remoteAddr,
		ReadChan: make(chan []byte, 10),
		ErrChan:  make(chan error, 1),
		closeSig: make(chan struct{}),
	}
}

func (m *MockWebSocket) Read(p []byte) (n int, err error) {
	select {
	case data := <-m.ReadChan:
		n = copy(p, data)
		return n, nil
	case err = <-m.ErrChan:
		return 0, err
	case <-m.closeSig:
		return 0, fmt.Errorf("mock connection closed") // Simulate closure error
	case <-time.After(500 * time.Millisecond): // Increased read timeout
		return 0, fmt.Errorf("mock read timeout")
	}
}
func (m *MockWebSocket) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Closed {
		return 0, fmt.Errorf("use of closed network connection")
	}
	msgCopy := make([]byte, len(p))
	copy(msgCopy, p)
	m.Written = append(m.Written, msgCopy)
	// fmt.Printf("MockWS %s Write: %s\n", m.Remote, string(p)) // Debug log
	return len(p), nil
}
func (m *MockWebSocket) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Closed {
		return fmt.Errorf("already closed")
	}
	m.Closed = true
	select {
	case <-m.closeSig: // Already closed
	default:
		close(m.closeSig) // Signal Read to return error
	}
	// fmt.Printf("MockWS %s Closed\n", m.Remote) // Debug log
	return nil
}
func (m *MockWebSocket) RemoteAddr() net.Addr { return &MockAddr{Addr: m.Remote} }

// Mock net.Addr
type MockAddr struct{ Addr string }

func (m *MockAddr) Network() string { return "mock" }
func (m *MockAddr) String() string  { return m.Addr }

// --- Test Receiver Actor (Used in Paddle Forwarding Test) ---
// MockGameActor is defined in paddle_actor_test.go, no need to redefine

// --- Helper to wait for a specific message ---
// Increased timeout significantly
const waitForStateTimeout = 3500 * time.Millisecond // Further increased timeout

func waitForGameState(t *testing.T, mockWs *MockWebSocket, check func(gs GameState) bool) bool {
	t.Helper() // Mark as test helper
	deadline := time.Now().Add(waitForStateTimeout)
	lastCheckedCount := 0
	for time.Now().Before(deadline) {
		mockWs.mu.Lock()
		currentWritten := make([][]byte, len(mockWs.Written))
		copy(currentWritten, mockWs.Written)
		currentCount := len(currentWritten)
		mockWs.mu.Unlock()

		// Only check new messages since last iteration
		for i := lastCheckedCount; i < currentCount; i++ {
			var gs GameState
			if err := json.Unmarshal(currentWritten[i], &gs); err == nil {
				// Add more detailed logging inside the check
				// t.Logf("waitForGameState (%s): Checking state %d: P:%v Pad:%v B:%v",
				// 	mockWs.Remote, i, gs.Players, gs.Paddles, gs.Balls) // More detailed debug
				if check(gs) {
					// t.Logf("waitForGameState (%s): Found matching state %d", mockWs.Remote, i) // Debug
					return true
				}
			} else {
				// t.Logf("waitForGameState (%s): Failed to unmarshal state %d: %v", mockWs.Remote, i, err) // Debug
			}
		}
		lastCheckedCount = currentCount // Update the count of checked messages

		// Slightly longer sleep within the loop
		time.Sleep(utils.Period * 5) // Sleep for ~120ms
	}
	t.Logf("waitForGameState (%s): Timeout waiting for state", mockWs.Remote) // Log timeout using t.Logf
	return false
}

// Helper to wait for connection close
const waitForCloseTimeout = 2500 * time.Millisecond // Increased timeout

func waitForClose(t *testing.T, mockWs *MockWebSocket) bool {
	t.Helper()
	deadline := time.Now().Add(waitForCloseTimeout)
	for time.Now().Before(deadline) {
		mockWs.mu.Lock()
		closed := mockWs.Closed
		mockWs.mu.Unlock()
		if closed {
			return true
		}
		time.Sleep(utils.Period * 3) // Sleep ~72ms
	}
	t.Logf("waitForClose (%s): Timeout waiting for close", mockWs.Remote)
	return false
}

// --- Tests ---

// Increased shutdown timeout for all tests
const testShutdownTimeout = 8 * time.Second // Increased shutdown timeout

func TestGameActor_PlayerConnect_FirstPlayer(t *testing.T) {
	engine := bollywood.NewEngine()
	defer engine.Shutdown(testShutdownTimeout)

	gameActorPID := engine.Spawn(bollywood.NewProps(NewGameActorProducer(engine)))
	assert.NotNil(t, gameActorPID)
	time.Sleep(250 * time.Millisecond) // Longer wait for actor start

	mockWs := NewMockWebSocket("mock-addr-1:1234")
	connectMsg := PlayerConnectRequest{WsConn: mockWs}

	engine.Send(gameActorPID, connectMsg, nil)

	// Add a longer delay *after* sending connect, before checking state
	time.Sleep(400 * time.Millisecond)

	// Simplify the check: just see if *any* message was written
	foundAnyMessage := false
	deadline := time.Now().Add(waitForStateTimeout)
	for time.Now().Before(deadline) {
		mockWs.mu.Lock()
		if len(mockWs.Written) > 0 {
			foundAnyMessage = true
			// Optionally log the first message found
			// t.Logf("Found first message: %s", string(mockWs.Written[0]))
		}
		mockWs.mu.Unlock()
		if foundAnyMessage {
			break
		}
		time.Sleep(utils.Period * 2)
	}

	assert.True(t, foundAnyMessage, "Mock WebSocket should receive at least one message")

	// Keep the original check as well, but it might still fail if the state isn't exactly right yet
	foundState := waitForGameState(t, mockWs, func(gs GameState) bool {
		playerOk := gs.Players[0] != nil && gs.Players[0].Index == 0
		paddleOk := gs.Paddles[0] != nil
		ballOk := false
		if len(gs.Balls) > 0 {
			for _, b := range gs.Balls {
				if b != nil && b.OwnerIndex == 0 {
					ballOk = true
					break
				}
			}
		}
		t.Logf("Check Connect_FirstPlayer: PlayerOK=%v, PaddleOK=%v, BallOK=%v (Balls: %d)", playerOk, paddleOk, ballOk, len(gs.Balls))
		return playerOk && paddleOk && ballOk
	})
	assert.True(t, foundState, "Mock WebSocket should eventually receive valid initial game state")

}

func TestGameActor_PlayerConnect_ServerFull(t *testing.T) {
	engine := bollywood.NewEngine()
	defer engine.Shutdown(testShutdownTimeout)

	gameActorPID := engine.Spawn(bollywood.NewProps(NewGameActorProducer(engine)))
	assert.NotNil(t, gameActorPID)
	time.Sleep(250 * time.Millisecond)

	mocks := make([]*MockWebSocket, MaxPlayers)
	for i := 0; i < MaxPlayers; i++ {
		mocks[i] = NewMockWebSocket(fmt.Sprintf("mock-addr-%d:1234", i))
		engine.Send(gameActorPID, PlayerConnectRequest{WsConn: mocks[i]}, nil)
		// Wait a bit longer for each connection to be processed
		time.Sleep(utils.Period * 10)
	}

	// Wait for all players to potentially be processed and broadcast
	time.Sleep(500 * time.Millisecond)

	mockWs5 := NewMockWebSocket("mock-addr-5:1234")
	engine.Send(gameActorPID, PlayerConnectRequest{WsConn: mockWs5}, nil)

	// Wait significantly longer for rejection processing and close
	closed := waitForClose(t, mockWs5)
	assert.True(t, closed, "5th WebSocket should have been closed")

	mockWs5.mu.Lock()
	assert.Empty(t, mockWs5.Written, "5th WebSocket should not receive game state")
	mockWs5.mu.Unlock()
}

func TestGameActor_PlayerDisconnect(t *testing.T) {
	engine := bollywood.NewEngine()
	defer engine.Shutdown(testShutdownTimeout)

	gameActorPID := engine.Spawn(bollywood.NewProps(NewGameActorProducer(engine)))
	assert.NotNil(t, gameActorPID)
	time.Sleep(250 * time.Millisecond)

	mockWs0 := NewMockWebSocket("mock-addr-0:1234")
	engine.Send(gameActorPID, PlayerConnectRequest{WsConn: mockWs0}, nil)
	time.Sleep(utils.Period * 10) // Allow setup

	mockWs1 := NewMockWebSocket("mock-addr-1:1234")
	engine.Send(gameActorPID, PlayerConnectRequest{WsConn: mockWs1}, nil)
	time.Sleep(utils.Period * 20) // Allow setup and first broadcast

	// Send disconnect for Player 0
	engine.Send(gameActorPID, PlayerDisconnect{PlayerIndex: -1, WsConn: mockWs0}, nil)

	// Wait longer for disconnect state propagation to Player 1
	foundState := waitForGameState(t, mockWs1, func(gs GameState) bool {
		player0Nil := gs.Players[0] == nil
		player1Exists := gs.Players[1] != nil && gs.Players[1].Index == 1
		paddle0Nil := gs.Paddles[0] == nil
		paddle1Exists := gs.Paddles[1] != nil
		noP0Balls := true
		for _, ball := range gs.Balls {
			if ball != nil && ball.OwnerIndex == 0 {
				noP0Balls = false
				break
			}
		}
		t.Logf("Check Disconnect: P0Nil=%v, P1Exists=%v, Pad0Nil=%v, Pad1Exists=%v, NoP0Balls=%v (Balls: %d)", player0Nil, player1Exists, paddle0Nil, paddle1Exists, noP0Balls, len(gs.Balls)) // Debug log with t.Logf
		return player0Nil && player1Exists && paddle0Nil && paddle1Exists && noP0Balls
	})

	assert.True(t, foundState, "Player 1 should receive game state reflecting Player 0 disconnect")

	mockWs1.mu.Lock()
	assert.NotEmpty(t, mockWs1.Written, "Player 1 should have received messages")
	mockWs1.mu.Unlock()

	// Wait significantly longer before checking if the connection is closed
	closed := waitForClose(t, mockWs0)
	assert.True(t, closed, "Player 0's WebSocket should be closed after disconnect")
}

func TestGameActor_LastPlayerDisconnect(t *testing.T) {
	engine := bollywood.NewEngine()
	defer engine.Shutdown(testShutdownTimeout)

	gameActorPID := engine.Spawn(bollywood.NewProps(NewGameActorProducer(engine)))
	assert.NotNil(t, gameActorPID)
	time.Sleep(250 * time.Millisecond)

	mockWs0 := NewMockWebSocket("mock-addr-0:1234")
	engine.Send(gameActorPID, PlayerConnectRequest{WsConn: mockWs0}, nil)
	time.Sleep(utils.Period * 20) // Allow setup and first broadcast

	engine.Send(gameActorPID, PlayerDisconnect{PlayerIndex: -1, WsConn: mockWs0}, nil)

	// Wait significantly longer before checking if the connection is closed
	closed := waitForClose(t, mockWs0)
	assert.True(t, closed, "Player 0's WebSocket should be closed")
}

func TestGameActor_PaddleMovementForwarding(t *testing.T) {
	engine := bollywood.NewEngine()
	defer engine.Shutdown(testShutdownTimeout)

	// No need for the separate TestReceiver anymore, we check broadcast state
	gameActorPID := engine.Spawn(bollywood.NewProps(NewGameActorProducer(engine)))
	assert.NotNil(t, gameActorPID)
	time.Sleep(250 * time.Millisecond) // Allow GameActor to start

	mockWs0 := NewMockWebSocket("mock-addr-0:1234")
	engine.Send(gameActorPID, PlayerConnectRequest{WsConn: mockWs0}, nil)
	time.Sleep(utils.Period * 20) // Allow connection and paddle actor spawn

	// Send "ArrowRight", which should translate to internal "right"
	directionPayload, _ := json.Marshal(Direction{Direction: "ArrowRight"})
	forwardMsg := ForwardedPaddleDirection{
		WsConn:    mockWs0,
		Direction: directionPayload,
	}
	engine.Send(gameActorPID, forwardMsg, nil)

	// Wait longer for forwarding and state update/broadcast
	// Check broadcasted state for the correct direction
	foundMovedState := waitForGameState(t, mockWs0, func(gs GameState) bool {
		paddle := gs.Paddles[0]
		t.Logf("Check PaddleMoveForward: Paddle=%+v", paddle) // Debug log with t.Logf
		// Check that the paddle exists AND its internal direction state is "right"
		return paddle != nil && paddle.Direction == "right"
	})
	assert.True(t, foundMovedState, "Game state broadcast should show paddle direction updated to 'right'")
}
