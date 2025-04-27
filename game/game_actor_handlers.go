// File: game/game_actor_handlers.go
package game

import (
	"fmt"
	"time"

	"github.com/lguibr/pongo/bollywood"
	"github.com/lguibr/pongo/utils"
)

// handlePlayerConnect processes a new player connection request.
func (a *GameActor) handlePlayerConnect(ctx bollywood.Context, ws PlayerConnection) {
	actorPIDStr := "nil"
	if a.selfPID != nil {
		actorPIDStr = a.selfPID.String()
	}
	fmt.Printf("GameActor %s: handlePlayerConnect - Start.\n", actorPIDStr)

	remoteAddr := "unknown"
	if ws != nil {
		remoteAddr = ws.RemoteAddr().String()
	} else {
		fmt.Printf("GameActor %s: Received connect request with nil connection.\n", actorPIDStr)
		return // Cannot proceed with nil connection
	}

	// --- Section 1: Find slot, check validity (Requires Lock) ---
	fmt.Printf("GameActor %s: handlePlayerConnect - Acquiring lock for slot check...\n", actorPIDStr)
	a.mu.Lock()

	// Check if this connection is already associated with a player
	if existingIndex, ok := a.connToIndex[ws]; ok {
		fmt.Printf("GameActor %s: Connection %s already associated with player %d. Ignoring connect request.\n", actorPIDStr, remoteAddr, existingIndex)
		a.mu.Unlock()
		fmt.Printf("GameActor %s: handlePlayerConnect - Lock released (already connected).\n", actorPIDStr)
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
		fmt.Printf("GameActor %s: Server full, rejecting connection from: %s\n", actorPIDStr, remoteAddr)
		a.mu.Unlock() // Release lock before closing connection
		fmt.Printf("GameActor %s: handlePlayerConnect - Lock released (server full).\n", actorPIDStr)
		_ = ws.Close() // Close the connection directly
		return         // Return without adding the player
	}

	fmt.Printf("GameActor %s: Assigning player index %d to %s\n", actorPIDStr, playerIndex, remoteAddr)

	isFirstPlayer := true
	for i, p := range a.players {
		if p != nil && i != playerIndex { // Check other slots
			isFirstPlayer = false
			break
		}
	}
	if isFirstPlayer {
		fmt.Printf("GameActor %s: First player joining, initializing grid.\n", actorPIDStr)
		a.canvas.Grid.Fill(0, 0, 0, 0) // Modifies canvas state under lock
	}

	// Create player info and paddle data under lock
	player := &playerInfo{
		Index:       playerIndex,
		ID:          fmt.Sprintf("player%d", playerIndex),
		Score:       utils.InitialScore,
		Color:       utils.NewRandomColor(),
		Ws:          ws,
		IsConnected: true, // Mark as connected
	}
	a.players[playerIndex] = player
	a.connToIndex[ws] = playerIndex // Add mapping

	paddleData := NewPaddle(a.canvas.CanvasSize, playerIndex)
	a.paddles[playerIndex] = paddleData

	// --- Release Lock BEFORE Spawning Actors ---
	a.mu.Unlock()
	fmt.Printf("GameActor %s: handlePlayerConnect - Lock released before spawning actors.\n", actorPIDStr)

	// --- Section 2: Spawn Actors (No Lock Held) ---
	paddleProducer := NewPaddleActorProducer(*paddleData, ctx.Self())
	fmt.Printf("GameActor %s: handlePlayerConnect - Spawning PaddleActor for player %d...\n", actorPIDStr, playerIndex)
	paddlePID := a.engine.Spawn(bollywood.NewProps(paddleProducer))
	fmt.Printf("GameActor %s: handlePlayerConnect - Spawned PaddleActor %s for player %d.\n", actorPIDStr, paddlePID, playerIndex)

	// Store Paddle PID (Requires Lock)
	fmt.Printf("GameActor %s: handlePlayerConnect - Acquiring lock to store Paddle PID...\n", actorPIDStr)
	a.mu.Lock()
	fmt.Printf("GameActor %s: handlePlayerConnect - Lock acquired for Paddle PID.\n", actorPIDStr)
	// Check if player slot is still valid before storing PID
	if a.players[playerIndex] != nil && a.players[playerIndex].Ws == ws {
		a.paddleActors[playerIndex] = paddlePID
	} else {
		// Player disconnected between releasing lock and spawning
		fmt.Printf("GameActor %s: Player %d disconnected before PaddleActor PID %s could be stored. Stopping actor.\n", actorPIDStr, playerIndex, paddlePID)
		a.mu.Unlock() // Release lock before stopping
		fmt.Printf("GameActor %s: handlePlayerConnect - Lock released (player disconnected before paddle PID store).\n", actorPIDStr)
		if paddlePID != nil {
			a.engine.Stop(paddlePID)
		}
		// Don't proceed to spawn ball or broadcast
		return
	}
	a.mu.Unlock()
	fmt.Printf("GameActor %s: handlePlayerConnect - Lock released after storing Paddle PID.\n", actorPIDStr)

	// Spawn ball (No Lock Held during spawn itself)
	fmt.Printf("GameActor %s: handlePlayerConnect - Calling spawnBall for player %d...\n", actorPIDStr, playerIndex)
	a.spawnBall(ctx, playerIndex, 0, 0, 0) // spawnBall now manages its own lock internally for map updates
	fmt.Printf("GameActor %s: handlePlayerConnect - Returned from spawnBall for player %d.\n", actorPIDStr, playerIndex)

	// --- Section 3: Broadcast Initial State (No Lock Held) ---
	fmt.Printf("GameActor %s: handlePlayerConnect - Calling broadcastGameState...\n", actorPIDStr)
	a.broadcastGameState() // broadcastGameState acquires RLock internally
	fmt.Printf("GameActor %s: handlePlayerConnect - Finished broadcastGameState.\n", actorPIDStr)
	fmt.Printf("GameActor %s: handlePlayerConnect - Completed for %s.\n", actorPIDStr, remoteAddr)
}

// handlePlayerDisconnect processes a player disconnection event. It's the central cleanup point.
func (a *GameActor) handlePlayerDisconnect(ctx bollywood.Context, playerIndex int, conn PlayerConnection) {
	actorPIDStr := "nil"
	if a.selfPID != nil {
		actorPIDStr = a.selfPID.String()
	}
	fmt.Printf("GameActor %s: handlePlayerDisconnect - Acquiring lock...\n", actorPIDStr)
	a.mu.Lock() // Lock for modifying players, paddles, actors, connToIndex
	fmt.Printf("GameActor %s: handlePlayerDisconnect - Lock acquired.\n", actorPIDStr)

	// If index is unknown (-1), try to find it using the connection
	if playerIndex == -1 {
		if conn == nil {
			fmt.Printf("GameActor %s: Received disconnect with no index and no connection.\n", actorPIDStr)
			a.mu.Unlock()
			fmt.Printf("GameActor %s: handlePlayerDisconnect - Lock released (no index/conn).\n", actorPIDStr)
			return // Cannot proceed
		}
		if idx, ok := a.connToIndex[conn]; ok {
			playerIndex = idx
			fmt.Printf("GameActor %s: Found index %d for disconnecting connection %s\n", actorPIDStr, playerIndex, conn.RemoteAddr())
		} else {
			// Connection might have already been cleaned up by a previous disconnect signal for the same player.
			fmt.Printf("GameActor %s: Received disconnect for unknown or already cleaned up connection %s\n", actorPIDStr, conn.RemoteAddr())
			a.mu.Unlock()
			fmt.Printf("GameActor %s: handlePlayerDisconnect - Lock released (conn not found).\n", actorPIDStr)
			return // Cannot proceed without index
		}
	}

	// Validate index and check if player exists and is connected
	if playerIndex < 0 || playerIndex >= MaxPlayers || a.players[playerIndex] == nil { // Use exported constant
		fmt.Printf("GameActor %s: Received disconnect for invalid or already disconnected player index %d\n", actorPIDStr, playerIndex)
		a.mu.Unlock()
		fmt.Printf("GameActor %s: handlePlayerDisconnect - Lock released (invalid index).\n", actorPIDStr)
		return
	}

	// Check if the connection provided matches the one stored (if conn is not nil)
	pInfo := a.players[playerIndex]
	if conn != nil && pInfo.Ws != conn {
		// This might happen if disconnect signals race (e.g., read error and write error close together)
		fmt.Printf("GameActor %s: Received disconnect for player %d, but connection %s doesn't match stored connection %s. Assuming already handled.\n",
			actorPIDStr, playerIndex, conn.RemoteAddr(), pInfo.Ws.RemoteAddr())
		a.mu.Unlock()
		fmt.Printf("GameActor %s: handlePlayerDisconnect - Lock released (conn mismatch).\n", actorPIDStr)
		return
	}

	// Proceed with cleanup only if the player is currently marked as connected
	if !pInfo.IsConnected {
		fmt.Printf("GameActor %s: Player %d already marked as disconnected. Ignoring redundant disconnect signal.\n", actorPIDStr, playerIndex)
		a.mu.Unlock()
		fmt.Printf("GameActor %s: handlePlayerDisconnect - Lock released (already disconnected).\n", actorPIDStr)
		return
	}

	fmt.Printf("GameActor %s: Handling disconnect for player %d (%s)\n", actorPIDStr, playerIndex, pInfo.Ws.RemoteAddr())

	// Mark as disconnected immediately
	pInfo.IsConnected = false
	wsToClose := pInfo.Ws // Capture the connection interface before clearing

	// Collect PIDs to stop
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

	// Clear state maps/arrays while holding lock
	a.paddleActors[playerIndex] = nil // Clear PID
	for _, ballID := range ballsToRemoveFromState {
		delete(a.balls, ballID)      // Remove from live state
		delete(a.ballActors, ballID) // Remove PID reference
	}
	if pInfo.Ws != nil {
		delete(a.connToIndex, pInfo.Ws) // Remove connection mapping
	}
	a.players[playerIndex] = nil // Clear player info slot
	a.paddles[playerIndex] = nil // Clear paddle state

	a.mu.Unlock() // Unlock before potentially blocking operations (Stop, Close)
	fmt.Printf("GameActor %s: handlePlayerDisconnect - Lock released before stopping actors.\n", actorPIDStr)

	// Stop actors
	if paddleToStop != nil {
		fmt.Printf("GameActor %s: Stopping PaddleActor %s for player %d\n", actorPIDStr, paddleToStop, playerIndex)
		a.engine.Stop(paddleToStop)
	}
	for _, pid := range ballsToStop {
		fmt.Printf("GameActor %s: Stopping BallActor %s for disconnected player %d\n", actorPIDStr, pid, playerIndex)
		a.engine.Stop(pid)
	}

	// Close WebSocket
	if wsToClose != nil {
		fmt.Printf("GameActor %s: Closing WebSocket for player %d\n", actorPIDStr, playerIndex)
		_ = wsToClose.Close() // Attempt close, ignore error
	}

	fmt.Printf("GameActor %s: Player %d disconnected and cleaned up.\n", actorPIDStr, playerIndex)

	// Check remaining players (needs read lock)
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
		// Optionally: Reset grid or perform other cleanup
		// a.mu.Lock() // Requires write lock if done here
		// a.canvas.Grid.Fill(0, 0, 0, 0)
		// a.mu.Unlock()
	}

	// Broadcast state update to remaining players
	fmt.Printf("GameActor %s: handlePlayerDisconnect - Calling broadcastGameState...\n", actorPIDStr)
	a.broadcastGameState()
	fmt.Printf("GameActor %s: handlePlayerDisconnect - Finished broadcastGameState.\n", actorPIDStr)
}

// handlePaddleDirection forwards direction commands to the appropriate PaddleActor.
func (a *GameActor) handlePaddleDirection(ctx bollywood.Context, wsConn PlayerConnection, directionData []byte) {
	a.mu.RLock() // Read lock to access connToIndex and paddleActors
	playerIndex, playerFound := a.connToIndex[wsConn]
	var pid *bollywood.PID
	if playerFound && playerIndex >= 0 && playerIndex < MaxPlayers { // Use exported constant
		// Check if player is still considered connected before getting PID
		if pInfo := a.players[playerIndex]; pInfo != nil && pInfo.IsConnected {
			pid = a.paddleActors[playerIndex]
		}
	}
	a.mu.RUnlock()

	if pid == nil {
		// Don't log spam if player/connection is simply unknown or disconnected
		// fmt.Printf("GameActor: Ignoring paddle direction for unknown/disconnected player connection %s\n", wsConn.RemoteAddr())
		return
	}
	// Forward the raw JSON bytes
	a.engine.Send(pid, PaddleDirectionMessage{Direction: directionData}, ctx.Self())
}

// handlePaddlePositionUpdate updates the GameActor's internal state for a paddle.
func (a *GameActor) handlePaddlePositionUpdate(ctx bollywood.Context, paddleState *Paddle) {
	if paddleState == nil {
		return
	}
	a.mu.Lock() // Lock for write access
	defer a.mu.Unlock()

	idx := paddleState.Index
	// Only update if the player slot is still active and connected
	if idx >= 0 && idx < MaxPlayers && a.players[idx] != nil && a.players[idx].IsConnected { // Use exported constant
		// Update the state, ensuring not to overwrite the Ws field if paddleState doesn't include it
		// Best practice: Only update fields relevant to position/state from the message
		if a.paddles[idx] != nil {
			a.paddles[idx].X = paddleState.X
			a.paddles[idx].Y = paddleState.Y
			a.paddles[idx].Direction = paddleState.Direction // Update direction too
		} else {
			// This case shouldn't happen if player is connected, but handle defensively
			a.paddles[idx] = paddleState
		}
	} else {
		// fmt.Printf("GameActor: Received paddle update for inactive/invalid index %d\n", idx)
	}
}

// handleBallPositionUpdate updates the GameActor's internal state for a ball.
func (a *GameActor) handleBallPositionUpdate(ctx bollywood.Context, ballState *Ball) {
	if ballState == nil {
		return
	}
	a.mu.Lock() // Lock for write access
	defer a.mu.Unlock()

	// Only update if the ball actor still exists in our map
	if _, ok := a.ballActors[ballState.Id]; ok {
		// Update the state, ensuring not to overwrite fields GameActor manages (like OwnerIndex after collision)
		// Best practice: Only update fields relevant to position/velocity from the message
		if existingBall, exists := a.balls[ballState.Id]; exists {
			existingBall.X = ballState.X
			existingBall.Y = ballState.Y
			existingBall.Vx = ballState.Vx
			existingBall.Vy = ballState.Vy
			existingBall.Phasing = ballState.Phasing // BallActor manages phasing
			existingBall.Mass = ballState.Mass
			existingBall.Radius = ballState.Radius
			// DO NOT update OwnerIndex here, GameActor manages that based on collisions
		} else {
			// Ball actor exists but ball state doesn't? Inconsistent state.
			fmt.Printf("WARN: BallActor %d exists but no corresponding state in GameActor map.\n", ballState.Id)
			// Add it back? Or ignore? Ignoring is safer.
		}
	} else {
		// fmt.Printf("GameActor: Received ball update for unknown/stopped ball ID %d\n", ballState.Id)
	}
}

// spawnBall creates a new Ball and its corresponding BallActor.
// IMPORTANT: This function should NOT be called while holding the GameActor's main lock (a.mu).
func (a *GameActor) spawnBall(ctx bollywood.Context, ownerIndex, x, y int, expireIn time.Duration) {
	actorPIDStr := "nil"
	if a.selfPID != nil {
		actorPIDStr = a.selfPID.String()
	}
	fmt.Printf("GameActor %s: spawnBall called for owner %d.\n", actorPIDStr, ownerIndex)

	// Ensure ownerIndex is valid before proceeding (Read lock needed for players check)
	a.mu.RLock()
	ownerValidAndConnected := ownerIndex >= 0 && ownerIndex < MaxPlayers && a.players[ownerIndex] != nil && a.players[ownerIndex].IsConnected
	ownerWs := a.players[ownerIndex].Ws // Capture Ws for check later
	a.mu.RUnlock()

	if !ownerValidAndConnected {
		fmt.Printf("GameActor %s: Cannot spawn ball for invalid or disconnected owner index %d\n", actorPIDStr, ownerIndex)
		return
	}

	ballID := time.Now().Nanosecond() + ownerIndex // Add index to reduce collision chance slightly
	ballData := NewBall(x, y, 0, a.canvas.CanvasSize, ownerIndex, ballID)

	selfPID := a.selfPID
	if selfPID == nil && ctx != nil {
		selfPID = ctx.Self()
	}
	if selfPID == nil {
		fmt.Printf("ERROR: GameActor %s cannot spawn ball, self PID is nil.\n", actorPIDStr)
		return // Don't modify state if spawn fails early
	}

	// --- Spawn Ball Actor (No Lock Held) ---
	ballProducer := NewBallActorProducer(*ballData, selfPID)
	fmt.Printf("GameActor %s: spawnBall - Spawning BallActor for ball %d...\n", actorPIDStr, ballID)
	ballPID := a.engine.Spawn(bollywood.NewProps(ballProducer))
	if ballPID == nil {
		fmt.Printf("ERROR: GameActor %s failed to spawn BallActor for player %d, ball %d\n", actorPIDStr, ownerIndex, ballID)
		return // Don't modify state if spawn fails
	}
	fmt.Printf("GameActor %s: spawnBall - Spawned BallActor %s for ball %d.\n", actorPIDStr, ballPID, ballID)

	// --- Add Ball to State Maps (Requires Lock) ---
	fmt.Printf("GameActor %s: spawnBall - Acquiring lock to add ball %d...\n", actorPIDStr, ballID)
	a.mu.Lock()
	fmt.Printf("GameActor %s: spawnBall - Lock acquired for ball %d.\n", actorPIDStr, ballID)
	// Check again if owner is still connected *and* it's the same connection
	// before adding ball state. This prevents adding a ball if the player
	// disconnected and reconnected quickly between the initial check and now.
	if a.players[ownerIndex] != nil && a.players[ownerIndex].IsConnected && a.players[ownerIndex].Ws == ownerWs {
		a.balls[ballID] = ballData
		a.ballActors[ballID] = ballPID
		fmt.Printf("GameActor %s: Added Ball %d (Actor %s) for player %d to state.\n", actorPIDStr, ballID, ballPID, ownerIndex)
	} else {
		// Owner disconnected or changed connection between start of function and acquiring lock
		fmt.Printf("GameActor %s: Owner %d disconnected/changed before ball %d could be added. Stopping spawned actor %s.\n", actorPIDStr, ownerIndex, ballID, ballPID)
		a.mu.Unlock() // Unlock before stopping
		fmt.Printf("GameActor %s: spawnBall - Lock released (owner disconnected/changed).\n", actorPIDStr)
		a.engine.Stop(ballPID) // Stop the newly spawned actor
		return                 // Don't schedule expiry timer
	}
	a.mu.Unlock()
	fmt.Printf("GameActor %s: spawnBall - Lock released after adding ball %d.\n", actorPIDStr, ballID)

	// Handle ball expiration timer (No Lock Held)
	if expireIn > 0 {
		fmt.Printf("GameActor %s: Scheduling expiry for ball %d in %v.\n", actorPIDStr, ballID, expireIn)
		// Send message back to self after duration
		time.AfterFunc(expireIn, func() {
			// Capture necessary PIDs safely
			currentSelfPID := a.selfPID
			currentEngine := a.engine
			if currentEngine != nil && currentSelfPID != nil {
				currentEngine.Send(currentSelfPID, DestroyExpiredBall{BallID: ballID}, nil)
			} else {
				fmt.Printf("ERROR: Cannot send DestroyExpiredBall for %d, engine/selfPID invalid in timer.\n", ballID)
			}
		})
	}
	fmt.Printf("GameActor %s: spawnBall finished for ball %d.\n", actorPIDStr, ballID)
}

// handleDestroyExpiredBall handles the message sent by the ball expiry timer.
func (a *GameActor) handleDestroyExpiredBall(ctx bollywood.Context, ballID int) {
	actorPIDStr := "nil"
	if a.selfPID != nil {
		actorPIDStr = a.selfPID.String()
	}
	fmt.Printf("GameActor %s: handleDestroyExpiredBall - Acquiring lock for ball %d...\n", actorPIDStr, ballID)
	a.mu.Lock() // Lock to modify maps
	fmt.Printf("GameActor %s: handleDestroyExpiredBall - Lock acquired for ball %d.\n", actorPIDStr, ballID)

	pidToStop, actorExists := a.ballActors[ballID]
	_, stateExists := a.balls[ballID]

	// Only proceed if both actor and state exist
	if actorExists && stateExists && pidToStop != nil {
		fmt.Printf("GameActor %s: Handling DestroyExpiredBall for BallID %d, stopping actor %s\n", actorPIDStr, ballID, pidToStop)
		delete(a.balls, ballID) // Remove from state maps first
		delete(a.ballActors, ballID)
		a.mu.Unlock() // Unlock before stopping actor
		fmt.Printf("GameActor %s: handleDestroyExpiredBall - Lock released before stopping actor %s.\n", actorPIDStr, pidToStop)

		a.engine.Stop(pidToStop) // Request stop
	} else {
		// Already removed by other means (e.g., player disconnect)
		fmt.Printf("GameActor %s: Received DestroyExpiredBall for already removed/unknown BallID %d\n", actorPIDStr, ballID)
		a.mu.Unlock()
		fmt.Printf("GameActor %s: handleDestroyExpiredBall - Lock released (ball %d already removed).\n", actorPIDStr, ballID)
	}
	// No broadcast needed here, removal will reflect in next GameTick broadcast
}
