// File: game/game_actor_broadcast.go
package game

import (
	"encoding/json"
	"fmt"
)

// GameState struct for JSON marshalling (used in broadcast/updateJSON)
type GameState struct {
	Canvas  *Canvas             `json:"canvas"`
	Players [MaxPlayers]*Player `json:"players"` // Use exported constant
	Paddles [MaxPlayers]*Paddle `json:"paddles"` // Use exported constant
	Balls   []*Ball             `json:"balls"`   // Filtered list of non-nil balls
}

// broadcastGameState sends the current game state to all connected clients.
func (a *GameActor) broadcastGameState() {
	a.mu.RLock() // Read lock needed to access players, paddles, balls, canvas, connToIndex

	actorPIDStr := "nil"
	if a.selfPID != nil {
		actorPIDStr = a.selfPID.String()
	}

	// --- Prepare the GameState snapshot ---
	state := GameState{
		Canvas:  a.canvas,
		Players: [MaxPlayers]*Player{}, // Use exported constant
		Paddles: [MaxPlayers]*Paddle{}, // Use exported constant
		Balls:   make([]*Ball, 0, len(a.balls)),
	}
	// Copy player info
	for i, pi := range a.players {
		if pi != nil && pi.IsConnected { // Only include connected players
			state.Players[i] = &Player{
				Index: pi.Index,
				Id:    pi.ID,
				Color: pi.Color,
				Score: pi.Score,
			}
		} else {
			state.Players[i] = nil
		}
	}
	// Copy paddle info (create copies)
	for i, p := range a.paddles {
		if p != nil && a.players[i] != nil && a.players[i].IsConnected { // Check player connected
			paddleCopy := *p // Create a copy
			state.Paddles[i] = &paddleCopy
		} else {
			state.Paddles[i] = nil
		}
	}
	// Filter out nil balls and create copies
	validBalls := make([]*Ball, 0, len(a.balls))
	for _, b := range a.balls {
		if b != nil {
			ballCopy := *b // Create a copy
			validBalls = append(validBalls, &ballCopy)
		}
	}
	state.Balls = validBalls

	// --- Collect active connections ---
	// Iterate through connToIndex which maps active connections to player indices
	type writeTarget struct {
		ws    PlayerConnection
		index int
		addr  string
	}
	targets := []writeTarget{}
	for conn, index := range a.connToIndex {
		// Double-check the player slot still exists and is marked connected
		if index >= 0 && index < MaxPlayers && a.players[index] != nil && a.players[index].IsConnected && a.players[index].Ws == conn { // Use exported constant
			targets = append(targets, writeTarget{ws: conn, index: index, addr: conn.RemoteAddr().String()})
		} else {
			// This indicates an inconsistency, connToIndex might be stale.
			// Should be cleaned up by disconnect logic, but log if seen.
			fmt.Printf("WARN: Stale entry in connToIndex during broadcast? Conn: %s, Index: %d\n", conn.RemoteAddr(), index)
		}
	}
	a.mu.RUnlock() // Unlock before marshalling and writing

	if len(targets) == 0 {
		// fmt.Printf("GameActor %s: No active connections to broadcast to.\n", actorPIDStr) // Reduce noise
		return
	}

	// --- Marshal state once ---
	stateJSON, err := json.Marshal(state)
	if err != nil {
		fmt.Printf("GameActor %s: Error marshalling game state: %v\n", actorPIDStr, err)
		return
	}

	// --- Write to connections ---
	// fmt.Printf("GameActor %s: Broadcasting state to %d connections.\n", actorPIDStr, len(targets)) // Reduce noise
	for _, target := range targets {
		// fmt.Printf("GameActor %s: Attempting write to player %d (%s)\n", actorPIDStr, target.index, target.addr) // Debug log
		_, err := target.ws.Write(stateJSON)
		if err != nil {
			fmt.Printf("GameActor %s: Error writing state to player %d (%s): %v. Triggering disconnect.\n", actorPIDStr, target.index, target.addr, err)

			// Send disconnect message to self for proper cleanup within actor context
			// Pass the connection object that failed.
			if a.selfPID != nil && a.engine != nil {
				// Use the connection object from the target struct
				a.engine.Send(a.selfPID, PlayerDisconnect{PlayerIndex: target.index, WsConn: target.ws}, nil)
			} else {
				fmt.Printf("ERROR: Cannot send disconnect message for player %d, selfPID/engine is nil\n", target.index)
				// Attempt manual cleanup as fallback (less ideal)
				a.mu.Lock()
				if pInfo := a.players[target.index]; pInfo != nil && pInfo.Ws == target.ws {
					pInfo.IsConnected = false
					delete(a.connToIndex, target.ws)
					// Don't close connection here, let potential readLoop handle it or rely on OS cleanup
				}
				a.mu.Unlock()
			}
		} else {
			// fmt.Printf("GameActor %s: Write successful to player %d (%s)\n", actorPIDStr, target.index, target.addr) // Debug log
		}
	}
}

// updateGameStateJSON updates the atomically stored JSON representation of the game state.
func (a *GameActor) updateGameStateJSON() {
	a.mu.RLock() // Read lock needed
	// This logic is identical to the start of broadcastGameState, could be refactored
	state := GameState{
		Canvas:  a.canvas,
		Players: [MaxPlayers]*Player{}, // Use exported constant
		Paddles: [MaxPlayers]*Paddle{}, // Use exported constant
		Balls:   make([]*Ball, 0, len(a.balls)),
	}
	for i, pi := range a.players {
		if pi != nil && pi.IsConnected { // Only include connected players
			state.Players[i] = &Player{Index: pi.Index, Id: pi.ID, Color: pi.Color, Score: pi.Score}
		} else {
			state.Players[i] = nil
		}
	}
	for i, p := range a.paddles {
		if p != nil && a.players[i] != nil && a.players[i].IsConnected { // Check player connected
			paddleCopy := *p
			state.Paddles[i] = &paddleCopy
		} else {
			state.Paddles[i] = nil
		}
	}
	validBalls := make([]*Ball, 0, len(a.balls))
	for _, b := range a.balls {
		if b != nil {
			ballCopy := *b
			validBalls = append(validBalls, &ballCopy)
		}
	}
	state.Balls = validBalls
	a.mu.RUnlock() // Unlock after reading

	stateJSON, err := json.Marshal(state)
	if err != nil {
		fmt.Println("GameActor: Error marshalling game state for HTTP:", err)
		a.gameStateJSON.Store([]byte(`{"error": "failed to marshal state"}`)) // Store error state
		return
	}
	a.gameStateJSON.Store(stateJSON)
}

// GetGameStateJSON retrieves the latest marshalled game state for HTTP handlers.
// (Keep the implementation from game_actor.go)
