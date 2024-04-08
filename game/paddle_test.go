package game

import (
	"testing"

	"github.com/lguibr/pongo/utils"
)

func TestPaddle_SetDirection(t *testing.T) {
	paddle := Paddle{}
	testCases := []struct {
		buffer     []byte
		direction  string
		shouldPass bool
	}{
		{[]byte(`{"direction": "ArrowLeft"}`), "left", true},
		{[]byte(`{"direction": "ArrowRight"}`), "right", true},
		{[]byte(`{"direction": "invalid"}`), "", false},
		{[]byte(`{}`), "", false},
		{[]byte(``), "", false},
	}
	for _, tc := range testCases {
		_, err := paddle.SetDirection(tc.buffer)
		if err != nil {
			t.Errorf("Failed Setting direction")
		}
		if tc.shouldPass {
			if paddle.Direction != tc.direction {
				t.Errorf("Expected direction to be %s but got %s", tc.direction, paddle.Direction)
			}
		} else {
			if paddle.Direction != "" {
				t.Errorf("Expected direction to be empty but got %s", paddle.Direction)
			}
		}
	}
}

func TestPaddle_Move(t *testing.T) {
	paddle := Paddle{X: 10, Y: 20, Width: 30, Height: 40, Velocity: 5, canvasSize: utils.CanvasSize}
	testCases := []struct {
		index      int
		direction  string
		finalX     int
		finalY     int
		shouldMove bool
	}{
		{3, "left", 5, 20, true},
		{3, "right", 10, 20, true},
		{3, "", 10, 20, false},
		{3, "up", 10, 20, false},
		{3, "down", 10, 20, false},
		{3, "invalid", 10, 20, false},
		{2, "right", 10, 25, true},
		{3, "left", 5, 25, true},
	}

	for index, tc := range testCases {
		paddle.Index = tc.index
		paddle.Direction = tc.direction
		paddle.Move()
		if tc.shouldMove {
			if paddle.X != tc.finalX || paddle.Y != tc.finalY {
				t.Errorf("Expected paddle to move to (%d, %d) but got (%d, %d) in index : %d", tc.finalX, tc.finalY, paddle.X, paddle.Y, index)
			}
		} else {
			if paddle.X != 10 || paddle.Y != 20 {
				t.Errorf("Expected paddle to remain at (10, 20) but got (%d, %d)", paddle.X, paddle.Y)
			}
		}
	}

	// test case when paddle is at the boundary
	paddle.X = utils.CanvasSize - paddle.Width
	paddle.Y = utils.CanvasSize - paddle.Height
	paddle.Direction = "right"
	paddle.Move()
	if paddle.X != utils.CanvasSize-paddle.Width || paddle.Y != utils.CanvasSize-paddle.Height {
		t.Errorf("Expected paddle to remain at (%d, %d) but got (%d, %d)", utils.CanvasSize-paddle.Width, utils.CanvasSize-paddle.Height, paddle.X, paddle.Y)
	}
}
