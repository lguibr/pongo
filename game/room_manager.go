package game

import (
	"crypto/rand"
	"encoding/hex"
	bollywood "github.com/lguibr/pongo/internal/actor"
	"github.com/lguibr/pongo/internal/transport"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
	"strings"
	"sync"
)

const maxRooms = 75

type RoomInfo struct {
	PID         *bollywood.PID
	PlayerCount int
	Code        string
	IsPublic    bool
	Phase       Phase
	Sessions    map[string]bool
}
type pendingAdmission struct {
	room     *bollywood.PID
	reply    *bollywood.PID
	session  string
	reserved bool
}
type RoomManagerActor struct {
	pending map[string]pendingAdmission
	engine  *bollywood.Engine
	cfg     utils.Config
	rooms   map[string]*RoomInfo
	mu      sync.RWMutex
	selfPID *bollywood.PID
}

func NewRoomManagerProducer(e *bollywood.Engine, cfg utils.Config) bollywood.Producer {
	return func() bollywood.Actor {
		return &RoomManagerActor{engine: e, cfg: cfg, rooms: make(map[string]*RoomInfo)}
	}
}
func (a *RoomManagerActor) generateRoomCode() string {
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return strings.ToUpper(hex.EncodeToString(b[:]))
}
func (a *RoomManagerActor) Receive(ctx bollywood.Context) {
	a.selfPID = ctx.Self()
	if a.pending == nil {
		a.pending = make(map[string]pendingAdmission)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	switch m := ctx.Message().(type) {
	case CreateRoomActorRequest:
		a.admit(m.ReplyTo, m.Conn, m.Client, m.SessionID, "", true, m.IsPublic, false)
	case JoinRoomActorRequest:
		a.admit(m.ReplyTo, m.Conn, m.Client, m.SessionID, m.Code, false, false, false)
	case QuickPlayActorRequest:
		a.admit(m.ReplyTo, m.Conn, m.Client, m.SessionID, "", false, true, true)
	case AdmissionRejected:
		if m.ReplyTo != nil {
			if pending, ok := a.pending[m.ReplyTo.ID]; ok && pending.room.ID == m.RoomPID.ID {
				delete(a.pending, m.ReplyTo.ID)
				if pending.reserved {
					a.release(m.RoomPID, pending.session, true)
				}
			}
		}
	case AdmissionAccepted:
		if m.ReplyTo != nil {
			delete(a.pending, m.ReplyTo.ID)
		}
	case PlayerLeftRoom:
		a.release(m.RoomPID, m.SessionID, false)
	case GameRoomEmpty:
		if m.RoomPID != nil {
			for id, pending := range a.pending {
				if pending.room.ID == m.RoomPID.ID {
					a.engine.Send(pending.reply, RoomJoinedResponse{MessageType: "roomJoined", Success: false, Reason: "Room closed during admission"}, a.selfPID)
					delete(a.pending, id)
				}
			}
			delete(a.rooms, m.RoomPID.ID)
			a.engine.Stop(m.RoomPID)
		}
	case RoomPhaseUpdate:
		if m.RoomPID != nil {
			if r := a.rooms[m.RoomPID.ID]; r != nil {
				r.Phase = m.Phase
			}
		}
	case GetRoomListRequest:
		rooms := make(map[string]int, len(a.rooms))
		for id, r := range a.rooms {
			rooms[id] = r.PlayerCount
		}
		ctx.Reply(RoomListResponse{Rooms: rooms})
	case bollywood.Stopping:
		for _, r := range a.rooms {
			a.engine.Stop(r.PID)
		}
		a.rooms = make(map[string]*RoomInfo)
	}
}
func (a *RoomManagerActor) release(pid *bollywood.PID, session string, rejected bool) {
	if pid == nil {
		return
	}
	r := a.rooms[pid.ID]
	if r == nil {
		return
	}
	if r.PlayerCount > 0 {
		r.PlayerCount--
	}
	delete(r.Sessions, session)
	if rejected && r.PlayerCount == 0 {
		delete(a.rooms, pid.ID)
		a.engine.Stop(pid)
	}
}
func (a *RoomManagerActor) admit(reply *bollywood.PID, ws *websocket.Conn, client *transport.Client, session, code string, create, public, quick bool) {
	if reply == nil || client == nil || ws == nil {
		return
	}
	fail := func(reason string) {
		a.engine.Send(reply, RoomJoinedResponse{MessageType: "roomJoined", Success: false, Reason: reason}, a.selfPID)
	}
	var room *RoomInfo
	if !create {
		for _, r := range a.rooms {
			if !quick && r.Code == code {
				room = r
				break
			}
			if quick && r.IsPublic && (r.PlayerCount < utils.MaxPlayers || r.Sessions[session]) {
				if room == nil || r.Phase < room.Phase {
					room = r
				}
			}
		}
	}
	if room == nil && !create && !quick {
		fail("Room not found")
		return
	}
	created := room == nil
	if created {
		if len(a.rooms) >= maxRooms {
			fail("Server is full")
			return
		}
		for {
			code = a.generateRoomCode()
			unique := true
			for _, r := range a.rooms {
				if r.Code == code {
					unique = false
					break
				}
			}
			if unique {
				break
			}
		}
		pid := a.engine.Spawn(bollywood.NewProps(NewGameActorProducer(a.engine, a.cfg, a.selfPID)))
		if pid == nil {
			fail("Server is stopping")
			return
		}
		room = &RoomInfo{PID: pid, Code: code, IsPublic: public, Phase: PhaseLobby, Sessions: make(map[string]bool)}
		a.rooms[pid.ID] = room
	}
	for _, pending := range a.pending {
		if session != "" && pending.room.ID == room.PID.ID && pending.session == session {
			fail("Session admission is pending")
			return
		}
	}
	reserved := session == "" || !room.Sessions[session]
	if reserved {
		if room.PlayerCount >= utils.MaxPlayers {
			fail("Room is full")
			return
		}
		room.PlayerCount++
		if session != "" {
			room.Sessions[session] = true
		}
	}
	responsePhase := room.Phase
	if quick && created {
		responsePhase = PhasePlaying
	}
	var response interface{} = RoomJoinedResponse{MessageType: "roomJoined", Success: true, RoomPID: room.PID.ID, Code: room.Code, Phase: phaseName(responsePhase)}
	if create {
		response = RoomCreatedResponse{MessageType: "roomCreated", RoomPID: room.PID.ID, Code: room.Code}
	}
	msg := AssignPlayerToRoom{WsConn: ws, Client: client, SessionID: session, ReplyTo: reply, Response: response, Reserved: reserved, AutoStart: quick && created}
	a.pending[reply.ID] = pendingAdmission{room: room.PID, reply: reply, session: session, reserved: reserved}
	if !a.engine.Send(room.PID, msg, a.selfPID) {
		delete(a.pending, reply.ID)
		if reserved {
			a.release(room.PID, session, true)
		}
		fail("Room is unavailable")
	}
}
func phaseName(p Phase) string {
	switch p {
	case PhasePlaying:
		return "playing"
	case PhaseCountingDown:
		return "countingDown"
	default:
		return "lobby"
	}
}
