package game

import (
	"testing"
	"time"

	"github.com/lguibr/pongo/internal/actor"
	"github.com/lguibr/pongo/utils"
	"github.com/stretchr/testify/assert"
)

// --- Test Setup ---
func setupRoomManagerTest(t *testing.T) (*actor.Engine, *actor.PID, *RoomManagerActor) {
	engine := actor.NewEngine()
	cfg := utils.DefaultConfig()

	producer := NewRoomManagerProducer(engine, cfg)
	actorInstance := producer().(*RoomManagerActor) // Get the instance

	roomManagerPID := engine.Spawn(actor.NewProps(func() actor.Actor { return actorInstance }))

	assert.NotNil(t, roomManagerPID, "RoomManager PID should not be nil")
	time.Sleep(50 * time.Millisecond)            // Allow actor to start
	return engine, roomManagerPID, actorInstance // Return the instance
}

// --- Tests ---

func TestRoomManager_StartsEmpty(t *testing.T) {
	engine, _, managerActor := setupRoomManagerTest(t)
	defer engine.Shutdown(1 * time.Second)

	assert.Empty(t, managerActor.rooms, "Room manager should start with no rooms")
}

func TestRoomManager_GetRoomList(t *testing.T) {
	engine, rmPID, managerActor := setupRoomManagerTest(t)
	defer engine.Shutdown(1 * time.Second)

	// Manually add some mock rooms to the manager's state for testing GetRoomList
	mockRoomPID1 := &actor.PID{ID: "room-1"}
	mockRoomPID2 := &actor.PID{ID: "room-2"}
	managerActor.rooms[mockRoomPID1.String()] = &RoomInfo{PID: mockRoomPID1, PlayerCount: 2, Code: "CODE1", IsPublic: true}
	managerActor.rooms[mockRoomPID2.String()] = &RoomInfo{PID: mockRoomPID2, PlayerCount: 4, Code: "CODE2", IsPublic: false}

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

func TestRoomManager_GenerateCode(t *testing.T) {
	_, _, managerActor := setupRoomManagerTest(t)
	code := managerActor.generateRoomCode()
	assert.Len(t, code, 6, "Room code should be 6 characters")
}
