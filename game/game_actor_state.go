package game

import (
	bollywood "github.com/lguibr/pongo/internal/actor"
	"github.com/lguibr/pongo/utils"
	"time"
)

func (a *GameActor) addUpdate(msg interface{}) {
	a.updatesMu.Lock()
	a.pendingUpdates = append(a.pendingUpdates, msg)
	a.updatesMu.Unlock()
}

// fullGridUpdate preserves the client's complete-grid replacement contract.
func (a *GameActor) fullGridUpdate() *FullGridUpdate {
	if a.canvas == nil || a.canvas.Grid == nil {
		return nil
	}
	cells := make([]BrickStateUpdate, 0, a.cfg.GridSize*a.cfg.GridSize)
	for r, row := range a.canvas.Grid {
		for c, cell := range row {
			life, kind := 0, utils.Cells.Empty
			if cell.Data != nil {
				life, kind = cell.Data.Life, cell.Data.Type
			}
			x, y := mapToR3FCoords(c*a.cfg.CellSize+a.cfg.CellSize/2, r*a.cfg.CellSize+a.cfg.CellSize/2, a.cfg.CanvasSize)
			cells = append(cells, BrickStateUpdate{X: x, Y: y, Life: life, Type: kind})
		}
	}
	return &FullGridUpdate{MessageType: "fullGridUpdate", CellSize: a.cfg.CellSize, Bricks: cells}
}
func (a *GameActor) handleBroadcastTick(ctx bollywood.Context) {
	if a.broadcasterPID == nil {
		return
	}
	if a.gridDirty {
		if grid := a.fullGridUpdate(); grid != nil {
			a.addUpdate(grid)
		}
		a.gridDirty = false
	}
	a.updatesMu.Lock()
	// Hand over ownership of the immutable batch without copying the slice.
	updates := a.pendingUpdates
	if len(updates) > 0 {
		a.pendingUpdates = make([]interface{}, 0, len(updates))
	}
	a.updatesMu.Unlock()
	if len(updates) == 0 && time.Since(a.lastHeartbeat) < 15*time.Second {
		return
	}
	a.lastHeartbeat = time.Now()
	a.engine.Send(a.broadcasterPID, BroadcastUpdatesCommand{Updates: updates}, a.selfPID)
}
func mapToR3FCoords(x, y, canvasSize int) (float64, float64) {
	half := float64(canvasSize) / 2
	return float64(x) - half, half - float64(y)
}
