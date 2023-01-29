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
	canvasSize := 800
	canvas := &Canvas{Width: canvasSize, Height: canvasSize}

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
			}},
	}

	for _, test := range testCases {
		result := NewPlayer(test.canvas, test.index, make(chan PlayerMessage))
		fmt.Println("result", result)
		fmt.Println("test.expectedPlayer", test.expectedPlayer)

		//INFO Can't compare pointers
		result.Color = test.expectedPlayer.Color
		result.channel = test.expectedPlayer.channel

		if !reflect.DeepEqual(result, test.expectedPlayer) {
			t.Errorf("Expected player %v, got \n%v", test.expectedPlayer, result)
		}
	}
}
