// File: server/handlers_test.go
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/game" // Import utils
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/websocket"
)

// --- Mock Actor (Captures Sent Messages) ---
type MockActor struct {
	mu       sync.Mutex
	Received []interface{}
	PID      *bollywood.PID // Store its own PID
}

func (a *MockActor) Receive(ctx bollywood.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	// fmt.Printf("MockActor %s received: %T\n", ctx.Self(), ctx.Message()) // Debugging
	a.Received = append(a.Received, ctx.Message())
}

func (a *MockActor) GetReceived() []interface{} {
	a.mu.Lock()
	defer a.mu.Unlock()
	// Return copy
	msgs := make([]interface{}, len(a.Received))
	copy(msgs, a.Received)
	return msgs
}

func (a *MockActor) ClearMessages() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Received = nil
}

// Mock net.Addr
type MockAddr struct{ Addr string }

func (m *MockAddr) Network() string { return "mock" }
func (m *MockAddr) String() string  { return m.Addr }

// --- Test Setup ---
func setupTestServer(t *testing.T) (*Server, *bollywood.Engine, *MockActor, *bollywood.PID) {
	engine := bollywood.NewEngine()
	mockGameActor := &MockActor{}
	// Use NewProps to ensure the actor function is correctly passed
	gameActorPID := engine.Spawn(bollywood.NewProps(func() bollywood.Actor { return mockGameActor }))
	assert.NotNil(t, gameActorPID, "GameActor PID should not be nil")
	mockGameActor.PID = gameActorPID // Store PID in mock actor
	server := New(engine, gameActorPID)
	time.Sleep(50 * time.Millisecond) // Allow actor to start
	return server, engine, mockGameActor, gameActorPID
}

// Helper to wait for a specific message type with timeout
func waitForMessage(t *testing.T, mockActor *MockActor, targetType interface{}, timeout time.Duration) (interface{}, bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		received := mockActor.GetReceived()
		for _, msg := range received {
			if fmt.Sprintf("%T", msg) == fmt.Sprintf("%T", targetType) {
				return msg, true
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, false
}

// --- Tests ---

func TestHandleSubscribe_SendsConnectRequest(t *testing.T) {
	server, engine, mockGameActor, _ := setupTestServer(t)
	defer engine.Shutdown(2 * time.Second)

	// Use httptest to simulate a WebSocket connection
	s := httptest.NewServer(websocket.Handler(server.HandleSubscribe()))
	defer s.Close()

	wsURL := "ws" + strings.TrimPrefix(s.URL, "http")
	ws, err := websocket.Dial(wsURL, "", s.URL) // Use actual websocket dial
	assert.NoError(t, err)
	assert.NotNil(t, ws, "WebSocket connection should not be nil")
	defer ws.Close()

	// Wait for the connect message to be processed by the mock actor
	msg, found := waitForMessage(t, mockGameActor, game.PlayerConnectRequest{}, 1*time.Second)
	assert.True(t, found, "MockGameActor should have received PlayerConnectRequest")

	if found {
		req, ok := msg.(game.PlayerConnectRequest)
		assert.True(t, ok)
		// Check if the connection object is not nil and matches the established one (by type and remote addr)
		assert.NotNil(t, req.WsConn, "PlayerConnectRequest should contain a non-nil WsConn")
		assert.IsType(t, &websocket.Conn{}, req.WsConn, "WsConn should be of type *websocket.Conn")
		// Note: Comparing remote addresses might be flaky with httptest, focus on non-nil and type.
	}
}

func TestReadLoop_ForwardsDirection(t *testing.T) {
	// Unskip this test
	server, engine, mockGameActor, _ := setupTestServer(t)
	defer engine.Shutdown(2 * time.Second)

	s := httptest.NewServer(websocket.Handler(server.HandleSubscribe()))
	defer s.Close()

	wsURL := "ws" + strings.TrimPrefix(s.URL, "http")
	ws, err := websocket.Dial(wsURL, "", s.URL)
	assert.NoError(t, err)
	assert.NotNil(t, ws)
	defer ws.Close()

	// Wait for the initial connect message to be processed
	_, found := waitForMessage(t, mockGameActor, game.PlayerConnectRequest{}, 1*time.Second)
	assert.True(t, found, "Connect request should be received first")
	mockGameActor.ClearMessages() // Clear connect message

	// Send a direction message
	direction := game.Direction{Direction: "ArrowRight"}
	err = websocket.JSON.Send(ws, direction)
	assert.NoError(t, err, "Sending direction message should succeed")

	// Wait for the forwarded message
	msg, found := waitForMessage(t, mockGameActor, game.ForwardedPaddleDirection{}, 1*time.Second)
	assert.True(t, found, "MockGameActor should have received ForwardedPaddleDirection")

	if found {
		fwdMsg, ok := msg.(game.ForwardedPaddleDirection)
		assert.True(t, ok)
		assert.NotNil(t, fwdMsg.WsConn, "Forwarded message should contain WsConn")
		assert.IsType(t, &websocket.Conn{}, fwdMsg.WsConn)

		// Verify the content of the forwarded direction
		var receivedDir game.Direction
		err = json.Unmarshal(fwdMsg.Direction, &receivedDir)
		assert.NoError(t, err, "Unmarshalling forwarded direction should succeed")
		assert.Equal(t, "ArrowRight", receivedDir.Direction, "Forwarded direction content mismatch")
	}
}

func TestReadLoop_SendsDisconnectOnError(t *testing.T) {
	// Unskip this test
	server, engine, mockGameActor, _ := setupTestServer(t)
	defer engine.Shutdown(2 * time.Second)

	s := httptest.NewServer(websocket.Handler(server.HandleSubscribe()))
	defer s.Close()

	wsURL := "ws" + strings.TrimPrefix(s.URL, "http")
	ws, err := websocket.Dial(wsURL, "", s.URL)
	assert.NoError(t, err)
	assert.NotNil(t, ws)
	// Don't defer close immediately, we'll close it manually

	// Wait for the initial connect message
	_, found := waitForMessage(t, mockGameActor, game.PlayerConnectRequest{}, 1*time.Second)
	assert.True(t, found, "Connect request should be received first")
	mockGameActor.ClearMessages()

	// Send invalid data (not JSON) to trigger a read error in the handler's loop
	_, err = ws.Write([]byte("this is not json"))
	assert.NoError(t, err) // Write itself might succeed

	// Wait for the disconnect message triggered by the read error
	msg, found := waitForMessage(t, mockGameActor, game.PlayerDisconnect{}, 2*time.Second) // Increase timeout slightly
	assert.True(t, found, "MockGameActor should have received PlayerDisconnect after read error")

	if found {
		disMsg, ok := msg.(game.PlayerDisconnect)
		assert.True(t, ok)
		assert.NotNil(t, disMsg.WsConn, "Disconnect message should contain WsConn")
		assert.IsType(t, &websocket.Conn{}, disMsg.WsConn)
		// Index might be -1 if it couldn't be determined before disconnect
	}
	ws.Close() // Close afterwards
}

func TestReadLoop_SendsDisconnectOnClose(t *testing.T) {
	// Unskip this test
	server, engine, mockGameActor, _ := setupTestServer(t)
	defer engine.Shutdown(2 * time.Second)

	s := httptest.NewServer(websocket.Handler(server.HandleSubscribe()))
	defer s.Close()

	wsURL := "ws" + strings.TrimPrefix(s.URL, "http")
	ws, err := websocket.Dial(wsURL, "", s.URL)
	assert.NoError(t, err)
	assert.NotNil(t, ws)

	// Wait for the initial connect message
	_, found := waitForMessage(t, mockGameActor, game.PlayerConnectRequest{}, 1*time.Second)
	assert.True(t, found, "Connect request should be received first")
	mockGameActor.ClearMessages()

	// Close the connection from the client side
	err = ws.Close()
	assert.NoError(t, err, "Closing websocket should succeed")

	// Wait for the disconnect message triggered by the closed connection (EOF)
	msg, found := waitForMessage(t, mockGameActor, game.PlayerDisconnect{}, 2*time.Second) // Increase timeout slightly
	assert.True(t, found, "MockGameActor should have received PlayerDisconnect after client close")

	if found {
		disMsg, ok := msg.(game.PlayerDisconnect)
		assert.True(t, ok)
		assert.NotNil(t, disMsg.WsConn, "Disconnect message should contain WsConn")
		assert.IsType(t, &websocket.Conn{}, disMsg.WsConn)
		// Index might be -1 if it couldn't be determined before disconnect
	}
}

func TestHandleGetSit_ReturnsPlaceholder(t *testing.T) {
	server, engine, _, _ := setupTestServer(t)
	defer engine.Shutdown(2 * time.Second)

	req, err := http.NewRequest("GET", "/", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(server.HandleGetSit())

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Handler returned wrong status code")
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"), "Handler returned wrong content type")

	// The actual implementation in game_actor.go for GetGameStateJSON is complex.
	// The server handler currently returns a placeholder. Test for that placeholder.
	// If GetGameStateJSON were implemented, this test would need adjustment.
	expectedBody := `{"error": "Live state query not implemented via HTTP GET in actor model"}`
	assert.JSONEq(t, expectedBody, rr.Body.String(), "Handler returned unexpected body")
}
