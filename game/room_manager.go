// File: game/room_manager.go
package game

import (
	"fmt"
	"math/rand"
	"runtime/debug"
	"sync"
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
)

// Increase maxRooms significantly to support ~200 players (200 / 4 = 50 rooms)
// Add some buffer.
const maxRooms = 75 // Limit the number of concurrent rooms

// RoomInfo holds information about an active game room.
type RoomInfo struct {
	PID         *bollywood.PID
	PlayerCount int             // Approximate count
	Code        string          // 6-character unique code
	IsPublic    bool            // Public rooms can be joined via Quick Play
	Phase       Phase           // Current phase
	Sessions    map[string]bool // Track active sessions to handle reconnections
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
		rand.Seed(time.Now().UnixNano()) // Seed random number generator

	case FindRoomRequest:
		// Deprecated or used for internal fallback?
		// For now, treat as QuickPlay
		a.handleQuickPlay(ctx, msg.ReplyTo, "")

	case CreateRoomActorRequest:
		a.handleCreateRoom(ctx, msg.ReplyTo, msg.IsPublic, msg.SessionID)

	case JoinRoomActorRequest:
		a.handleJoinRoom(ctx, msg.ReplyTo, msg.Code, msg.SessionID)

	case QuickPlayActorRequest:
		a.handleQuickPlay(ctx, msg.ReplyTo, msg.SessionID)

	case GameRoomEmpty:
		a.handleGameRoomEmpty(ctx, msg.RoomPID)

	case PlayerLeftRoom:
		a.handlePlayerLeftRoom(ctx, msg.RoomPID, msg.SessionID)

	case RoomPhaseUpdate:
		a.handleRoomPhaseUpdate(ctx, msg.RoomPID, msg.Phase)

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

// Handler Methods

func (a *RoomManagerActor) generateRoomCode() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// internal helper to create room and return PID
func (a *RoomManagerActor) createRoomInternal(ctx bollywood.Context, isPublic bool) (*bollywood.PID, string) {
	if len(a.rooms) >= maxRooms {
		return nil, ""
	}

	// Generate unique code
	var code string
	for {
		code = a.generateRoomCode()
		unique := true
		for _, info := range a.rooms {
			if info.Code == code {
				unique = false
				break
			}
		}
		if unique {
			break
		}
	}

	gameActorProps := bollywood.NewProps(NewGameActorProducer(a.engine, a.cfg, a.selfPID))
	gameActorPID := a.engine.Spawn(gameActorProps)
	if gameActorPID == nil {
		return nil, ""
	}

	roomIDStr := gameActorPID.String()
	roomInfo := &RoomInfo{
		PID:         gameActorPID,
		PlayerCount: 0,
		Code:        code,
		IsPublic:    isPublic,
		Phase:       PhaseLobby,
		Sessions:    make(map[string]bool),
	}

	a.rooms[roomIDStr] = roomInfo
	a.nextRoomID++

	fmt.Printf("RoomManagerActor %s: Created room %s (Code: %s, Public: %t)\n", a.selfPID, roomIDStr, code, isPublic)
	return gameActorPID, code
}

func (a *RoomManagerActor) handleCreateRoom(ctx bollywood.Context, replyTo *bollywood.PID, isPublic bool, sessionID string) {
	if replyTo == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	pid, code := a.createRoomInternal(ctx, isPublic)
	if pid == nil {
		fmt.Printf("RoomManagerActor %s: Failed to create room for %s.\n", a.selfPID, replyTo)
		a.engine.Send(replyTo, AssignRoomResponse{RoomPID: nil}, a.selfPID)
		return
	}

	// Increment player count for the creator and track session
	if info, ok := a.rooms[pid.String()]; ok {
		info.PlayerCount = 1
		if sessionID != "" {
			info.Sessions[sessionID] = true
		}
	}

	// Send specific response with code
	a.engine.Send(replyTo, RoomCreatedResponse{
		MessageType: "roomCreated",
		Code:        code,
		RoomPID:     pid.String(),
	}, a.selfPID)

	// Also send the standard AssignRoomResponse so ConnectionHandler knows to proceed
	a.engine.Send(replyTo, AssignRoomResponse{RoomPID: pid}, a.selfPID)
}

func (a *RoomManagerActor) handleJoinRoom(ctx bollywood.Context, replyTo *bollywood.PID, code string, sessionID string) {
	if replyTo == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	var targetRoom *RoomInfo
	var targetRoomID string

	for id, info := range a.rooms {
		if info.Code == code {
			targetRoom = info
			targetRoomID = id
			break
		}
	}

	if targetRoom == nil {
		fmt.Printf("RoomManagerActor %s: Join failed - Room code %s not found.\n", a.selfPID, code)
		a.engine.Send(replyTo, RoomJoinedResponse{MessageType: "roomJoined", Success: false, Reason: "Room not found"}, a.selfPID)
		return
	}

	// Check if it's a reconnection
	isReconnect := false
	if sessionID != "" && targetRoom.Sessions[sessionID] {
		isReconnect = true
		fmt.Printf("RoomManagerActor %s: Reconnection detected for session %s in room %s\n", a.selfPID, sessionID, code)
	}

	if !isReconnect && targetRoom.PlayerCount >= utils.MaxPlayers {
		fmt.Printf("RoomManagerActor %s: Join failed - Room %s is full.\n", a.selfPID, code)
		a.engine.Send(replyTo, RoomJoinedResponse{MessageType: "roomJoined", Success: false, Reason: "Room is full"}, a.selfPID)
		return
	}

	if !isReconnect {
		targetRoom.PlayerCount++
		if sessionID != "" {
			targetRoom.Sessions[sessionID] = true
		}
	}

	fmt.Printf("RoomManagerActor %s: Assigning %s to room %s (Code: %s)\n", a.selfPID, replyTo, targetRoomID, code)

	a.engine.Send(replyTo, RoomJoinedResponse{MessageType: "roomJoined", Success: true, RoomPID: targetRoomID, Code: code}, a.selfPID)
	a.engine.Send(replyTo, AssignRoomResponse{RoomPID: targetRoom.PID}, a.selfPID)
}

func (a *RoomManagerActor) handleQuickPlay(ctx bollywood.Context, replyTo *bollywood.PID, sessionID string) {
	if replyTo == nil {
		return
	}
	a.mu.Lock()
	// Don't defer unlock here because we might call createRoomInternal which doesn't lock but we need to manage lock manually

	// 1. Priority: Find existing public room with space
	// Preference order: Lobby > CountingDown > Playing
	var bestMatch *RoomInfo
	var bestMatchID string

	for roomID, roomInfo := range a.rooms {
		if roomInfo.PID != nil && roomInfo.IsPublic && roomInfo.PlayerCount < utils.MaxPlayers {
			// If we find a Lobby room, it's the best match, take it immediately
			if roomInfo.Phase == PhaseLobby {
				bestMatch = roomInfo
				bestMatchID = roomID
				break
			}
			// If we find a Countdown room, it's a good match, but keep looking for Lobby
			if roomInfo.Phase == PhaseCountingDown {
				if bestMatch == nil || bestMatch.Phase == PhasePlaying {
					bestMatch = roomInfo
					bestMatchID = roomID
				}
			}
			// If we find a Playing room, it's a fallback match
			if roomInfo.Phase == PhasePlaying {
				if bestMatch == nil {
					bestMatch = roomInfo
					bestMatchID = roomID
				}
			}
		}
	}

	if bestMatch != nil {
		bestMatch.PlayerCount++
		if sessionID != "" {
			bestMatch.Sessions[sessionID] = true
		}
		roomPID := bestMatch.PID
		code := bestMatch.Code
		phaseStr := "lobby"
		if bestMatch.Phase == PhaseCountingDown {
			phaseStr = "countingDown"
		} else if bestMatch.Phase == PhasePlaying {
			phaseStr = "playing"
		}

		a.mu.Unlock() // Unlock before sending
		fmt.Printf("RoomManagerActor %s: QuickPlay assigning %s to %s room %s\n", a.selfPID, replyTo, phaseStr, bestMatchID)

		// Send Join response
		a.engine.Send(replyTo, RoomJoinedResponse{MessageType: "roomJoined", Success: true, RoomPID: bestMatchID, Code: code, Phase: phaseStr}, a.selfPID)
		a.engine.Send(replyTo, AssignRoomResponse{RoomPID: roomPID}, a.selfPID)
		return
	}

	// 2. Fallback: Create new public room and FORCE START
	// Note: createRoomInternal assumes caller holds lock? No, I removed lock from it?
	// Wait, createRoomInternal accesses a.rooms, so it NEEDS lock or I need to pass it?
	// My previous edit made createRoomInternal NOT lock.
	// So I should hold lock here.

	pid, code := a.createRoomInternal(ctx, true)
	if pid == nil {
		a.mu.Unlock()
		fmt.Printf("RoomManagerActor %s: QuickPlay failed to create room for %s\n", a.selfPID, replyTo)
		a.engine.Send(replyTo, RoomJoinedResponse{MessageType: "roomJoined", Success: false, Reason: "Failed to create room"}, a.selfPID)
		return
	}

	// Increment player count
	if info, ok := a.rooms[pid.String()]; ok {
		info.PlayerCount = 1
		info.Phase = PhasePlaying // Update local state immediately
		if sessionID != "" {
			info.Sessions[sessionID] = true
		}
	}
	a.mu.Unlock()

	fmt.Printf("RoomManagerActor %s: QuickPlay created new room %s for %s. Force starting.\n", a.selfPID, pid, replyTo)

	// Send ForceStartGame to the new room
	a.engine.Send(pid, ForceStartGame{}, a.selfPID)

	// Send Join response with Phase: playing
	a.engine.Send(replyTo, RoomJoinedResponse{MessageType: "roomJoined", Success: true, RoomPID: pid.String(), Code: code, Phase: "playing"}, a.selfPID)
	a.engine.Send(replyTo, AssignRoomResponse{RoomPID: pid}, a.selfPID)
}

func (a *RoomManagerActor) handleRoomPhaseUpdate(ctx bollywood.Context, roomPID *bollywood.PID, phase Phase) {
	if roomPID == nil {
		return
	}
	roomIDStr := roomPID.String()
	a.mu.Lock()
	defer a.mu.Unlock()

	if roomInfo, exists := a.rooms[roomIDStr]; exists {
		roomInfo.Phase = phase
	}
}

func (a *RoomManagerActor) handleGameRoomEmpty(ctx bollywood.Context, roomPID *bollywood.PID) {
	if roomPID == nil {
		return
	}
	roomIDStr := roomPID.String()
	a.mu.Lock() // Lock for writing
	roomInfo, exists := a.rooms[roomIDStr]
	pidToStop := (*bollywood.PID)(nil)
	if exists {
		fmt.Printf("RoomManagerActor %s: Room %s reported empty/finished. Removing and stopping.\n", a.selfPID, roomIDStr)
		if roomInfo != nil && roomInfo.PID != nil {
			pidToStop = roomInfo.PID
		}
		delete(a.rooms, roomIDStr)
	} // Else: Already removed, ignore.
	a.mu.Unlock() // Unlock before stopping actor

	if pidToStop != nil && a.engine != nil {
		a.engine.Stop(pidToStop) // Ensure engine is not nil before stopping
	}
}

func (a *RoomManagerActor) handlePlayerLeftRoom(ctx bollywood.Context, roomPID *bollywood.PID, sessionID string) {
	if roomPID == nil {
		return
	}
	roomIDStr := roomPID.String()
	a.mu.Lock()
	defer a.mu.Unlock()

	if roomInfo, exists := a.rooms[roomIDStr]; exists {
		if roomInfo.PlayerCount > 0 {
			roomInfo.PlayerCount--
			if sessionID != "" {
				delete(roomInfo.Sessions, sessionID)
			}
			fmt.Printf("RoomManagerActor %s: Player left room %s. Count decremented to %d.\n", a.selfPID, roomIDStr, roomInfo.PlayerCount)
		} else {
			fmt.Printf("WARN: RoomManagerActor %s: Received PlayerLeftRoom for %s but count is already 0.\n", a.selfPID, roomIDStr)
		}
	}
}

// handleGetRoomList now uses ctx.Reply if the request came via Ask.
func (a *RoomManagerActor) handleGetRoomList(ctx bollywood.Context) {
	a.mu.RLock() // Read lock is sufficient
	roomList := make(map[string]int)
	for roomID, roomInfo := range a.rooms {
		if roomInfo != nil && roomInfo.PID != nil {
			// Use the PID string (roomID) as the key for the response map
			roomList[roomID] = roomInfo.PlayerCount
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
