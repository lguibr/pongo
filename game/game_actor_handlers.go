package game

import (
	"fmt"
	"math/rand"
	"runtime/debug"
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

// --- Player Handlers ---

// handlePlayerConnect processes a player connection, sends initial state,
// and generates PlayerJoined update.
func (a *GameActor) handlePlayerConnect(ctx bollywood.Context, ws *websocket.Conn, sessionID string) {
	// Cancel cleanup timer if active, as a player is joining
	if a.roomCleanupTimer != nil {
		fmt.Printf("GameActor %s: Player joining, cancelling room cleanup timer.\n", a.selfPID)
		a.roomCleanupTimer.Stop()
		a.roomCleanupTimer = nil
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("ERROR: Recovered from panic in handlePlayerConnect: %v\nStack: %s\n", r, string(debug.Stack()))
			// Close the connection that caused the panic to avoid inconsistent state
			if ws != nil {
				_ = ws.Close()
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
				if !p.IsConnected && p.SessionID == sessionID {
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
		_ = ws.Close()
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

	if isFirstPlayerInRoom {
		fmt.Printf("GameActor %s: First player joined. Initializing grid and starting tickers.\n", a.selfPID)
		if a.canvas == nil {
			a.canvas = NewCanvas(a.cfg.CanvasSize, a.cfg.GridSize)
		}
		// Use config when filling grid
		a.canvas.Grid.FillSymmetrical(a.cfg)
		a.startBroadcastTicker(ctx) // Only start broadcast ticker in lobby
	} else if a.canvas == nil || a.canvas.Grid == nil {
		fmt.Printf("ERROR: GameActor %s: Joining player %d but grid/canvas not initialized!\n", a.selfPID, playerIndex)
		_ = ws.Close()
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

			// Check if actor exists, if not spawn it
			if a.paddleActors[playerIndex] == nil {
				paddleProducer := NewPaddleActorProducer(*paddleDataPtr, a.selfPID, a.cfg)
				paddlePID := a.engine.Spawn(bollywood.NewProps(paddleProducer))
				a.paddleActors[playerIndex] = paddlePID
			}
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

		// Spawn Paddle Actor
		paddleProducer := NewPaddleActorProducer(*paddleDataPtr, a.selfPID, a.cfg) // Pass copy to producer
		paddlePID := a.engine.Spawn(bollywood.NewProps(paddleProducer))
		if paddlePID == nil {
			fmt.Printf("ERROR: GameActor %s failed to spawn PaddleActor for player %d\n", a.selfPID, playerIndex)
			delete(a.connToIndex, ws)
			a.players[playerIndex] = nil
			a.paddles[playerIndex] = nil
			a.playerConns[playerIndex] = nil
			if a.broadcasterPID != nil {
				a.engine.Send(a.broadcasterPID, RemoveClient{Conn: ws}, a.selfPID)
			}
			_ = ws.Close()
			return
		}
		a.paddleActors[playerIndex] = paddlePID
	}

	a.connToIndex[ws] = playerIndex
	a.playerConns[playerIndex] = ws

	// --- Send Initial State Directly to Client using JSON.Send ---
	assignmentMsg := PlayerAssignmentMessage{
		MessageType: "playerAssignment",
		PlayerIndex: playerIndex,
		Phase:       a.phaseToString(),
	}
	errAssign := websocket.JSON.Send(ws, assignmentMsg)
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
	errEntities := websocket.JSON.Send(ws, initialEntitiesMsg)
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
	a.spawnBall(ctx, playerIndex, 0, 0, 0, true, false)

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
		a.engine.Send(a.broadcasterPID, AddClient{Conn: ws}, a.selfPID)
	} else {
		fmt.Printf("WARN: GameActor %s: BroadcasterPID is nil. Client %s will not receive updates.\n", a.selfPID, remoteAddr)
	}

	// --- Send Initial Grid State Immediately ---
	// This ensures the client has the bricks to render the game (R3FCanvas waits for brickStates > 0)
	// without waiting for the next broadcast tick (which might be delayed or interval-based).
	if a.canvas != nil && a.canvas.Grid != nil {
		brickUpdates := []BrickStateUpdate{}
		rows := len(a.canvas.Grid)
		if rows > 0 {
			cols := len(a.canvas.Grid[0])
			for r := 0; r < rows; r++ {
				for c := 0; c < cols; c++ {
					cell := a.canvas.Grid[r][c]
					if cell.Data != nil {
						// Calculate R3F coords for cell center
						r3fX, r3fY := mapToR3FCoords(int(float64(c)*float64(a.cfg.GridSize)+float64(a.cfg.GridSize)/2), int(float64(r)*float64(a.cfg.GridSize)+float64(a.cfg.GridSize)/2), a.cfg.CanvasSize)
						brickUpdates = append(brickUpdates, BrickStateUpdate{
							X:    r3fX,
							Y:    r3fY,
							Life: cell.Data.Life,
							Type: cell.Data.Type,
						})
					}
				}
			}
		}

		fullGridUpdate := FullGridUpdate{
			MessageType: "fullGridUpdate",
			CellSize:    a.cfg.GridSize,
			Bricks:      brickUpdates,
		}

		// Wrap in batch as client expects updates in batch or specific messages
		// Client handles isFullGridUpdate inside isGameUpdatesBatch
		batchMsg := GameUpdatesBatch{
			MessageType: "gameUpdates",
			Updates:     []interface{}{fullGridUpdate},
		}

		errGrid := websocket.JSON.Send(ws, batchMsg)
		if errGrid != nil {
			fmt.Printf("WARN: GameActor %s: Failed to send initial FullGridUpdate to player %d: %v\n", a.selfPID, playerIndex, errGrid)
		} else {
			fmt.Printf("GameActor %s: Sent initial FullGridUpdate to player %d.\n", a.selfPID, playerIndex)
		}
	}

	fmt.Printf("GameActor %s: Player %d (%s) setup complete with score %d.\n", a.selfPID, playerIndex, remoteAddr, initialPlayerScore)
}

// handlePlayerDisconnect processes disconnect and generates PlayerLeft update.
func (a *GameActor) handlePlayerDisconnect(ctx bollywood.Context, conn *websocket.Conn) {
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
	pInfo.IsConnected = false // Mark as disconnected first

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

	// --- Stop Actors and Manage Persistent Ball ---
	// DO NOT stop actors yet! We are in grace period.
	// We only stop them if the timer expires.

	/*
		paddleToStop := a.paddleActors[playerIndex]
		a.paddleActors[playerIndex] = nil
		a.paddles[playerIndex] = nil // Clear paddle cache
		...
	*/

	// --- Clean up GameActor state ---

	// --- Clean up GameActor state ---
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
	a.reconnectTimers[playerIndex] = time.AfterFunc(30*time.Second, func() {
		if a.engine != nil && a.selfPID != nil {
			a.engine.Send(a.selfPID, stopReconnectTimerMsg{PlayerIndex: playerIndex}, nil)
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
func (a *GameActor) handleStopReconnectTimerMsg(ctx bollywood.Context, playerIndex int) {
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

	// --- Clean up GameActor state ---
	// Capture SessionID before clearing player data
	sessionID := ""
	if a.players[playerIndex] != nil {
		sessionID = a.players[playerIndex].SessionID
	}

	a.playerConns[playerIndex] = nil
	a.players[playerIndex] = nil
	delete(a.reconnectTimers, playerIndex)

	// --- Stop Actors ---
	if paddleToStop != nil {
		a.engine.Stop(paddleToStop)
	}

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
		a.roomCleanupTimer = time.AfterFunc(30*time.Second, func() {
			if a.engine != nil && a.selfPID != nil {
				a.engine.Send(a.selfPID, RoomCleanupTimeout{}, nil)
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
func (a *GameActor) handleRoomCleanupTimeout(ctx bollywood.Context) {
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

// handlePaddleDirection forwards command to PaddleActor.
func (a *GameActor) handlePaddleDirection(ctx bollywood.Context, wsConn *websocket.Conn, directionData []byte) {
	if wsConn == nil {
		return
	}
	playerIndex, playerFound := a.connToIndex[wsConn]
	var pid *bollywood.PID
	isValidPlayer := playerFound && playerIndex >= 0 && playerIndex < utils.MaxPlayers && a.players[playerIndex] != nil && a.players[playerIndex].IsConnected && a.players[playerIndex].Ws == wsConn
	if isValidPlayer {
		pid = a.paddleActors[playerIndex]
	}
	if pid != nil {
		a.engine.Send(pid, PaddleDirectionMessage{Direction: directionData}, ctx.Self())
	}
}

// --- Ball Handlers ---

// spawnBall spawns actor and generates BallSpawned update including R3F coords.
// The setInitialPhasing flag determines if the ball starts phasing (used by power-ups).
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

		// If initial phasing is requested (e.g., by a power-up), update cache, send command, and start GameActor timer
		if setInitialPhasing {
			ballDataPtr.Phasing = true  // Update cache immediately
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
// It now updates the GameActor's cache and sends StopPhasingCommand to BallActor.
func (a *GameActor) handleStopPhasingTimerMsg(ctx bollywood.Context, ballID int) {
	a.phasingTimersMu.Lock()
	delete(a.phasingTimers, ballID) // Remove timer reference
	a.phasingTimersMu.Unlock()

	ball, ballExists := a.balls[ballID]
	ballActorPID, actorPIDExists := a.ballActors[ballID]

	if ballExists && ball != nil {
		if ball.Phasing { // Only update cache if cache thinks it's phasing
			ball.Phasing = false // Update cache immediately
			// Send StopPhasingCommand to BallActor to synchronize its state
			if actorPIDExists && ballActorPID != nil && a.engine != nil && a.selfPID != nil {
				a.engine.Send(ballActorPID, StopPhasingCommand{}, a.selfPID)
			}
		}
	}
}

// --- Lobby Handlers ---

// handlePlayerReady toggles a player's ready state and checks if all players are ready.
func (a *GameActor) handlePlayerReady(ctx bollywood.Context, wsConn *websocket.Conn, isReady bool) {
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
		if a.phase == PhaseCountingDown {
			// Cancel countdown
			if a.countdownTimer != nil {
				a.countdownTimer.Stop()
				a.countdownTimer = nil
			}
			a.phase = PhaseLobby
			a.addUpdate(&GameStartCancelled{
				MessageType: "gameStartCancelled",
				Reason:      "A player is not ready",
			})
			fmt.Printf("GameActor %s: Countdown cancelled.\n", a.selfPID)
		}
	}
}

// startCountdown initiates the 3-second countdown.
func (a *GameActor) startCountdown(ctx bollywood.Context) {
	if a.phase != PhaseLobby {
		return
	}
	a.phase = PhaseCountingDown
	// Start the countdown sequence at 3
	a.handleCountdownTick(ctx, 3)
}

// handleCountdownTick processes a tick of the countdown.
func (a *GameActor) handleCountdownTick(ctx bollywood.Context, secondsRemaining int) {
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
		a.countdownTimer = time.AfterFunc(1*time.Second, func() {
			if a.engine != nil && a.selfPID != nil {
				if secondsRemaining > 1 {
					a.engine.Send(a.selfPID, CountdownTick{SecondsRemaining: secondsRemaining - 1}, nil)
				} else {
					a.engine.Send(a.selfPID, startGameMsg{}, nil)
				}
			}
		})
	}
}

// startGame transitions the room to the playing phase.
func (a *GameActor) startGame(ctx bollywood.Context) {
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
func (a *GameActor) handleForceStartGame(ctx bollywood.Context) {
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
