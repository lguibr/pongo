package game

import (
	"testing"

	"github.com/lguibr/pongo/utils"
)

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
		expectedType utils.CellType
	}{
		{
			name:         "collides with brick",
			oldIndices:   [2]int{1, 2},
			newIndices:   [2]int{1, 3},
			life:         5,
			expectedVx:   1,
			expectedVy:   -1,
			expectedLife: 4,
			expectedType: utils.Cells.Brick,
		},
		{
			name:         "collides with brick with life of 1",
			oldIndices:   [2]int{2, 1},
			newIndices:   [2]int{1, 1},
			life:         1,
			expectedVx:   -1,
			expectedVy:   1,
			expectedLife: 0,
			expectedType: utils.Cells.Empty,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			data := &Cell{Data: NewBrickData(utils.Cells.Brick, tc.life)}
			grid := NewGrid(10)
			grid[tc.newIndices[0]][tc.newIndices[1]] = *data
			ballChannel := NewBallChannel()
			ball := &Ball{Channel: ballChannel, Vx: 1, Vy: 1, Mass: 1}

			go func() {
				for message := range ballChannel {
					switch message {
					default:
						continue
					}
				}
			}()

			ball.handleCollideBrick(tc.oldIndices, tc.newIndices, grid)

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
			if grid[tc.newIndices[0]][tc.newIndices[1]].Data.Life != tc.expectedLife {
				t.Errorf("Expected life = %d but got life = %d for oldIndices = %v, newIndices = %v",
					tc.expectedLife,
					grid[tc.newIndices[0]][tc.newIndices[1]].Data.Life,
					tc.oldIndices,
					tc.newIndices,
				)
			}
			if grid[tc.newIndices[0]][tc.newIndices[1]].Data.Type != tc.expectedType {
				t.Errorf("Expected type = %s but got type = %s for oldIndices = %v, newIndices = %v",
					tc.expectedType,
					grid[tc.newIndices[0]][tc.newIndices[1]].Data.Type,
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
		paddles            [4]*Paddle
		expectedCollision  bool
		expectedVelocityVx int
		expectedVelocityVy int
	}{
		{
			name:               "Collision with first player's paddle",
			paddles:            [4]*Paddle{{X: 50, Y: 50, Width: 30, Height: 30}, nil, nil, nil},
			expectedCollision:  true,
			expectedVelocityVx: -1,
			expectedVelocityVy: 1,
		},
		{
			name:               "No collision with any paddle",
			paddles:            [4]*Paddle{},
			expectedCollision:  false,
			expectedVelocityVx: 1,
			expectedVelocityVy: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ball := &Ball{X: 75, Y: 75, Vx: 1, Vy: 1, Radius: 10}

			ball.CollidePaddles(tc.paddles)
			if ball.Vx != tc.expectedVelocityVx || ball.Vy != tc.expectedVelocityVy {
				t.Errorf("Expected Vx = %d, Vy = %d but got Vx = %d, Vy = %d for paddles = %v",
					tc.expectedVelocityVx,
					tc.expectedVelocityVy,
					ball.Vx,
					ball.Vy,
					tc.paddles,
				)
			}
		})
	}
}

func TestBall_CollideWalls(t *testing.T) {
	testCases := []struct {
		name       string
		ballX      int
		ballY      int
		ballVx     int
		ballVy     int
		ballRadius int
		canvasSize int
		expectedVx int
		expectedVy int
	}{
		{
			name:       "Collide bottom wall",
			ballX:      75,
			ballY:      100,
			ballVx:     1,
			ballVy:     1,
			ballRadius: 10,
			canvasSize: 100,
			expectedVx: 1,
			expectedVy: -1,
		},
		{
			name:       "Collide top wall",
			ballX:      75,
			ballY:      10,
			ballVx:     1,
			ballVy:     -1,
			ballRadius: 10,
			canvasSize: 100,
			expectedVx: 1,
			expectedVy: 1,
		},
		{
			name:       "Collide Right wall",
			ballX:      95,
			ballY:      10,
			ballVx:     1,
			ballVy:     -1,
			ballRadius: 10,
			canvasSize: 100,
			expectedVx: -1,
			expectedVy: -1,
		},
		{
			name:       "Collide All Walls, last collisions at Top and Right ",
			ballX:      50,
			ballY:      50,
			ballVx:     1,
			ballVy:     1,
			ballRadius: 60,
			canvasSize: 100,
			expectedVx: -1,
			expectedVy: 1,
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {

			ball := &Ball{
				X:          test.ballX,
				Y:          test.ballY,
				Radius:     test.ballRadius,
				Vx:         test.ballVx,
				Vy:         test.ballVy,
				canvasSize: test.canvasSize,
				Channel:    NewBallChannel(),
			}

			go func() {
				for message := range ball.Channel {
					switch message {
					default:
						continue
					}
				}
			}()

			ball.CollideWalls()

			if ball.Vx != test.expectedVx || ball.Vy != test.expectedVy {
				t.Errorf("Test case %s: expected Vx = %d, Vy = %d but got Vx = %d, Vy = %d", test.name, test.expectedVx, test.expectedVy, ball.Vx, ball.Vy)
			}

		})
	}
}
func TestBall_Move(t *testing.T) {
	ball := NewBall(NewBallChannel(), 10, 20, 30, utils.CanvasSize, 1, 1)
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
	ball := NewBall(NewBallChannel(), 10, 20, 30, utils.CanvasSize, 1, 1)
	paddle := NewPaddle(make(chan PaddleMessage), utils.CanvasSize, 0)
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
	ball := NewBall(NewBallChannel(), 10, 10, 30, 12, 1, 1)

	// Set up test cases
	testCases := []struct {
		name                  string
		ballX, ballY          int
		theType               utils.CellType
		life                  int
		expectedCollisionType utils.CellType
		expectedLife          int
	}{
		{
			"TestCase1",
			10, 20,
			utils.Cells.Brick,
			2,
			utils.Cells.Brick,
			1,
		},
		{
			"TestCase2",
			15, 25,
			utils.Cells.Block,
			0,
			utils.Cells.Block,
			0,
		},
		{
			"TestCase3",
			15, 25,
			utils.Cells.Empty,
			0,
			utils.Cells.Empty,
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

			ball.CollideCells(grid, 1)
			if grid[0][0].Data.Type != tc.expectedCollisionType {
				t.Errorf("Expected collision type to be %s, but got %s", tc.expectedCollisionType, grid[0][0].Data.Type)
			}
			if grid[0][0].Data.Life != tc.expectedLife {
				t.Errorf("Expected life to be %d, but got %d", tc.expectedLife, grid[0][0].Data.Life)
			}
		})
	}
}
