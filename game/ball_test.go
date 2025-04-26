package game

import (
	"testing"

	"github.com/lguibr/pongo/utils"
)

func TestNewBall(t *testing.T) {
	canvasSize := utils.CanvasSize
	testCases := []struct {
		name                                 string
		x, y, radius, ownerIndex, id         int // Added ownerIndex, id params
		expectedX, expectedY, expectedRadius int
	}{
		{
			"TestCase1",
			10, 10, 0, 1, 1, // x, y, radius, ownerIndex, id
			10, 10, utils.BallSize,
		},
		{
			"TestCase2",
			10, 20, 30, 2, 2, // x, y, radius, ownerIndex, id
			10, 20, 30,
		},
	}
	// ballChannel := NewBallChannel() // Removed
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call NewBall with correct signature (no channel, ownerIndex added)
			ball := NewBall(tc.x, tc.y, tc.radius, canvasSize, tc.ownerIndex, tc.id)
			if ball.X != tc.expectedX {
				t.Errorf("Expected X to be %d, but got %d", tc.expectedX, ball.X)
			}
			if ball.Y != tc.expectedY {
				t.Errorf("Expected Y to be %d, but got %d", tc.expectedY, ball.Y)
			}
			if ball.Radius != tc.expectedRadius {
				t.Errorf("Expected Radius to be %d, but got %d", tc.expectedRadius, ball.Radius)
			}
			// Check owner index and ID if needed
			if ball.OwnerIndex != tc.ownerIndex {
				 t.Errorf("Expected OwnerIndex to be %d, but got %d", tc.ownerIndex, ball.OwnerIndex)
			}
			if ball.Id != tc.id {
				t.Errorf("Expected Id to be %d, but got %d", tc.id, ball.Id)
			}
		})
	}
}

func TestBall_BallInterceptPaddles(t *testing.T) {
	ball := &Ball{X: 100, Y: 100, Radius: 10}
	testCases := []struct {
		paddle     *Paddle
		intercepts bool
	}{
		{&Paddle{X: 90, Y: 90, Width: 20, Height: 20}, true},
		{&Paddle{X: 110, Y: 110, Width: 20, Height: 20}, false},
		{&Paddle{X: 90, Y: 110, Width: 20, Height: 20}, false},
		{&Paddle{X: 110, Y: 90, Width: 20, Height: 20}, false},
		{&Paddle{X: 80, Y: 80, Width: 20, Height: 20}, true},
		{&Paddle{X: 120, Y: 120, Width: 20, Height: 20}, false},
	}

	for index, tc := range testCases {
		result := ball.BallInterceptPaddles(tc.paddle)
		if result != tc.intercepts {
			t.Errorf("Expected BallInterceptPaddles to return %t but got %t in test case index %d", tc.intercepts, result, index)
		}
	}
}

func TestBall_InterceptsIndex(t *testing.T) {
	tests := []struct {
		name           string
		ball           *Ball
		x, y, cellSize int
		want           bool
	}{
		{
			name: "Intercepts top-left corner",
			ball: &Ball{X: 25, Y: 25, Radius: 10},
			x:    0, y: 0, cellSize: 50,
			want: true,
		},
		{
			name: "Does not intercept top-left corner",
			ball: &Ball{X: 25, Y: 25, Radius: 5},
			x:    0, y: 0,
			want:     false,
			cellSize: 10,
		},
		{
			name: "Intercepts center of cell",
			ball: &Ball{X: 75, Y: 75, Radius: 10},
			x:    1, y: 1, cellSize: 50,
			want: true,
		},
		{
			name: "Intercepts bottom-right corner",
			ball: &Ball{X: 75, Y: 75, Radius: 10},
			x:    1, y: 1, cellSize: 50,
			want: true,
		},
		{
			name: "Does not intercept bottom-right corner",
			ball: &Ball{X: 75, Y: 75, Radius: 5},
			x:    0, y: 0, cellSize: 50,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ball.InterceptsIndex(tt.x, tt.y, tt.cellSize); got != tt.want {
				t.Errorf("InterceptsIndex() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestBall_CollidesLeftWall removed
// TestBall_CollideRightWall removed
// TestBall_CollidesBottomWall removed
// TestBall_CollidesTopWall removed

func TestBall_HandleCollideBottom(t *testing.T) {
	testCases := []struct {
		name       string
		ballVy     int
		expectedVy int
	}{
		//INFO VY Should always be negative after bottom collision
		{"vy is positive", 5, -5},
		{"vy is negative", -5, -5},
		{"vy is zero", 0, 0},
	}
	for _, tc := range testCases {
		ball := &Ball{Vy: tc.ballVy}
		ball.HandleCollideBottom()
		if ball.Vy != tc.expectedVy {
			t.Errorf("Test case: %s. Expected Vy: %d, got: %d", tc.name, tc.expectedVy, ball.Vy)
		}
	}
}

func TestBall_HandleCollideTop(t *testing.T) {
	testCases := []struct {
		name       string
		ballVy     int
		expectedVy int
	}{
		//INFO VY Should always be positive after bottom collision
		{"vy is positive", 5, 5},
		{"vy is negative", -5, 5},
		{"vy is zero", 0, 0},
	}
	for _, tc := range testCases {
		ball := &Ball{Vy: tc.ballVy}
		ball.HandleCollideTop()
		if ball.Vy != tc.expectedVy {
			t.Errorf("Test case: %s. Expected Vy: %d, got: %d", tc.name, tc.expectedVy, ball.Vy)
		}
	}
}

func TestBall_HandleCollideLeft(t *testing.T) {
	testCases := []struct {
		name                string
		ballVx              int
		expectedCollisionVx int
	}{
		//INFO VX Should always be positive after bottom collision
		{name: "negative vx", ballVx: -10, expectedCollisionVx: 10},
		{name: "positive vx", ballVx: 10, expectedCollisionVx: 10},
		{name: "zero vx", ballVx: 0, expectedCollisionVx: 0},
	}

	for _, tc := range testCases {
		ball := &Ball{Vx: tc.ballVx}
		ball.HandleCollideLeft()
		if ball.Vx != tc.expectedCollisionVx {
			t.Errorf("Test case %s failed: expected Vx to be %d but got %d", tc.name, tc.expectedCollisionVx, ball.Vx)
		}
	}
}

func TestBall_HandleCollideRight(t *testing.T) {
	testCases := []struct {
		name                string
		ballVx              int
		expectedCollisionVx int
	}{
		//INFO VX Should always be negative after bottom collision
		{name: "negative vx", ballVx: -10, expectedCollisionVx: -10},
		{name: "positive vx", ballVx: 10, expectedCollisionVx: -10},
		{name: "zero vx", ballVx: 0, expectedCollisionVx: 0},
	}

	for _, tc := range testCases {
		ball := &Ball{Vx: tc.ballVx}
		ball.HandleCollideRight()
		if ball.Vx != tc.expectedCollisionVx {
			t.Errorf("Test case %s failed: expected Vx to be %d but got %d", tc.name, tc.expectedCollisionVx, ball.Vx)
		}
	}
}

func TestBall_GetCenterIndex(t *testing.T) {
	testCases := []struct {
		name        string
		ballX       int
		ballY       int
		expectedRow int
		expectedCol int
	}{
		{
			name:        "center of cell",
			// Use actual utils.CellSize for calculation
			ballX:       utils.CellSize / 2,
			ballY:       utils.CellSize / 2,
			expectedRow: 0,
			expectedCol: 0,
		},
		{
			name:        "bottom right of cell",
			ballX:       utils.CellSize - 1,
			ballY:       utils.CellSize - 1,
			expectedRow: 0,
			expectedCol: 0,
		},
		{
			name:        "top left of cell",
			ballX:       0,
			ballY:       0,
			expectedRow: 0,
			expectedCol: 0,
		},
		{
			name:        "specific cell",
			// Calculate X and Y to fall within the expected cell (3, 2) using the actual CellSize
			ballX:       utils.CellSize * 3 + utils.CellSize/2, // Center of cell (3, ...)
			ballY:       utils.CellSize * 2 + utils.CellSize/2, // Center of cell (..., 2)
			expectedRow: 3,
			expectedCol: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) { // Use t.Run for subtests
			ball := &Ball{X: tc.ballX, Y: tc.ballY}
			// Call the function using the actual utils.CellSize internally
		    row, col := ball.getCenterIndex()
		    if row != tc.expectedRow || col != tc.expectedCol {
			    t.Errorf("Test case %s failed: expected row %d and col %d but got row %d and col %d", tc.name, tc.expectedRow, tc.expectedCol, row, col)
		    }
		})
	}
}



