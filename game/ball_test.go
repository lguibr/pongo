// File: game/ball_test.go
package game

import (
	"fmt" // Import fmt for debugging
	"testing"

	"github.com/lguibr/pongo/utils"
)

func TestNewBall(t *testing.T) {
	cfg := utils.DefaultConfig() // Create default config
	canvasSize := cfg.CanvasSize
	ballRadius := cfg.BallRadius
	paddleWidth := cfg.PaddleWidth

	testCases := []struct {
		name                                 string
		x, y, ownerIndex, id                 int
		isPermanent                          bool // Add isPermanent flag
		expectedX, expectedY, expectedRadius int
	}{
		{
			"TestCase1",
			10, 10, 1, 1, false, // Provide isPermanent
			10, 10, ballRadius,
		},
		{
			"TestCase2",         // Test custom radius (though NewBall now uses config)
			10, 20, 2, 2, false, // Provide isPermanent
			10, 20, ballRadius, // Expect config radius
		},
		{
			"TestCaseZeroPosPlayer0", // Test initial position calculation
			0, 0, 0, 3, true,         // Provide isPermanent (e.g., true for initial player ball)
			canvasSize - paddleWidth*2 - ballRadius, canvasSize / 2, ballRadius,
		},
		{
			"TestCaseZeroPosPlayer1", // Test initial position calculation
			0, 0, 1, 4, true,         // Provide isPermanent
			canvasSize / 2, paddleWidth*2 + ballRadius, ballRadius,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Pass config and isPermanent to NewBall
			ball := NewBall(cfg, tc.x, tc.y, tc.ownerIndex, tc.id, tc.isPermanent)
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
			if ball.IsPermanent != tc.isPermanent { // Check isPermanent flag
				t.Errorf("Expected IsPermanent to be %t, but got %t", tc.isPermanent, ball.IsPermanent)
			}
			if ball.Vx == 0 && ball.Vy == 0 {
				t.Errorf("Expected non-zero velocity components, but got Vx=%d, Vy=%d", ball.Vx, ball.Vy)
			}
		})
	}
}

func TestBall_BallInterceptPaddles(t *testing.T) {
	cfg := utils.DefaultConfig()
	ball := &Ball{X: 100, Y: 100, Radius: cfg.BallRadius} // Use config radius
	testCases := []struct {
		name       string
		paddle     *Paddle
		intercepts bool
	}{
		{"Overlap Center", &Paddle{X: 95, Y: 95, Width: 10, Height: 10}, true},
		{"Overlap TopLeft Corner", &Paddle{X: 90, Y: 90, Width: 15, Height: 15}, true},
		{"No Overlap Corner", &Paddle{X: 110 + cfg.BallRadius, Y: 110 + cfg.BallRadius, Width: 20, Height: 20}, false},                // Adjust based on radius
		{"Overlap Left Edge", &Paddle{X: 100 - cfg.BallRadius - 5, Y: 95, Width: 10, Height: 10}, true},                               // Adjust based on radius
		{"No Overlap Far", &Paddle{X: 120 + cfg.BallRadius, Y: 120 + cfg.BallRadius, Width: 20, Height: 20}, false},                   // Adjust based on radius
		{"Overlap Top Edge", &Paddle{X: 95, Y: 100 - cfg.BallRadius - 5, Width: 10, Height: 10}, true},                                // Adjust based on radius
		{"Touching Top Edge", &Paddle{X: 95, Y: 100 - cfg.BallRadius - 10, Width: 10, Height: 10}, false},                             // Adjust based on radius
		{"Intercepts Corner 1", &Paddle{X: 100 - cfg.BallRadius, Y: 100 - cfg.BallRadius, Width: 10, Height: 10}, true},               // Adjust based on radius
		{"Clearly Outside Corner 2", &Paddle{X: 100 + cfg.BallRadius + 1, Y: 100 + cfg.BallRadius + 1, Width: 10, Height: 10}, false}, // Adjust based on radius
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ball.BallInterceptPaddles(tc.paddle)
			if result != tc.intercepts {
				paddleBounds := fmt.Sprintf("X:[%d,%d], Y:[%d,%d]", tc.paddle.X, tc.paddle.X+tc.paddle.Width, tc.paddle.Y, tc.paddle.Y+tc.paddle.Height)
				t.Errorf("Ball(X:%d,Y:%d,R:%d) vs Paddle(%s): Expected BallInterceptPaddles to return %t but got %t",
					ball.X, ball.Y, ball.Radius, paddleBounds, tc.intercepts, result)
			}
		})
	}
}

func TestBall_InterceptsIndex(t *testing.T) {
	cfg := utils.DefaultConfig()
	tests := []struct {
		name               string
		ball               *Ball
		col, row, cellSize int
		want               bool
	}{
		{
			name: "Intercepts top-left corner",
			ball: &Ball{X: 25, Y: 25, Radius: cfg.BallRadius}, // Use config radius
			col:  0, row: 0, cellSize: 50,
			want: true,
		},
		{
			name: "Does not intercept top-left corner",
			ball: &Ball{X: 25, Y: 25, Radius: 5}, // Keep small radius for this case
			col:  0, row: 0, cellSize: 10,
			want: false,
		},
		{
			name: "Intercepts center of cell",
			ball: &Ball{X: 75, Y: 75, Radius: cfg.BallRadius}, // Use config radius
			col:  1, row: 1, cellSize: 50,
			want: true,
		},
		{
			name: "Intercepts bottom-right corner",
			ball: &Ball{X: 45, Y: 45, Radius: cfg.BallRadius}, // Use config radius
			col:  0, row: 0, cellSize: 50,
			want: true,
		},
		{
			name: "Does not intercept bottom-right corner",
			ball: &Ball{X: 55, Y: 55, Radius: 5}, // Keep small radius
			col:  0, row: 0, cellSize: 50,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ball.InterceptsIndex(tt.col, tt.row, tt.cellSize); got != tt.want {
				t.Errorf("InterceptsIndex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBall_GetCenterIndex(t *testing.T) {
	cfg := utils.DefaultConfig() // Create default config
	canvasSize := cfg.CanvasSize
	gridSize := cfg.GridSize
	cellSize := cfg.CellSize

	testCases := []struct {
		name        string
		ballX       int
		ballY       int
		expectedCol int
		expectedRow int
	}{
		{
			name:        "center of cell (0,0)",
			ballX:       cellSize / 2,
			ballY:       cellSize / 2,
			expectedCol: 0,
			expectedRow: 0,
		},
		{
			name:        "bottom right corner of cell (0,0)",
			ballX:       cellSize - 1,
			ballY:       cellSize - 1,
			expectedCol: 0,
			expectedRow: 0,
		},
		{
			name:        "top left corner of cell (1,1)",
			ballX:       cellSize,
			ballY:       cellSize,
			expectedCol: 1,
			expectedRow: 1,
		},
		{
			name:        "specific cell (3, 2)",
			ballX:       cellSize*3 + cellSize/2,
			ballY:       cellSize*2 + cellSize/2,
			expectedCol: 3,
			expectedRow: 2,
		},
		{
			name:        "outside left",
			ballX:       -10, // X < 0
			ballY:       120, // Y = 120 -> 120 / 64 = 1
			expectedCol: 0,   // Clamped from -10/64=0
			expectedRow: 1,   // Clamped from 120/64=1
		},
		{
			name:        "outside bottom",
			ballX:       168,             // X = 168 -> 168 / 64 = 2
			ballY:       canvasSize + 10, // Y > canvasSize -> 1034 / 64 = 16
			expectedCol: 2,               // Clamped from 168/64=2
			expectedRow: gridSize - 1,    // Clamped from 16
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set canvasSize in the ball struct for the method
			ball := &Ball{X: tc.ballX, Y: tc.ballY, canvasSize: canvasSize}
			// Pass config to getCenterIndex
			col, row := ball.getCenterIndex(cfg)
			if row != tc.expectedRow || col != tc.expectedCol {
				t.Errorf("Test case %s failed: expected col %d, row %d but got col %d, row %d", tc.name, tc.expectedCol, tc.expectedRow, col, row)
			}
		})
	}
}
