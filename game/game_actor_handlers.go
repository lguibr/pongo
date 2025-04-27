// File: game/game_actor_handlers.go
package game

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

// handlePlayerConnect processes a new player connection request.
func (a *GameActor) handlePlayerConnect(ctx bollywood.Context, ws *websocket.Conn) {
	actorPIDStr := "nil"
	if a.selfPID != nil {
		actorPIDStr = a.selfPID.String()
	}

	remoteAddr := "unknown"
	if ws != nil {
		remoteAddr = ws.RemoteAddr().String()
	} else {
		fmt.Printf("GameActor %s: Received connect request with nil connection.\n", actorPIDStr)
		return
	}

	a.mu.Lock() // Lock for finding slot and initial setup

	if existingIndex, ok := a.connToIndex[ws]; ok {
		if pInfo := a.players[existingIndex]; pInfo != nil && pInfo.IsConnected {
			fmt.Printf("GameActor %s: Connection %s already associated with active player %d. Ignoring.\n", actorPIDStr, remoteAddr, existingIndex)
			a.mu.Unlock()
			return
		}
		fmt.Printf("GameActor %s: Connection %s was previously associated with player %d but marked disconnected. Proceeding with new connection.\n", actorPIDStr, remoteAddr, existingIndex)
		delete(a.connToIndex, ws)
		if a.players[existingIndex] != nil {
			a.players[existingIndex].IsConnected = false
		}
	}

	playerIndex := -1
	for i, p := range a.players {
		if p == nil {
			playerIndex = i
			break
		}
	}

	if playerIndex == -1 {
		fmt.Printf("GameActor %s: Server full, rejecting connection from: %s\n", actorPIDStr, remoteAddr)
		a.mu.Unlock()
		_ = ws.Close()
		return
	}

	fmt.Printf("GameActor %s: Assigning player index %d to %s\n", actorPIDStr, playerIndex, remoteAddr)

	isFirstPlayer := true
	for i, p := range a.players {
		if p != nil && i != playerIndex {
			isFirstPlayer = false
			break
		}
	}
	if isFirstPlayer {
		fmt.Printf("GameActor %s: First player joining, initializing grid.\n", actorPIDStr)
		a.canvas.Grid.Fill(a.cfg.GridFillVectors, a.cfg.GridFillVectorSize, a.cfg.GridFillWalkers, a.cfg.GridFillSteps)
	}

	player := &playerInfo{
		Index:       playerIndex,
		ID:          fmt.Sprintf("player%d", playerIndex),
		Score:       a.cfg.InitialScore,
		Color:       utils.NewRandomColor(),
		Ws:          ws,
		IsConnected: true,
	}
	a.players[playerIndex] = player
	a.connToIndex[ws] = playerIndex

	paddleData := NewPaddle(a.cfg, playerIndex)
	a.paddles[playerIndex] = paddleData

	a.mu.Unlock() // Unlock before spawning actors

	a.mu.RLock()
	stillConnected := a.players[playerIndex] != nil && a.players[playerIndex].IsConnected && a.players[playerIndex].Ws == ws
	a.mu.RUnlock()

	if !stillConnected {
		fmt.Printf("GameActor %s: Player %d disconnected immediately after slot assignment, before actor spawn. Aborting setup.\n", actorPIDStr, playerIndex)
		a.mu.Lock()
		if pInfo := a.players[playerIndex]; pInfo != nil && pInfo.Ws == ws {
			delete(a.connToIndex, ws)
			a.players[playerIndex] = nil
			a.paddles[playerIndex] = nil
		}
		a.mu.Unlock()
		return
	}

	paddleProducer := NewPaddleActorProducer(*paddleData, ctx.Self(), a.cfg)
	paddlePID := a.engine.Spawn(bollywood.NewProps(paddleProducer))
	if paddlePID == nil {
		fmt.Printf("ERROR: GameActor %s failed to spawn PaddleActor for player %d\n", actorPIDStr, playerIndex)
		a.mu.Lock()
		if pInfo := a.players[playerIndex]; pInfo != nil && pInfo.Ws == ws {
			delete(a.connToIndex, ws)
			a.players[playerIndex] = nil
			a.paddles[playerIndex] = nil
		}
		a.mu.Unlock()
		_ = ws.Close()
		return
	}

	a.mu.Lock()
	if pInfo := a.players[playerIndex]; pInfo != nil && pInfo.IsConnected && pInfo.Ws == ws {
		a.paddleActors[playerIndex] = paddlePID
	} else {
		fmt.Printf("GameActor %s: Player %d disconnected before PaddleActor PID %s could be stored. Stopping actor.\n", actorPIDStr, playerIndex, paddlePID)
		a.mu.Unlock()
		if paddlePID != nil {
			a.engine.Stop(paddlePID)
		}
		return
	}
	a.mu.Unlock()

	a.spawnBall(ctx, playerIndex, 0, 0, 0, true)

	fmt.Printf("GameActor %s: Player %d setup complete. Initial broadcast will happen on next tick.\n", actorPIDStr, playerIndex)
}

// handlePlayerDisconnect processes a player disconnection event.
func (a *GameActor) handlePlayerDisconnect(ctx bollywood.Context, playerIndex int, conn *websocket.Conn) {
	actorPIDStr := "nil"
	if a.selfPID != nil {
		actorPIDStr = a.selfPID.String()
	}
	a.mu.Lock()

	if playerIndex == -1 {
		if conn == nil {
			fmt.Printf("GameActor %s: Received disconnect with no index and no connection.\n", actorPIDStr)
			a.mu.Unlock()
			return
		}
		if idx, ok := a.connToIndex[conn]; ok {
			playerIndex = idx
		} else {
			a.mu.Unlock()
			return
		}
	}

	if playerIndex < 0 || playerIndex >= utils.MaxPlayers || a.players[playerIndex] == nil {
		a.mu.Unlock()
		return
	}

	pInfo := a.players[playerIndex]
	if !pInfo.IsConnected {
		a.mu.Unlock()
		return
	}

	if conn != nil && pInfo.Ws != conn {
		a.mu.Unlock()
		return
	}

	fmt.Printf("GameActor %s: Handling disconnect for player %d (%s)\n", actorPIDStr, playerIndex, pInfo.Ws.RemoteAddr())

	pInfo.IsConnected = false
	wsToClose := pInfo.Ws

	paddleToStop := a.paddleActors[playerIndex]
	ballsToStop := []*bollywood.PID{}
	ballsToRemoveFromState := []int{}

	for ballID, ball := range a.balls {
		if ball != nil && ball.OwnerIndex == playerIndex {
			if pid, ok := a.ballActors[ballID]; ok && pid != nil {
				ballsToStop = append(ballsToStop, pid)
			}
			ballsToRemoveFromState = append(ballsToRemoveFromState, ballID)
		}
	}

	a.paddleActors[playerIndex] = nil
	for _, ballID := range ballsToRemoveFromState {
		delete(a.balls, ballID)
		delete(a.ballActors, ballID)
	}
	if pInfo.Ws != nil {
		delete(a.connToIndex, pInfo.Ws)
	}
	a.players[playerIndex] = nil
	a.paddles[playerIndex] = nil

	a.mu.Unlock()

	if paddleToStop != nil {
		a.engine.Stop(paddleToStop)
	}
	for _, pid := range ballsToStop {
		a.engine.Stop(pid)
	}

	if wsToClose != nil {
		_ = wsToClose.Close()
	}

	fmt.Printf("GameActor %s: Player %d disconnected and cleaned up.\n", actorPIDStr, playerIndex)

	a.mu.RLock()
	playersLeft := false
	for _, p := range a.players {
		if p != nil {
			playersLeft = true
			break
		}
	}
	a.mu.RUnlock()

	if !playersLeft {
		fmt.Printf("GameActor %s: Last player disconnected. Game inactive.\n", actorPIDStr)
	}

	a.broadcastGameState()
}

// handlePaddleDirection only forwards direction commands to the appropriate PaddleActor.
func (a *GameActor) handlePaddleDirection(ctx bollywood.Context, wsConn *websocket.Conn, directionData []byte) {
	a.mu.RLock() // Read lock sufficient to find PID
	playerIndex, playerFound := a.connToIndex[wsConn]
	var pid *bollywood.PID

	isValidPlayer := playerFound &&
		playerIndex >= 0 &&
		playerIndex < utils.MaxPlayers &&
		a.players[playerIndex] != nil &&
		a.players[playerIndex].IsConnected

	if isValidPlayer {
		pid = a.paddleActors[playerIndex]
	}
	a.mu.RUnlock() // Unlock before sending message

	if pid != nil {
		a.engine.Send(pid, PaddleDirectionMessage{Direction: directionData}, ctx.Self())
	}
}

// handlePaddlePositionUpdate updates the GameActor's internal state for a paddle
// by simply accepting the state reported by the PaddleActor.
func (a *GameActor) handlePaddlePositionUpdate(ctx bollywood.Context, incomingPaddleState *Paddle) {
	if incomingPaddleState == nil {
		return
	}
	a.mu.Lock() // Lock for write access
	defer a.mu.Unlock()

	idx := incomingPaddleState.Index
	// *** ADD LOGGING ***
	actorPIDStr := "nil"
	if a.selfPID != nil {
		actorPIDStr = a.selfPID.String()
	}
	fmt.Printf("GameActor %s: Received PaddlePositionMessage for P%d (IsMoving: %t, Vx: %d, Vy: %d)\n",
		actorPIDStr, idx, incomingPaddleState.IsMoving, incomingPaddleState.Vx, incomingPaddleState.Vy)

	if idx >= 0 && idx < utils.MaxPlayers && a.players[idx] != nil && a.players[idx].IsConnected {
		if currentGamePaddleState := a.paddles[idx]; currentGamePaddleState != nil {
			currentGamePaddleState.X = incomingPaddleState.X
			currentGamePaddleState.Y = incomingPaddleState.Y
			currentGamePaddleState.Direction = incomingPaddleState.Direction
			currentGamePaddleState.Vx = incomingPaddleState.Vx
			currentGamePaddleState.Vy = incomingPaddleState.Vy
			currentGamePaddleState.IsMoving = incomingPaddleState.IsMoving // Copy IsMoving flag
			if currentGamePaddleState.canvasSize == 0 {
				if incomingPaddleState.canvasSize != 0 {
					currentGamePaddleState.canvasSize = incomingPaddleState.canvasSize
				} else if a.canvas != nil {
					currentGamePaddleState.canvasSize = a.canvas.CanvasSize
				}
			}
			// *** ADD LOGGING ***
			// fmt.Printf("GameActor %s: Updated internal state for P%d (IsMoving: %t)\n", actorPIDStr, idx, currentGamePaddleState.IsMoving)
		} else {
			fmt.Printf("WARN: GameActor received paddle update for player %d but paddle state was nil. Creating.\n", idx)
			if incomingPaddleState.canvasSize == 0 && a.canvas != nil {
				incomingPaddleState.canvasSize = a.canvas.CanvasSize
			}
			paddleCopy := *incomingPaddleState
			a.paddles[idx] = &paddleCopy
		}
	}
}

// handleBallPositionUpdate updates the GameActor's internal state for a ball.
func (a *GameActor) handleBallPositionUpdate(ctx bollywood.Context, ballState *Ball) {
	if ballState == nil {
		return
	}
	a.mu.Lock() // Lock for write access
	defer a.mu.Unlock()

	if _, actorExists := a.ballActors[ballState.Id]; actorExists {
		if existingBall, stateExists := a.balls[ballState.Id]; stateExists {
			existingBall.X = ballState.X
			existingBall.Y = ballState.Y
			existingBall.Vx = ballState.Vx
			existingBall.Vy = ballState.Vy
			existingBall.Phasing = ballState.Phasing
			existingBall.Mass = ballState.Mass
			existingBall.Radius = ballState.Radius
			if existingBall.canvasSize == 0 {
				if ballState.canvasSize != 0 {
					existingBall.canvasSize = ballState.canvasSize
				} else if a.canvas != nil {
					existingBall.canvasSize = a.canvas.CanvasSize
				}
			}
		} else {
			fmt.Printf("WARN: BallActor %d exists but no corresponding state in GameActor map.\n", ballState.Id)
		}
	} else {
		delete(a.balls, ballState.Id)
	}
}

// spawnBall creates a new Ball and its corresponding BallActor.
func (a *GameActor) spawnBall(ctx bollywood.Context, ownerIndex, x, y int, expireIn time.Duration, isPermanent bool) {
	actorPIDStr := "nil"
	if a.selfPID != nil {
		actorPIDStr = a.selfPID.String()
	}

	a.mu.RLock()
	ownerValidAndConnected := ownerIndex >= 0 && ownerIndex < utils.MaxPlayers && a.players[ownerIndex] != nil && a.players[ownerIndex].IsConnected
	var ownerWs *websocket.Conn
	if ownerValidAndConnected {
		ownerWs = a.players[ownerIndex].Ws
	}
	cfg := a.cfg
	a.mu.RUnlock()

	if !ownerValidAndConnected {
		fmt.Printf("GameActor %s: Cannot spawn ball for invalid or disconnected owner index %d\n", actorPIDStr, ownerIndex)
		return
	}

	ballID := time.Now().Nanosecond() + ownerIndex
	ballData := NewBall(cfg, x, y, ownerIndex, ballID, isPermanent)

	selfPID := a.selfPID
	if selfPID == nil && ctx != nil {
		selfPID = ctx.Self()
	}
	if selfPID == nil {
		fmt.Printf("ERROR: GameActor %s cannot spawn ball, self PID is nil.\n", actorPIDStr)
		return
	}

	ballProducer := NewBallActorProducer(*ballData, selfPID, cfg)
	ballPID := a.engine.Spawn(bollywood.NewProps(ballProducer))
	if ballPID == nil {
		fmt.Printf("ERROR: GameActor %s failed to spawn BallActor for player %d, ball %d\n", actorPIDStr, ownerIndex, ballID)
		return
	}

	a.mu.Lock()
	if pInfo := a.players[ownerIndex]; pInfo != nil && pInfo.IsConnected && pInfo.Ws == ownerWs {
		a.balls[ballID] = ballData
		a.ballActors[ballID] = ballPID
	} else {
		fmt.Printf("GameActor %s: Owner %d disconnected/changed before ball %d could be added. Stopping spawned actor %s.\n", actorPIDStr, ownerIndex, ballID, ballPID)
		a.mu.Unlock()
		a.engine.Stop(ballPID)
		return
	}
	a.mu.Unlock()

	if !isPermanent && expireIn > 0 {
		randomOffset := time.Duration(rand.Intn(4000)-2000) * time.Millisecond
		actualExpireIn := expireIn + randomOffset
		if actualExpireIn <= 0 {
			actualExpireIn = 500 * time.Millisecond
		}

		// fmt.Printf("GameActor %s: Scheduling expiry for temporary ball %d in %v.\n", actorPIDStr, ballID, actualExpireIn) // Reduce noise
		time.AfterFunc(actualExpireIn, func() {
			currentSelfPID := a.selfPID
			currentEngine := a.engine
			if currentEngine != nil && currentSelfPID != nil {
				currentEngine.Send(currentSelfPID, DestroyExpiredBall{BallID: ballID}, nil)
			} else {
				fmt.Printf("ERROR: Cannot send DestroyExpiredBall for %d, engine/selfPID invalid in timer.\n", ballID)
			}
		})
	} else if isPermanent {
		// fmt.Printf("GameActor %s: Spawned permanent ball %d for player %d.\n", actorPIDStr, ballID, ownerIndex) // Reduce noise
	}
}

// handleDestroyExpiredBall handles the message sent by the ball expiry timer.
func (a *GameActor) handleDestroyExpiredBall(ctx bollywood.Context, ballID int) {
	actorPIDStr := "nil"
	if a.selfPID != nil {
		actorPIDStr = a.selfPID.String()
	}
	a.mu.Lock()

	pidToStop, actorExists := a.ballActors[ballID]
	ballState, stateExists := a.balls[ballID]

	if stateExists && ballState.IsPermanent {
		fmt.Printf("WARN: GameActor %s received DestroyExpiredBall for permanent ball %d. Ignoring.\n", actorPIDStr, ballID)
		a.mu.Unlock()
		return
	}

	if actorExists && stateExists && pidToStop != nil {
		// fmt.Printf("GameActor %s: Handling DestroyExpiredBall for BallID %d, stopping actor %s\n", actorPIDStr, ballID, pidToStop) // Reduce noise
		delete(a.balls, ballID)
		delete(a.ballActors, ballID)
		a.mu.Unlock()
		a.engine.Stop(pidToStop)
	} else {
		a.mu.Unlock()
	}
}
