// File: game/game_actor_handlers.go
package game

import (
	// Keep json import for potential future use or debugging
	"fmt"
	"math/rand"
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

// handlePlayerConnect processes a player connection assigned by the ConnectionHandlerActor.
// Assumes called within the actor's message loop (no lock needed for main state).
func (a *GameActor) handlePlayerConnect(ctx bollywood.Context, ws *websocket.Conn) {
	remoteAddr := "unknown"
	if ws != nil {
		remoteAddr = ws.RemoteAddr().String()
	} else {
		fmt.Printf("GameActor %s: Received connect assignment with nil connection.\n", a.selfPID)
		return
	}

	// Check if connection is already mapped
	if existingIndex, ok := a.connToIndex[ws]; ok {
		if pInfo := a.players[existingIndex]; pInfo != nil && pInfo.IsConnected && pInfo.Ws == ws {
			fmt.Printf("GameActor %s: Connection %s already associated with active player %d. Ignoring assignment.\n", a.selfPID, remoteAddr, existingIndex)
			return
		}
		fmt.Printf("GameActor %s: Connection %s was previously mapped but inactive/mismatched. Cleaning up before reassignment.\n", a.selfPID, remoteAddr)
		delete(a.connToIndex, ws)
		if a.players[existingIndex] != nil {
			a.players[existingIndex].IsConnected = false
			a.playerConns[existingIndex] = nil
		}
	}

	// Find the first available player slot
	playerIndex := -1
	for i, p := range a.players {
		if p == nil {
			playerIndex = i
			break
		}
	}

	if playerIndex == -1 {
		fmt.Printf("GameActor %s: Room is full (%d players). Rejecting connection %s.\n", a.selfPID, utils.MaxPlayers, remoteAddr)
		_ = ws.Close()
		return
	}

	fmt.Printf("GameActor %s: Assigning player index %d to %s\n", a.selfPID, playerIndex, remoteAddr)

	// Check if this is the first player joining this specific room instance
	isFirstPlayerInRoom := true
	for i, p := range a.players {
		if p != nil && i != playerIndex {
			isFirstPlayerInRoom = false
			break
		}
	}
	// Regenerate grid only if it's the very first player
	if isFirstPlayerInRoom {
		fmt.Printf("GameActor %s: First player joining this room, initializing grid.\n", a.selfPID)
		if a.canvas == nil {
			a.canvas = NewCanvas(a.cfg.CanvasSize, a.cfg.GridSize)
		}
		a.canvas.Grid = NewGrid(a.cfg.GridSize)
		a.canvas.Grid.Fill(a.cfg.GridFillVectors, a.cfg.GridFillVectorSize, a.cfg.GridFillWalkers, a.cfg.GridFillSteps)
	}

	// Create player info and paddle data
	player := &playerInfo{
		Index:       playerIndex,
		ID:          fmt.Sprintf("player%d", playerIndex),
		Color:       utils.NewRandomColor(),
		Ws:          ws,
		IsConnected: true,
	}
	player.Score.Store(int32(a.cfg.InitialScore)) // Set initial score atomically

	a.players[playerIndex] = player
	a.connToIndex[ws] = playerIndex // Map connection to index
	a.playerConns[playerIndex] = ws // Map index back to connection

	paddleData := NewPaddle(a.cfg, playerIndex)
	a.paddles[playerIndex] = paddleData

	// Cache necessary variables
	selfPID := a.selfPID
	engine := a.engine
	cfg := a.cfg
	broadcasterPID := a.broadcasterPID

	// Tell broadcaster to add the client
	if broadcasterPID != nil {
		engine.Send(broadcasterPID, AddClient{Conn: ws}, selfPID)
	} else {
		fmt.Printf("ERROR: GameActor %s: Broadcaster PID is nil during player connect for %s.\n", selfPID, remoteAddr)
	}

	// Spawn Paddle Actor
	paddleProducer := NewPaddleActorProducer(*paddleData, selfPID, cfg)
	paddlePID := engine.Spawn(bollywood.NewProps(paddleProducer))
	if paddlePID == nil {
		fmt.Printf("ERROR: GameActor %s failed to spawn PaddleActor for player %d\n", a.selfPID, playerIndex)
		// Clean up player state if actor spawn failed (still within actor loop)
		if pInfo := a.players[playerIndex]; pInfo != nil && pInfo.Ws == ws {
			delete(a.connToIndex, ws)
			a.players[playerIndex] = nil
			a.paddles[playerIndex] = nil
			a.playerConns[playerIndex] = nil
		}
		if broadcasterPID != nil {
			engine.Send(broadcasterPID, RemoveClient{Conn: ws}, selfPID)
		}
		_ = ws.Close()
		return
	}

	// Store Paddle Actor PID (still within actor loop)
	if pInfo := a.players[playerIndex]; pInfo != nil && pInfo.IsConnected && pInfo.Ws == ws {
		a.paddleActors[playerIndex] = paddlePID
	} else {
		// Player disconnected very quickly? Should be rare now.
		fmt.Printf("GameActor %s: Player %d (%s) disconnected before PaddleActor PID %s could be stored. Stopping actor.\n", a.selfPID, playerIndex, remoteAddr, paddlePID)
		if paddlePID != nil {
			engine.Stop(paddlePID)
		}
		if broadcasterPID != nil {
			engine.Send(broadcasterPID, RemoveClient{Conn: ws}, selfPID)
		}
		return
	}

	// Spawn initial Ball Actor
	a.spawnBall(ctx, playerIndex, 0, 0, 0, true) // Spawn permanent ball

	// Send initial state immediately
	initialSnapshot := a.createGameStateSnapshot()
	currentBroadcasterPID := a.broadcasterPID // Re-fetch broadcaster PID

	if currentBroadcasterPID != nil {
		fmt.Printf("GameActor %s: Sending initial state broadcast for player %d.\n", a.selfPID, playerIndex)
		// Send the GameState struct directly
		engine.Send(currentBroadcasterPID, BroadcastStateCommand{State: initialSnapshot}, selfPID)
	} else {
		fmt.Printf("WARN: GameActor %s: Broadcaster PID nil, cannot send initial state for player %d.\n", a.selfPID, playerIndex)
	}

	fmt.Printf("GameActor %s: Player %d setup complete.\n", a.selfPID, playerIndex)
}

// handlePlayerDisconnect processes a player disconnection event.
// Assumes called within the actor's message loop (no lock needed for main state).
func (a *GameActor) handlePlayerDisconnect(ctx bollywood.Context, conn *websocket.Conn) {
	if conn == nil {
		fmt.Printf("GameActor %s: Received disconnect with nil connection.\n", a.selfPID)
		return
	}
	connAddr := conn.RemoteAddr().String()

	playerIndex, playerFound := a.connToIndex[conn]

	if !playerFound || playerIndex < 0 || playerIndex >= utils.MaxPlayers || a.players[playerIndex] == nil || a.players[playerIndex].Ws != conn {
		if playerFound {
			delete(a.connToIndex, conn)
		}
		return
	}

	pInfo := a.players[playerIndex]

	if !pInfo.IsConnected {
		return
	}

	fmt.Printf("GameActor %s: Handling disconnect for player %d (%s)\n", a.selfPID, playerIndex, connAddr)

	// Mark as disconnected
	pInfo.IsConnected = false

	// --- Stop Actors and Manage Persistent Ball ---
	paddleToStop := a.paddleActors[playerIndex]
	a.paddleActors[playerIndex] = nil

	ownedBallIDs := []int{}
	ownedPermanentBallIDs := []int{}
	for ballID, ball := range a.balls {
		if ball != nil && ball.OwnerIndex == playerIndex {
			ownedBallIDs = append(ownedBallIDs, ballID)
			if ball.IsPermanent {
				ownedPermanentBallIDs = append(ownedPermanentBallIDs, ballID)
			}
		}
	}

	ballToKeepID := -1
	remainingBallCount := 0
	for id, b := range a.balls {
		isOwnedByDisconnectingPlayer := false
		for _, ownedID := range ownedBallIDs {
			if id == ownedID {
				isOwnedByDisconnectingPlayer = true
				break
			}
		}
		if b != nil && !isOwnedByDisconnectingPlayer {
			remainingBallCount++
		}
	}

	if remainingBallCount == 0 && len(ownedBallIDs) > 0 {
		if len(ownedPermanentBallIDs) > 0 {
			ballToKeepID = ownedPermanentBallIDs[0]
		} else {
			ballToKeepID = ownedBallIDs[0]
		}
		fmt.Printf("GameActor %s: Player %d owned last ball(s). Keeping ball %d.\n", a.selfPID, playerIndex, ballToKeepID)
	}

	ballsToStopPIDs := []*bollywood.PID{}
	for _, ballID := range ownedBallIDs {
		if ballID == ballToKeepID {
			if keptBall, ok := a.balls[ballID]; ok {
				fmt.Printf("GameActor %s: Making kept ball %d ownerless and permanent.\n", a.selfPID, ballID)
				keptBall.OwnerIndex = -1
				keptBall.IsPermanent = true
			}
		} else {
			if pid, ok := a.ballActors[ballID]; ok && pid != nil {
				ballsToStopPIDs = append(ballsToStopPIDs, pid)
			}
			delete(a.balls, ballID)
			delete(a.ballActors, ballID)
		}
	}

	// --- Clean up GameActor state ---
	delete(a.connToIndex, conn)
	a.players[playerIndex] = nil
	a.paddles[playerIndex] = nil
	a.playerConns[playerIndex] = nil

	// Check if room is now empty
	playersLeft := false
	for _, p := range a.players {
		if p != nil {
			playersLeft = true
			break
		}
	}
	roomIsEmpty := !playersLeft

	// Cache PIDs needed outside actor loop
	roomManagerPID := a.roomManagerPID
	selfPID := a.selfPID
	engine := a.engine
	broadcasterPID := a.broadcasterPID

	// --- Stop Actors ---
	if paddleToStop != nil {
		engine.Stop(paddleToStop)
	}
	for _, pid := range ballsToStopPIDs {
		engine.Stop(pid)
	}

	// Tell broadcaster to remove the client
	if broadcasterPID != nil {
		engine.Send(broadcasterPID, RemoveClient{Conn: conn}, selfPID)
	}

	fmt.Printf("GameActor %s: Player %d disconnected and cleaned up.\n", a.selfPID, playerIndex)

	// --- Notify RoomManager if Empty ---
	if roomIsEmpty {
		fmt.Printf("GameActor %s: Last player disconnected. Room is empty. Notifying RoomManager %s.\n", a.selfPID, roomManagerPID)
		if roomManagerPID != nil && selfPID != nil {
			engine.Send(roomManagerPID, GameRoomEmpty{RoomPID: selfPID}, nil)
		} else {
			fmt.Printf("ERROR: GameActor %s cannot notify RoomManager, PID is nil. Stopping self.\n", a.selfPID)
			if selfPID != nil {
				engine.Stop(selfPID)
			}
		}
	}
}

// handlePaddleDirection finds the player index from the connection and forwards.
// Assumes called within the actor's message loop (no lock needed).
func (a *GameActor) handlePaddleDirection(ctx bollywood.Context, wsConn *websocket.Conn, directionData []byte) {
	if wsConn == nil {
		return
	}

	playerIndex, playerFound := a.connToIndex[wsConn]
	var pid *bollywood.PID

	isValidPlayer := playerFound &&
		playerIndex >= 0 &&
		playerIndex < utils.MaxPlayers &&
		a.players[playerIndex] != nil &&
		a.players[playerIndex].IsConnected &&
		a.players[playerIndex].Ws == wsConn

	if isValidPlayer {
		pid = a.paddleActors[playerIndex]
	}

	if pid != nil {
		a.engine.Send(pid, PaddleDirectionMessage{Direction: directionData}, ctx.Self())
	}
}

// handlePaddlePositionUpdate - No longer needed as GameActor queries state via Ask.
// func (a *GameActor) handlePaddlePositionUpdate(ctx bollywood.Context, incomingPaddleState *Paddle) { ... }

// handleBallPositionUpdate - No longer needed as GameActor queries state via Ask.
// func (a *GameActor) handleBallPositionUpdate(ctx bollywood.Context, ballState *Ball) { ... }

// spawnBall - Assumes called within the actor's message loop (no lock needed).
func (a *GameActor) spawnBall(ctx bollywood.Context, ownerIndex, x, y int, expireIn time.Duration, isPermanent bool) {
	ownerValidAndConnected := ownerIndex >= 0 && ownerIndex < utils.MaxPlayers && a.players[ownerIndex] != nil && a.players[ownerIndex].IsConnected
	ownerWs := (*websocket.Conn)(nil)
	if ownerValidAndConnected {
		ownerWs = a.players[ownerIndex].Ws // Still need WS for re-verification check
	}
	cfg := a.cfg
	selfPID := a.selfPID
	engine := a.engine

	if !ownerValidAndConnected {
		fmt.Printf("GameActor %s: Cannot spawn ball for invalid or disconnected owner index %d\n", a.selfPID, ownerIndex)
		return
	}

	if selfPID == nil || engine == nil {
		fmt.Printf("ERROR: GameActor %s cannot spawn ball, self PID or engine is nil.\n", a.selfPID)
		return
	}

	ballID := time.Now().Nanosecond() + ownerIndex + rand.Intn(1000)
	ballData := NewBall(cfg, x, y, ownerIndex, ballID, isPermanent)

	ballProducer := NewBallActorProducer(*ballData, selfPID, cfg)
	ballPID := engine.Spawn(bollywood.NewProps(ballProducer))
	if ballPID == nil {
		fmt.Printf("ERROR: GameActor %s failed to spawn BallActor for player %d, ball %d\n", a.selfPID, ownerIndex, ballID)
		return
	}

	// Re-verify owner is still connected with the *same* websocket connection before adding
	// This check still needs to happen even within the actor loop, as the disconnect message
	// might be processed between the start of this handler and now.
	if pInfo := a.players[ownerIndex]; pInfo != nil && pInfo.IsConnected && pInfo.Ws == ownerWs {
		a.balls[ballID] = ballData
		a.ballActors[ballID] = ballPID
	} else {
		fmt.Printf("GameActor %s: Owner %d disconnected/changed before ball %d could be added. Stopping spawned actor %s.\n", a.selfPID, ownerIndex, ballID, ballPID)
		engine.Stop(ballPID)
		return
	}

	if !isPermanent && expireIn > 0 {
		randomOffset := time.Duration(rand.Intn(4000)-2000) * time.Millisecond
		actualExpireIn := expireIn + randomOffset
		if actualExpireIn <= 0 {
			actualExpireIn = 500 * time.Millisecond
		}

		time.AfterFunc(actualExpireIn, func() {
			currentSelfPID := a.selfPID
			currentEngine := a.engine
			if currentEngine != nil && currentSelfPID != nil {
				currentEngine.Send(currentSelfPID, DestroyExpiredBall{BallID: ballID}, nil)
			} else {
				fmt.Printf("ERROR: Cannot send DestroyExpiredBall for %d, engine/selfPID invalid in timer.\n", ballID)
			}
		})
	}
}

// handleDestroyExpiredBall - Assumes called within the actor's message loop (no lock needed).
func (a *GameActor) handleDestroyExpiredBall(ctx bollywood.Context, ballID int) {
	pidToStop, actorExists := a.ballActors[ballID]
	ballState, stateExists := a.balls[ballID]

	if stateExists && ballState.IsPermanent {
		return
	}

	if actorExists && stateExists && pidToStop != nil {
		delete(a.balls, ballID)
		delete(a.ballActors, ballID)
		engine := a.engine
		if engine != nil {
			engine.Stop(pidToStop)
		}
	}
}
