// File: game/paddle_test.go
package game

import (
	"testing"

	"github.com/lguibr/pongo/utils"
)

func TestPaddle_Move(t *testing.T) {
	cfg := utils.DefaultConfig()
	initialX, initialY := 100, 200 // Use different initial values for clarity

	testCases := []struct {
		name       string
		index      int
		direction  string
		expectedX  int
		expectedY  int
		shouldMove bool
		expectedVx int
		expectedVy int
	}{
		// Recalculate expected values based on initialX/Y and cfg.PaddleVelocity
		// cfg.PaddleVelocity = 16 (assuming default config: 1024/16/4)

		// H paddle index 3 (Bottom)
		{"H_Left", 3, "left", initialX - cfg.PaddleVelocity, initialY, true, -cfg.PaddleVelocity, 0},  // 100-16=84
		{"H_Right", 3, "right", initialX + cfg.PaddleVelocity, initialY, true, cfg.PaddleVelocity, 0}, // 100+16=116
		{"H_None", 3, "", initialX, initialY, false, 0, 0},
		{"H_Up", 3, "up", initialX, initialY, false, 0, 0},
		{"H_Down", 3, "down", initialX, initialY, false, 0, 0},
		{"H_Invalid", 3, "invalid", initialX, initialY, false, 0, 0},

		// V paddle index 2 (Left)
		{"V_Right(Down)", 2, "right", initialX, initialY + cfg.PaddleVelocity, true, 0, cfg.PaddleVelocity}, // 200+16=216
		{"V_Left(Up)", 2, "left", initialX, initialY - cfg.PaddleVelocity, true, 0, -cfg.PaddleVelocity},    // 200-16=184
		{"V_None", 2, "", initialX, initialY, false, 0, 0},

		// V paddle index 0 (Right)
		{"V0_Left(Up)", 0, "left", initialX, initialY - cfg.PaddleVelocity, true, 0, -cfg.PaddleVelocity},    // 200-16=184
		{"V0_Right(Down)", 0, "right", initialX, initialY + cfg.PaddleVelocity, true, 0, cfg.PaddleVelocity}, // 200+16=216

		// H paddle index 1 (Top)
		{"H1_Left", 1, "left", initialX - cfg.PaddleVelocity, initialY, true, -cfg.PaddleVelocity, 0},  // 100-16=84
		{"H1_Right", 1, "right", initialX + cfg.PaddleVelocity, initialY, true, cfg.PaddleVelocity, 0}, // 100+16=116

		{"V0_Invalid", 0, "invalid", initialX, initialY, false, 0, 0},
		{"H1_Invalid", 1, "invalid", initialX, initialY, false, 0, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new paddle instance for each test case to ensure isolation
			paddle := Paddle{
				X:          initialX,
				Y:          initialY,
				Index:      tc.index,
				Direction:  tc.direction,
				Velocity:   cfg.PaddleVelocity,
				canvasSize: cfg.CanvasSize,
			}
			// Set dimensions based on index
			if tc.index == 0 || tc.index == 2 { // Vertical
				paddle.Width = cfg.PaddleWidth
				paddle.Height = cfg.PaddleLength
			} else { // Horizontal
				paddle.Width = cfg.PaddleLength
				paddle.Height = cfg.PaddleWidth
			}

			// Perform the move
			paddle.Move()

			// Assert the outcome
			if paddle.X != tc.expectedX || paddle.Y != tc.expectedY {
				t.Errorf("Position Fail: Expected paddle (Index %d, Dir %s) starting at (%d,%d) to move to (%d, %d) but got (%d, %d)",
					tc.index, tc.direction, initialX, initialY, tc.expectedX, tc.expectedY, paddle.X, paddle.Y)
			}
			if paddle.IsMoving != tc.shouldMove {
				t.Errorf("IsMoving Fail: Expected IsMoving=%t but got %t", tc.shouldMove, paddle.IsMoving)
			}
			if paddle.Vx != tc.expectedVx || paddle.Vy != tc.expectedVy {
				t.Errorf("Velocity Fail: Expected Vx=%d, Vy=%d but got Vx=%d, Vy=%d", tc.expectedVx, tc.expectedVy, paddle.Vx, paddle.Vy)
			}
		})
	}

	// --- Boundary Tests ---
	t.Run("Boundaries", func(t *testing.T) {
		// Create paddle specifically for boundary tests
		paddle := Paddle{Velocity: cfg.PaddleVelocity, canvasSize: cfg.CanvasSize}

		// Test case: Vertical paddle at bottom edge, try moving down
		paddle.Index = 0 // Vertical paddle on right
		paddle.Width = cfg.PaddleWidth
		paddle.Height = cfg.PaddleLength
		paddle.X = cfg.CanvasSize - paddle.Width
		paddle.Y = cfg.CanvasSize - paddle.Height
		paddle.Direction = "right" // Try to move down
		paddle.Move()
		if paddle.Y != cfg.CanvasSize-paddle.Height {
			t.Errorf("Boundary Test (Vertical Down): Expected Y=%d but got %d", cfg.CanvasSize-paddle.Height, paddle.Y)
		}

		// Test case: Vertical paddle at top edge, try moving up
		paddle.Y = 0
		paddle.Direction = "left" // Try to move up
		paddle.Move()
		if paddle.Y != 0 {
			t.Errorf("Boundary Test (Vertical Up): Expected Y=%d but got %d", 0, paddle.Y)
		}

		// Test case: Horizontal paddle at right edge, try moving right
		paddle.Index = 3 // Horizontal paddle on bottom
		paddle.Width = cfg.PaddleLength
		paddle.Height = cfg.PaddleWidth
		paddle.X = cfg.CanvasSize - paddle.Width
		paddle.Y = cfg.CanvasSize - paddle.Height
		paddle.Direction = "right" // Try to move right
		paddle.Move()
		if paddle.X != cfg.CanvasSize-paddle.Width {
			t.Errorf("Boundary Test (Horizontal Right): Expected X=%d but got %d", cfg.CanvasSize-paddle.Width, paddle.X)
		}

		// Test case: Horizontal paddle at left edge, try moving left
		paddle.X = 0
		paddle.Direction = "left" // Try to move left
		paddle.Move()
		if paddle.X != 0 {
			t.Errorf("Boundary Test (Horizontal Left): Expected X=%d but got %d", 0, paddle.X)
		}
	})
}
