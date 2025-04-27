// File: game/game_actor_broadcast.go
package game

import (
	"encoding/json"
	"fmt"
	"strings" // Import strings for error checking

	"github.com/lguibr/pongo/utils" // Import utils
	"golang.org/x/net/websocket"
)

// GameState struct for JSON marshalling (used in broadcast/updateJSON)
// Ensure this matches frontend/src/types/game.ts
type GameState struct {
	Canvas  *Canvas                   `json:"canvas"`  // Pointer is fine, frontend expects potentially null
	Players [utils.MaxPlayers]*Player `json:"players"` // Use constant from utils
	Paddles [utils.MaxPlayers]*Paddle `json:"paddles"` // Use constant from utils
	Balls   []*Ball                   `json:"balls"`   // Slice of non-nil balls
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
		Players: [utils.MaxPlayers]*Player{}, // Use constant from utils
		Paddles: [utils.MaxPlayers]*Paddle{}, // Use constant from utils
		Balls:   make([]*Ball, 0, len(a.balls)),
	}

	// Copy player info
	for i, pi := range a.players {
		if pi != nil && pi.IsConnected {
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

	// Copy paddle info
	for i, p := range a.paddles {
		if p != nil && a.players[i] != nil && a.players[i].IsConnected {
			paddleCopy := *p
			if paddleCopy.canvasSize == 0 && a.canvas != nil {
				paddleCopy.canvasSize = a.canvas.CanvasSize
			}
			state.Paddles[i] = &paddleCopy
		} else {
			state.Paddles[i] = nil
		}
	}

	// Filter out nil balls and create copies
	for _, b := range a.balls {
		if b != nil {
			ballCopy := *b
			if ballCopy.canvasSize == 0 && a.canvas != nil {
				ballCopy.canvasSize = a.canvas.CanvasSize
			}
			state.Balls = append(state.Balls, &ballCopy)
		}
	}

	// --- Collect active connections ---
	type writeTarget struct {
		ws    *websocket.Conn
		index int
		addr  string
	}
	targets := []writeTarget{}
	for conn, index := range a.connToIndex {
		// Use constant from utils
		if index >= 0 && index < utils.MaxPlayers && a.players[index] != nil && a.players[index].IsConnected && a.players[index].Ws == conn {
			targets = append(targets, writeTarget{ws: conn, index: index, addr: conn.RemoteAddr().String()})
		} else {
			fmt.Printf("WARN: Stale entry in connToIndex during broadcast? Conn: %s, Index: %d\n", conn.RemoteAddr(), index)
		}
	}
	a.mu.RUnlock() // Unlock before potentially blocking operations

	if len(targets) == 0 {
		return
	}

	// --- Write to connections using websocket.JSON.Send ---
	for _, target := range targets {
		err := websocket.JSON.Send(target.ws, state) // Send the state struct directly

		if err != nil {
			fmt.Printf("GameActor %s: websocket.JSON.Send error for player %d (%s): %v\n", actorPIDStr, target.index, target.addr, err)

			isClosedErr := strings.Contains(err.Error(), "use of closed network connection") ||
				strings.Contains(err.Error(), "broken pipe") ||
				strings.Contains(err.Error(), "connection reset by peer") ||
				strings.Contains(err.Error(), "connection timed out")

			logPrefix := fmt.Sprintf("GameActor %s: ", actorPIDStr)
			errMsg := fmt.Sprintf("Error writing state to player %d (%s): %v.", target.index, target.addr, err)

			if isClosedErr {
				errMsg = fmt.Sprintf("Write failed to player %d (%s) because connection is closed/timed out.", target.index, target.addr)
			}

			fmt.Println(logPrefix + errMsg + " Triggering disconnect.")

			if a.selfPID != nil && a.engine != nil {
				a.engine.Send(a.selfPID, PlayerDisconnect{PlayerIndex: target.index, WsConn: target.ws}, nil)
			} else {
				fmt.Printf("ERROR: Cannot send disconnect message for player %d, selfPID/engine is nil\n", target.index)
				a.mu.Lock()
				if pInfo := a.players[target.index]; pInfo != nil && pInfo.Ws == target.ws {
					pInfo.IsConnected = false
					delete(a.connToIndex, target.ws)
				}
				a.mu.Unlock()
			}
		}
	}
}

// updateGameStateJSON updates the atomically stored JSON representation of the game state.
func (a *GameActor) updateGameStateJSON() {
	a.mu.RLock() // Read lock needed
	// Logic is similar to broadcastGameState, create a snapshot
	state := GameState{
		Canvas:  a.canvas,
		Players: [utils.MaxPlayers]*Player{}, // Use constant from utils
		Paddles: [utils.MaxPlayers]*Paddle{}, // Use constant from utils
		Balls:   make([]*Ball, 0, len(a.balls)),
	}
	for i, pi := range a.players {
		if pi != nil && pi.IsConnected {
			state.Players[i] = &Player{Index: pi.Index, Id: pi.ID, Color: pi.Color, Score: pi.Score}
		} else {
			state.Players[i] = nil
		}
	}
	for i, p := range a.paddles {
		if p != nil && a.players[i] != nil && a.players[i].IsConnected {
			paddleCopy := *p
			if paddleCopy.canvasSize == 0 && a.canvas != nil {
				paddleCopy.canvasSize = a.canvas.CanvasSize
			}
			state.Paddles[i] = &paddleCopy
		} else {
			state.Paddles[i] = nil
		}
	}
	for _, b := range a.balls {
		if b != nil {
			ballCopy := *b
			if ballCopy.canvasSize == 0 && a.canvas != nil {
				ballCopy.canvasSize = a.canvas.CanvasSize
			}
			state.Balls = append(state.Balls, &ballCopy)
		}
	}
	a.mu.RUnlock() // Unlock after reading

	stateJSON, err := json.Marshal(state)
	if err != nil {
		fmt.Println("GameActor: Error marshalling game state for HTTP:", err)
		a.gameStateJSON.Store([]byte(`{"error": "failed to marshal state"}`))
		return
	}
	a.gameStateJSON.Store(stateJSON)
}

// GetGameStateJSON retrieves the latest marshalled game state for HTTP handlers.
// (Keep the implementation from game_actor.go)
