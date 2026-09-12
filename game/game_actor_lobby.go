package game

import (
	"log/slog"
	"time"

	"github.com/lguibr/pongo/internal/actor"
	"golang.org/x/net/websocket"
)

// countdownSeconds is the length of the pre-game countdown.
const countdownSeconds = 3

// lobbyState lists the connected players and their readiness.
func (a *GameActor) lobbyState() *LobbyStateUpdate {
	state := &LobbyStateUpdate{MessageType: "lobbyState", Players: make([]LobbyPlayerState, 0)}
	for _, p := range a.players {
		if p != nil && p.IsConnected {
			state.Players = append(state.Players, LobbyPlayerState{Index: p.Index, IsReady: p.IsReady})
		}
	}
	return state
}

// handlePlayerReady records readiness, publishes the lobby and starts or cancels the countdown.
func (a *GameActor) handlePlayerReady(ctx actor.Context, wsConn *websocket.Conn, isReady bool) {
	if wsConn == nil {
		return
	}
	playerIndex, found := a.connToIndex[wsConn]
	if !found || a.players[playerIndex] == nil {
		return
	}
	slog.Debug("player readiness changed", "room", a.selfPID, "index", playerIndex, "ready", isReady)
	a.players[playerIndex].IsReady = isReady

	state := a.lobbyState()
	a.addUpdate(state)
	allReady := len(state.Players) > 0
	for _, p := range state.Players {
		if !p.IsReady {
			allReady = false
			break
		}
	}
	if !allReady {
		a.cancelCountdown(ctx)
		return
	}
	if a.phase == PhaseLobby {
		slog.Debug("all players ready; countdown started", "room", a.selfPID)
		a.startCountdown(ctx)
	}
}

// startCountdown moves the lobby into the countdown.
func (a *GameActor) startCountdown(ctx actor.Context) {
	if a.phase != PhaseLobby {
		return
	}
	a.phase = PhaseCountingDown
	a.countdownGeneration++
	a.engine.Send(a.roomManagerPID, RoomPhaseUpdate{RoomPID: a.selfPID, Phase: a.phase}, a.selfPID)
	a.handleCountdownTick(ctx, countdownSeconds)
}

// handleCountdownTick publishes the remaining seconds and schedules the next tick or the start.
func (a *GameActor) handleCountdownTick(ctx actor.Context, secondsRemaining int) {
	if a.phase != PhaseCountingDown {
		return // Countdown was cancelled or game started
	}
	a.addUpdate(&GameStartCountdown{MessageType: "gameStartCountdown", Seconds: secondsRemaining})
	if secondsRemaining <= 0 {
		return
	}
	generation := a.countdownGeneration
	engine, self := a.engine, a.selfPID
	a.countdownTimer = time.AfterFunc(1*time.Second, func() {
		if secondsRemaining > 1 {
			engine.Send(self, CountdownTick{SecondsRemaining: secondsRemaining - 1, Generation: generation}, nil)
		} else {
			engine.Send(self, startGameMsg{Generation: generation}, nil)
		}
	})
}

// cancelCountdown returns a counting-down room to the lobby.
func (a *GameActor) cancelCountdown(ctx actor.Context) {
	if a.phase != PhaseCountingDown {
		return
	}
	a.countdownGeneration++
	if a.countdownTimer != nil {
		a.countdownTimer.Stop()
		a.countdownTimer = nil
	}
	a.phase = PhaseLobby
	a.addUpdate(&GameStartCancelled{MessageType: "gameStartCancelled", Reason: "Lobby membership or readiness changed"})
	a.engine.Send(a.roomManagerPID, RoomPhaseUpdate{RoomPID: a.selfPID, Phase: a.phase}, a.selfPID)
}

// startGame moves the room from the countdown into play.
func (a *GameActor) startGame(ctx actor.Context) {
	slog.Debug("starting game", "room", a.selfPID, "phase", a.phase)
	if a.phase != PhaseCountingDown {
		return
	}
	a.enterPlaying(ctx)
}

// handleForceStartGame starts play immediately (Quick Play) once the grid exists.
func (a *GameActor) handleForceStartGame(ctx actor.Context) {
	if !a.gridInitialized {
		a.forceStartPending = true
		return
	}
	a.forceStartPending = false
	slog.Debug("force-starting game", "room", a.selfPID, "phase", a.phase)
	if a.phase == PhasePlaying {
		return
	}
	if a.countdownTimer != nil {
		a.countdownTimer.Stop()
		a.countdownTimer = nil
	}
	a.enterPlaying(ctx)
}

// enterPlaying switches to play, tells clients and the room manager, and starts physics.
func (a *GameActor) enterPlaying(ctx actor.Context) {
	a.phase = PhasePlaying
	a.addUpdate(&GameStarted{MessageType: "gameStarted"})
	if a.roomManagerPID != nil && a.engine != nil && a.selfPID != nil {
		a.engine.Send(a.roomManagerPID, RoomPhaseUpdate{RoomPID: a.selfPID, Phase: PhasePlaying}, a.selfPID)
	}
	a.startPhysicsTicker(ctx)
}

// phaseToString converts the Phase enum to the string the client expects.
func (a *GameActor) phaseToString() string {
	switch a.phase {
	case PhaseCountingDown:
		return "countingDown"
	case PhasePlaying:
		return "playing"
	default:
		return "lobby"
	}
}
