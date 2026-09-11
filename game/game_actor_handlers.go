package game

import (
	"encoding/json"
	"fmt"
	"github.com/lguibr/pongo/internal/transport"
	"math/rand"
	"runtime/debug"
	"time"

	"github.com/lguibr/pongo/internal/actor"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

// --- Player Handlers ---

// handlePlayerConnect processes a player connection, sends initial state,
// and generates PlayerJoined update.
func (a *GameActor) handlePlayerConnect(ctx actor.Context, ws *websocket.Conn, sessionID string, client *transport.Client, replyTo *actor.PID, response interface{}) (admitted bool) {
	// Cancel cleanup timer if active, as a player is joining
	if a.roomCleanupTimer != nil {
		fmt.Printf("GameActor %s: Player joining, cancelling room cleanup timer.\n", a.selfPID)
		a.roomCleanupTimer.Stop()
		a.cleanupGeneration++
		a.roomCleanupTimer = nil
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("ERROR: Recovered from panic in handlePlayerConnect: %v\nStack: %s\n", r, string(debug.Stack()))
			// Close the connection that caused the panic to avoid inconsistent state
			if ws != nil {
				client.Close()
				if admitted {
					a.handlePlayerDisconnect(ctx, ws)
				}
			}
		}
	}()

	// Ensure ws is not nil for real connections
	if ws == nil {
		fmt.Printf("ERROR: GameActor %s: Received connect assignment with nil websocket connection.\n", a.selfPID)
		return // Do not proceed if connection is nil in production path
	}
	remoteAddr := ws.RemoteAddr().String()

	playerIndex := -1
	if _, ok := a.connToIndex[ws]; ok {
		// Player already connected, ignore duplicate assignment attempt
		fmt.Printf("WARN: GameActor %s: Ignoring duplicate connect assignment for %s.\n", a.selfPID, remoteAddr)
		return
	}
	if playerIndex == -1 {
		// Check if this is a reconnection attempt
		for i, p := range a.players {
			if p != nil {
				fmt.Printf("GameActor %s: Checking player %d for reconnect. Stored SessionID: %s, Incoming: %s, Connected: %v\n", a.selfPID, i, p.SessionID, sessionID, p.IsConnected)
				if sessionID != "" && !p.IsConnected && p.SessionID == sessionID {
					playerIndex = i
					fmt.Printf("GameActor %s: MATCH FOUND! Reconnecting player %d (Session: %s)\n", a.selfPID, playerIndex, sessionID)
					break
				}
			}
		}
	}

	if playerIndex == -1 {
		// Check for empty slots for new player
		for i, p := range a.players {
			if p == nil {
				playerIndex = i
				break
			}
		}
	}

	if playerIndex == -1 {
		fmt.Printf("WARN: GameActor %s: Room is full (%d players). Rejecting connection %s.\n", a.selfPID, utils.MaxPlayers, remoteAddr)
		client.Close()
		return
	}

	// Stop reconnect timer if it exists for this player
	if timer, exists := a.reconnectTimers[playerIndex]; exists && timer != nil {
		timer.Stop()
		delete(a.reconnectTimers, playerIndex)
		fmt.Printf("GameActor %s: Stopped reconnect timer for player %d.\n", a.selfPID, playerIndex)
	}

	isFirstPlayerInRoom := true
	existingPlayerCountForAvgScore := 0
	var totalScoreOfExistingPlayers int32 = 0

	for i, p := range a.players {
		if p != nil && i != playerIndex { // Check other players
			isFirstPlayerInRoom = false
			if p.IsConnected {
				totalScoreOfExistingPlayers += p.Score.Load()
				existingPlayerCountForAvgScore++
			}
		}
	}

	initialPlayerScore := int32(a.cfg.InitialScore)
	if !isFirstPlayerInRoom && existingPlayerCountForAvgScore > 0 {
		initialPlayerScore = totalScoreOfExistingPlayers / int32(existingPlayerCountForAvgScore)
	}

	if !a.gridInitialized {
		fmt.Printf("GameActor %s: First player joined. Initializing grid and starting tickers.\n", a.selfPID)
		if a.canvas == nil {
			a.canvas = NewCanvas(a.cfg.CanvasSize, a.cfg.GridSize)
		}
		// Use config when filling grid
		a.canvas.Grid.FillSymmetrical(a.cfg)
		a.gridInitialized = true
		a.gridDirty = true
		a.startBroadcastTicker(ctx) // Only start broadcast ticker in lobby
	} else if a.canvas == nil || a.canvas.Grid == nil {
		fmt.Printf("ERROR: GameActor %s: Joining player %d but grid/canvas not initialized!\n", a.selfPID, playerIndex)
		client.Close()
		return
	}

	// Create player info and paddle data
	// If reconnecting, reuse existing data but update connection info
	var player *playerInfo
	if a.players[playerIndex] != nil {
		player = a.players[playerIndex]
		player.Ws = ws
		player.IsConnected = true
		// SessionID should match, but we can update it just in case? No, keep original or update if provided.
		// If it was a reconnect, sessionID matched.
		// If it was a new connection into a nil slot, we need to create new.

		// Verify paddle exists
		if a.paddles[playerIndex] == nil {
			fmt.Printf("ERROR: GameActor %s: Reconnecting player %d has NIL paddle! Re-creating.\n", a.selfPID, playerIndex)
			paddleDataPtr := NewPaddle(a.cfg, playerIndex)
			a.paddles[playerIndex] = paddleDataPtr

		}
	} else {
		// New Player
		playerDataPtr := NewPlayer(a.canvas, playerIndex) // Returns *Player
		playerDataPtr.Score = initialPlayerScore          // Apply calculated initial score

		player = &playerInfo{
			Index:       playerIndex,
			ID:          playerDataPtr.Id,
			Color:       playerDataPtr.Color,
			Ws:          ws, // Store the actual connection
			IsConnected: true,
			SessionID:   sessionID,
		}
		player.Score.Store(initialPlayerScore) // Set initial score atomically

		a.players[playerIndex] = player

		// Create paddle data only for new players
		paddleDataPtr := NewPaddle(a.cfg, playerIndex) // Returns *Paddle
		a.paddles[playerIndex] = paddleDataPtr         // Store pointer in cache

	}

	player.Client = client
	a.connToIndex[ws] = playerIndex
	a.playerConns[playerIndex] = ws
	admitted = true
	if !a.engine.Send(replyTo, AssignRoomResponse{RoomPID: a.selfPID}, a.selfPID) {
		a.handlePlayerDisconnect(ctx, ws)
		return
	}
	a.cancelCountdown(ctx)
	if response != nil {
		if client.Send(response) != nil {
			a.handlePlayerDisconnect(ctx, ws)
			return
		}
	}

	if a.forceStartPending {
		a.handleForceStartGame(ctx)
	}
	// Queue initial state after the handler has received its room identity.
	assignmentMsg := PlayerAssignmentMessage{
		MessageType: "playerAssignment",
		PlayerIndex: playerIndex,
		Phase:       a.phaseToString(),
	}
	errAssign := client.Send(assignmentMsg)
	if errAssign != nil {
		fmt.Printf("ERROR: GameActor %s: Failed to send PlayerAssignmentMessage to player %d (%s): %v\n", a.selfPID, playerIndex, remoteAddr, errAssign)
		a.handlePlayerDisconnect(ctx, ws) // Trigger disconnect handling
		return
	}

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
			r3fX, r3fY := mapToR3FCoords(paddle.X+paddle.Width/2, paddle.Y+paddle.Height/2, a.cfg.CanvasSize)
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
			r3fX, r3fY := mapToR3FCoords(ball.X, ball.Y, a.cfg.CanvasSize)
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
	errEntities := client.Send(initialEntitiesMsg)
	if errEntities != nil {
		fmt.Printf("ERROR: GameActor %s: Failed to send InitialPlayersAndBallsState to player %d (%s): %v\n", a.selfPID, playerIndex, remoteAddr, errEntities)
		a.handlePlayerDisconnect(ctx, ws) // Trigger disconnect handling
		return
	}
	// --- End Initial State Send ---

	// --- Generate Updates for Broadcast ---
	// Add PlayerJoined update for other clients (including R3F coords)
	// We send this for BOTH new players and reconnecting players to ensure everyone has the latest state/coords.
	if paddle := a.paddles[playerIndex]; paddle != nil {
		// Calculate R3F coords for the paddle
		r3fX, r3fY := mapToR3FCoords(paddle.X+paddle.Width/2, paddle.Y+paddle.Height/2, a.cfg.CanvasSize)

		// Construct Player struct manually since we don't have a helper
		pInfo := a.players[playerIndex]
		pData := Player{
			Index: pInfo.Index,
			Id:    pInfo.ID,
			Color: pInfo.Color,
			Score: pInfo.Score.Load(),
		}

		playerJoinedMsg := &PlayerJoined{
			MessageType: "playerJoined",
			Player:      pData,
			Paddle:      *paddle,
			R3fX:        r3fX,
			R3fY:        r3fY,
		}
		a.addUpdate(playerJoinedMsg)
		fmt.Printf("GameActor %s: Broadcasted PlayerJoined for player %d\n", a.selfPID, playerIndex)
	} else {
		fmt.Printf("WARN: GameActor %s: Cannot broadcast PlayerJoined for player %d - Paddle is nil!\n", a.selfPID, playerIndex)
	}

	// Spawn initial Ball Actor (will generate BallSpawned update with R3F coords)
	// Initial balls for players should not start phasing.
	hasPermanent := false
	for _, ball := range a.balls {
		if ball.IsPermanent && ball.OwnerIndex == playerIndex {
			hasPermanent = true
			break
		}
	}
	if !hasPermanent {
		a.spawnBall(ctx, playerIndex, 0, 0, 0, true, false)
	}

	// --- Broadcast Lobby State ---
	lobbyState := &LobbyStateUpdate{
		MessageType: "lobbyState",
		Players:     make([]LobbyPlayerState, 0),
	}
	for _, p := range a.players {
		if p != nil && p.IsConnected {
			lobbyState.Players = append(lobbyState.Players, LobbyPlayerState{
				Index:   p.Index,
				IsReady: p.IsReady,
			})
		}
	}
	a.addUpdate(lobbyState)

	// --- Register with Broadcaster ---
	if a.broadcasterPID != nil {
		a.engine.Send(a.broadcasterPID, AddClient{Conn: ws, Client: client}, a.selfPID)
	} else {
		fmt.Printf("WARN: GameActor %s: BroadcasterPID is nil. Client %s will not receive updates.\n", a.selfPID, remoteAddr)
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

// handlePlayerDisconnect processes disconnect and generates PlayerLeft update.
func (a *GameActor) handlePlayerDisconnect(ctx actor.Context, conn *websocket.Conn) {
	if conn == nil {
		return
	}
	connAddr := "unknown"
	func() {
		defer func() {
			if r := recover(); r != nil {
				connAddr = "unknown (panic)"
			}
		}()
		if conn.RemoteAddr() != nil {
			connAddr = conn.RemoteAddr().String()
		}
	}()
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
	pInfo.IsConnected = false
	pInfo.IsReady = false
	if paddle := a.paddles[playerIndex]; paddle != nil {
		paddle.Direction = ""
	}
	a.cancelCountdown(ctx)

	// Generate PlayerLeft update *before* stopping actors/cleaning state
	playerLeftUpdate := &PlayerLeft{
		MessageType: "playerLeft",
		Index:       playerIndex,
	}
	a.addUpdate(playerLeftUpdate)

	// --- Broadcast Lobby State (so Lobby UI updates) ---
	lobbyState := &LobbyStateUpdate{
		MessageType: "lobbyState",
		Players:     make([]LobbyPlayerState, 0),
	}
	for _, p := range a.players {
		if p != nil && p.IsConnected {
			lobbyState.Players = append(lobbyState.Players, LobbyPlayerState{
				Index:   p.Index,
				IsReady: p.IsReady,
			})
		}
	}
	a.addUpdate(lobbyState)

	// The slot, paddle and balls are held for the reconnect grace period; the timer releases them.
	delete(a.connToIndex, conn)
	a.playerConns[playerIndex] = nil
	// Keep player info slot nilled until next connect
	// a.players[playerIndex] = nil // MOVED TO TIMER

	selfPID := a.selfPID
	engine := a.engine
	broadcasterPID := a.broadcasterPID

	// --- Stop Actors ---

	// --- Notify Broadcaster ---
	if broadcasterPID != nil {
		engine.Send(broadcasterPID, RemoveClient{Conn: conn}, selfPID)
	}

	fmt.Printf("GameActor %s: Player %d (%s) disconnected. Starting 30s grace period.\n", a.selfPID, playerIndex, connAddr)

	// Start Reconnect Timer
	if a.reconnectTimers[playerIndex] != nil {
		a.reconnectTimers[playerIndex].Stop()
	}
	a.reconnectGeneration++
	a.players[playerIndex].DisconnectGeneration = a.reconnectGeneration
	generation := a.players[playerIndex].DisconnectGeneration
	a.reconnectTimers[playerIndex] = time.AfterFunc(30*time.Second, func() {
		if a.engine != nil && a.selfPID != nil {
			a.engine.Send(a.selfPID, stopReconnectTimerMsg{PlayerIndex: playerIndex, Generation: generation}, nil)
		}
	})

	// Notify RoomManager that a player has left (to decrement count)
	// Wait, if we are in grace period, do we decrement count?
	// If we decrement, someone else might join and take the slot.
	// We should NOT decrement count yet. We hold the slot.

	// But we should update LobbyState so others see "Disconnected" status?
	// My LobbyState struct only has `IsReady`.
	// Maybe I should add `IsConnected` to LobbyState?
	// For now, `IsReady` will likely be false if they disconnect?
	// Or I can just leave them as is.

	// If I don't decrement count, the room might stay "full" with a disconnected player.
	// That's what we want for 30s.

	// If timer expires, THEN we decrement count and remove player.
}

// handleStopReconnectTimerMsg handles the expiry of the reconnection grace period.
func (a *GameActor) handleStopReconnectTimerMsg(ctx actor.Context, playerIndex int) {
	// Actor context is single-threaded per actor, so no lock needed for state.
	// But `reconnectTimers` access might need care if accessed from other goroutines?
	// `time.AfterFunc` runs in its own goroutine, but it sends a message to the actor.
	// So `handleStopReconnectTimerMsg` runs in the actor's main loop. Safe.
	// `reconnectTimers` is accessed in Receive loop. Safe.

	// Check if player is still disconnected
	if playerIndex < 0 || playerIndex >= utils.MaxPlayers || a.players[playerIndex] == nil {
		return
	}

	pInfo := a.players[playerIndex]
	if pInfo.IsConnected {
		// Player reconnected before message was processed?
		// Timer should have been stopped.
		return
	}

	fmt.Printf("GameActor %s: Reconnect timer expired for player %d. Removing permanently.\n", a.selfPID, playerIndex)

	// Now perform the actual removal (logic from original handlePlayerDisconnect)

	// Generate PlayerLeft update
	playerLeftUpdate := &PlayerLeft{
		MessageType: "playerLeft",
		Index:       playerIndex,
	}
	a.addUpdate(playerLeftUpdate)

	a.paddles[playerIndex] = nil

	for id, ball := range a.balls {
		if ball.OwnerIndex == playerIndex {
			if ball.IsPermanent {
				ball.OwnerIndex = -1
				a.addUpdate(&BallOwnershipChange{MessageType: "ballOwnerChanged", ID: id, NewOwnerIndex: -1})
			} else {
				a.handleDestroyExpiredBall(ctx, id)
			}
		}
	}

	// --- Clean up GameActor state ---
	// Capture SessionID before clearing player data
	sessionID := ""
	if a.players[playerIndex] != nil {
		sessionID = a.players[playerIndex].SessionID
	}

	a.playerConns[playerIndex] = nil
	a.players[playerIndex] = nil
	delete(a.reconnectTimers, playerIndex)

	// Notify RoomManager that a player has left (to decrement count and clear session)
	if a.roomManagerPID != nil && a.selfPID != nil {
		a.engine.Send(a.roomManagerPID, PlayerLeftRoom{RoomPID: a.selfPID, SessionID: sessionID}, nil)
	}

	// Check if room is empty
	playersLeft := false
	for _, p := range a.players {
		if p != nil && p.IsConnected {
			playersLeft = true
			break
		}
	}

	if !playersLeft && !a.gameOver.Load() {
		fmt.Printf("GameActor %s: Room is empty after timeout. Starting cleanup timer.\n", a.selfPID)
		if a.roomCleanupTimer != nil {
			a.roomCleanupTimer.Stop()
		}
		a.cleanupGeneration++
		generation := a.cleanupGeneration
		a.roomCleanupTimer = time.AfterFunc(30*time.Second, func() {
			if a.engine != nil && a.selfPID != nil {
				a.engine.Send(a.selfPID, RoomCleanupTimeout{Generation: generation}, nil)
			}
		})
	}

	// Broadcast Lobby State
	lobbyState := &LobbyStateUpdate{
		MessageType: "lobbyState",
		Players:     make([]LobbyPlayerState, 0),
	}
	for _, p := range a.players {
		if p != nil && p.IsConnected {
			lobbyState.Players = append(lobbyState.Players, LobbyPlayerState{
				Index:   p.Index,
				IsReady: p.IsReady,
			})
		}
	}
	a.addUpdate(lobbyState)
}

// handleRoomCleanupTimeout is called when the empty room grace period expires.
func (a *GameActor) handleRoomCleanupTimeout(ctx actor.Context) {
	// Re-check if room is still empty (it should be, but good to verify)
	playersLeft := false
	for _, p := range a.players {
		if p != nil && p.IsConnected {
			playersLeft = true
			break
		}
	}

	if !playersLeft && !a.gameOver.Load() {
		fmt.Printf("GameActor %s: Cleanup timer expired. Room still empty. Notifying RoomManager %s.\n", a.selfPID, a.roomManagerPID)
		if a.roomManagerPID != nil && a.selfPID != nil {
			a.engine.Send(a.roomManagerPID, GameRoomEmpty{RoomPID: a.selfPID}, nil)
		} else {
			fmt.Printf("ERROR: GameActor %s cannot notify RoomManager, PID is nil. Stopping self.\n", a.selfPID)
			if a.selfPID != nil {
				a.engine.Stop(a.selfPID)
			}
		}
	} else {
		fmt.Printf("GameActor %s: Cleanup timer expired but room is not empty or game over. Ignoring.\n", a.selfPID)
	}
}

// --- Input Handler ---

// handlePaddleDirection sets the direction of the sending player's paddle.
func (a *GameActor) handlePaddleDirection(ctx actor.Context, wsConn *websocket.Conn, directionData []byte) {
	if wsConn == nil {
		return
	}
	playerIndex, playerFound := a.connToIndex[wsConn]
	if !playerFound || playerIndex < 0 || playerIndex >= utils.MaxPlayers || a.players[playerIndex] == nil || !a.players[playerIndex].IsConnected {
		return
	}
	var direction Direction
	if json.Unmarshal(directionData, &direction) != nil {
		return
	}
	if paddle := a.paddles[playerIndex]; paddle != nil {
		paddle.Direction = utils.DirectionFromString(direction.Direction)
	}

}

// --- Ball Handlers ---

// spawnBall adds a ball to the room and queues its BallSpawned update including R3F coords.
// The setInitialPhasing flag determines if the ball starts phasing (used by power-ups).
func (a *GameActor) spawnBall(ctx actor.Context, ownerIndex, x, y int, expireIn time.Duration, isPermanent bool, setInitialPhasing bool) {
	if ownerIndex < -1 || ownerIndex >= utils.MaxPlayers {
		fmt.Printf("WARN: GameActor %s received spawnBall request with invalid owner index %d.\n", a.selfPID, ownerIndex)
		return
	}
	if ownerIndex >= 0 && (a.players[ownerIndex] == nil || !a.players[ownerIndex].IsConnected) {
		return
	}
	a.nextBallID++
	ballID := a.nextBallID
	// Test fixtures may use fixed IDs; never replace an existing entity.
	for a.balls[ballID] != nil {
		a.nextBallID++
		ballID = a.nextBallID
	}
	ball := NewBall(a.cfg, x, y, ownerIndex, ballID, isPermanent)
	a.balls[ballID] = ball
	if setInitialPhasing {
		ball.Phasing = true
		a.startPhasingTimer(ballID)
	}
	rx, ry := mapToR3FCoords(ball.X, ball.Y, a.cfg.CanvasSize)
	a.addUpdate(&BallSpawned{MessageType: "ballSpawned", Ball: *ball, R3fX: rx, R3fY: ry})
	if !isPermanent && expireIn > 0 {
		offset := time.Duration(rand.Intn(4000)-2000) * time.Millisecond
		duration := expireIn + offset
		if duration <= 0 {
			duration = 500 * time.Millisecond
		}
		if a.expiryTimers == nil {
			a.expiryTimers = make(map[int]*time.Timer)
		}
		a.expiryTimers[ballID] = time.AfterFunc(duration, func() { a.engine.Send(a.selfPID, DestroyExpiredBall{BallID: ballID}, nil) })
	}
}

func (a *GameActor) handleDestroyExpiredBall(ctx actor.Context, ballID int) {
	ball := a.balls[ballID]
	if ball == nil || ball.IsPermanent {
		return
	}
	delete(a.balls, ballID)
	if timer := a.expiryTimers[ballID]; timer != nil {
		timer.Stop()
		delete(a.expiryTimers, ballID)
	}
	delete(a.phasingGeneration, ballID)
	a.stopPhasingTimer(ballID)
	for _, key := range a.activeCollisions.GetActiveCollisionsForKey1(ballID) {
		a.activeCollisions.EndCollision(key)
	}
	a.addUpdate(&BallRemoved{MessageType: "ballRemoved", ID: ballID})
}

func (a *GameActor) handleStopPhasingTimerMsg(ctx actor.Context, ballID int) {
	a.stopPhasingTimer(ballID)
	if ball := a.balls[ballID]; ball != nil {
		ball.Phasing = false
	}
}

// --- Lobby Handlers ---

// handlePlayerReady toggles a player's ready state and checks if all players are ready.
func (a *GameActor) handlePlayerReady(ctx actor.Context, wsConn *websocket.Conn, isReady bool) {
	if wsConn == nil {
		return
	}
	playerIndex, found := a.connToIndex[wsConn]
	if !found || a.players[playerIndex] == nil {
		return
	}

	fmt.Printf("GameActor %s: Player %d set ready to %v\n", a.selfPID, playerIndex, isReady)

	// Update readiness
	a.players[playerIndex].IsReady = isReady

	// Broadcast Lobby State
	lobbyState := &LobbyStateUpdate{
		MessageType: "lobbyState",
		Players:     make([]LobbyPlayerState, 0),
	}

	allReady := true
	playerCount := 0
	for _, p := range a.players {
		if p != nil && p.IsConnected {
			playerCount++
			lobbyState.Players = append(lobbyState.Players, LobbyPlayerState{
				Index:   p.Index,
				IsReady: p.IsReady,
			})
			if !p.IsReady {
				allReady = false
			}
		}
	}
	a.addUpdate(lobbyState)

	// Check if we should start countdown or cancel it
	if allReady && playerCount > 0 {
		if a.phase == PhaseLobby {
			fmt.Printf("GameActor %s: All players ready. Starting countdown.\n", a.selfPID)
			a.startCountdown(ctx)
		}
	} else {
		a.cancelCountdown(ctx)
	}
}

// startCountdown initiates the 3-second countdown.
func (a *GameActor) startCountdown(ctx actor.Context) {
	if a.phase != PhaseLobby {
		return
	}
	a.phase = PhaseCountingDown
	a.countdownGeneration++
	a.engine.Send(a.roomManagerPID, RoomPhaseUpdate{RoomPID: a.selfPID, Phase: a.phase}, a.selfPID)
	// Start the countdown sequence at 3
	a.handleCountdownTick(ctx, 3)
}

// handleCountdownTick processes a tick of the countdown.
func (a *GameActor) handleCountdownTick(ctx actor.Context, secondsRemaining int) {
	if a.phase != PhaseCountingDown {
		return // Countdown was cancelled or game started
	}

	// Broadcast current countdown value
	a.addUpdate(&GameStartCountdown{
		MessageType: "gameStartCountdown",
		Seconds:     secondsRemaining,
	})

	if secondsRemaining > 0 {
		// Schedule next tick or start game
		generation := a.countdownGeneration
		a.countdownTimer = time.AfterFunc(1*time.Second, func() {
			if a.engine != nil && a.selfPID != nil {
				if secondsRemaining > 1 {
					a.engine.Send(a.selfPID, CountdownTick{SecondsRemaining: secondsRemaining - 1, Generation: generation}, nil)
				} else {
					a.engine.Send(a.selfPID, startGameMsg{Generation: generation}, nil)
				}
			}
		})
	}
}

// startGame transitions the room to the playing phase.
func (a *GameActor) startGame(ctx actor.Context) {
	fmt.Printf("GameActor %s: startGame called. Current Phase: %v\n", a.selfPID, a.phase)
	if a.phase != PhaseCountingDown {
		return
	}
	a.phase = PhasePlaying
	a.addUpdate(&GameStarted{MessageType: "gameStarted"})

	// Notify RoomManager
	if a.roomManagerPID != nil && a.engine != nil && a.selfPID != nil {
		a.engine.Send(a.roomManagerPID, RoomPhaseUpdate{
			RoomPID: a.selfPID,
			Phase:   PhasePlaying,
		}, a.selfPID)
	}

	// Start the game loop (tickers) if not already running
	// (Tickers might be running for lobby physics if we wanted, but usually we start them here)
	// In current implementation, tickers start on first player join.
	// We might want to PAUSE physics in lobby?
	// For now, let's leave physics running (warmup) but maybe reset positions?
	// Let's just proceed with phase change.
	a.startPhysicsTicker(ctx)
}

// handleForceStartGame transitions the room to the playing phase immediately.
func (a *GameActor) handleForceStartGame(ctx actor.Context) {
	if !a.gridInitialized {
		a.forceStartPending = true
		return
	}
	a.forceStartPending = false

	fmt.Printf("GameActor %s: handleForceStartGame called. Current Phase: %v\n", a.selfPID, a.phase)
	if a.phase == PhasePlaying {
		return
	}

	// Cancel countdown if active
	if a.countdownTimer != nil {
		a.countdownTimer.Stop()
		a.countdownTimer = nil
	}

	a.phase = PhasePlaying
	a.addUpdate(&GameStarted{MessageType: "gameStarted"})

	// Notify RoomManager
	if a.roomManagerPID != nil && a.engine != nil && a.selfPID != nil {
		a.engine.Send(a.roomManagerPID, RoomPhaseUpdate{
			RoomPID: a.selfPID,
			Phase:   PhasePlaying,
		}, a.selfPID)
	}

	// Ensure tickers are running
	a.startPhysicsTicker(ctx)
}

// phaseToString converts the Phase enum to a string expected by the frontend.
func (a *GameActor) phaseToString() string {
	switch a.phase {
	case PhaseLobby:
		return "lobby"
	case PhaseCountingDown:
		return "countingDown"
	case PhasePlaying:
		return "playing"
	default:
		return "lobby"
	}
}

// handleAdmission validates before sending success, and rolls back reservations
// if the connection disappears before it becomes a room member.
func (a *GameActor) handleAdmission(ctx actor.Context, m AssignPlayerToRoom) {
	reject := func(reason string) {
		a.engine.Send(a.roomManagerPID, AdmissionRejected{RoomPID: a.selfPID, SessionID: m.SessionID, Reserved: m.Reserved, ReplyTo: m.ReplyTo}, a.selfPID)
		a.engine.Send(m.ReplyTo, RoomJoinedResponse{MessageType: "roomJoined", Success: false, Reason: reason}, a.selfPID)
	}
	if a.isStopping.Load() || a.gameOver.Load() {
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
