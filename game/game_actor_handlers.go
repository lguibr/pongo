// File: game/game_actor_handlers.go
package game

import (
	"fmt"
	"math/rand"
	"strings"
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
		// fmt.Printf("GameActor %s: Connection %s already mapped. Ignoring assignment.\n", a.selfPID, remoteAddr) // Reduce noise
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

	// fmt.Printf("GameActor %s: Assigning player index %d to %s\n", a.selfPID, playerIndex, remoteAddr) // Reduce noise

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

	// Send the PlayerAssignmentMessage directly to the newly connected client
	assignmentMsg := PlayerAssignmentMessage{PlayerIndex: playerIndex}
	err := websocket.JSON.Send(ws, assignmentMsg)
	if err != nil {
		fmt.Printf("WARN: GameActor %s: Failed to send PlayerAssignmentMessage to player %d (%s): %v\n", a.selfPID, playerIndex, remoteAddr, err)
		// If the error indicates a closed connection, trigger disconnect handling immediately
		errStr := err.Error()
		isClosedErr := strings.Contains(errStr, "use of closed network connection") ||
			strings.Contains(errStr, "broken pipe") ||
			strings.Contains(errStr, "connection reset by peer") ||
			strings.Contains(errStr, "EOF") ||
			strings.Contains(errStr, "write: connection timed out")
		if isClosedErr {
			a.handlePlayerDisconnect(ctx, ws)
			return // Stop further processing for this connection
		}
	}
	// fmt.Printf("GameActor %s: Sent PlayerAssignmentMessage (Index: %d) to %s\n", a.selfPID, playerIndex, remoteAddr) // Reduce noise

	// Send initial state broadcast (to all players in the room via broadcaster)
	initialSnapshot := a.createGameStateSnapshot()
	currentBroadcasterPID := a.broadcasterPID // Re-fetch broadcaster PID

	if currentBroadcasterPID != nil {
		// fmt.Printf("GameActor %s: Sending initial state broadcast after assigning player %d.\n", a.selfPID, playerIndex) // Reduce noise
		engine.Send(currentBroadcasterPID, BroadcastStateCommand{State: initialSnapshot}, selfPID)
	} else {
		fmt.Printf("WARN: GameActor %s: Broadcaster PID nil, cannot send initial state broadcast for player %d.\n", a.selfPID, playerIndex)
	}

	// fmt.Printf("GameActor %s: Player %d setup complete.\n", a.selfPID, playerIndex) // Reduce noise
}

// handlePlayerDisconnect processes a player disconnection event.
// Assumes called within the actor's message loop (no lock needed for main state).
func (a *GameActor) handlePlayerDisconnect(ctx bollywood.Context, conn *websocket.Conn) {
	if conn == nil {
		// fmt.Printf("GameActor %s: Received disconnect with nil connection.\n", a.selfPID) // Reduce noise
		return
	}
	// connAddr := conn.RemoteAddr().String() // Reduce noise

	playerIndex, playerFound := a.connToIndex[conn]

	if !playerFound || playerIndex < 0 || playerIndex >= utils.MaxPlayers || a.players[playerIndex] == nil || a.players[playerIndex].Ws != conn {
		// fmt.Printf("GameActor %s: Disconnect for %s - No valid player found. Ignoring.\n", a.selfPID, connAddr) // Reduce noise
		if playerFound {
			delete(a.connToIndex, conn) // Clean stale mapping
		}
		return
	}

	pInfo := a.players[playerIndex]

	if !pInfo.IsConnected {
		// fmt.Printf("GameActor %s: Received duplicate disconnect for player %d (%s). Ignoring.\n", a.selfPID, playerIndex, connAddr) // Reduce noise
		return
	}

	// fmt.Printf("GameActor %s: Handling disconnect for player %d (%s)\n", a.selfPID, playerIndex, connAddr) // Reduce noise
	pInfo.IsConnected = false

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
		// fmt.Printf("GameActor %s: Player %d owned last ball(s). Keeping ball %d.\n", a.selfPID, playerIndex, ballToKeepID) // Reduce noise
	}

	ballsToStopPIDs := []*bollywood.PID{}
	for _, ballID := range ownedBallIDs {
		if ballID == ballToKeepID {
			if keptBall, ok := a.balls[ballID]; ok && keptBall != nil {
				// fmt.Printf("GameActor %s: Making kept ball %d ownerless and permanent.\n", a.selfPID, ballID) // Reduce noise
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
	a.players[playerIndex] = nil
	a.playerConns[playerIndex] = nil

	playersLeft := false
	for _, p := range a.players {
		if p != nil {
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

	if broadcasterPID != nil {
		engine.Send(broadcasterPID, RemoveClient{Conn: conn}, selfPID)
	}

	// fmt.Printf("GameActor %s: Player %d disconnected and cleaned up.\n", a.selfPID, playerIndex) // Reduce noise

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
		// fmt.Printf("GameActor %s: Last player disconnected after game over. Room already notified/stopping.\n", a.selfPID) // Reduce noise
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
		// Log if input received for an unknown/disconnected player
		// connAddr := "unknown"
		// if wsConn != nil {
		// 	connAddr = wsConn.RemoteAddr().String()
		// }
		// fmt.Printf("WARN: GameActor %s: Received paddle input from invalid/unknown connection %s (Index: %d, Found: %t, Valid: %t). Dropping.\n",
		// 	a.selfPID, connAddr, playerIndex, playerFound, isValidPlayer) // Reduce noise
	}
}

// spawnBall - Assumes called within the actor's message loop (no lock needed).
func (a *GameActor) spawnBall(ctx bollywood.Context, ownerIndex, x, y int, expireIn time.Duration, isPermanent bool) {
	ownerValidAndConnected := ownerIndex >= 0 && ownerIndex < utils.MaxPlayers && a.players[ownerIndex] != nil && a.players[ownerIndex].IsConnected
	ownerWs := (*websocket.Conn)(nil)
	if ownerValidAndConnected {
		ownerWs = a.players[ownerIndex].Ws // Still need WS for re-verification check
	}
	cfg := a.cfg
	selfPID := a.selfPID
	engine := a.engine // Use the actor's engine field

	if !ownerValidAndConnected {
		// fmt.Printf("GameActor %s: Cannot spawn ball for invalid or disconnected owner index %d\n", a.selfPID, ownerIndex) // Reduce noise
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
		fmt.Printf("ERROR: GameActor %s failed to spawn BallActor for player %d, ball %d\n", a.selfPID, ownerIndex, ballID)
		return
	}

	// Re-verify owner is still connected with the *same* websocket connection before adding
	if pInfo := a.players[ownerIndex]; pInfo != nil && pInfo.IsConnected && pInfo.Ws == ownerWs {
		a.balls[ballID] = ballData // Initialize cache
		a.ballActors[ballID] = ballPID
	} else {
		// fmt.Printf("GameActor %s: Owner %d disconnected/changed before ball %d could be added. Stopping spawned actor %s.\n", a.selfPID, ownerIndex, ballID, ballPID) // Reduce noise
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
				// fmt.Printf("ERROR: Cannot send DestroyExpiredBall for %d, engine/selfPID invalid in timer.\n", ballID) // Reduce noise
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
