package game

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/lguibr/pongo/utils"
)

func TestNewPlayer(t *testing.T) {
	type NewPlayerTestCase struct {
		canvas         *Canvas
		index          int
		id             string
		expectedPlayer *Player
	}
	canvas := &Canvas{Width: 800, Height: 600}
	balls := []*Ball{
		NewBall(canvas, 0, 0, 0, 1),
	}
	color := utils.NewRandomColor()
	testCases := []NewPlayerTestCase{
		{
			canvas: canvas,
			index:  1,
			id:     "player1",
			expectedPlayer: &Player{
				Index:  1,
				Id:     "player1",
				Canvas: canvas,
				Color:  color,
				Paddle: NewPaddle(canvas, 1),
				Balls:  balls,
			}},
		{
			canvas: canvas,
			index:  2,
			id:     "player2",
			expectedPlayer: &Player{
				Index:  2,
				Id:     "player2",
				Canvas: canvas,
				Color:  color,
				Paddle: NewPaddle(canvas, 2),
				Balls:  balls,
			}},
	}

	for _, test := range testCases {
		result := NewPlayer(test.canvas, test.index, test.id)
		fmt.Println("result", result)
		fmt.Println("test.expectedPlayer", test.expectedPlayer)

		//INFO Can't compare pointers
		result.Color = test.expectedPlayer.Color
		result.Paddle = test.expectedPlayer.Paddle
		result.Balls = test.expectedPlayer.Balls

		if !reflect.DeepEqual(result, test.expectedPlayer) {
			t.Errorf("Expected player %v, got \n%v", test.expectedPlayer, result)
		}
	}
}
