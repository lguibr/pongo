package game

import (
	"log/slog"
	"runtime/debug"

	"github.com/lguibr/pongo/internal/actor"
	"github.com/lguibr/pongo/internal/transport"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

// handleAdmission validates before sending success, and rolls back reservations
// if the connection disappears before it becomes a room member.
func (a *GameActor) handleAdmission(ctx actor.Context, m AssignPlayerToRoom) {
	reject := func(reason string) {
		a.engine.Send(a.roomManagerPID, AdmissionRejected{RoomPID: a.selfPID, SessionID: m.SessionID, Reserved: m.Reserved, ReplyTo: m.ReplyTo}, a.selfPID)
		a.engine.Send(m.ReplyTo, RoomJoinedResponse{MessageType: "roomJoined", Success: false, Reason: reason}, a.selfPID)
	}
	if a.isStopping || a.gameOver {
		reject("Room is closing")
		return
	}
	if m.Client == nil || m.WsConn == nil {
		reject("Invalid connection")
		return
	}
	select {
	case <-m.Client.Done():
		reject("Connection closed")
		return
	default:
	}
	available := false
	for _, p := range a.players {
		if p == nil {
			available = true
			continue
		}
		if m.SessionID != "" && p.SessionID == m.SessionID {
			if p.IsConnected {
				reject("Session already connected")
				return
			}
			available = true
		}
	}
	if !available {
		reject("Room is full")
		return
	}
	if m.AutoStart {
		a.forceStartPending = true
	}
	admitted := a.handlePlayerConnect(ctx, m.WsConn, m.SessionID, m.Client, m.ReplyTo, m.Response)
	if !admitted {
		reject("Admission failed")
		return
	}
	// A committed player slot owns its reservation, including reconnect grace.
	a.engine.Send(a.roomManagerPID, AdmissionAccepted{RoomPID: a.selfPID, ReplyTo: m.ReplyTo}, a.selfPID)
}

// handlePlayerConnect commits a player to a slot, sends the initial state and
// announces the player to the room. It reports whether a slot was committed.
func (a *GameActor) handlePlayerConnect(ctx actor.Context, ws *websocket.Conn, sessionID string, client *transport.Client, replyTo *actor.PID, response interface{}) (admitted bool) {
	a.cancelRoomCleanup()

	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic while admitting player", "room", a.selfPID, "panic", r, "stack", string(debug.Stack()))
			// Close the connection that caused the panic to avoid inconsistent state
			if ws != nil {
				client.Close()
				if admitted {
					a.handlePlayerDisconnect(ctx, ws)
				}
			}
		}
	}()

	if ws == nil {
		slog.Error("admission without websocket connection", "room", a.selfPID)
		return
	}
	remoteAddr := ws.RemoteAddr().String()
	if _, ok := a.connToIndex[ws]; ok {
		slog.Warn("duplicate admission ignored", "room", a.selfPID, "remote", remoteAddr)
		return
	}
	playerIndex := a.findSlot(sessionID)
	if playerIndex == -1 {
		slog.Warn("room full; connection rejected", "room", a.selfPID, "remote", remoteAddr)
		client.Close()
		return
	}
	a.stopReconnectTimer(playerIndex)
	score := a.joiningScore(playerIndex)
	if !a.ensureGrid(ctx) {
		slog.Error("player joining before grid initialization", "room", a.selfPID, "index", playerIndex)
		client.Close()
		return
	}

	a.attachPlayer(playerIndex, ws, sessionID, client, score)
	admitted = true
	if !a.engine.Send(replyTo, AssignRoomResponse{RoomPID: a.selfPID}, a.selfPID) {
		a.handlePlayerDisconnect(ctx, ws)
		return
	}
	a.cancelCountdown(ctx)
	if response != nil && client.Send(response) != nil {
		a.handlePlayerDisconnect(ctx, ws)
		return
	}
	if a.forceStartPending {
		a.handleForceStartGame(ctx)
	}

	// Queue initial state after the handler has received its room identity.
	assignment := PlayerAssignmentMessage{MessageType: "playerAssignment", PlayerIndex: playerIndex, Phase: a.phaseToString()}
	if err := client.Send(assignment); err != nil {
		slog.Warn("failed to send player assignment", "room", a.selfPID, "index", playerIndex, "remote", remoteAddr, "err", err)
		a.handlePlayerDisconnect(ctx, ws)
		return
	}
	if err := client.Send(a.initialEntities()); err != nil {
		slog.Warn("failed to send initial state", "room", a.selfPID, "index", playerIndex, "remote", remoteAddr, "err", err)
		a.handlePlayerDisconnect(ctx, ws)
		return
	}

	a.announcePlayer(ctx, playerIndex)
	if a.broadcasterPID != nil {
		a.engine.Send(a.broadcasterPID, AddClient{Conn: ws, Client: client}, a.selfPID)
	} else {
		slog.Warn("room has no broadcaster; client will not receive updates", "room", a.selfPID, "remote", remoteAddr)
	}
	if err := client.Send(GameUpdatesBatch{MessageType: "gameUpdates", Updates: []interface{}{a.fullGridUpdate()}}); err != nil {
		a.handlePlayerDisconnect(ctx, ws)
		return
	}
	if a.forceStartPending {
		a.handleForceStartGame(ctx)
	}
	return
}

// cancelRoomCleanup stops a pending empty-room close because a player is joining.
func (a *GameActor) cancelRoomCleanup() {
	if a.roomCleanupTimer == nil {
		return
	}
	slog.Debug("player joining; room cleanup cancelled", "room", a.selfPID)
	a.roomCleanupTimer.Stop()
	a.cleanupGeneration++
	a.roomCleanupTimer = nil
}

// findSlot returns the slot held for a returning session, else the first free slot, else -1.
func (a *GameActor) findSlot(sessionID string) int {
	for i, p := range a.players {
		if p != nil && sessionID != "" && !p.IsConnected && p.SessionID == sessionID {
			slog.Debug("player reconnecting", "room", a.selfPID, "index", i)
			return i
		}
	}
	for i, p := range a.players {
		if p == nil {
			return i
		}
	}
	return -1
}

// stopReconnectTimer cancels the grace timer of a returning player.
func (a *GameActor) stopReconnectTimer(index int) {
	if timer := a.reconnectTimers[index]; timer != nil {
		timer.Stop()
		delete(a.reconnectTimers, index)
		slog.Debug("reconnect timer stopped", "room", a.selfPID, "index", index)
	}
}

// joiningScore is the average score of the other connected players, or the configured start score.
func (a *GameActor) joiningScore(index int) int32 {
	var total int32
	connected := 0
	for i, p := range a.players {
		if p != nil && i != index && p.IsConnected {
			total += p.Score
			connected++
		}
	}
	if connected == 0 {
		return int32(a.cfg.InitialScore)
	}
	return total / int32(connected)
}

// ensureGrid fills the grid for the first player and starts broadcasting.
// It reports false only if an initialized room has lost its grid.
func (a *GameActor) ensureGrid(ctx actor.Context) bool {
	if a.gridInitialized {
		return a.canvas != nil && a.canvas.Grid != nil
	}
	slog.Debug("first player joined; grid initialized", "room", a.selfPID)
	if a.canvas == nil {
		a.canvas = NewCanvas(a.cfg.CanvasSize, a.cfg.GridSize)
	}
	a.canvas.Grid.FillSymmetrical(a.cfg)
	a.gridInitialized = true
	a.gridDirty = true
	a.startBroadcastTicker(ctx)
	return true
}

// attachPlayer binds the connection to the slot, reusing a returning player's state.
func (a *GameActor) attachPlayer(index int, ws *websocket.Conn, sessionID string, client *transport.Client, score int32) {
	player := a.players[index]
	if player != nil {
		player.Ws = ws
		player.IsConnected = true
		if a.paddles[index] == nil {
			slog.Warn("reconnecting player had no paddle; recreated", "room", a.selfPID, "index", index)
			a.paddles[index] = NewPaddle(a.cfg, index)
		}
	} else {
		data := NewPlayer(a.canvas, index)
		player = &playerInfo{Index: index, ID: data.Id, Color: data.Color, Ws: ws, IsConnected: true, SessionID: sessionID, Score: score}
		a.players[index] = player
		a.paddles[index] = NewPaddle(a.cfg, index)
	}
	player.Client = client
	a.connToIndex[ws] = index
	a.playerConns[index] = ws
}

// playerData is the wire representation of a player slot.
func playerData(p *playerInfo) Player {
	return Player{Index: p.Index, Id: p.ID, Color: p.Color, Score: p.Score}
}

// initialEntities describes connected players, all paddles and all balls for a joining client.
func (a *GameActor) initialEntities() InitialPlayersAndBallsState {
	players := make([]*Player, 0, utils.MaxPlayers)
	paddles := make([]InitialPaddleState, 0, utils.MaxPlayers)
	balls := make([]InitialBallState, 0, len(a.balls))
	for i := 0; i < utils.MaxPlayers; i++ {
		if p := a.players[i]; p != nil && p.IsConnected {
			data := playerData(p)
			players = append(players, &data)
		}
		if paddle := a.paddles[i]; paddle != nil {
			x, y := mapToR3FCoords(paddle.X+paddle.Width/2, paddle.Y+paddle.Height/2, a.cfg.CanvasSize)
			paddles = append(paddles, InitialPaddleState{Paddle: *paddle, R3fX: x, R3fY: y})
		}
	}
	for _, ball := range a.balls {
		if ball != nil {
			x, y := mapToR3FCoords(ball.X, ball.Y, a.cfg.CanvasSize)
			balls = append(balls, InitialBallState{Ball: *ball, R3fX: x, R3fY: y})
		}
	}
	return InitialPlayersAndBallsState{MessageType: "initialPlayersAndBallsState", Players: players, Paddles: paddles, Balls: balls}
}

// announcePlayer tells the room about the player, gives them a permanent ball
// if they have none, and publishes the lobby state. New and returning players
// are both announced so every client has current coordinates.
func (a *GameActor) announcePlayer(ctx actor.Context, index int) {
	if paddle := a.paddles[index]; paddle != nil {
		x, y := mapToR3FCoords(paddle.X+paddle.Width/2, paddle.Y+paddle.Height/2, a.cfg.CanvasSize)
		a.addUpdate(&PlayerJoined{MessageType: "playerJoined", Player: playerData(a.players[index]), Paddle: *paddle, R3fX: x, R3fY: y})
		slog.Debug("player joined", "room", a.selfPID, "index", index)
	} else {
		slog.Warn("player joined without paddle; not announced", "room", a.selfPID, "index", index)
	}
	if !a.hasPermanentBall(index) {
		a.spawnBall(ctx, index, 0, 0, 0, true, false)
	}
	a.addUpdate(a.lobbyState())
}

// hasPermanentBall reports whether the player already owns a permanent ball.
func (a *GameActor) hasPermanentBall(owner int) bool {
	for _, ball := range a.balls {
		if ball.IsPermanent && ball.OwnerIndex == owner {
			return true
		}
	}
	return false
}
