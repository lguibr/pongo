package game

import (
	"github.com/lguibr/pongo/utils"
)

// GameState struct for JSON marshalling (used in broadcast)
// NO LONGER CONTAINS Canvas/Grid. Sent separately via InitialGridStateMessage.
type GameState struct {
	Players     [utils.MaxPlayers]*Player `json:"players"`
	Paddles     [utils.MaxPlayers]*Paddle `json:"paddles"`
	Balls       []*Ball                   `json:"balls"`
	MessageType string                    `json:"messageType"` // To help client distinguish messages
}

// deepCopyGrid creates a new Grid with copies of all Cells and BrickData.
// This is now used only for the initial grid send.
func deepCopyGrid(original Grid) Grid {
	if original == nil {
		return nil
	}
	newGrid := make(Grid, len(original))
	for i, row := range original {
		newRow := make([]Cell, len(row))
		for j, cell := range row {
			newCell := cell // Copy basic cell fields (X, Y)
			if cell.Data != nil {
				// Create a new BrickData struct and copy values
				newData := *cell.Data
				newCell.Data = &newData // Assign pointer to the new BrickData copy
			} else {
				newCell.Data = nil
			}
			newRow[j] = newCell
		}
		newGrid[i] = newRow
	}
	return newGrid
}

// createGameStateSnapshot creates a snapshot of the current DYNAMIC game state.
// Assumes it's called within the GameActor's sequential message processing loop.
// Creates deep copies of mutable slice/map elements (Players, Paddles, Balls).
// It also copies the Collided flag and then resets it in the GameActor's cache.
// The static grid is sent separately upon player connection.
func (a *GameActor) createGameStateSnapshot() GameState {
	// --- Prepare the GameState snapshot ---
	state := GameState{
		Players:     [utils.MaxPlayers]*Player{},
		Paddles:     [utils.MaxPlayers]*Paddle{},
		Balls:       make([]*Ball, 0, len(a.balls)),
		MessageType: "gameStateUpdate", // Add message type identifier
	}

	// --- Grid is NOT included in the regular snapshot ---

	// Copy player info (using local playerInfo cache)
	for i, pi := range a.players {
		if pi != nil && pi.IsConnected {
			// Create a copy of the player data, reading score atomically
			state.Players[i] = &Player{
				Index: pi.Index,
				Id:    pi.ID,
				Color: pi.Color,
				Score: pi.Score.Load(), // Use atomic LoadInt32
			}
		} else {
			state.Players[i] = nil
		}
	}

	// Copy paddle info - Create a deep copy (using local paddle cache)
	for i, p := range a.paddles {
		// Check if player exists and is connected for this paddle index
		if p != nil && a.players[i] != nil && a.players[i].IsConnected {
			paddleCopy := *p // Create a copy of the paddle struct
			if paddleCopy.canvasSize == 0 && a.canvas != nil {
				paddleCopy.canvasSize = a.canvas.CanvasSize
			}
			state.Paddles[i] = &paddleCopy // Add pointer to the copy

			// Reset the Collided flag in the GameActor's cache *after* copying
			p.Collided = false
		} else {
			state.Paddles[i] = nil
		}
	}

	// Copy ball info - Create a deep copy (using local ball cache)
	// Use _ for the key (ball ID) as it's not needed in the loop body itself
	for _, b := range a.balls {
		if b != nil {
			ballCopy := *b // Create a copy of the ball struct
			if ballCopy.canvasSize == 0 && a.canvas != nil {
				ballCopy.canvasSize = a.canvas.CanvasSize
			}
			state.Balls = append(state.Balls, &ballCopy) // Add pointer to the copy

			// Reset the Collided flag in the GameActor's cache *after* copying
			b.Collided = false
		}
	}

	return state
}
