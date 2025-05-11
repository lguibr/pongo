// File: game/game_actor_state.go
package game

import (
	// "fmt" // Removed unused import

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
)

// --- State Update Methods ---

// addUpdate adds a message pointer to the pending updates buffer safely.
func (a *GameActor) addUpdate(updateMsg interface{}) {
	a.updatesMu.Lock()
	a.pendingUpdates = append(a.pendingUpdates, updateMsg)
	a.updatesMu.Unlock()
}

// handleBroadcastTick gathers pending updates and sends them as a batch.
func (a *GameActor) handleBroadcastTick(ctx bollywood.Context) {
	broadcasterKnown := a.broadcasterPID != nil

	if !broadcasterKnown { // Check if broadcaster PID is known before proceeding
		return
	}

	// --- Generate FullGridUpdate (NEW FORMAT - ALL CELLS with R3F Coords) ---
	var gridUpdate *FullGridUpdate
	if a.canvas != nil && a.canvas.Grid != nil && a.cfg.GridSize > 0 {
		gridSize := a.cfg.GridSize
		cellSize := a.cfg.CellSize
		canvasSize := a.cfg.CanvasSize
		brickUpdates := make([]BrickStateUpdate, 0, gridSize*gridSize) // Allocate for all cells
		for r := 0; r < gridSize; r++ {
			for c := 0; c < gridSize; c++ {
				cell := a.canvas.Grid[r][c] // Access cell directly
				life := 0
				cellType := utils.Cells.Empty
				if cell.Data != nil {
					life = cell.Data.Life
					cellType = cell.Data.Type
				}
				// Calculate R3F coordinates for the center of the cell
				origCenterX := c*cellSize + cellSize/2
				origCenterY := r*cellSize + cellSize/2
				r3fX, r3fY := mapToR3FCoords(origCenterX, origCenterY, canvasSize)

				brickUpdates = append(brickUpdates, BrickStateUpdate{
					X:    r3fX,
					Y:    r3fY,
					Life: life,
					Type: cellType,
				})
			}
		}
		// Always create the update message now, as it contains all cells
		gridUpdate = &FullGridUpdate{
			MessageType: "fullGridUpdate",
			CellSize:    cellSize, // Include for geometry scaling
			Bricks:      brickUpdates,
		}
	}
	// --- End Generate FullGridUpdate ---

	a.updatesMu.Lock()
	// Add the grid update to the pending list *before* copying
	if gridUpdate != nil {
		a.pendingUpdates = append(a.pendingUpdates, gridUpdate)
	}

	if len(a.pendingUpdates) == 0 {
		a.updatesMu.Unlock()
		return
	}

	// Create a copy of the slice to send, then clear the original slice
	updatesToSend := make([]interface{}, len(a.pendingUpdates))
	copy(updatesToSend, a.pendingUpdates)
	a.pendingUpdates = a.pendingUpdates[:0] // Clear while keeping capacity
	a.updatesMu.Unlock()

	// Send the batch command to the broadcaster
	a.engine.Send(a.broadcasterPID, BroadcastUpdatesCommand{Updates: updatesToSend}, a.selfPID)
}

// updateInternalState is now DEPRECATED. Its logic has been moved into:
// GameActor.moveEntities()
// GameActor.detectCollisions() (already existed, but its role in the sequence is clarified)
// GameActor.generatePositionUpdates()
// GameActor.resetPerTickCollisionFlags()
// func (a *GameActor) updateInternalState() {
// THIS FUNCTION IS NO LONGER CALLED.
// }

// Helper to map original world coords (0,0 top-left) to R3F centered coords (0,0 center, Y-up)
// This is a pure function.
func mapToR3FCoords(x, y, canvasSize int) (float64, float64) {
	halfSize := float64(canvasSize) / 2.0
	r3fX := float64(x) - halfSize
	r3fY := -(float64(y) - halfSize) // Invert Y
	return r3fX, r3fY
}
