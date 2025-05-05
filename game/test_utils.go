// File: game/test_utils.go
package game

import (
	"encoding/json"
	"testing"

	"github.com/lguibr/pongo/utils"
)

// --- Test Helpers (Moved from test package to avoid circular dependency) ---

// LocalGameState is a simplified representation used for E2E testing with atomic updates.
type LocalGameState struct {
	Players     [utils.MaxPlayers]*Player
	Paddles     [utils.MaxPlayers]*Paddle // Stores original game.Paddle
	Balls       map[int]*Ball             // Stores original game.Ball
	BrickStates []BrickStateUpdate        // Flat list of bricks with R3F coords (Exported)
	cellSize    int                       // Cell size for geometry scaling
	Scores      [utils.MaxPlayers]int32
	// Add other fields if needed by specific tests
}

// NewLocalGameState creates an initial empty state.
func NewLocalGameState() *LocalGameState {
	return &LocalGameState{
		Players:     [utils.MaxPlayers]*Player{},
		Paddles:     [utils.MaxPlayers]*Paddle{},
		Balls:       make(map[int]*Ball),
		BrickStates: make([]BrickStateUpdate, 0), // Initialize empty slice
		cellSize:    0,
		Scores:      [utils.MaxPlayers]int32{},
	}
}

// ApplyUpdatesToLocalState processes a batch of updates and modifies the local state.
func ApplyUpdatesToLocalState(localState *LocalGameState, updates []interface{}, t *testing.T) {
	t.Helper()
	if localState == nil {
		t.Error("ApplyUpdatesToLocalState called with nil localState")
		return
	}

	for _, updateInterface := range updates {
		// Marshal and Unmarshal to get the correct type easily
		// This is slightly inefficient but simplifies testing logic
		updateBytes, err := json.Marshal(updateInterface)
		if err != nil {
			t.Errorf("Failed to marshal update for type checking: %v", err)
			continue
		}

		var header MessageHeader
		if err := json.Unmarshal(updateBytes, &header); err != nil {
			t.Errorf("Failed to unmarshal update header: %v", err)
			continue
		}

		switch header.MessageType {
		case "playerJoined":
			var update PlayerJoined
			if err := json.Unmarshal(updateBytes, &update); err == nil {
				// Player and Paddle are now value types in the message
				if update.Player.Index >= 0 && update.Player.Index < utils.MaxPlayers {
					// Store pointers to copies in the local state
					playerCopy := update.Player
					localState.Players[update.Player.Index] = &playerCopy
					localState.Scores[update.Player.Index] = update.Player.Score
				}
				if update.Paddle.Index >= 0 && update.Paddle.Index < utils.MaxPlayers {
					// Store paddle (without R3F coords)
					paddleCopy := update.Paddle
					localState.Paddles[update.Paddle.Index] = &paddleCopy
				}
			} else {
				t.Errorf("Failed to unmarshal PlayerJoined update: %v", err)
			}
		case "playerLeft":
			var update PlayerLeft
			if err := json.Unmarshal(updateBytes, &update); err == nil {
				if update.Index >= 0 && update.Index < utils.MaxPlayers {
					localState.Players[update.Index] = nil
					localState.Paddles[update.Index] = nil
					localState.Scores[update.Index] = 0 // Reset score visually
				}
			} else {
				t.Errorf("Failed to unmarshal PlayerLeft update: %v", err)
			}
		case "ballSpawned":
			var update BallSpawned
			// Ball is now a value type in the message
			if err := json.Unmarshal(updateBytes, &update); err == nil {
				// Store a pointer to a copy in the local state map (without R3F coords)
				ballCopy := update.Ball
				localState.Balls[update.Ball.Id] = &ballCopy
			} else {
				t.Errorf("Failed to unmarshal BallSpawned update: %v", err)
			}
		case "ballRemoved":
			var update BallRemoved
			if err := json.Unmarshal(updateBytes, &update); err == nil {
				delete(localState.Balls, update.ID)
			} else {
				t.Errorf("Failed to unmarshal BallRemoved update: %v", err)
			}
		case "ballPositionUpdate":
			var update BallPositionUpdate
			if err := json.Unmarshal(updateBytes, &update); err == nil {
				if ball, ok := localState.Balls[update.ID]; ok && ball != nil {
					ball.X = update.X
					ball.Y = update.Y
					ball.Vx = update.Vx
					ball.Vy = update.Vy
					ball.Collided = update.Collided // Update collision flag
					ball.Phasing = update.Phasing   // Update phasing state
				}
			} else {
				t.Errorf("Failed to unmarshal BallPositionUpdate: %v", err)
			}
		case "paddlePositionUpdate":
			var update PaddlePositionUpdate
			if err := json.Unmarshal(updateBytes, &update); err == nil {
				// Corrected: Check index bounds and access array element directly
				if update.Index >= 0 && update.Index < utils.MaxPlayers {
					paddle := localState.Paddles[update.Index] // Direct access
					if paddle != nil {                         // Check if pointer is nil
						paddle.X = update.X
						paddle.Y = update.Y
						paddle.Width = update.Width   // Update dimensions
						paddle.Height = update.Height
						paddle.Vx = update.Vx
						paddle.Vy = update.Vy
						paddle.IsMoving = update.IsMoving
						paddle.Collided = update.Collided // Update collision flag
					}
				} else {
					t.Errorf("Received PaddlePositionUpdate with out-of-bounds index: %d", update.Index)
				}
			} else {
				t.Errorf("Failed to unmarshal PaddlePositionUpdate: %v", err)
			}
		case "fullGridUpdate": // Handle the new full grid update format
			var update FullGridUpdate
			if err := json.Unmarshal(updateBytes, &update); err == nil {
				localState.cellSize = update.CellSize // Store cell size
				// Directly replace the brick states list
				localState.BrickStates = update.Bricks // Use exported field
			} else {
				t.Errorf("Failed to unmarshal FullGridUpdate: %v", err)
			}
		case "scoreUpdate":
			var update ScoreUpdate
			if err := json.Unmarshal(updateBytes, &update); err == nil {
				if update.Index >= 0 && update.Index < utils.MaxPlayers {
					localState.Scores[update.Index] = update.Score
				}
			} else {
				t.Errorf("Failed to unmarshal ScoreUpdate: %v", err)
			}
		case "ballOwnerChanged":
			var update BallOwnershipChange
			if err := json.Unmarshal(updateBytes, &update); err == nil {
				if ball, ok := localState.Balls[update.ID]; ok && ball != nil {
					ball.OwnerIndex = update.NewOwnerIndex
				}
			} else {
				t.Errorf("Failed to unmarshal BallOwnershipChange: %v", err)
			}
		// Add cases for other update types if needed by tests
		default:
			// Ignore unknown message types in the batch for now
			// t.Logf("Ignoring unknown message type in batch: %s", header.MessageType)
		}
	}
}