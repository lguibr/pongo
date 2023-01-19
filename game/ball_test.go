package game

import (
	"testing"

	"github.com/lguibr/pongo/utils"
)

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
		name string
		ball *Ball
		x, y int
		want bool
	}{
		{
			name: "Intercepts top-left corner",
			ball: &Ball{X: 25, Y: 25, Radius: 10, Canvas: &Canvas{CellSize: 50}},
			x:    0, y: 0,
			want: true,
		},
		{
			name: "Does not intercept top-left corner",
			ball: &Ball{X: 25, Y: 25, Radius: 5, Canvas: &Canvas{CellSize: 10}},
			x:    0, y: 0,
			want: false,
		},
		{
			name: "Intercepts center of cell",
			ball: &Ball{X: 75, Y: 75, Radius: 10, Canvas: &Canvas{CellSize: 50}},
			x:    1, y: 1,
			want: true,
		},
		{
			name: "Intercepts bottom-right corner",
			ball: &Ball{X: 75, Y: 75, Radius: 10, Canvas: &Canvas{CellSize: 50}},
			x:    1, y: 1,
			want: true,
		},
		{
			name: "Does not intercept bottom-right corner",
			ball: &Ball{X: 75, Y: 75, Radius: 5, Canvas: &Canvas{CellSize: 50}},
			x:    0, y: 0,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ball.InterceptsIndex(tt.x, tt.y); got != tt.want {
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
			ball:     Ball{X: 900, Radius: 10, Canvas: &Canvas{Width: 900}},
			expected: true,
		},
		{
			name:     "ball is still inside the canvas",
			ball:     Ball{X: 800, Radius: 10, Canvas: &Canvas{Width: 900}},
			expected: false,
		},
		{
			name:     "ball exceeds the right wall",
			ball:     Ball{X: 905, Radius: 10, Canvas: &Canvas{Width: 900}},
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
			ball := &Ball{Y: tc.ballY, Radius: tc.ballRadius, Canvas: &Canvas{Height: tc.canvasHeight}}
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

func TestBall_HandleCollideBlock(t *testing.T) {
	testCases := []struct {
		name       string
		oldIndices [2]int
		newIndices [2]int
		expectedVx int
		expectedVy int
	}{
		{
			name:       "Collision Top Down",
			oldIndices: [2]int{1, 2},
			newIndices: [2]int{1, 3},
			expectedVx: 1,
			expectedVy: -1,
		},
		{
			name:       "Collision Left Right",
			oldIndices: [2]int{2, 1},
			newIndices: [2]int{1, 1},
			expectedVx: -1,
			expectedVy: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ball := &Ball{Vx: 1, Vy: 1}
			ball.handleCollideBlock(tc.oldIndices, tc.newIndices)
			if ball.Vx != tc.expectedVx || ball.Vy != tc.expectedVy {
				t.Errorf("Expected Vx = %d, Vy = %d but got Vx = %d, Vy = %d for oldIndices = %v, newIndices = %v",
					tc.expectedVx,
					tc.expectedVy,
					ball.Vx,
					ball.Vy,
					tc.oldIndices,
					tc.newIndices,
				)
			}
		})
	}
}

func TestBall_HandleCollideBrick(t *testing.T) {
	testCases := []struct {
		name         string
		oldIndices   [2]int
		newIndices   [2]int
		life         int
		expectedVx   int
		expectedVy   int
		expectedLife int
		expectedType string
	}{
		{
			name:         "collides with brick",
			oldIndices:   [2]int{1, 2},
			newIndices:   [2]int{1, 3},
			life:         5,
			expectedVx:   1,
			expectedVy:   -1,
			expectedLife: 4,
			expectedType: "Brick",
		},
		{
			name:         "collides with brick with life of 1",
			oldIndices:   [2]int{2, 1},
			newIndices:   [2]int{1, 1},
			life:         1,
			expectedVx:   -1,
			expectedVy:   1,
			expectedLife: 0,
			expectedType: "Empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			data := &Cell{Data: NewBrickData("Brick", tc.life)}
			grid := NewGrid(10)
			grid[tc.newIndices[0]][tc.newIndices[1]] = *data
			ball := &Ball{Vx: 1, Vy: 1, Canvas: &Canvas{Grid: grid}}

			ball.handleCollideBrick(tc.oldIndices, tc.newIndices)

			if ball.Vx != tc.expectedVx || ball.Vy != tc.expectedVy {
				t.Errorf("Expected Vx = %d, Vy = %d but got Vx = %d, Vy = %d for oldIndices = %v, newIndices = %v",
					tc.expectedVx,
					tc.expectedVy,
					ball.Vx,
					ball.Vy,
					tc.oldIndices,
					tc.newIndices,
				)
			}
			if ball.Canvas.Grid[tc.newIndices[0]][tc.newIndices[1]].Data.Life != tc.expectedLife {
				t.Errorf("Expected life = %d but got life = %d for oldIndices = %v, newIndices = %v",
					tc.expectedLife,
					ball.Canvas.Grid[tc.newIndices[0]][tc.newIndices[1]].Data.Life,
					tc.oldIndices,
					tc.newIndices,
				)
			}
			if ball.Canvas.Grid[tc.newIndices[0]][tc.newIndices[1]].Data.Type != tc.expectedType {
				t.Errorf("Expected type = %s but got type = %s for oldIndices = %v, newIndices = %v",
					tc.expectedType,
					ball.Canvas.Grid[tc.newIndices[0]][tc.newIndices[1]].Data.Type,
					tc.oldIndices,
					tc.newIndices,
				)
			}
		})
	}
}

func TestBall_CollidePaddles(t *testing.T) {
	// Create test cases with different scenarios
	testCases := []struct {
		name               string
		players            [4]*Player
		expectedCollision  bool
		expectedVelocityVx int
		expectedVelocityVy int
	}{
		{
			name: "Collision with first player's paddle",
			players: [4]*Player{
				{Paddle: &Paddle{X: 50, Y: 50, Width: 30, Height: 30}},
				nil,
				nil,
				nil,
			},
			expectedCollision:  true,
			expectedVelocityVx: -1,
			expectedVelocityVy: 1,
		},
		{
			name: "No collision with any paddle",
			players: [4]*Player{
				{Paddle: &Paddle{X: 100, Y: 100, Width: 30, Height: 30}},
				{Paddle: &Paddle{X: 200, Y: 200, Width: 30, Height: 30}},
				{Paddle: &Paddle{X: 300, Y: 300, Width: 30, Height: 30}},
				{Paddle: &Paddle{X: 400, Y: 400, Width: 30, Height: 30}},
			},
			expectedCollision:  false,
			expectedVelocityVx: 1,
			expectedVelocityVy: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ball := &Ball{X: 75, Y: 75, Vx: 1, Vy: 1, Radius: 10}

			ball.CollidePaddles(tc.players)
			if ball.Vx != tc.expectedVelocityVx || ball.Vy != tc.expectedVelocityVy {
				t.Errorf("Expected Vx = %d, Vy = %d but got Vx = %d, Vy = %d for players = %v",
					tc.expectedVelocityVx,
					tc.expectedVelocityVy,
					ball.Vx,
					ball.Vy,
					tc.players,
				)
			}
		})
	}
}

func TestBall_CollideWalls(t *testing.T) {
	testCases := []struct {
		name         string
		ballX        int
		ballY        int
		ballVx       int
		ballVy       int
		ballRadius   int
		canvasWidth  int
		canvasHeight int
		expectedVx   int
		expectedVy   int
	}{
		{
			name:         "Collide bottom wall",
			ballX:        75,
			ballY:        100,
			ballVx:       1,
			ballVy:       1,
			ballRadius:   10,
			canvasWidth:  100,
			canvasHeight: 100,
			expectedVx:   1,
			expectedVy:   -1,
		},
		{
			name:         "Collide top wall",
			ballX:        75,
			ballY:        10,
			ballVx:       1,
			ballVy:       -1,
			ballRadius:   10,
			canvasWidth:  100,
			canvasHeight: 100,
			expectedVx:   1,
			expectedVy:   1,
		},
		{
			name:         "Collide Right wall and top wall",
			ballX:        95,
			ballY:        10,
			ballVx:       1,
			ballVy:       -1,
			ballRadius:   10,
			canvasWidth:  100,
			canvasHeight: 100,
			expectedVx:   -1,
			expectedVy:   1,
		},
		{
			name:         "Collide All Walls, last collisions at Top and Right ",
			ballX:        50,
			ballY:        50,
			ballVx:       1,
			ballVy:       1,
			ballRadius:   60,
			canvasWidth:  100,
			canvasHeight: 100,
			expectedVx:   -1,
			expectedVy:   1,
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {

			ball := &Ball{
				X:      test.ballX,
				Y:      test.ballY,
				Radius: test.ballRadius,
				Vx:     test.ballVx,
				Vy:     test.ballVy,
				Canvas: &Canvas{Width: test.canvasWidth, Height: test.canvasHeight},
			}

			ball.CollideWalls()

			if ball.Vx != test.expectedVx || ball.Vy != test.expectedVy {
				t.Errorf("Test case %s: expected Vx = %d, Vy = %d but got Vx = %d, Vy = %d", test.name, test.expectedVx, test.expectedVy, ball.Vx, ball.Vy)
			}

		})
	}
}
func TestBall_Move(t *testing.T) {
	canvas := NewCanvas(utils.CanvasSize, utils.GridSize)
	ball := NewBall(canvas, 10, 20, 30, 1)
	ball.Ax = 1
	ball.Ay = 2
	testCases := []struct {
		name                                         string
		vx, vy, ax, ay                               int
		expectedX, expectedY, expectedVx, expectedVy int
	}{
		{"TestCase1",
			10, 20, 1, 2,
			20, 41, 11, 22,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			ball.Vx = tc.vx
			ball.Vy = tc.vy
			ball.Ax = tc.ax
			ball.Ay = tc.ay
			ball.Move()

			if ball.X != tc.expectedX {
				t.Errorf("Expected X to be %d, but got %d", tc.expectedX, ball.X)
			}
			if ball.Y != tc.expectedY {
				t.Errorf("Expected Y to be %d, but got %d", tc.expectedY, ball.Y)
			}
			if ball.Vx != tc.expectedVx {
				t.Errorf("Expected Vx to be %d, but got %d", tc.expectedVx, ball.Vx)
			}
			if ball.Vy != tc.expectedVy {
				t.Errorf("Expected Vy to be %d, but got %d", tc.expectedVy, ball.Vy)
			}
		})
	}
}

func TestBall_CollidePaddle(t *testing.T) {
	canvas := NewCanvas(utils.CanvasSize, utils.GridSize)
	ball := NewBall(canvas, 10, 20, 30, 1)
	paddle := NewPaddle(canvas, 0)
	testCases := []struct {
		name                                        string
		ballX, ballY, ballVx, ballVy                int
		paddleX, paddleY, paddleWidth, paddleHeight int
		expectedVx, expectedVy                      int
	}{
		{
			"TestCase1",
			10, 20, 1, 1,
			10, 20, 30, 30,
			-1, 1,
		},
		{
			"TestCase2",
			10, 20, 1, 1,
			30, 30, 30, 30,
			-1, 1,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ball.X, ball.Y = tc.ballX, tc.ballY
			ball.Vx, ball.Vy = tc.ballVx, tc.ballVy
			paddle.X, paddle.Y, paddle.Width, paddle.Height = tc.paddleX, tc.paddleY, tc.paddleWidth, tc.paddleHeight
			ball.CollidePaddle(paddle)
			if ball.Vx != tc.expectedVx {
				t.Errorf("Expected Vx to be %d, but got %d", tc.expectedVx, ball.Vx)
			}
			if ball.Vy != tc.expectedVy {
				t.Errorf("Expected Vy to be %d, but got %d", tc.expectedVy, ball.Vy)
			}
		})
	}
}

func TestCollideCells(t *testing.T) {
	canvas := NewCanvas(12, 6)
	ball := NewBall(canvas, 10, 10, 30, 1)

	// Set up test cases
	testCases := []struct {
		name                  string
		ballX, ballY          int
		theType               string
		life                  int
		expectedCollisionType string
		expectedLife          int
	}{
		{
			"TestCase1",
			10, 20,
			"Brick",
			2,
			"Brick",
			1,
		},
		{
			"TestCase2",
			15, 25,
			"Block",
			0,
			"Block",
			0,
		},
		{
			"TestCase3",
			15, 25,
			"Empty",
			0,
			"Empty",
			0,
		},
	}

	// Override the grid of the canvas
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ball.X, ball.Y = tc.ballX, tc.ballY
			grid := make(Grid, 10)
			for i := range grid {
				grid[i] = make([]Cell, 10)
				for j := range grid[i] {
					grid[i][j] = NewCell(i, j, tc.life, tc.theType)
				}
			}
			ball.Canvas.Grid = grid
			ball.CollideCells()
			if ball.Canvas.Grid[0][0].Data.Type != tc.expectedCollisionType {
				t.Errorf("Expected collision type to be %s, but got %s", tc.expectedCollisionType, ball.Canvas.Grid[0][0].Data.Type)
			}
			if ball.Canvas.Grid[0][0].Data.Life != tc.expectedLife {
				t.Errorf("Expected life to be %d, but got %d", tc.expectedLife, ball.Canvas.Grid[0][0].Data.Life)
			}
		})
	}
}
func TestNewBall(t *testing.T) {
	canvas := NewCanvas(utils.CanvasSize, utils.GridSize)
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
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ball := NewBall(canvas, tc.x, tc.y, tc.radius, tc.index)
			if ball.X != tc.expectedX {
				t.Errorf("Expected X to be %d, but got %d", tc.expectedX, ball.X)
			}
			if ball.Y != tc.expectedY {
				t.Errorf("Expected Y to be %d, but got %d", tc.expectedY, ball.Y)
			}
			if ball.Radius != tc.expectedRadius {
				t.Errorf("Expected Radius to be %d, but got %d", tc.expectedRadius, ball.Radius)
			}

			if ball.Canvas != canvas {
				t.Errorf("Expected Canvas to be %v, but got %v", canvas, ball.Canvas)
			}
			if ball.Index != tc.index {
				t.Errorf("Expected Index to be %d, but got %d", tc.index, ball.Index)
			}
		})
	}
}
