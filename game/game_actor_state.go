// File: game/game_actor_state.go
package game

import (
	// "sync" // Removed unused import

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

// updateInternalState applies velocity/direction, updates positions in cache,
// and generates BallPositionUpdate/PaddlePositionUpdate messages with R3F coords.
func (a *GameActor) updateInternalState() {
	canvasSize := a.cfg.CanvasSize
	// Update paddles
	for i, paddle := range a.paddles {
		if paddle != nil {
			oldX, oldY, oldVx, oldVy, oldMoving := paddle.X, paddle.Y, paddle.Vx, paddle.Vy, paddle.IsMoving
			collidedState := paddle.Collided // Capture collision state BEFORE move/reset
			paddle.Collided = false          // Reset collision flag BEFORE move

			paddle.Move() // Updates internal X, Y, Vx, Vy, IsMoving

			// Check if state relevant for update has changed
			if paddle.X != oldX || paddle.Y != oldY || paddle.Vx != oldVx || paddle.Vy != oldVy || paddle.IsMoving != oldMoving || collidedState {
				// Calculate R3F coords for the paddle center
				r3fX, r3fY := mapToR3FCoords(paddle.X+paddle.Width/2, paddle.Y+paddle.Height/2, canvasSize)
				update := &PaddlePositionUpdate{
					MessageType: "paddlePositionUpdate", Index: i,
					X: paddle.X, Y: paddle.Y, // Original coords
					R3fX: r3fX, R3fY: r3fY, // R3F coords
					Width: paddle.Width, Height: paddle.Height, // Dimensions for frontend geometry
					Vx: paddle.Vx, Vy: paddle.Vy, IsMoving: paddle.IsMoving, Collided: collidedState,
				}
				a.addUpdate(update)
			}
		}
	}

	// Update balls
	for id, ball := range a.balls {
		if ball != nil {
			oldX, oldY := ball.X, ball.Y
			collidedState := ball.Collided // Capture collision state BEFORE move/reset
			ball.Collided = false          // Reset collision flag BEFORE move

			ball.Move() // Updates internal X, Y

			// Check if state relevant for update has changed
			if ball.X != oldX || ball.Y != oldY || collidedState {
				// Calculate R3F coords
				r3fX, r3fY := mapToR3FCoords(ball.X, ball.Y, canvasSize)
				update := &BallPositionUpdate{
					MessageType: "ballPositionUpdate", ID: id,
					X: ball.X, Y: ball.Y, // Original coords
					R3fX: r3fX, R3fY: r3fY, // R3F coords
					Vx: ball.Vx, Vy: ball.Vy, Collided: collidedState,
				}
				a.addUpdate(update)
			}
		}
	}
}

// Helper to map original world coords (0,0 top-left) to R3F centered coords (0,0 center, Y-up)
// This is a pure function.
func mapToR3FCoords(x, y, canvasSize int) (float64, float64) {
	halfSize := float64(canvasSize) / 2.0
	r3fX := float64(x) - halfSize
	r3fY := -(float64(y) - halfSize) // Invert Y
	return r3fX, r3fY
}