package game

import (
	"testing"

	"github.com/lguibr/pongo/utils"
)

func TestNewBall(t *testing.T) {
	canvasSize := utils.CanvasSize
	testCases := []struct {
		name                                 string
		x, y, radius, index                  int
		expectedX, expectedY, expectedRadius int
	}{
		{
			"TestCase1",
			10, 10, 0, 1,
			10, 10, utils.BallSize,
		},
		{
			"TestCase2",
			10, 20, 30, 1,
			10, 20, 30,
		},
	}
	ballChannel := make(chan BallMessage)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ball := NewBall(ballChannel, tc.x, tc.y, tc.radius, canvasSize, tc.index, tc.index)
			if ball.X != tc.expectedX {
				t.Errorf("Expected X to be %d, but got %d", tc.expectedX, ball.X)
			}
			if ball.Y != tc.expectedY {
				t.Errorf("Expected Y to be %d, but got %d", tc.expectedY, ball.Y)
			}
			if ball.Radius != tc.expectedRadius {
				t.Errorf("Expected Radius to be %d, but got %d", tc.expectedRadius, ball.Radius)
			}

			if ball.Index != tc.index {
				t.Errorf("Expected Index to be %d, but got %d", tc.index, ball.Index)
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

func TestBall_CollidesLeftWall(t *testing.T) {
	testCases := []struct {
		name              string
		ballX             int
		ballRadius        int
		expectedCollision bool
	}{
		{"ball is inside the wall", 10, 5, false},
		{"ball touches the left wall", 5, 5, true},
		{"ball is on the left wall", 0, 5, true},
		{"ball is outside of the left wall", -5, 5, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ball := &Ball{X: tc.ballX, Radius: tc.ballRadius}
			if collision := ball.CollidesLeftWall(); collision != tc.expectedCollision {
				t.Errorf("Expected collision to be %t but got %t for ballX = %d, ballRadius = %d", tc.expectedCollision, collision, tc.ballX, tc.ballRadius)
			}
		})
	}
}

func TestBall_CollideRightWall(t *testing.T) {
	type CollideRightWallTestCase struct {
		name     string
		ball     Ball
		expected bool
	}
	testCases := []CollideRightWallTestCase{
		{
			name:     "ball just touches the right wall",
			ball:     Ball{X: 900, Radius: 10, canvasSize: 900},
			expected: true,
		},
		{
			name:     "ball is still inside the canvas",
			ball:     Ball{X: 800, Radius: 10, canvasSize: 900},
			expected: false,
		},
		{
			name:     "ball exceeds the right wall",
			ball:     Ball{X: 905, Radius: 10, canvasSize: 900},
			expected: true,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := testCase.ball.CollidesRightWall()
			if result != testCase.expected {
				t.Errorf("For test case %s expected %v but got %v", testCase.name, testCase.expected, result)
			}
		})
	}
}

func TestBall_CollidesBottomWall(t *testing.T) {
	testCases := []struct {
		name              string
		ballY             int
		ballRadius        int
		canvasHeight      int
		expectedCollision bool
	}{
		{
			name:              "ball with bottom edge outside canvas",
			ballY:             10,
			ballRadius:        5,
			canvasHeight:      15,
			expectedCollision: true,
		},
		{
			name:              "ball with bottom edge inside canvas",
			ballY:             5,
			ballRadius:        5,
			canvasHeight:      15,
			expectedCollision: false,
		},
		{
			name:              "ball with top edge outside canvas",
			ballY:             -5,
			ballRadius:        5,
			canvasHeight:      15,
			expectedCollision: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ball := &Ball{Y: tc.ballY, Radius: tc.ballRadius, canvasSize: tc.canvasHeight}
			if collision := ball.CollidesBottomWall(); collision != tc.expectedCollision {
				t.Errorf("Expected collision to be %t but got %t for ballY = %d, ballRadius = %d, canvasHeight = %d", tc.expectedCollision, collision, tc.ballY, tc.ballRadius, tc.canvasHeight)
			}
		})
	}
}

func TestBall_CollidesTopWall(t *testing.T) {
	testCases := []struct {
		name              string
		ballY             int
		ballRadius        int
		expectedCollision bool
	}{
		{"ball above top wall", 10, 5, false},
		{"ball at top wall", 5, 5, true},
		{"ball at 0", 0, 5, true},
		{"ball inside top wall", -5, 5, true},
	}

	for _, tc := range testCases {
		ball := &Ball{Y: tc.ballY, Radius: tc.ballRadius}
		if collision := ball.CollidesTopWall(); collision != tc.expectedCollision {
			t.Errorf("Test case %s: Expected collision to be %t but got %t for ballY = %d, ballRadius = %d", tc.name, tc.expectedCollision, collision, tc.ballY, tc.ballRadius)
		}
	}
}

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

	cellSize := 10
	gridSize := 50

	testCases := []struct {
		name        string
		ballX       int
		ballY       int
		expectedRow int
		expectedCol int
	}{

		{
			name:        "center of cell",
			ballX:       cellSize / 2,
			ballY:       cellSize / 2,
			expectedRow: 0,
			expectedCol: 0,
		},
		{
			name:        "bottom right of cell",
			ballX:       cellSize - 1,
			ballY:       cellSize - 1,
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
			name:        "center of grid",
			ballX:       cellSize * gridSize / 2,
			ballY:       cellSize * gridSize / 2,
			expectedRow: gridSize / cellSize,
			expectedCol: gridSize / cellSize,
		},
	}

	for _, tc := range testCases {
		ball := &Ball{X: tc.ballX, Y: tc.ballY}
		row, col := ball.getCenterIndex(NewGrid(gridSize))
		if row != tc.expectedRow || col != tc.expectedCol {
			t.Errorf("Test case %s failed: expected row %d and col %d but got row %d and col %d", tc.name, tc.expectedRow, tc.expectedCol, row, col)
		}
	}
}
