// File: game/room_manager.go
package game

import (
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
)

// Increase maxRooms significantly to support ~200 players (200 / 4 = 50 rooms)
// Add some buffer.
const maxRooms = 75 // Limit the number of concurrent rooms

// RoomInfo holds information about an active game room.
type RoomInfo struct {
	PID         *bollywood.PID
	PlayerCount int // Approximate count
}

// RoomManagerActor manages multiple GameActor instances (rooms).
type RoomManagerActor struct {
	engine     *bollywood.Engine
	cfg        utils.Config
	rooms      map[string]*RoomInfo // Map room ID (PID string) to RoomInfo
	mu         sync.RWMutex
	selfPID    *bollywood.PID
	nextRoomID int
}

// NewRoomManagerProducer creates a producer for the RoomManagerActor.
func NewRoomManagerProducer(engine *bollywood.Engine, cfg utils.Config) bollywood.Producer {
	return func() bollywood.Actor {
		return &RoomManagerActor{
			engine:     engine,
			cfg:        cfg,
			rooms:      make(map[string]*RoomInfo),
			nextRoomID: 1,
		}
	}
}

// Receive Method
func (a *RoomManagerActor) Receive(ctx bollywood.Context) {
	defer func() {
		if r := recover(); r != nil {
			pidStr := "unknown"
			if a.selfPID != nil {
				pidStr = a.selfPID.String()
			}
			fmt.Printf("PANIC recovered in RoomManagerActor %s Receive: %v\nStack trace:\n%s\n", pidStr, r, string(debug.Stack()))
			// If this was an Ask request, reply with error
			if ctx.RequestID() != "" {
				ctx.Reply(fmt.Errorf("room manager panicked: %v", r))
			}
		}
	}()

	if a.selfPID == nil {
		a.selfPID = ctx.Self()
	}

	switch msg := ctx.Message().(type) {
	case bollywood.Started:
		fmt.Printf("RoomManagerActor %s: Started.\n", a.selfPID)

	case FindRoomRequest:
		a.handleFindRoom(ctx, msg.ReplyTo)

	case GameRoomEmpty:
		a.handleGameRoomEmpty(ctx, msg.RoomPID)

	case GetRoomListRequest:
		// This message now likely comes via Ask
		a.handleGetRoomList(ctx) // Pass context for Reply

	case bollywood.Stopping:
		fmt.Printf("RoomManagerActor %s: Stopping. Shutting down all rooms.\n", a.selfPID)
		a.mu.Lock()
		pidsToStop := []*bollywood.PID{}
		for _, roomInfo := range a.rooms {
			if roomInfo.PID != nil {
				pidsToStop = append(pidsToStop, roomInfo.PID)
			}
		}
		a.rooms = make(map[string]*RoomInfo)
		a.mu.Unlock()
		for _, pid := range pidsToStop {
			a.engine.Stop(pid)
		}

	case bollywood.Stopped:
		fmt.Printf("RoomManagerActor %s: Stopped.\n", a.selfPID)

	default:
		fmt.Printf("RoomManagerActor %s: Received unknown message type: %T\n", a.selfPID, msg)
		// If it was an Ask, reply with error
		if ctx.RequestID() != "" {
			ctx.Reply(fmt.Errorf("unknown message type: %T", msg))
		}
	}
}

// Handler Methods

func (a *RoomManagerActor) handleFindRoom(ctx bollywood.Context, replyTo *bollywood.PID) {
	if replyTo == nil {
		return
	}
	a.mu.Lock()

	// Find existing room
	for _, roomInfo := range a.rooms {
		if roomInfo.PID != nil && roomInfo.PlayerCount < utils.MaxPlayers {
			roomInfo.PlayerCount++ // Increment approximate count
			roomPID := roomInfo.PID
			a.mu.Unlock()
			a.engine.Send(replyTo, AssignRoomResponse{RoomPID: roomPID}, a.selfPID)
			return
		}
	}

	// Check max rooms
	if len(a.rooms) >= maxRooms {
		fmt.Printf("RoomManagerActor %s: Max rooms (%d) reached. Rejecting request from %s.\n", a.selfPID, maxRooms, replyTo)
		a.mu.Unlock()
		a.engine.Send(replyTo, AssignRoomResponse{RoomPID: nil}, a.selfPID)
		return
	}

	// Create new room
	roomIDStr := fmt.Sprintf("room-%d", a.nextRoomID)
	a.nextRoomID++
	gameActorProps := bollywood.NewProps(NewGameActorProducer(a.engine, a.cfg, a.selfPID))
	gameActorPID := a.engine.Spawn(gameActorProps)
	if gameActorPID == nil {
		fmt.Printf("ERROR: RoomManagerActor %s: Failed to spawn GameActor for room %s. Replying nil to %s.\n", a.selfPID, roomIDStr, replyTo)
		a.mu.Unlock()
		a.engine.Send(replyTo, AssignRoomResponse{RoomPID: nil}, a.selfPID)
		return
	}

	roomInfo := &RoomInfo{PID: gameActorPID, PlayerCount: 1}
	a.rooms[gameActorPID.String()] = roomInfo
	a.mu.Unlock()
	a.engine.Send(replyTo, AssignRoomResponse{RoomPID: gameActorPID}, a.selfPID)
}

func (a *RoomManagerActor) handleGameRoomEmpty(ctx bollywood.Context, roomPID *bollywood.PID) {
	if roomPID == nil {
		return
	}
	roomIDStr := roomPID.String()
	a.mu.Lock()
	_, exists := a.rooms[roomIDStr]
	pidToStop := (*bollywood.PID)(nil)
	if exists {
		fmt.Printf("RoomManagerActor %s: Room %s reported empty. Removing and stopping.\n", a.selfPID, roomIDStr)
		if roomInfo := a.rooms[roomIDStr]; roomInfo != nil && roomInfo.PID != nil {
			pidToStop = roomInfo.PID
		}
		delete(a.rooms, roomIDStr)
	} // Else: Already removed, ignore.
	a.mu.Unlock()
	if pidToStop != nil {
		a.engine.Stop(pidToStop)
	}
}

// handleGetRoomList now uses ctx.Reply if the request came via Ask.
func (a *RoomManagerActor) handleGetRoomList(ctx bollywood.Context) {
	a.mu.RLock()
	roomList := make(map[string]int)
	for _, roomInfo := range a.rooms {
		if roomInfo != nil && roomInfo.PID != nil {
			// Use the PID string as the key for the response map
			roomList[roomInfo.PID.String()] = roomInfo.PlayerCount
		}
	}
	a.mu.RUnlock()

	response := RoomListResponse{Rooms: roomList}

	// Check if this was an Ask request and reply accordingly
	if ctx.RequestID() != "" {
		ctx.Reply(response)
	} else {
		// Fallback or error? This case shouldn't happen if HandleGetSit always uses Ask.
		fmt.Printf("WARN: RoomManagerActor %s received GetRoomListRequest not via Ask.\n", a.selfPID)
	}
}
