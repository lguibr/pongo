package game

import (
	"testing"

	"github.com/lguibr/pongo/utils"
	"github.com/stretchr/testify/assert"
)

func TestNewPlayer(t *testing.T) {
	type NewPlayerTestCase struct {
		name          string
		canvas        *Canvas // Keep canvas for context if needed, but not stored in Player
		index         int
		expectedID    string
		expectedScore int32
	}
	canvasSize := 800
	canvas := &Canvas{Width: canvasSize, Height: canvasSize}
	cfg := utils.DefaultConfig()

	testCases := []NewPlayerTestCase{
		{
			name:          "Player 1",
			canvas:        canvas,
			index:         1,
			expectedID:    "player1",
			expectedScore: int32(cfg.InitialScore),
		},
		{
			name:          "Player 2",
			canvas:        canvas,
			index:         2,
			expectedID:    "player2",
			expectedScore: int32(cfg.InitialScore),
		},
		{
			name:          "Player 0",
			canvas:        canvas,
			index:         0,
			expectedID:    "player0",
			expectedScore: int32(cfg.InitialScore),
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			result := NewPlayer(test.canvas, test.index)

			assert.Equal(t, test.index, result.Index)
			assert.Equal(t, test.expectedID, result.Id)
			assert.Equal(t, test.expectedScore, result.Score)
			assert.Len(t, result.Color, 3, "Color should have 3 components")

			// Optional: Check color values are within range
			for _, c := range result.Color {
				assert.GreaterOrEqual(t, c, 0, "Color component should be >= 0")
				assert.LessOrEqual(t, c, 255, "Color component should be <= 255")
			}
		})
	}
}
