package server

import (
	"encoding/json"
	"github.com/lguibr/pongo/game"
	bollywood "github.com/lguibr/pongo/internal/actor"
	"github.com/lguibr/pongo/internal/transport"
	"golang.org/x/net/websocket"
	"time"
)

const maxInputBytes = 4096
const inputBurst = 120

type ConnectionHandlerArgs struct {
	Conn           *websocket.Conn
	Engine         *bollywood.Engine
	RoomManagerPID *bollywood.PID
	Done           chan struct{}
}
type ConnectionHandlerActor struct {
	handshakeTimer                        *time.Timer
	admissionTimer                        *time.Timer
	admissionGeneration                   uint64
	conn                                  *websocket.Conn
	client                                *transport.Client
	engine                                *bollywood.Engine
	roomManagerPID, selfPID, gameActorPID *bollywood.PID
	done                                  chan struct{}
	readDone                              chan struct{}
	pending                               bool
	sessionID                             string
}
type connectionClosed struct{}
type handshakeTimeout struct{}
type admissionTimeout struct{ generation uint64 }

func NewConnectionHandlerProducer(args ConnectionHandlerArgs) bollywood.Producer {
	return func() bollywood.Actor {
		return &ConnectionHandlerActor{conn: args.Conn, engine: args.Engine, roomManagerPID: args.RoomManagerPID, done: args.Done, readDone: make(chan struct{})}
	}
}
func (a *ConnectionHandlerActor) Receive(ctx bollywood.Context) {
	a.selfPID = ctx.Self()
	switch m := ctx.Message().(type) {
	case bollywood.Started:
		a.conn.MaxPayloadBytes = maxInputBytes
		a.client = transport.New(a.conn)
		engine, self := a.engine, a.selfPID
		a.handshakeTimer = time.AfterFunc(10*time.Second, func() { engine.Send(self, handshakeTimeout{}, nil) })
		go a.readLoop(a.conn, a.engine, a.selfPID)
	case game.AssignRoomResponse:
		a.pending = false
		if a.admissionTimer != nil {
			a.admissionTimer.Stop()
		}
		a.gameActorPID = m.RoomPID
	case game.RoomJoinedResponse:
		a.pending = false
		a.gameActorPID = nil
		if a.admissionTimer != nil {
			a.admissionTimer.Stop()
		}
		if a.client.Send(m) != nil {
			a.engine.Stop(a.selfPID)
		}
	case game.InternalReadLoopMsg:
		var h game.MessageHeader
		if json.Unmarshal(m.Payload, &h) != nil {
			return
		}
		switch h.MessageType {
		case "createRoom", "joinRoom", "quickPlay":
			if a.pending || a.gameActorPID != nil {
				return
			}
			var req struct {
				SessionID string `json:"sessionId"`
				Code      string `json:"code"`
				IsPublic  bool   `json:"isPublic"`
			}
			if json.Unmarshal(m.Payload, &req) != nil {
				return
			}
			if req.SessionID == "" || len(req.SessionID) > 128 {
				return
			}
			a.sessionID = req.SessionID
			a.pending = true
			a.admissionGeneration++
			generation := a.admissionGeneration
			engine, self := a.engine, a.selfPID
			a.admissionTimer = time.AfterFunc(5*time.Second, func() { engine.Send(self, admissionTimeout{generation: generation}, nil) })
			var request interface{}
			switch h.MessageType {
			case "createRoom":
				request = game.CreateRoomActorRequest{ReplyTo: a.selfPID, Conn: a.conn, Client: a.client, SessionID: req.SessionID, IsPublic: req.IsPublic}
			case "joinRoom":
				request = game.JoinRoomActorRequest{ReplyTo: a.selfPID, Conn: a.conn, Client: a.client, SessionID: req.SessionID, Code: req.Code}
			case "quickPlay":
				request = game.QuickPlayActorRequest{ReplyTo: a.selfPID, Conn: a.conn, Client: a.client, SessionID: req.SessionID}
			}
			if !a.engine.TrySend(a.roomManagerPID, request, a.selfPID) {
				a.engine.Stop(a.selfPID)
			}
		case "direction":
			if a.gameActorPID != nil {
				if !a.engine.TrySend(a.gameActorPID, game.ForwardedPaddleDirection{WsConn: a.conn, Direction: m.Payload}, a.selfPID) {
					a.engine.Stop(a.selfPID)
				}
			}
		case "playerReady":
			var r game.PlayerReadyRequest
			if a.gameActorPID != nil && json.Unmarshal(m.Payload, &r) == nil {
				if !a.engine.TrySend(a.gameActorPID, game.ForwardedPlayerReady{WsConn: a.conn, IsReady: r.IsReady}, a.selfPID) {
					a.engine.Stop(a.selfPID)
				}
			}
		}
	case handshakeTimeout:
		if a.gameActorPID == nil && !a.pending {
			a.engine.Stop(a.selfPID)
		}
	case admissionTimeout:
		if a.pending && m.generation == a.admissionGeneration {
			a.engine.Stop(a.selfPID)
		}
	case connectionClosed:
		a.engine.Stop(a.selfPID)
	case bollywood.Stopping:
		if a.handshakeTimer != nil {
			a.handshakeTimer.Stop()
		}
		if a.admissionTimer != nil {
			a.admissionTimer.Stop()
		}
		if a.client != nil {
			a.client.Close()
		} else {
			_ = a.conn.Close()
		}
		if a.gameActorPID != nil {
			a.engine.Send(a.gameActorPID, game.PlayerDisconnect{WsConn: a.conn}, a.selfPID)
		}
	case bollywood.Stopped:
		if a.done != nil {
			close(a.done)
		}
	}
}

// The read goroutine only uses captured immutable references.
func (a *ConnectionHandlerActor) readLoop(conn *websocket.Conn, e *bollywood.Engine, self *bollywood.PID) {
	defer close(a.readDone)
	defer e.Send(self, connectionClosed{}, nil)
	tokens := float64(inputBurst)
	last := time.Now()
	for {
		// Idle players may watch indefinitely; transport close interrupts Receive.
		var raw []byte
		if websocket.Message.Receive(conn, &raw) != nil {
			return
		}
		now := time.Now()
		tokens += now.Sub(last).Seconds() * 60
		last = now
		if tokens > inputBurst {
			tokens = inputBurst
		}
		tokens--
		if tokens < 0 || len(raw) > maxInputBytes {
			return
		}
		if !e.TrySend(self, game.InternalReadLoopMsg{Payload: raw}, nil) {
			return
		}
	}
}
