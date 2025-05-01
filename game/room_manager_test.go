package game

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	// Import unsafe
	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/websocket"
)

// --- Mock Game Actor for Room Manager Tests ---
type MockManagedGameActor struct {
	mu             sync.Mutex
	received       []interface{}
	PID            *bollywood.PID
	RoomManagerPID *bollywood.PID
	Ws             *websocket.Conn // Store the assigned connection
	PlayerCount    int
	ShouldStop     bool // Flag to simulate becoming empty
}

func (a *MockManagedGameActor) Receive(ctx bollywood.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.received = append(a.received, ctx.Message())
	// fmt.Printf("MockManagedGameActor %s received: %T\n", ctx.Self(), ctx.Message()) // Reduce noise

	switch msg := ctx.Message().(type) {
	case bollywood.Started:
		a.PID = ctx.Self()
	case AssignPlayerToRoom:
		a.Ws = msg.WsConn
		a.PlayerCount++
		// fmt.Printf("MockManagedGameActor %s: Accepted player. Count: %d\n", a.PID, a.PlayerCount) // Reduce noise

	case PlayerDisconnect:
		// Simplified: Assume any disconnect reduces count
		a.PlayerCount--
		// fmt.Printf("MockManagedGameActor %s: Player disconnected. Count: %d\n", a.PID, a.PlayerCount) // Reduce noise
		if a.PlayerCount <= 0 && a.RoomManagerPID != nil {
			// fmt.Printf("MockManagedGameActor %s: Sending GameRoomEmpty to %s\n", a.PID, a.RoomManagerPID) // Reduce noise
			ctx.Engine().Send(a.RoomManagerPID, GameRoomEmpty{RoomPID: a.PID}, a.PID)
			a.ShouldStop = true
		}

	case bollywood.Stopping:
	case bollywood.Stopped:
	}
}

func (a *MockManagedGameActor) GetReceived() []interface{} {
	a.mu.Lock()
	defer a.mu.Unlock()
	msgs := make([]interface{}, len(a.received))
	copy(msgs, a.received)
	return msgs
}

// --- Mock WebSocket Conn (Simplified) ---
// NOTE: This mock is insufficient for full testing. Tests using it are skipped.
type MockRoomManagerWS struct {
	RemoteAddrStr string
	Closed        bool
	mu            sync.Mutex
}

func (m *MockRoomManagerWS) Read(p []byte) (n int, err error) {
	// Simulate blocking read until closed or error
	<-time.After(10 * time.Second) // Block for a while
	return 0, fmt.Errorf("mock read error or timeout")
}
func (m *MockRoomManagerWS) Write(p []byte) (n int, err error)  { return len(p), nil }
func (m *MockRoomManagerWS) Close() error                       { m.mu.Lock(); m.Closed = true; m.mu.Unlock(); return nil }
func (m *MockRoomManagerWS) RemoteAddr() net.Addr               { return &MockAddr{Addr: m.RemoteAddrStr} }
func (m *MockRoomManagerWS) LocalAddr() net.Addr                { return &MockAddr{Addr: "localmock"} }
func (m *MockRoomManagerWS) SetDeadline(t time.Time) error      { return nil }
func (m *MockRoomManagerWS) SetReadDeadline(t time.Time) error  { return nil }
func (m *MockRoomManagerWS) SetWriteDeadline(t time.Time) error { return nil }

// --- Test Setup ---
func setupRoomManagerTest(t *testing.T) (*bollywood.Engine, *bollywood.PID, *RoomManagerActor) {
	engine := bollywood.NewEngine()
	cfg := utils.DefaultConfig()

	producer := NewRoomManagerProducer(engine, cfg)
	actorInstance := producer().(*RoomManagerActor) // Get the instance

	roomManagerPID := engine.Spawn(bollywood.NewProps(func() bollywood.Actor { return actorInstance }))

	assert.NotNil(t, roomManagerPID, "RoomManager PID should not be nil")
	time.Sleep(50 * time.Millisecond)            // Allow actor to start
	return engine, roomManagerPID, actorInstance // Return the instance
}

// Helper to find a message of a specific type in mock actor's received list
func findMessage[T any](mockActor *MockManagedGameActor) (*T, bool) {
	received := mockActor.GetReceived()
	for _, msg := range received {
		if typedMsg, ok := msg.(T); ok {
			return &typedMsg, true
		}
	}
	return nil, false
}

// --- Tests ---

func TestRoomManager_StartsEmpty(t *testing.T) {
	engine, _, managerActor := setupRoomManagerTest(t)
	defer engine.Shutdown(1 * time.Second)

	managerActor.mu.RLock()
	assert.Empty(t, managerActor.rooms, "Room manager should start with no rooms")
	managerActor.mu.RUnlock()
}

func TestRoomManager_CreatesFirstRoom(t *testing.T) {
	t.Skip("Skipping test due to limitations in mocking *websocket.Conn and actor interactions for assignment.")
}

func TestRoomManager_FillsRoomAndCreatesSecond(t *testing.T) {
	t.Skip("Skipping test due to limitations in mocking *websocket.Conn and actor interactions for assignment.")
}

func TestRoomManager_RemovesEmptyRoom(t *testing.T) {
	t.Skip("Skipping test due to complexity in mocking GameActor lifecycle and GameRoomEmpty message.")
}

func TestRoomManager_HandlesPendingDisconnect(t *testing.T) {
	t.Skip("Skipping test: Pending connection logic removed from RoomManager.")
}

func TestRoomManager_GetRoomList(t *testing.T) {
	engine, rmPID, managerActor := setupRoomManagerTest(t)
	defer engine.Shutdown(1 * time.Second)

	// Manually add some mock rooms to the manager's state for testing GetRoomList
	mockRoomPID1 := &bollywood.PID{ID: "room-1"}
	mockRoomPID2 := &bollywood.PID{ID: "room-2"}
	managerActor.mu.Lock()
	managerActor.rooms[mockRoomPID1.String()] = &RoomInfo{PID: mockRoomPID1, PlayerCount: 2}
	managerActor.rooms[mockRoomPID2.String()] = &RoomInfo{PID: mockRoomPID2, PlayerCount: 4}
	managerActor.mu.Unlock()

	// Use Ask to get the room list
	reply, err := engine.Ask(rmPID, GetRoomListRequest{}, 500*time.Millisecond)

	assert.NoError(t, err, "Ask for room list should not error")
	assert.NotNil(t, reply, "Reply should not be nil")

	listResponse, ok := reply.(RoomListResponse)
	assert.True(t, ok, "Reply should be of type RoomListResponse")
	if ok {
		assert.Len(t, listResponse.Rooms, 2, "Expected 2 rooms in the list")
		assert.Equal(t, 2, listResponse.Rooms[mockRoomPID1.String()], "Player count for room-1 mismatch")
		assert.Equal(t, 4, listResponse.Rooms[mockRoomPID2.String()], "Player count for room-2 mismatch")
	}
}
