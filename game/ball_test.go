package game

import (
	"fmt" // Import fmt for debugging
	"testing"

	"github.com/lguibr/pongo/utils"
)

func TestNewBall(t *testing.T) {
	canvasSize := utils.CanvasSize
	testCases := []struct {
		name                                 string
		x, y, radius, ownerIndex, id         int
		expectedX, expectedY, expectedRadius int
	}{
		{
			"TestCase1",
			10, 10, 0, 1, 1,
			10, 10, utils.BallSize,
		},
		{
			"TestCase2",
			10, 20, 30, 2, 2,
			10, 20, 30,
		},
		{
			"TestCaseZeroPosPlayer0", // Test initial position calculation
			0, 0, 0, 0, 3,
			canvasSize - utils.PaddleWeight*2 - utils.BallSize, canvasSize / 2, utils.BallSize,
		},
		{
			"TestCaseZeroPosPlayer1", // Test initial position calculation
			0, 0, 0, 1, 4,
			canvasSize / 2, utils.PaddleWeight*2 + utils.BallSize, utils.BallSize,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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
			if ball.OwnerIndex != tc.ownerIndex {
				t.Errorf("Expected OwnerIndex to be %d, but got %d", tc.ownerIndex, ball.OwnerIndex)
			}
			if ball.Id != tc.id {
				t.Errorf("Expected Id to be %d, but got %d", tc.id, ball.Id)
			}
			if ball.Vx == 0 && ball.Vy == 0 {
				t.Errorf("Expected non-zero velocity components, but got Vx=%d, Vy=%d", ball.Vx, ball.Vy)
			}
		})
	}
}

func TestBall_BallInterceptPaddles(t *testing.T) {
	ball := &Ball{X: 100, Y: 100, Radius: 10} // Center (100,100), R=10
	testCases := []struct {
		name       string
		paddle     *Paddle
		intercepts bool
	}{
		{"Overlap Center", &Paddle{X: 95, Y: 95, Width: 10, Height: 10}, true},
		{"Overlap TopLeft Corner", &Paddle{X: 90, Y: 90, Width: 15, Height: 15}, true},
		{"No Overlap Corner", &Paddle{X: 110, Y: 110, Width: 20, Height: 20}, false},
		{"Overlap Left Edge", &Paddle{X: 88, Y: 95, Width: 10, Height: 10}, true},
		{"No Overlap Far", &Paddle{X: 120, Y: 120, Width: 20, Height: 20}, false},
		{"Overlap Top Edge", &Paddle{X: 95, Y: 88, Width: 10, Height: 10}, true},
		{"Touching Top Edge", &Paddle{X: 95, Y: 80, Width: 10, Height: 10}, false},
		// Adjust "Just Outside" cases to be clearly outside
		{"Clearly Outside Corner 1", &Paddle{X: 85, Y: 85, Width: 10, Height: 10}, false},   // Closest (90,90), dist^2=200 > 100
		{"Clearly Outside Corner 2", &Paddle{X: 111, Y: 111, Width: 10, Height: 10}, false}, // Closest (111,111), dist^2=11^2+11^2=242 > 100
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ball.BallInterceptPaddles(tc.paddle)
			if result != tc.intercepts {
				t.Errorf("Expected BallInterceptPaddles to return %t but got %t", tc.intercepts, result)
			}
		})
	}
}

func TestBall_InterceptsIndex(t *testing.T) {
	tests := []struct {
		name               string
		ball               *Ball
		col, row, cellSize int // Use col, row
		want               bool
	}{
		{
			name: "Intercepts top-left corner",
			ball: &Ball{X: 25, Y: 25, Radius: 10},
			col:  0, row: 0, cellSize: 50,
			want: true,
		},
		{
			name: "Does not intercept top-left corner",
			ball: &Ball{X: 25, Y: 25, Radius: 5},
			col:  0, row: 0, cellSize: 10,
			want: false,
		},
		{
			name: "Intercepts center of cell",
			ball: &Ball{X: 75, Y: 75, Radius: 10},
			col:  1, row: 1, cellSize: 50,
			want: true,
		},
		{
			name: "Intercepts bottom-right corner",
			ball: &Ball{X: 45, Y: 45, Radius: 10},
			col:  0, row: 0, cellSize: 50,
			want: true,
		},
		{
			name: "Does not intercept bottom-right corner",
			ball: &Ball{X: 55, Y: 55, Radius: 5},
			col:  0, row: 0, cellSize: 50,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Pass col, row to the function
			if got := tt.ball.InterceptsIndex(tt.col, tt.row, tt.cellSize); got != tt.want {
				t.Errorf("InterceptsIndex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBall_GetCenterIndex(t *testing.T) {
	canvasSize := utils.CanvasSize    // 576
	gridSize := utils.GridSize        // 12
	cellSize := canvasSize / gridSize // 48

	testCases := []struct {
		name        string
		ballX       int
		ballY       int
		expectedCol int
		expectedRow int
	}{
		{
			name:        "center of cell (0,0)",
			ballX:       cellSize / 2, // 24
			ballY:       cellSize / 2, // 24
			expectedCol: 0,
			expectedRow: 0,
		},
		{
			name:        "bottom right corner of cell (0,0)",
			ballX:       cellSize - 1, // 47
			ballY:       cellSize - 1, // 47
			expectedCol: 0,
			expectedRow: 0,
		},
		{
			name:        "top left corner of cell (1,1)",
			ballX:       cellSize, // 48
			ballY:       cellSize, // 48
			expectedCol: 1,
			expectedRow: 1,
		},
		{
			name:        "specific cell (3, 2)",
			ballX:       cellSize*3 + cellSize/2, // 168
			ballY:       cellSize*2 + cellSize/2, // 120
			expectedCol: 3,
			expectedRow: 2,
		},
		{
			name:        "outside left",
			ballX:       -10,
			ballY:       120, // Row 2
			expectedCol: 0,   // Clamped
			expectedRow: 2,
		},
		{
			name:        "outside bottom",
			ballX:       168,             // Col 3
			ballY:       canvasSize + 10, // 586
			expectedCol: 3,
			expectedRow: gridSize - 1, // Clamped (11)
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ball := &Ball{X: tc.ballX, Y: tc.ballY, canvasSize: canvasSize}
			col, row := ball.getCenterIndex()
			// Add debug print inside the test as well
			fmt.Printf("Test: %s, Input: (%d, %d), CellSize: %d, GridSize: %d -> Got: (col=%d, row=%d), Expected: (col=%d, row=%d)\n",
				tc.name, tc.ballX, tc.ballY, cellSize, gridSize, col, row, tc.expectedCol, tc.expectedRow)
			if row != tc.expectedRow || col != tc.expectedCol {
				t.Errorf("Test case %s failed: expected col %d, row %d but got col %d, row %d", tc.name, tc.expectedCol, tc.expectedRow, col, row)
			}
		})
	}
}
