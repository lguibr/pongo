// File: game/paddle_test.go
package game

import (
	"testing"

	"github.com/lguibr/pongo/utils"
)

// TestPaddle_SetDirection removed as the method is deprecated and removed.

func TestPaddle_Move(t *testing.T) {
	// Initialize paddle with canvasSize
	initialX, initialY := 10, 20
	paddle := Paddle{X: initialX, Y: initialY, Width: 30, Height: 40, Velocity: 5, canvasSize: utils.CanvasSize}

	testCases := []struct {
		name       string
		index      int
		direction  string
		expectedX  int
		expectedY  int
		shouldMove bool
	}{
		// Test cases remain the same, but ensure initial paddle has canvasSize
		{"H_Left", 3, "left", 5, 20, true},
		{"H_Right", 3, "right", 10, 20, true}, // Start from X=5, move right by 5 -> 10
		{"H_None", 3, "", 10, 20, false},      // Start from X=10, no move -> 10
		{"H_Up", 3, "up", 10, 20, false},
		{"H_Down", 3, "down", 10, 20, false},
		{"H_Invalid", 3, "invalid", 10, 20, false},
		{"V_Right(Down)", 2, "right", 10, 25, true},  // Start from Y=20, move down by 5 -> 25
		{"V_Left(Up)", 2, "left", 10, 20, true},      // Start from Y=25, move up by 5 -> 20
		{"V_None", 2, "", 10, 20, false},             // Start from Y=20, no move -> 20
		{"V0_Left(Up)", 0, "left", 10, 15, true},     // Start from Y=20, move up by 5 -> 15
		{"V0_Right(Down)", 0, "right", 10, 20, true}, // Start from Y=15, move down by 5 -> 20
		{"H1_Left", 1, "left", 5, 20, true},          // Start from X=10, move left by 5 -> 5
		{"H1_Right", 1, "right", 10, 20, true},       // Start from X=5, move right by 5 -> 10
		{"V0_Invalid", 0, "invalid", 10, 20, false},
		{"H1_Invalid", 1, "invalid", 10, 20, false},
	}

	currentX, currentY := initialX, initialY
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set paddle state for this test case
			paddle.X = currentX
			paddle.Y = currentY
			paddle.Index = tc.index
			paddle.Direction = tc.direction

			// Perform the move
			paddle.Move()

			// Assert the outcome
			if tc.shouldMove {
				if paddle.X != tc.expectedX || paddle.Y != tc.expectedY {
					t.Errorf("Expected paddle (Index %d, Dir %s) starting at (%d,%d) to move to (%d, %d) but got (%d, %d)",
						tc.index, tc.direction, currentX, currentY, tc.expectedX, tc.expectedY, paddle.X, paddle.Y)
				}
				// Update current position for the next test if moved
				currentX = paddle.X
				currentY = paddle.Y
			} else {
				if paddle.X != currentX || paddle.Y != currentY {
					t.Errorf("Expected paddle (Index %d, Dir %s) starting at (%d,%d) to remain but got (%d, %d)",
						tc.index, tc.direction, currentX, currentY, paddle.X, paddle.Y)
				}
				// Position remains the same for the next test
			}
		})
	}

	// --- Boundary Tests ---
	t.Run("Boundaries", func(t *testing.T) {
		// test case when paddle is at the boundary
		paddle.Index = 0 // Vertical paddle on right
		paddle.X = utils.CanvasSize - paddle.Width
		paddle.Y = utils.CanvasSize - paddle.Height
		paddle.Direction = "right" // Try to move down
		paddle.Move()
		if paddle.X != utils.CanvasSize-paddle.Width || paddle.Y != utils.CanvasSize-paddle.Height {
			t.Errorf("Boundary Test (Vertical Down): Expected paddle to remain at (%d, %d) but got (%d, %d)", utils.CanvasSize-paddle.Width, utils.CanvasSize-paddle.Height, paddle.X, paddle.Y)
		}

		paddle.Y = 0
		paddle.Direction = "left" // Try to move up
		paddle.Move()
		if paddle.X != utils.CanvasSize-paddle.Width || paddle.Y != 0 {
			t.Errorf("Boundary Test (Vertical Up): Expected paddle to remain at (%d, %d) but got (%d, %d)", utils.CanvasSize-paddle.Width, 0, paddle.X, paddle.Y)
		}

		paddle.Index = 3 // Horizontal paddle on bottom
		paddle.X = utils.CanvasSize - paddle.Width
		paddle.Y = utils.CanvasSize - paddle.Height
		paddle.Direction = "right" // Try to move right
		paddle.Move()
		if paddle.X != utils.CanvasSize-paddle.Width || paddle.Y != utils.CanvasSize-paddle.Height {
			t.Errorf("Boundary Test (Horizontal Right): Expected paddle to remain at (%d, %d) but got (%d, %d)", utils.CanvasSize-paddle.Width, utils.CanvasSize-paddle.Height, paddle.X, paddle.Y)
		}

		paddle.X = 0
		paddle.Direction = "left" // Try to move left
		paddle.Move()
		if paddle.X != 0 || paddle.Y != utils.CanvasSize-paddle.Height {
			t.Errorf("Boundary Test (Horizontal Left): Expected paddle to remain at (%d, %d) but got (%d, %d)", 0, utils.CanvasSize-paddle.Height, paddle.X, paddle.Y)
		}
	})
}
