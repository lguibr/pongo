package game

import (
	"testing"

	"github.com/lguibr/pongo/utils"
)

func TestNewPlayer(t *testing.T) {
	type NewPlayerTestCase struct {
		canvas         *Canvas // Keep canvas for context if needed, but not stored in Player
		index          int
		id             string
		expectedPlayer *Player
	}
	canvasSize := 800
	canvas := &Canvas{Width: canvasSize, Height: canvasSize}

	// Expected player doesn't store canvas or channel anymore
	expectedPlayer1 := &Player{
		Score: utils.InitialScore,
		Index: 1,
		Id:    "player1",
		Color: [3]int{0, 0, 0}, // We'll overwrite color below
	}
	expectedPlayer2 := &Player{
		Score: utils.InitialScore,
		Index: 2,
		Id:    "player2",
		Color: [3]int{0, 0, 0}, // We'll overwrite color below
	}

	testCases := []NewPlayerTestCase{
		{
			canvas:         canvas,
			index:          1,
			id:             "player1",
			expectedPlayer: expectedPlayer1,
		},
		{
			canvas:         canvas,
			index:          2,
			id:             "player2",
			expectedPlayer: expectedPlayer2,
		},
	}

	for _, test := range testCases {
		// Call NewPlayer with the current signature (no channel)
		result := NewPlayer(test.canvas, test.index)

		// Compare relevant fields
		// We can't compare Color directly as it's random.
		// Check other fields and that Color has 3 elements.
		if result.Index != test.expectedPlayer.Index ||
			result.Id != test.expectedPlayer.Id ||
			result.Score != test.expectedPlayer.Score ||
			len(result.Color) != 3 {
			t.Errorf("Expected player %+v (ignoring color), got %+v", *(test.expectedPlayer), *result)
		}

		// Optional: Check color values are within range
		for _, c := range result.Color {
			if c < 0 || c > 255 {
				t.Errorf("Expected color component between 0 and 255, got %d", c)
			}
		}

		// Use DeepEqual only if we manually set the color for comparison (less ideal)
		// test.expectedPlayer.Color = result.Color // Make colors match for DeepEqual
		// if !reflect.DeepEqual(result, test.expectedPlayer) {
		// 	t.Errorf("Expected player %v, got \n%v", test.expectedPlayer, result)
		// }
	}
}
