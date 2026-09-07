package game

import (
	"encoding/json"
	bollywood "github.com/lguibr/pongo/internal/actor"
	"github.com/lguibr/pongo/internal/transport"
	"golang.org/x/net/websocket"
)

// BroadcasterActor encodes each room batch once. Each client owns its writer.
type BroadcasterActor struct {
	clients      map[*websocket.Conn]*transport.Client
	gameActorPID *bollywood.PID
}

func NewBroadcasterProducer(room *bollywood.PID) bollywood.Producer {
	return func() bollywood.Actor {
		return &BroadcasterActor{clients: make(map[*websocket.Conn]*transport.Client), gameActorPID: room}
	}
}
func (a *BroadcasterActor) Receive(ctx bollywood.Context) {
	switch msg := ctx.Message().(type) {
	case AddClient:
		if msg.Client != nil {
			a.clients[msg.Conn] = msg.Client
		}
	case RemoveClient:
		delete(a.clients, msg.Conn)
	case BroadcastUpdatesCommand:
		updates := msg.Updates
		if updates == nil {
			updates = []interface{}{}
		}
		data, err := json.Marshal(GameUpdatesBatch{MessageType: "gameUpdates", Updates: updates})
		if err != nil {
			return
		}
		text := string(data)
		for ws, c := range a.clients {
			if c.SendText(text) != nil {
				delete(a.clients, ws)
				ctx.Engine().Send(a.gameActorPID, PlayerDisconnect{WsConn: ws}, ctx.Self())
			}
		}
	case GameOverMessage:
		for ws, c := range a.clients {
			_ = c.Finish(msg)
			delete(a.clients, ws)
		}
		// All final messages are now queued ahead of close in each writer.
		ctx.Engine().Stop(ctx.Self())
	case bollywood.Stopping:
		for ws, c := range a.clients {
			c.Close()
			delete(a.clients, ws)
		}
	}
}
