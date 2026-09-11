package game

import (
	"log/slog"
	"time"

	"github.com/lguibr/pongo/internal/actor"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

// handleStart is called when the actor receives the Started message.
func (a *GameActor) handleStart(ctx actor.Context) {
	// Only spawn broadcaster if one wasn't injected (e.g., for testing)
	if a.broadcasterPID == nil {
		broadcasterProps := actor.NewProps(NewBroadcasterProducer(a.selfPID))
		a.broadcasterPID = a.engine.Spawn(broadcasterProps)
		if a.broadcasterPID == nil {
			slog.Error("failed to spawn broadcaster; stopping room", "room", a.selfPID)
			a.engine.Stop(a.selfPID) // Stop self if broadcaster fails
			return
		}
		slog.Debug("room started", "room", a.selfPID, "broadcaster", a.broadcasterPID)
	} else {
		slog.Debug("room started with injected broadcaster", "room", a.selfPID, "broadcaster", a.broadcasterPID)
	}
	// Tickers are started when the first player joins or via internal test message
}

// startPhysicsTicker starts the physics ticker once.
func (a *GameActor) startPhysicsTicker(ctx actor.Context) {
	if a.physicsTicker != nil {
		return
	}
	a.physicsTicker = time.NewTicker(a.cfg.GameTickPeriod)
	a.stopPhysicsCh = make(chan struct{})
	go forwardTicks(a.physicsTicker.C, a.stopPhysicsCh, &a.physicsPending, a.engine, a.selfPID, GameTick{})
}

// startBroadcastTicker starts the broadcast ticker once. It never runs faster than physics.
func (a *GameActor) startBroadcastTicker(ctx actor.Context) {
	if a.broadcastTicker != nil {
		return
	}
	rate := a.cfg.BroadcastRateHz
	if rate <= 0 {
		rate = 40
	}
	interval := time.Second / time.Duration(rate)
	if interval < a.cfg.GameTickPeriod {
		interval = a.cfg.GameTickPeriod
	}
	if interval <= 0 {
		interval = 16 * time.Millisecond
	}
	a.broadcastTicker = time.NewTicker(interval)
	a.stopBroadcastCh = make(chan struct{})
	go forwardTicks(a.broadcastTicker.C, a.stopBroadcastCh, &a.broadcastPending, a.engine, a.selfPID, BroadcastTick{})
}

// forwardTicks turns ticker fires into coalesced actor messages until stop is closed.
// It uses only the values it is given and never reads the actor's fields.
func forwardTicks(ticks <-chan time.Time, stop <-chan struct{}, pending *actor.PendingTick, engine *actor.Engine, self *actor.PID, msg interface{}) {
	for {
		select {
		case <-stop:
			return
		case <-ticks:
			pending.Send(engine, self, msg)
		}
	}
}

// stopTickers stops both tickers and their forwarding goroutines.
func (a *GameActor) stopTickers() {
	if a.physicsTicker != nil {
		a.physicsTicker.Stop()
		close(a.stopPhysicsCh)
		a.physicsTicker = nil
	}
	if a.broadcastTicker != nil {
		a.broadcastTicker.Stop()
		close(a.stopBroadcastCh)
		a.broadcastTicker = nil
	}
}

// performCleanup ensures cleanup logic runs exactly once.
func (a *GameActor) performCleanup() {
	a.cleanupOnce.Do(func() {
		slog.Debug("room cleanup started", "room", a.selfPID)
		a.stopTickers()
		a.cleanupChildActorsAndConnections()
		a.cleanupPhasingTimers()
		if a.countdownTimer != nil {
			a.countdownTimer.Stop()
		}
		if a.roomCleanupTimer != nil {
			a.roomCleanupTimer.Stop()
		}
		for _, timer := range a.expiryTimers {
			timer.Stop()
		}
		for _, timer := range a.reconnectTimers {
			timer.Stop()
		}

		a.logPerformanceMetrics()
		slog.Debug("room cleanup complete", "room", a.selfPID)
	})
}

// cleanupChildActorsAndConnections closes clients, clears room state and stops the broadcaster.
func (a *GameActor) cleanupChildActorsAndConnections() {
	a.pendingUpdates = a.pendingUpdates[:0]

	for i := 0; i < utils.MaxPlayers; i++ {
		a.paddles[i] = nil
		if pInfo := a.players[i]; pInfo != nil {
			if !a.finalDeliveryHandedOff && pInfo.Client != nil {
				pInfo.Client.Close()
			}
			pInfo.Ws = nil
			pInfo.IsConnected = false
		}
		a.players[i] = nil
		a.playerConns[i] = nil
	}
	a.balls = make(map[int]*Ball)
	a.connToIndex = make(map[*websocket.Conn]int)

	if a.broadcasterPID != nil && a.engine != nil {
		a.engine.Stop(a.broadcasterPID)
		a.broadcasterPID = nil
	}
}

// cleanupPhasingTimers stops all active phasing timers.
func (a *GameActor) cleanupPhasingTimers() {
	for id, timer := range a.phasingTimers {
		if timer != nil {
			timer.Stop()
		}
		delete(a.phasingTimers, id)
	}
}

// checkGameOver checks if all bricks are destroyed and triggers the end sequence.
func (a *GameActor) checkGameOver(ctx actor.Context) {
	if a.gameOver || a.canvas == nil || a.canvas.Grid == nil {
		return
	}
	allBricksGone := true
	for _, row := range a.canvas.Grid {
		for _, cell := range row {
			if cell.Data != nil && cell.Data.Type == utils.Cells.Brick {
				allBricksGone = false
				break
			}
		}
		if !allBricksGone {
			break
		}
	}

	if allBricksGone {
		a.gameOver = true
		a.isStopping = true

		slog.Debug("all bricks destroyed", "room", a.selfPID)

		winnerIndex := -1
		highestScore := int32(-999999)
		var finalScores [utils.MaxPlayers]int32
		tie := false
		for i, p := range a.players {
			if p != nil {
				score := p.Score
				finalScores[i] = score
				if p.IsConnected {
					if score > highestScore {
						highestScore = score
						winnerIndex = i
						tie = false
					} else if score == highestScore && score > -999999 {
						tie = true
					}
				}
			} else {
				finalScores[i] = 0
			}
		}
		if tie {
			winnerIndex = -1
		}
		slog.Info("game over", "room", a.selfPID, "winner", winnerIndex, "score", highestScore)

		// Send any remaining pending updates immediately
		a.handleBroadcastTick(ctx)

		// Send GameOverMessage via Broadcaster
		if a.broadcasterPID != nil {
			gameOverMsg := GameOverMessage{
				MessageType: "gameOver", WinnerIndex: winnerIndex, FinalScores: finalScores,
				Reason: "All bricks destroyed", RoomPID: a.selfPID.String(),
			}
			a.finalDeliveryHandedOff = a.engine.Send(a.broadcasterPID, gameOverMsg, a.selfPID)
			a.broadcasterPID = nil // Broadcaster owns final delivery and its own termination.
		}

		// Notify RoomManager
		if a.roomManagerPID != nil {
			slog.Debug("notifying room manager of game over", "room", a.selfPID)
			a.engine.Send(a.roomManagerPID, GameRoomEmpty{RoomPID: a.selfPID}, nil)
		}

		// Initiate Self Stop (Cleanup will happen via Stopping message or panic recovery)
		if a.engine != nil && a.selfPID != nil {
			a.engine.Stop(a.selfPID)
		}
	}
}

// handleStopping is called when the actor receives the Stopping message.
func (a *GameActor) handleStopping(ctx actor.Context) {
	a.isStopping = true
	a.gameOver = true
	a.performCleanup()
	if a.roomManagerPID != nil {
		a.engine.Send(a.roomManagerPID, GameRoomEmpty{RoomPID: a.selfPID}, a.selfPID)
	}

}

// handleStopped is called when the actor receives the Stopped message.
func (a *GameActor) handleStopped(ctx actor.Context) {
	slog.Debug("room stopped", "room", a.selfPID)
}

// logPerformanceMetrics calculates and prints the average tick duration.
func (a *GameActor) logPerformanceMetrics() {
	if a.tickCount > 0 {
		avgDuration := a.tickDurationSum / time.Duration(a.tickCount)
		slog.Info("room metrics", "room", a.selfPID, "avgPhysicsTick", avgDuration, "ticks", a.tickCount)
	} else {
		slog.Debug("room metrics", "room", a.selfPID, "ticks", 0)
	}
}
