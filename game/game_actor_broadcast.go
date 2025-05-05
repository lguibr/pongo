// File: game/game_actor_broadcast.go
package game

// GameState struct is NO LONGER USED for broadcasting.
// Individual update messages are used instead.

// deepCopyGrid creates a new Grid with copies of all Cells and BrickData.
// This is now used only for the initial grid send.
// func deepCopyGrid(original Grid) Grid { // Removed unused function
// 	if original == nil {
// 		return nil
// 	}
// 	newGrid := make(Grid, len(original))
// 	for i, row := range original {
// 		newRow := make([]Cell, len(row))
// 		for j, cell := range row {
// 			newCell := cell // Copy basic cell fields (X, Y)
// 			if cell.Data != nil {
// 				// Create a new BrickData struct and copy values
// 				newData := *cell.Data
// 				newCell.Data = &newData // Assign pointer to the new BrickData copy
// 			} else {
// 				newCell.Data = nil
// 			}
// 			newRow[j] = newCell
// 		}
// 		newGrid[i] = newRow
// 	}
// 	return newGrid
// }