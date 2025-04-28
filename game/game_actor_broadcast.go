// File: game/game_actor_broadcast.go
package game

import (
	"github.com/lguibr/pongo/utils"
)

// GameState struct for JSON marshalling (used in broadcast)
type GameState struct {
	Canvas  *Canvas                   `json:"canvas"`
	Players [utils.MaxPlayers]*Player `json:"players"`
	Paddles [utils.MaxPlayers]*Paddle `json:"paddles"`
	Balls   []*Ball                   `json:"balls"`
}

// deepCopyGrid creates a new Grid with copies of all Cells and BrickData.
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

// createGameStateSnapshot creates a snapshot of the current game state.
// Assumes it's called within the GameActor's sequential message processing loop.
// Creates deep copies of mutable slice/map elements and the Canvas grid.
func (a *GameActor) createGameStateSnapshot() GameState {
	// --- Prepare the GameState snapshot ---
	state := GameState{
		// Canvas:  a.canvas, // Shallow copy - replaced below
		Players: [utils.MaxPlayers]*Player{},
		Paddles: [utils.MaxPlayers]*Paddle{},
		Balls:   make([]*Ball, 0, len(a.balls)),
	}

	// Deep copy Canvas and Grid
	if a.canvas != nil {
		canvasCopy := *a.canvas                       // Copy basic canvas fields
		canvasCopy.Grid = deepCopyGrid(a.canvas.Grid) // Deep copy the grid
		state.Canvas = &canvasCopy
	} else {
		state.Canvas = nil
	}

	// Copy player info
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

	// Copy paddle info - Create a deep copy
	for i, p := range a.paddles {
		if p != nil && a.players[i] != nil && a.players[i].IsConnected {
			paddleCopy := *p // Create a copy of the paddle struct
			if paddleCopy.canvasSize == 0 && a.canvas != nil {
				paddleCopy.canvasSize = a.canvas.CanvasSize
			}
			state.Paddles[i] = &paddleCopy // Add pointer to the copy
		} else {
			state.Paddles[i] = nil
		}
	}

	// Copy ball info - Create a deep copy
	for _, b := range a.balls {
		if b != nil {
			ballCopy := *b // Create a copy of the ball struct
			if ballCopy.canvasSize == 0 && a.canvas != nil {
				ballCopy.canvasSize = a.canvas.CanvasSize
			}
			state.Balls = append(state.Balls, &ballCopy) // Add pointer to the copy
		}
	}

	return state
}
