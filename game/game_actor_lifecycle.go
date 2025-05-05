// File: game/game_actor_lifecycle.go
package game

import (
	"fmt"
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket" // Added import
)

// handleStart is called when the actor receives the Started message.
func (a *GameActor) handleStart(ctx bollywood.Context) {
	// Only spawn broadcaster if one wasn't injected (e.g., for testing)
	if a.broadcasterPID == nil {
		broadcasterProps := bollywood.NewProps(NewBroadcasterProducer(a.selfPID))
		a.broadcasterPID = a.engine.Spawn(broadcasterProps)
		if a.broadcasterPID == nil {
			fmt.Printf("FATAL: GameActor %s failed to spawn BroadcasterActor. Stopping self.\n", a.selfPID)
			a.engine.Stop(a.selfPID) // Stop self if broadcaster fails
			return
		}
		fmt.Printf("GameActor %s: Started. Spawned Broadcaster: %s.\n", a.selfPID, a.broadcasterPID)
	} else {
		fmt.Printf("GameActor %s: Started. Using pre-assigned Broadcaster: %s.\n", a.selfPID, a.broadcasterPID)
	}
	// Tickers are started when the first player joins or via internal test message
}

// startTickers starts the physics and broadcast tickers.
// Now callable internally via message or by handlePlayerConnect.
func (a *GameActor) startTickers(ctx bollywood.Context) {
	a.tickerMu.Lock()
	defer a.tickerMu.Unlock() // Ensure unlock happens

	if a.physicsTicker == nil {
		a.physicsTicker = time.NewTicker(a.cfg.GameTickPeriod)
		// Ensure channel is fresh if previously stopped
		select {
		case <-a.stopPhysicsCh: a.stopPhysicsCh = make(chan struct{})
		default:
		}
		stopCh := a.stopPhysicsCh
		tickerCh := a.physicsTicker.C
		// Unlock before starting goroutine to avoid deadlock if goroutine accesses mutex
		// a.tickerMu.Unlock() // Moved to defer

		go func() {
			defer func() { if r := recover(); r != nil { fmt.Printf("PANIC recovered in GameActor %s Physics Ticker: %v\n", a.selfPID, r) } }()
			for {
				select {
				case <-stopCh: return
				case _, ok := <-tickerCh:
					if !ok { return }
					if a.isStopping.Load() || a.gameOver.Load() { return }
					currentEngine := a.engine; currentSelfPID := a.selfPID
					if currentEngine != nil && currentSelfPID != nil { currentEngine.Send(currentSelfPID, GameTick{}, nil) } else { return }
				}
			}
		}()
	} // else { a.tickerMu.Unlock() } // Removed redundant unlock

	// a.tickerMu.Lock() // Lock already held
	if a.broadcastTicker == nil {
		broadcastInterval := time.Second / time.Duration(a.cfg.BroadcastRateHz)
		if broadcastInterval <= 0 { broadcastInterval = 16 * time.Millisecond } // Default to ~60Hz if config is 0 or less
		a.broadcastTicker = time.NewTicker(broadcastInterval)
		// Ensure channel is fresh if previously stopped
		select {
		case <-a.stopBroadcastCh: a.stopBroadcastCh = make(chan struct{})
		default:
		}
		stopCh := a.stopBroadcastCh
		tickerCh := a.broadcastTicker.C
		// Unlock before starting goroutine
		// a.tickerMu.Unlock() // Moved to defer

		go func() {
			defer func() { if r := recover(); r != nil { fmt.Printf("PANIC recovered in GameActor %s Broadcast Ticker: %v\n", a.selfPID, r) } }()
			for {
				select {
				case <-stopCh: return
				case _, ok := <-tickerCh:
					if !ok { return }
					if a.isStopping.Load() || a.gameOver.Load() { return }
					currentEngine := a.engine; currentSelfPID := a.selfPID
					if currentEngine != nil && currentSelfPID != nil { currentEngine.Send(currentSelfPID, BroadcastTick{}, nil) } else { return }
				}
			}
		}()
	} // else { a.tickerMu.Unlock() } // Removed redundant unlock
}

// stopTickers stops the physics and broadcast tickers safely using the mutex.
func (a *GameActor) stopTickers() {
	a.tickerMu.Lock()
	defer a.tickerMu.Unlock()

	if a.physicsTicker != nil {
		a.physicsTicker.Stop()
		select { case <-a.stopPhysicsCh: default: close(a.stopPhysicsCh) }
		a.physicsTicker = nil
	}
	if a.broadcastTicker != nil {
		a.broadcastTicker.Stop()
		select { case <-a.stopBroadcastCh: default: close(a.stopBroadcastCh) }
		a.broadcastTicker = nil
	}
}

// performCleanup ensures cleanup logic runs exactly once.
func (a *GameActor) performCleanup() {
	a.cleanupOnce.Do(func() {
		fmt.Printf("GameActor %s: Performing cleanup...\n", a.selfPID)
		a.stopTickers()
		a.cleanupChildActorsAndConnections()
		a.cleanupPhasingTimers() // Clean up phasing timers
		a.logPerformanceMetrics()
		fmt.Printf("GameActor %s: Cleanup complete.\n", a.selfPID)
	})
}

// cleanupChildActorsAndConnections stops all managed actors and cleans caches.
func (a *GameActor) cleanupChildActorsAndConnections() {
	a.updatesMu.Lock()
	a.pendingUpdates = a.pendingUpdates[:0]
	a.updatesMu.Unlock()

	paddlesToStop := make([]*bollywood.PID, 0, utils.MaxPlayers)
	ballsToStop := make([]*bollywood.PID, 0, len(a.ballActors))
	broadcasterToStop := a.broadcasterPID

	for i := 0; i < utils.MaxPlayers; i++ {
		if pid := a.paddleActors[i]; pid != nil { paddlesToStop = append(paddlesToStop, pid); a.paddleActors[i] = nil }
		a.paddles[i] = nil
		if pInfo := a.players[i]; pInfo != nil {
			if pInfo.Ws != nil { delete(a.connToIndex, pInfo.Ws) }
			pInfo.Ws = nil; pInfo.IsConnected = false
		}
		a.players[i] = nil; a.playerConns[i] = nil
	}

	for ballID, pid := range a.ballActors {
		if pid != nil { ballsToStop = append(ballsToStop, pid) }
		delete(a.ballActors, ballID); delete(a.balls, ballID)
	}

	if len(a.connToIndex) > 0 { a.connToIndex = make(map[*websocket.Conn]int) }

	currentEngine := a.engine
	if currentEngine != nil {
		if broadcasterToStop != nil && a.broadcasterPID != nil { currentEngine.Stop(broadcasterToStop); a.broadcasterPID = nil }
		for _, pid := range paddlesToStop { if pid != nil { currentEngine.Stop(pid) } }
		for _, pid := range ballsToStop { if pid != nil { currentEngine.Stop(pid) } }
	} else {
		fmt.Printf("WARN: GameActor %s: Engine is nil during cleanupChildActorsAndConnections.\n", a.selfPID)
	}
}

// cleanupPhasingTimers stops all active phasing timers.
func (a *GameActor) cleanupPhasingTimers() {
	a.phasingTimersMu.Lock()
	defer a.phasingTimersMu.Unlock()
	for id, timer := range a.phasingTimers {
		if timer != nil {
			timer.Stop()
		}
		delete(a.phasingTimers, id)
	}
}

// checkGameOver checks if all bricks are destroyed and triggers the end sequence.
func (a *GameActor) checkGameOver(ctx bollywood.Context) {
	if a.gameOver.Load() || a.canvas == nil || a.canvas.Grid == nil {
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
		if !a.gameOver.CompareAndSwap(false, true) {
			return // Already processing game over
		}
		// if !a.isStopping.CompareAndSwap(false, true) { // Removed SA9003
		// If not already stopping, mark as stopping now due to game over
		// }
		a.isStopping.CompareAndSwap(false, true) // Mark as stopping if game over triggered

		fmt.Printf("GameActor %s: GAME_OVER - All bricks destroyed.\n", a.selfPID)

		winnerIndex := -1
		highestScore := int32(-999999)
		var finalScores [utils.MaxPlayers]int32
		tie := false
		for i, p := range a.players {
			if p != nil {
				score := p.Score.Load()
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
		if tie { winnerIndex = -1 }
		fmt.Printf("GameActor %s: GAME_OVER - Winner Index: %d (Score: %d)\n", a.selfPID, winnerIndex, highestScore)

		// Send any remaining pending updates immediately
		a.handleBroadcastTick(ctx)

		// Send GameOverMessage via Broadcaster
		if a.broadcasterPID != nil {
			gameOverMsg := GameOverMessage{
				MessageType: "gameOver", WinnerIndex: winnerIndex, FinalScores: finalScores,
				Reason: "All bricks destroyed", RoomPID: a.selfPID.String(),
			}
			a.engine.Send(a.broadcasterPID, gameOverMsg, a.selfPID)
		}

		// Notify RoomManager
		if a.roomManagerPID != nil {
			fmt.Printf("GameActor %s: GAME_OVER - Notifying RoomManager %s.\n", a.selfPID, a.roomManagerPID)
			a.engine.Send(a.roomManagerPID, GameRoomEmpty{RoomPID: a.selfPID}, nil)
		}

		// Initiate Self Stop (Cleanup will happen via Stopping message or panic recovery)
		if a.engine != nil && a.selfPID != nil {
			a.engine.Stop(a.selfPID)
		}
	}
}

// handleStopping is called when the actor receives the Stopping message.
func (a *GameActor) handleStopping(ctx bollywood.Context) {
	if a.isStopping.CompareAndSwap(false, true) {
		fmt.Printf("GameActor %s: Stopping.\n", a.selfPID)
		a.gameOver.Store(true) // Ensure game over flag is set
		a.performCleanup()     // Perform cleanup once
	}
}

// handleStopped is called when the actor receives the Stopped message.
func (a *GameActor) handleStopped(ctx bollywood.Context) {
	fmt.Printf("GameActor %s: Stopped.\n", a.selfPID)
}

// logPerformanceMetrics calculates and prints the average tick duration.
func (a *GameActor) logPerformanceMetrics() {
	a.metricsMu.Lock()
	defer a.metricsMu.Unlock()
	if a.tickCount > 0 {
		avgDuration := a.tickDurationSum / time.Duration(a.tickCount)
		fmt.Printf("PERF_METRIC GameActor %s: AvgPhysicsTick=%v Ticks=%d\n", a.selfPID, avgDuration, a.tickCount)
	} else {
		fmt.Printf("PERF_METRIC GameActor %s: No physics ticks processed.\n", a.selfPID)
	}
}