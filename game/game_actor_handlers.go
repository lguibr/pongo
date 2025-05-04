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

// handlePlayerConnect processes a player connection assigned by the ConnectionHandlerActor.
// Assumes called within the actor's message loop (no lock needed for main state).
func (a *GameActor) handlePlayerConnect(ctx bollywood.Context, ws *websocket.Conn) {
	remoteAddr := "unknown"
	if ws != nil {
		remoteAddr = ws.RemoteAddr().String()
	} else {
		fmt.Printf("ERROR: GameActor %s: Received connect assignment with nil connection.\n", a.selfPID)
		return
	}

	if _, ok := a.connToIndex[ws]; ok {
		// Connection already mapped, potentially stale. Let disconnect handle cleanup if needed.
		return
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
		fmt.Printf("WARN: GameActor %s: Room is full (%d players). Rejecting connection %s.\n", a.selfPID, utils.MaxPlayers, remoteAddr)
		_ = ws.Close()
		return
	}

	// Check if this is the first player joining this specific room instance
	isFirstPlayerInRoom := true
	for i, p := range a.players {
		if p != nil && i != playerIndex {
			isFirstPlayerInRoom = false
			break
		}
	}
	if isFirstPlayerInRoom {
		fmt.Printf("GameActor %s: First player joined. Initializing grid and starting tickers.\n", a.selfPID)
		if a.canvas == nil { // Should exist from producer
			a.canvas = NewCanvas(a.cfg.CanvasSize, a.cfg.GridSize)
		}
		a.canvas.Grid = NewGrid(a.cfg.GridSize) // Create new grid
		a.canvas.Grid.Fill(a.cfg.GridFillVectors, a.cfg.GridFillVectorSize, a.cfg.GridFillWalkers, a.cfg.GridFillSteps)
		a.startTickers(ctx) // Start tickers only when first player joins
	} else if a.canvas == nil || a.canvas.Grid == nil {
		// This case should not happen if first player initializes correctly
		fmt.Printf("ERROR: GameActor %s: Joining player %d but grid/canvas not initialized!\n", a.selfPID, playerIndex)
		_ = ws.Close()
		return
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
	a.paddles[playerIndex] = paddleData // Initialize cache

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
		// Clean up player state if actor spawn failed
		delete(a.connToIndex, ws)
		a.players[playerIndex] = nil
		a.paddles[playerIndex] = nil
		a.playerConns[playerIndex] = nil
		if broadcasterPID != nil {
			engine.Send(broadcasterPID, RemoveClient{Conn: ws}, selfPID)
		}
		_ = ws.Close()
		return
	}

	// Store Paddle Actor PID
	if pInfo := a.players[playerIndex]; pInfo != nil && pInfo.IsConnected && pInfo.Ws == ws {
		a.paddleActors[playerIndex] = paddlePID
	} else {
		// Player disconnected very quickly?
		fmt.Printf("WARN: GameActor %s: Player %d (%s) disconnected before PaddleActor PID %s could be stored. Stopping actor.\n", a.selfPID, playerIndex, remoteAddr, paddlePID)
		if paddlePID != nil {
			engine.Stop(paddlePID)
		}
		if broadcasterPID != nil {
			engine.Send(broadcasterPID, RemoveClient{Conn: ws}, selfPID)
		}
		// Don't return, let disconnect logic handle full cleanup if needed
	}

	// Spawn initial Ball Actor
	a.spawnBall(ctx, playerIndex, 0, 0, 0, true) // Spawn permanent ball

	// --- Send Initial State Directly to Client ---
	fmt.Printf("GameActor %s: Sending initial messages to player %d (%s)...\n", a.selfPID, playerIndex, remoteAddr)

	// 1. Send PlayerAssignmentMessage
	assignmentMsg := PlayerAssignmentMessage{
		PlayerIndex: playerIndex,
		MessageType: "playerAssignment", // Add message type
	}
	errAssign := websocket.JSON.Send(ws, assignmentMsg)
	if errAssign != nil {
		fmt.Printf("ERROR: GameActor %s: Failed to send PlayerAssignmentMessage to player %d (%s): %v\n", a.selfPID, playerIndex, remoteAddr, errAssign)
		// Trigger disconnect if send fails
		a.handlePlayerDisconnect(ctx, ws) // Use the handler to ensure proper cleanup
		return
	}
	fmt.Printf("GameActor %s: Sent PlayerAssignmentMessage to player %d.\n", a.selfPID, playerIndex)

	// 2. Send InitialGridStateMessage
	gridMsg := InitialGridStateMessage{
		CanvasWidth:  a.canvas.Width,
		CanvasHeight: a.canvas.Height,
		GridSize:     a.canvas.GridSize,
		CellSize:     a.canvas.CellSize,
		Grid:         deepCopyGrid(a.canvas.Grid), // Send a deep copy of the current grid
		MessageType:  "initialGridState",
	}
	errGrid := websocket.JSON.Send(ws, gridMsg)
	if errGrid != nil {
		fmt.Printf("ERROR: GameActor %s: Failed to send InitialGridStateMessage to player %d (%s): %v\n", a.selfPID, playerIndex, remoteAddr, errGrid)
		// Trigger disconnect if send fails
		a.handlePlayerDisconnect(ctx, ws) // Use the handler to ensure proper cleanup
		return
	}
	fmt.Printf("GameActor %s: Sent InitialGridStateMessage to player %d.\n", a.selfPID, playerIndex)

	// --- End Initial State Send ---

	// Send initial dynamic state broadcast (to all players in the room via broadcaster)
	initialSnapshot := a.createGameStateSnapshot()
	currentBroadcasterPID := a.broadcasterPID // Re-fetch broadcaster PID

	if currentBroadcasterPID != nil {
		engine.Send(currentBroadcasterPID, BroadcastStateCommand{State: initialSnapshot}, selfPID)
	} else {
		fmt.Printf("WARN: GameActor %s: Broadcaster PID nil, cannot send initial state broadcast for player %d.\n", a.selfPID, playerIndex)
	}
	fmt.Printf("GameActor %s: Player %d (%s) setup complete.\n", a.selfPID, playerIndex, remoteAddr)
}

// handlePlayerDisconnect processes a player disconnection event.
// Assumes called within the actor's message loop (no lock needed for main state).
func (a *GameActor) handlePlayerDisconnect(ctx bollywood.Context, conn *websocket.Conn) {
	if conn == nil {
		return
	}
	connAddr := conn.RemoteAddr().String() // Get address for logging

	playerIndex, playerFound := a.connToIndex[conn]

	// Check if player is valid and actually connected through this specific websocket instance
	if !playerFound || playerIndex < 0 || playerIndex >= utils.MaxPlayers || a.players[playerIndex] == nil || a.players[playerIndex].Ws != conn {
		// Player not found or connection mismatch (might be a stale disconnect message)
		if playerFound {
			// Clean up potentially stale mapping if player is already gone or uses a different connection
			delete(a.connToIndex, conn)
		}
		fmt.Printf("GameActor %s: Ignoring disconnect for %s - No valid player found or connection mismatch.\n", a.selfPID, connAddr)
		return
	}

	pInfo := a.players[playerIndex]

	if !pInfo.IsConnected {
		// Already processed disconnect for this player/connection
		fmt.Printf("GameActor %s: Ignoring duplicate disconnect for player %d (%s).\n", a.selfPID, playerIndex, connAddr)
		return
	}

	fmt.Printf("GameActor %s: Handling disconnect for player %d (%s)\n", a.selfPID, playerIndex, connAddr)
	pInfo.IsConnected = false // Mark as disconnected

	// --- Stop Actors and Manage Persistent Ball ---
	paddleToStop := a.paddleActors[playerIndex]
	a.paddleActors[playerIndex] = nil
	a.paddles[playerIndex] = nil // Clean paddle cache

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
	}

	ballsToStopPIDs := []*bollywood.PID{}
	for _, ballID := range ownedBallIDs {
		if ballID == ballToKeepID {
			if keptBall, ok := a.balls[ballID]; ok && keptBall != nil {
				keptBall.OwnerIndex = -1
				keptBall.IsPermanent = true
			}
		} else {
			if pid, ok := a.ballActors[ballID]; ok && pid != nil {
				ballsToStopPIDs = append(ballsToStopPIDs, pid)
			}
			delete(a.balls, ballID) // Clean ball cache
			delete(a.ballActors, ballID)
		}
	}

	// --- Clean up GameActor state ---
	delete(a.connToIndex, conn)
	// Keep player info struct but mark as disconnected, useful for final scores
	// a.players[playerIndex] = nil // Don't nil player info entirely
	a.playerConns[playerIndex] = nil // Remove connection reference

	playersLeft := false
	for _, p := range a.players {
		// Check IsConnected flag now
		if p != nil && p.IsConnected {
			playersLeft = true
			break
		}
	}
	roomIsEmpty := !playersLeft

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

	// --- Notify Broadcaster ---
	if broadcasterPID != nil {
		engine.Send(broadcasterPID, RemoveClient{Conn: conn}, selfPID)
	}

	fmt.Printf("GameActor %s: Player %d (%s) disconnected and cleaned up.\n", a.selfPID, playerIndex, connAddr)

	// --- Notify RoomManager if Empty ---
	if roomIsEmpty && !a.gameOver.Load() { // Only notify if not already game over
		fmt.Printf("GameActor %s: Last player disconnected. Room is empty. Notifying RoomManager %s.\n", a.selfPID, roomManagerPID)
		if roomManagerPID != nil && selfPID != nil {
			engine.Send(roomManagerPID, GameRoomEmpty{RoomPID: selfPID}, nil)
		} else {
			fmt.Printf("ERROR: GameActor %s cannot notify RoomManager, PID is nil. Stopping self.\n", a.selfPID)
			if selfPID != nil {
				engine.Stop(selfPID)
			}
		}
	} else if roomIsEmpty && a.gameOver.Load() {
		// Game already ended, room manager was likely notified already
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
	} else {
		// Ignore input from invalid/unknown connection
	}
}

// spawnBall - Assumes called within the actor's message loop (no lock needed).
func (a *GameActor) spawnBall(ctx bollywood.Context, ownerIndex, x, y int, expireIn time.Duration, isPermanent bool) {
	// Check if owner index is valid before proceeding
	if ownerIndex < -1 || ownerIndex >= utils.MaxPlayers { // Allow -1 for ownerless spawn
		fmt.Printf("WARN: GameActor %s received spawnBall request with invalid owner index %d.\n", a.selfPID, ownerIndex)
		return
	}

	ownerValidAndConnected := false
	ownerWs := (*websocket.Conn)(nil)
	if ownerIndex != -1 { // Only check connection if owner is not -1
		ownerValidAndConnected = a.players[ownerIndex] != nil && a.players[ownerIndex].IsConnected
		if ownerValidAndConnected {
			ownerWs = a.players[ownerIndex].Ws // Still need WS for re-verification check
		}
	}

	cfg := a.cfg
	selfPID := a.selfPID
	engine := a.engine // Use the actor's engine field

	// Allow spawning ownerless balls (-1) or balls for connected players
	if ownerIndex != -1 && !ownerValidAndConnected {
		fmt.Printf("WARN: GameActor %s cannot spawn ball for disconnected owner %d.\n", a.selfPID, ownerIndex)
		return
	}

	if selfPID == nil || engine == nil {
		fmt.Printf("ERROR: GameActor %s cannot spawn ball, self PID or engine is nil.\n", a.selfPID)
		return
	}

	ballID := time.Now().Nanosecond() + ownerIndex + rand.Intn(1000) // Simple unique enough ID
	ballData := NewBall(cfg, x, y, ownerIndex, ballID, isPermanent)

	ballProducer := NewBallActorProducer(*ballData, selfPID, cfg)
	ballPID := engine.Spawn(bollywood.NewProps(ballProducer))
	if ballPID == nil {
		fmt.Printf("ERROR: GameActor %s failed to spawn BallActor for owner %d, ball %d\n", a.selfPID, ownerIndex, ballID)
		return
	}

	// Re-verify owner is still connected with the *same* websocket connection before adding (if owner != -1)
	if ownerIndex == -1 || (a.players[ownerIndex] != nil && a.players[ownerIndex].IsConnected && a.players[ownerIndex].Ws == ownerWs) {
		a.balls[ballID] = ballData // Initialize cache
		a.ballActors[ballID] = ballPID
	} else {
		// Owner disconnected between check and spawn completion
		fmt.Printf("WARN: GameActor %s: Owner %d disconnected before BallActor %s could be fully registered. Stopping actor.\n", a.selfPID, ownerIndex, ballPID)
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
			// Use the captured engine and selfPID from the outer scope
			currentSelfPID := selfPID
			currentEngine := engine
			if currentEngine != nil && currentSelfPID != nil {
				currentEngine.Send(currentSelfPID, DestroyExpiredBall{BallID: ballID}, nil)
			} else {
				// Error logging if needed
			}
		})
	}
}

// handleDestroyExpiredBall - Assumes called within the actor's message loop (no lock needed).
func (a *GameActor) handleDestroyExpiredBall(ctx bollywood.Context, ballID int) {
	pidToStop, actorExists := a.ballActors[ballID]
	ballState, stateExists := a.balls[ballID]

	if stateExists && ballState != nil && ballState.IsPermanent {
		return // Don't destroy permanent balls via expiry timer
	}

	// Use a.engine directly
	currentEngine := a.engine
	if currentEngine == nil {
		fmt.Printf("ERROR: GameActor %s: Engine is nil in handleDestroyExpiredBall.\n", a.selfPID)
		// Attempt cleanup without engine reference if possible
		if stateExists {
			delete(a.balls, ballID)
		}
		if actorExists {
			delete(a.ballActors, ballID)
		}
		return
	}

	if actorExists && stateExists && pidToStop != nil {
		delete(a.balls, ballID) // Clean cache
		delete(a.ballActors, ballID)
		currentEngine.Stop(pidToStop) // Use a.engine
	} else {
		// Clean up potentially inconsistent state
		if stateExists {
			delete(a.balls, ballID)
		}
		if actorExists {
			delete(a.ballActors, ballID)
			if pidToStop != nil {
				currentEngine.Stop(pidToStop) // Use a.engine
			}
		}
	}
}