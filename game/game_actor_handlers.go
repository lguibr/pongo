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

// --- Player Handlers ---

// handlePlayerConnect processes a player connection, sends initial state,
// and generates PlayerJoined update.
func (a *GameActor) handlePlayerConnect(ctx bollywood.Context, ws *websocket.Conn) {
	// Ensure ws is not nil for real connections
	if ws == nil {
		fmt.Printf("ERROR: GameActor %s: Received connect assignment with nil websocket connection.\n", a.selfPID)
		return // Do not proceed if connection is nil in production path
	}
	remoteAddr := ws.RemoteAddr().String()

	if _, ok := a.connToIndex[ws]; ok {
		// Player already connected, ignore duplicate assignment attempt
		fmt.Printf("WARN: GameActor %s: Ignoring duplicate connect assignment for %s.\n", a.selfPID, remoteAddr)
		return
	}

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

	isFirstPlayerInRoom := true
	for i, p := range a.players {
		if p != nil && i != playerIndex {
			isFirstPlayerInRoom = false
			break
		}
	}
	if isFirstPlayerInRoom {
		fmt.Printf("GameActor %s: First player joined. Initializing grid and starting tickers.\n", a.selfPID)
		if a.canvas == nil {
			a.canvas = NewCanvas(a.cfg.CanvasSize, a.cfg.GridSize)
		}
		// Use config when filling grid
		a.canvas.Grid.FillSymmetrical(a.cfg)
		a.startTickers(ctx)
	} else if a.canvas == nil || a.canvas.Grid == nil {
		fmt.Printf("ERROR: GameActor %s: Joining player %d but grid/canvas not initialized!\n", a.selfPID, playerIndex)
		_ = ws.Close()
		return
	}

	// Create player info and paddle data
	playerDataPtr := NewPlayer(a.canvas, playerIndex) // Returns *Player
	player := &playerInfo{
		Index:       playerIndex,
		ID:          playerDataPtr.Id,
		Color:       playerDataPtr.Color,
		Ws:          ws, // Store the actual connection
		IsConnected: true,
	}
	player.Score.Store(playerDataPtr.Score) // Set initial score atomically

	a.players[playerIndex] = player
	a.connToIndex[ws] = playerIndex
	a.playerConns[playerIndex] = ws

	paddleDataPtr := NewPaddle(a.cfg, playerIndex) // Returns *Paddle
	a.paddles[playerIndex] = paddleDataPtr         // Store pointer in cache

	// Cache necessary variables
	selfPID := a.selfPID
	engine := a.engine
	cfg := a.cfg
	broadcasterPID := a.broadcasterPID
	canvasSize := cfg.CanvasSize

	// Tell broadcaster to add the client
	if broadcasterPID != nil {
		engine.Send(broadcasterPID, AddClient{Conn: ws}, selfPID)
	} else {
		fmt.Printf("ERROR: GameActor %s: Broadcaster PID is nil during player connect for %s.\n", selfPID, remoteAddr)
	}

	// Spawn Paddle Actor
	paddleProducer := NewPaddleActorProducer(*paddleDataPtr, selfPID, cfg) // Pass copy to producer
	paddlePID := engine.Spawn(bollywood.NewProps(paddleProducer))
	if paddlePID == nil {
		fmt.Printf("ERROR: GameActor %s failed to spawn PaddleActor for player %d\n", a.selfPID, playerIndex)
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
	a.paddleActors[playerIndex] = paddlePID

	// --- Send Initial State Directly to Client using JSON.Send ---
	// fmt.Printf("GameActor %s: Sending initial messages to player %d (%s)...\n", a.selfPID, playerIndex, remoteAddr) // Removed log
	assignmentMsg := PlayerAssignmentMessage{MessageType: "playerAssignment", PlayerIndex: playerIndex}
	errAssign := websocket.JSON.Send(ws, assignmentMsg)
	if errAssign != nil {
		fmt.Printf("ERROR: GameActor %s: Failed to send PlayerAssignmentMessage to player %d (%s): %v\n", a.selfPID, playerIndex, remoteAddr, errAssign)
		a.handlePlayerDisconnect(ctx, ws) // Trigger disconnect handling
		return
	}
	// fmt.Printf("GameActor %s: Sent PlayerAssignmentMessage to player %d.\n", a.selfPID, playerIndex) // Removed log

	// --- Send Initial State of Other Entities ---
	existingPlayers := make([]*Player, 0, utils.MaxPlayers)
	// Use the new InitialPaddleState and InitialBallState types
	existingPaddlesWithCoords := make([]InitialPaddleState, 0, utils.MaxPlayers)
	existingBallsWithCoords := make([]InitialBallState, 0, len(a.balls))

	for i := 0; i < utils.MaxPlayers; i++ {
		// Include the newly joined player's state as well
		if pInfo := a.players[i]; pInfo != nil && pInfo.IsConnected {
			pData := &Player{
				Index: pInfo.Index,
				Id:    pInfo.ID,
				Color: pInfo.Color,
				Score: pInfo.Score.Load(),
			}
			existingPlayers = append(existingPlayers, pData)
		}
		// Include all non-nil paddles
		if paddle := a.paddles[i]; paddle != nil {
			// Calculate R3F coords
			r3fX, r3fY := mapToR3FCoords(paddle.X+paddle.Width/2, paddle.Y+paddle.Height/2, canvasSize)
			// Create the combined struct
			initialPaddle := InitialPaddleState{
				Paddle: *paddle, // Embed core Paddle data
				R3fX:   r3fX,
				R3fY:   r3fY,
			}
			existingPaddlesWithCoords = append(existingPaddlesWithCoords, initialPaddle)
		}
	}
	for _, ball := range a.balls {
		if ball != nil {
			// Calculate R3F coords
			r3fX, r3fY := mapToR3FCoords(ball.X, ball.Y, canvasSize)
			// Create the combined struct
			initialBall := InitialBallState{
				Ball: *ball, // Embed original data
				R3fX: r3fX,
				R3fY: r3fY,
			}
			existingBallsWithCoords = append(existingBallsWithCoords, initialBall)
		}
	}

	initialEntitiesMsg := InitialPlayersAndBallsState{
		MessageType: "initialPlayersAndBallsState",
		Players:     existingPlayers,
		Paddles:     existingPaddlesWithCoords, // Now includes R3F coords
		Balls:       existingBallsWithCoords,   // Now includes R3F coords
	}
	errEntities := websocket.JSON.Send(ws, initialEntitiesMsg)
	if errEntities != nil {
		fmt.Printf("ERROR: GameActor %s: Failed to send InitialPlayersAndBallsState to player %d (%s): %v\n", a.selfPID, playerIndex, remoteAddr, errEntities)
		a.handlePlayerDisconnect(ctx, ws) // Trigger disconnect handling
		return
	}
	// fmt.Printf("GameActor %s: Sent InitialPlayersAndBallsState to player %d.\n", a.selfPID, playerIndex) // Removed log
	// --- End Initial State Send ---

	// --- Generate Updates for Broadcast ---
	// Add PlayerJoined update for other clients (including R3F coords)
	initialPaddleR3fX, initialPaddleR3fY := mapToR3FCoords(paddleDataPtr.X+paddleDataPtr.Width/2, paddleDataPtr.Y+paddleDataPtr.Height/2, canvasSize)
	playerJoinedUpdate := &PlayerJoined{
		MessageType: "playerJoined",
		Player:      *playerDataPtr, // Dereference pointer to copy
		Paddle:      *paddleDataPtr, // Dereference pointer to copy
		R3fX:        initialPaddleR3fX,
		R3fY:        initialPaddleR3fY,
	}
	a.addUpdate(playerJoinedUpdate)

	// Spawn initial Ball Actor (will generate BallSpawned update with R3F coords)
	// Corrected call with the final 'setInitialPhasing' argument
	a.spawnBall(ctx, playerIndex, 0, 0, 0, true, false)

	fmt.Printf("GameActor %s: Player %d (%s) setup complete.\n", a.selfPID, playerIndex, remoteAddr)
}

// handlePlayerDisconnect processes disconnect and generates PlayerLeft update.
func (a *GameActor) handlePlayerDisconnect(ctx bollywood.Context, conn *websocket.Conn) {
	if conn == nil {
		// fmt.Printf("WARN: GameActor %s received PlayerDisconnect with nil connection (likely internal cleanup).\n", a.selfPID) // Removed log
		// Attempt cleanup based on finding a player with nil Ws if necessary,
		// but usually this means the connection was already handled or is a test scenario.
		// For now, we just return to avoid panicking on nil conn.
		return
	}
	connAddr := conn.RemoteAddr().String()
	playerIndex, playerFound := a.connToIndex[conn]

	if !playerFound || playerIndex < 0 || playerIndex >= utils.MaxPlayers || a.players[playerIndex] == nil || a.players[playerIndex].Ws != conn {
		if playerFound {
			delete(a.connToIndex, conn)
		}
		// fmt.Printf("WARN: GameActor %s received PlayerDisconnect for unknown/mismatched connection %s (Index: %d, Found: %t)\n", a.selfPID, connAddr, playerIndex, playerFound) // Removed log
		return
	}

	pInfo := a.players[playerIndex]
	if !pInfo.IsConnected {
		// fmt.Printf("WARN: GameActor %s received PlayerDisconnect for already disconnected player %d (%s)\n", a.selfPID, playerIndex, connAddr) // Removed log
		return
	}

	fmt.Printf("GameActor %s: Handling disconnect for player %d (%s)\n", a.selfPID, playerIndex, connAddr)
	pInfo.IsConnected = false // Mark as disconnected first

	// Generate PlayerLeft update *before* stopping actors/cleaning state
	playerLeftUpdate := &PlayerLeft{
		MessageType: "playerLeft",
		Index:       playerIndex,
	}
	a.addUpdate(playerLeftUpdate)

	// --- Stop Actors and Manage Persistent Ball ---
	paddleToStop := a.paddleActors[playerIndex]
	a.paddleActors[playerIndex] = nil
	a.paddles[playerIndex] = nil // Clear paddle cache

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
				// Generate BallOwnershipChange update
				ownerUpdate := &BallOwnershipChange{
					MessageType:   "ballOwnerChanged",
					ID:            ballID,
					NewOwnerIndex: -1,
				}
				a.addUpdate(ownerUpdate)
			}
		} else {
			if pid, ok := a.ballActors[ballID]; ok && pid != nil {
				ballsToStopPIDs = append(ballsToStopPIDs, pid)
			}
			delete(a.balls, ballID)
			delete(a.ballActors, ballID)
			// Stop any active phasing timer for this ball
			a.stopPhasingTimer(ballID)
			// Generate BallRemoved update
			removedUpdate := &BallRemoved{
				MessageType: "ballRemoved",
				ID:          ballID,
			}
			a.addUpdate(removedUpdate)
		}
	}

	// --- Clean up GameActor state ---
	delete(a.connToIndex, conn)
	a.playerConns[playerIndex] = nil
	// Keep player info slot nilled until next connect
	a.players[playerIndex] = nil

	playersLeft := false
	for _, p := range a.players {
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
	if roomIsEmpty && !a.gameOver.Load() {
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

// --- Input Handler ---

// handlePaddleDirection forwards command to PaddleActor.
func (a *GameActor) handlePaddleDirection(ctx bollywood.Context, wsConn *websocket.Conn, directionData []byte) {
	if wsConn == nil {
		// fmt.Printf("WARN: GameActor %s received paddle direction with nil connection.\n", a.selfPID) // Removed log
		return
	}
	playerIndex, playerFound := a.connToIndex[wsConn]
	var pid *bollywood.PID
	isValidPlayer := playerFound && playerIndex >= 0 && playerIndex < utils.MaxPlayers && a.players[playerIndex] != nil && a.players[playerIndex].IsConnected && a.players[playerIndex].Ws == wsConn
	if isValidPlayer {
		pid = a.paddleActors[playerIndex]
	}
	if pid != nil {
		// Need to unmarshal here to log the direction string
		// var receivedDirection Direction // Removed unmarshal just for logging
		// err := json.Unmarshal(directionData, &receivedDirection)
		// if err == nil {
		// fmt.Printf("GameActor %s: Forwarding direction '%s' for player %d to PaddleActor %s\n", a.selfPID, receivedDirection.Direction, playerIndex, pid)
		// } else {
		// fmt.Printf("GameActor %s: Forwarding unparseable direction data for player %d to PaddleActor %s\n", a.selfPID, playerIndex, pid)
		// }
		a.engine.Send(pid, PaddleDirectionMessage{Direction: directionData}, ctx.Self())
	} else {
		// fmt.Printf("WARN: GameActor %s received paddle direction for invalid/unknown player via connection %s\n", a.selfPID, wsConn.RemoteAddr()) // Removed log
	}
}

// --- Ball Handlers ---

// spawnBall spawns actor and generates BallSpawned update including R3F coords.
// Now accepts setInitialPhasing flag.
func (a *GameActor) spawnBall(ctx bollywood.Context, ownerIndex, x, y int, expireIn time.Duration, isPermanent bool, setInitialPhasing bool) {
	if ownerIndex < -1 || ownerIndex >= utils.MaxPlayers {
		fmt.Printf("WARN: GameActor %s received spawnBall request with invalid owner index %d.\n", a.selfPID, ownerIndex)
		return
	}
	ownerValidAndConnected := false
	ownerWs := (*websocket.Conn)(nil) // Keep track of original Ws if owner exists
	if ownerIndex != -1 {
		if a.players[ownerIndex] != nil && a.players[ownerIndex].IsConnected {
			ownerValidAndConnected = true
			ownerWs = a.players[ownerIndex].Ws // Store the Ws associated with this player index
		}
	}

	cfg := a.cfg
	selfPID := a.selfPID
	engine := a.engine
	canvasSize := cfg.CanvasSize
	if ownerIndex != -1 && !ownerValidAndConnected {
		// fmt.Printf("WARN: GameActor %s cannot spawn ball for disconnected owner %d.\n", a.selfPID, ownerIndex) // Removed log
		return
	}
	if selfPID == nil || engine == nil {
		fmt.Printf("ERROR: GameActor %s cannot spawn ball, self PID or engine is nil.\n", a.selfPID)
		return
	}

	ballID := time.Now().Nanosecond() + ownerIndex + rand.Intn(1000)
	ballDataPtr := NewBall(cfg, x, y, ownerIndex, ballID, isPermanent) // Returns *Ball

	ballProducer := NewBallActorProducer(*ballDataPtr, selfPID, cfg) // Pass copy to producer
	ballPID := engine.Spawn(bollywood.NewProps(ballProducer))
	if ballPID == nil {
		fmt.Printf("ERROR: GameActor %s failed to spawn BallActor for owner %d, ball %d\n", a.selfPID, ownerIndex, ballID)
		return
	}

	// Re-verify owner connection before adding (important due to async nature)
	// Check if owner is still connected AND if the Ws connection matches (if applicable)
	stillValid := false
	if ownerIndex == -1 {
		stillValid = true // Ownerless balls are always valid to add
	} else if a.players[ownerIndex] != nil && a.players[ownerIndex].IsConnected && a.players[ownerIndex].Ws == ownerWs {
		stillValid = true
	}

	if stillValid {
		a.balls[ballID] = ballDataPtr // Store pointer in cache
		a.ballActors[ballID] = ballPID

		// Calculate initial R3F coords
		r3fX, r3fY := mapToR3FCoords(ballDataPtr.X, ballDataPtr.Y, canvasSize)

		// Generate BallSpawned update with R3F coords
		spawnUpdate := &BallSpawned{
			MessageType: "ballSpawned",
			Ball:        *ballDataPtr, // Dereference pointer to copy
			R3fX:        r3fX,
			R3fY:        r3fY,
		}
		a.addUpdate(spawnUpdate)

		// If initial phasing is requested, update cache, send command, and start GameActor timer
		if setInitialPhasing {
			ballDataPtr.Phasing = true // Update cache immediately
			a.startPhasingTimer(ballID) // Start GameActor's timer
			// Send SetPhasingCommand to BallActor so its internal state matches
			engine.Send(ballPID, SetPhasingCommand{}, selfPID)
		}

	} else {
		fmt.Printf("WARN: GameActor %s: Owner %d disconnected or changed before BallActor %s could be fully registered. Stopping actor.\n", a.selfPID, ownerIndex, ballPID)
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
			currentSelfPID := selfPID
			currentEngine := engine
			if currentEngine != nil && currentSelfPID != nil {
				currentEngine.Send(currentSelfPID, DestroyExpiredBall{BallID: ballID}, nil)
			}
		})
	}
}

// handleDestroyExpiredBall stops actor and generates BallRemoved update.
func (a *GameActor) handleDestroyExpiredBall(ctx bollywood.Context, ballID int) {
	pidToStop, actorExists := a.ballActors[ballID]
	ballState, stateExists := a.balls[ballID]

	if stateExists && ballState != nil && ballState.IsPermanent {
		return // Don't destroy permanent balls via expiry timer
	}

	currentEngine := a.engine
	if currentEngine == nil {
		fmt.Printf("ERROR: GameActor %s: Engine is nil in handleDestroyExpiredBall.\n", a.selfPID)
		if stateExists {
			delete(a.balls, ballID)
		}
		if actorExists {
			delete(a.ballActors, ballID)
		}
		return
	}

	// Check if both actor and state exist before proceeding
	if actorExists && stateExists && pidToStop != nil {
		delete(a.balls, ballID)
		delete(a.ballActors, ballID)
		a.stopPhasingTimer(ballID) // Stop phasing timer if it exists
		currentEngine.Stop(pidToStop)
		// Generate BallRemoved update
		removedUpdate := &BallRemoved{
			MessageType: "ballRemoved",
			ID:          ballID,
		}
		a.addUpdate(removedUpdate)
	} else {
		// Clean up maps even if one part is missing (e.g., state removed but actor stop failed)
		if stateExists {
			delete(a.balls, ballID)
		}
		if actorExists {
			delete(a.ballActors, ballID)
			if pidToStop != nil {
				// Attempt to stop actor even if state was missing
				currentEngine.Stop(pidToStop)
			}
		}
		a.stopPhasingTimer(ballID) // Attempt to stop timer regardless
	}
}

// handleStopPhasingTimerMsg is called internally when a phasing timer expires.
// It now ONLY updates the GameActor's cache. The BallPositionUpdate reflects this change.
func (a *GameActor) handleStopPhasingTimerMsg(ctx bollywood.Context, ballID int) {
	a.phasingTimersMu.Lock()
	delete(a.phasingTimers, ballID) // Remove timer reference
	a.phasingTimersMu.Unlock()

	ball, ballExists := a.balls[ballID]

	if ballExists && ball != nil {
		if ball.Phasing { // Only update cache if cache thinks it's phasing
			ball.Phasing = false // Update cache immediately
			// Log the change
			// if a.selfPID != nil {
			// fmt.Printf("PhasingTimer %s Ball %d: Timer expired. Set ball.Phasing = false in cache.\n", a.selfPID.String(), ballID) // Removed log
			// }
			// DO NOT send StopPhasingCommand to BallActor here.
			// The BallPositionUpdate sent on the next broadcast tick will contain Phasing: false.
		}
	}
}