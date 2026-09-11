package game

import (
	"log/slog"
	"time"

	"github.com/lguibr/pongo/internal/actor"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

// reconnectGrace is how long a disconnected player's slot, paddle and balls are held.
const reconnectGrace = 30 * time.Second

// emptyRoomGrace is how long an empty room waits for a player before it closes.
const emptyRoomGrace = 30 * time.Second

// handlePlayerDisconnect marks the player disconnected and holds the slot for
// the reconnect grace period. The room manager keeps counting the slot until
// the grace expires.
func (a *GameActor) handlePlayerDisconnect(ctx actor.Context, conn *websocket.Conn) {
	if conn == nil {
		return
	}
	playerIndex, found := a.connToIndex[conn]
	if !found || playerIndex < 0 || playerIndex >= utils.MaxPlayers || a.players[playerIndex] == nil || a.players[playerIndex].Ws != conn {
		if found {
			delete(a.connToIndex, conn)
		}
		return
	}
	player := a.players[playerIndex]
	if !player.IsConnected {
		return
	}

	slog.Debug("handling disconnect", "room", a.selfPID, "index", playerIndex, "remote", peerAddr(conn))
	player.IsConnected = false
	player.IsReady = false
	if paddle := a.paddles[playerIndex]; paddle != nil {
		paddle.Direction = ""
	}
	a.cancelCountdown(ctx)
	a.addUpdate(&PlayerLeft{MessageType: "playerLeft", Index: playerIndex})
	a.addUpdate(a.lobbyState())

	delete(a.connToIndex, conn)
	a.playerConns[playerIndex] = nil
	if a.broadcasterPID != nil {
		a.engine.Send(a.broadcasterPID, RemoveClient{Conn: conn}, a.selfPID)
	}
	slog.Info("player disconnected; holding slot", "room", a.selfPID, "index", playerIndex, "grace", reconnectGrace)

	if timer := a.reconnectTimers[playerIndex]; timer != nil {
		timer.Stop()
	}
	a.reconnectGeneration++
	player.DisconnectGeneration = a.reconnectGeneration
	engine, self, generation := a.engine, a.selfPID, player.DisconnectGeneration
	a.reconnectTimers[playerIndex] = time.AfterFunc(reconnectGrace, func() {
		engine.Send(self, stopReconnectTimerMsg{PlayerIndex: playerIndex, Generation: generation}, nil)
	})
}

// peerAddr names the peer for logs without failing on a closed or placeholder connection.
func peerAddr(conn *websocket.Conn) (addr string) {
	defer func() {
		if recover() != nil {
			addr = "unknown"
		}
	}()
	if ra := conn.RemoteAddr(); ra != nil {
		return ra.String()
	}
	return "unknown"
}

// handleStopReconnectTimerMsg removes a player whose reconnect grace expired
// and releases the slot to the room manager.
func (a *GameActor) handleStopReconnectTimerMsg(ctx actor.Context, playerIndex int) {
	if playerIndex < 0 || playerIndex >= utils.MaxPlayers || a.players[playerIndex] == nil || a.players[playerIndex].IsConnected {
		return
	}
	slog.Info("reconnect grace expired; player removed", "room", a.selfPID, "index", playerIndex)
	a.addUpdate(&PlayerLeft{MessageType: "playerLeft", Index: playerIndex})
	a.paddles[playerIndex] = nil
	for id, ball := range a.balls {
		if ball.OwnerIndex != playerIndex {
			continue
		}
		if ball.IsPermanent {
			ball.OwnerIndex = -1
			a.addUpdate(&BallOwnershipChange{MessageType: "ballOwnerChanged", ID: id, NewOwnerIndex: -1})
		} else {
			a.handleDestroyExpiredBall(ctx, id)
		}
	}

	sessionID := a.players[playerIndex].SessionID
	a.playerConns[playerIndex] = nil
	a.players[playerIndex] = nil
	delete(a.reconnectTimers, playerIndex)
	if a.roomManagerPID != nil && a.selfPID != nil {
		a.engine.Send(a.roomManagerPID, PlayerLeftRoom{RoomPID: a.selfPID, SessionID: sessionID}, nil)
	}

	if !a.anyPlayerConnected() && !a.gameOver {
		slog.Debug("room empty; cleanup timer started", "room", a.selfPID)
		if a.roomCleanupTimer != nil {
			a.roomCleanupTimer.Stop()
		}
		a.cleanupGeneration++
		engine, self, generation := a.engine, a.selfPID, a.cleanupGeneration
		a.roomCleanupTimer = time.AfterFunc(emptyRoomGrace, func() {
			engine.Send(self, RoomCleanupTimeout{Generation: generation}, nil)
		})
	}
	a.addUpdate(a.lobbyState())
}

// handleRoomCleanupTimeout closes a room that stayed empty for the whole grace period.
func (a *GameActor) handleRoomCleanupTimeout(ctx actor.Context) {
	if a.anyPlayerConnected() || a.gameOver {
		slog.Debug("cleanup timer expired but room is active; ignored", "room", a.selfPID)
		return
	}
	slog.Info("empty room closed", "room", a.selfPID)
	if a.roomManagerPID != nil && a.selfPID != nil {
		a.engine.Send(a.roomManagerPID, GameRoomEmpty{RoomPID: a.selfPID}, nil)
		return
	}
	slog.Error("room has no manager to notify; stopping", "room", a.selfPID)
	if a.selfPID != nil {
		a.engine.Stop(a.selfPID)
	}
}

// anyPlayerConnected reports whether any slot has a live connection.
func (a *GameActor) anyPlayerConnected() bool {
	for _, p := range a.players {
		if p != nil && p.IsConnected {
			return true
		}
	}
	return false
}
