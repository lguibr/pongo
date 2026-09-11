package server

import (
	"encoding/json" // Import errors
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/internal/actor"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/websocket"
)

// --- Mock Room Manager Actor ---
type MockRoomManager struct {
	mu           sync.Mutex
	Received     []interface{}
	PID          *actor.PID
	Rooms        map[string]int
	AssignErr    error
	AssignPID    *actor.PID // PID to assign in response
	GetListReply game.RoomListResponse
	GetListErr   error
	ShouldReply  bool // Flag to control reply behavior in tests
}

func (a *MockRoomManager) Receive(ctx actor.Context) {
	a.mu.Lock()
	shouldReply := a.ShouldReply
	reply := a.GetListReply
	replyErr := a.GetListErr
	a.mu.Unlock()

	msg := ctx.Message()
	a.mu.Lock()
	a.Received = append(a.Received, msg)
	a.mu.Unlock()

	switch msg.(type) {
	case game.GetRoomListRequest:
		if shouldReply {
			// ReplyTo is now implicit via Ask/Reply
			if replyErr != nil {
				ctx.Reply(replyErr) // Use ctx.Reply for Ask
			} else {
				ctx.Reply(reply) // Use ctx.Reply for Ask
			}
		}
	case game.PlayerDisconnect:
		// No action needed in mock for this test file
	case game.ForwardedPaddleDirection:
		// No action needed in mock for this test file
	}
}

func (a *MockRoomManager) GetReceived() []interface{} {
	a.mu.Lock()
	defer a.mu.Unlock()
	msgs := make([]interface{}, len(a.Received))
	copy(msgs, a.Received)
	return msgs
}

func (a *MockRoomManager) ClearMessages() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Received = nil
}

// --- Test Setup ---
func setupServerWithMockManager(t *testing.T) (*Server, *actor.Engine, *MockRoomManager, *actor.PID) {
	engine := actor.NewEngine()
	mockRoomManager := &MockRoomManager{
		Rooms: make(map[string]int),
		GetListReply: game.RoomListResponse{
			Rooms: map[string]int{"mock-room-1": 2},
		},
		ShouldReply: true,                                  // Default to replying
		AssignPID:   &actor.PID{ID: "mock-game-actor-pid"}, // Default PID to assign
	}
	// Assign PID directly to the mock actor and capture the return value
	roomManagerPID := engine.Spawn(actor.NewProps(func() actor.Actor { return mockRoomManager }))
	assert.NotNil(t, roomManagerPID, "MockRoomManager PID should not be nil")
	mockRoomManager.PID = roomManagerPID // Store the PID in the mock

	server := New(engine, roomManagerPID)
	time.Sleep(50 * time.Millisecond)
	// Return the PID obtained from Spawn
	return server, engine, mockRoomManager, roomManagerPID
}

// Helper to wait for a specific message type with timeout
func waitForManagerMessage(t *testing.T, mockManager *MockRoomManager, targetType interface{}, timeout time.Duration) (interface{}, bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		received := mockManager.GetReceived()
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

func TestHandleSubscribe_ForwardsQuickPlayRequest(t *testing.T) {
	server, engine, mockManager, _ := setupServerWithMockManager(t)
	defer engine.Shutdown(2 * time.Second)

	s := httptest.NewServer(websocket.Handler(server.HandleSubscribe()))
	defer s.Close()

	wsURL := "ws" + strings.TrimPrefix(s.URL, "http")
	ws, err := websocket.Dial(wsURL, "", s.URL)
	assert.NoError(t, err)
	assert.NotNil(t, ws, "WebSocket connection should not be nil")
	defer func() { _ = ws.Close() }() // Ignore error on close in test defer

	assert.NoError(t, websocket.JSON.Send(ws, game.QuickPlayRequest{MessageType: "quickPlay", SessionID: "test-session"}))
	msg, found := waitForManagerMessage(t, mockManager, game.QuickPlayActorRequest{}, 1*time.Second)
	assert.True(t, found, "MockRoomManager should have received QuickPlayActorRequest")

	if found {
		req, ok := msg.(game.QuickPlayActorRequest)
		assert.True(t, ok)
		assert.NotNil(t, req.ReplyTo, "Request should contain a non-nil ReplyTo PID")
	}
}

func TestHandleGetRooms_QueriesManagerAndReturnsList(t *testing.T) {
	server, engine, mockManager, _ := setupServerWithMockManager(t) // Use underscore for managerPID if not needed directly
	defer engine.Shutdown(2 * time.Second)

	// Configure mock response
	expectedRooms := map[string]int{"actor-10": 3, "actor-11": 1}
	mockManager.mu.Lock()
	mockManager.GetListReply = game.RoomListResponse{
		Rooms: expectedRooms,
	}
	mockManager.ShouldReply = true
	mockManager.mu.Unlock()

	req, err := http.NewRequest("GET", "/rooms/", nil) // Use correct path
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(server.HandleGetRooms()) // Use correct handler

	handler.ServeHTTP(rr, req)

	// Check if RoomManager received the request
	// Note: We don't check ReplyTo anymore as Ask handles it
	_, found := waitForManagerMessage(t, mockManager, game.GetRoomListRequest{}, 1*time.Second)
	assert.True(t, found, "MockRoomManager should have received GetRoomListRequest")

	// Check HTTP response
	assert.Equal(t, http.StatusOK, rr.Code, "Handler returned wrong status code")
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"), "Handler returned wrong content type")

	// Use JSONEq for robust comparison
	expectedBody, _ := json.Marshal(game.RoomListResponse{Rooms: expectedRooms})
	assert.JSONEq(t, string(expectedBody), rr.Body.String(), "Handler returned unexpected body")
}

func TestHandleGetRooms_HandlesManagerTimeout(t *testing.T) {
	server, engine, mockManager, _ := setupServerWithMockManager(t) // Use underscore for managerPID if not needed directly
	// Use a shorter shutdown to speed up test end
	defer engine.Shutdown(1 * time.Second)

	mockManager.mu.Lock()
	mockManager.ShouldReply = false // Configure mock to not reply
	mockManager.mu.Unlock()

	req, err := http.NewRequest("GET", "/rooms/", nil) // Use correct path
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(server.HandleGetRooms()) // Use correct handler

	// Run the handler in a goroutine so we can timeout waiting for it
	handlerDone := make(chan bool)
	go func() {
		handler.ServeHTTP(rr, req)
		close(handlerDone)
	}()

	// Wait for handler to finish or timeout
	select {
	case <-handlerDone:
		// Handler finished, check status code
		assert.Equal(t, http.StatusGatewayTimeout, rr.Code, "Handler should return 504 on timeout")
	case <-time.After(3 * time.Second): // Wait slightly longer than Ask timeout
		t.Fatal("HTTP handler did not return within timeout")
	}

	// Check that the manager still received the request
	_, found := waitForManagerMessage(t, mockManager, game.GetRoomListRequest{}, 1*time.Second)
	assert.True(t, found, "MockRoomManager should have received GetRoomListRequest even if not replying")
}

func TestHandleHealthCheck(t *testing.T) {
	req, err := http.NewRequest("GET", "/health-check/", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(HandleHealthCheck())

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Health check handler returned wrong status code")
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"), "Health check handler returned wrong content type")
	assert.JSONEq(t, `{"status": "ok"}`, rr.Body.String(), "Health check handler returned unexpected body")
}

func TestHandleHealthCheck_RootPath(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(HandleHealthCheck()) // Same handler used for both paths

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Root health check handler returned wrong status code")
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"), "Root health check handler returned wrong content type")
	assert.JSONEq(t, `{"status": "ok"}`, rr.Body.String(), "Root health check handler returned unexpected body")
}

func TestHandleHealthCheck_WrongMethod(t *testing.T) {
	req, err := http.NewRequest("POST", "/health-check/", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(HandleHealthCheck())

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code, "Health check handler should return 405 for wrong method")
}
