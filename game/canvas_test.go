package game

import (
	"fmt"
	"testing"

	"github.com/lguibr/pongo/utils"
	"github.com/stretchr/testify/assert"
)

func TestNewCanvas(t *testing.T) {
	type testCase struct {
		size, gridSize int
		panics         bool
	}
	testCases := []testCase{
		{0, 0, false},     // Will use defaults
		{100, 0, true},    // 100 not divisible by default grid size (16)
		{0, 8, false},     // Default size (1024) divisible by 8
		{100, 7, true},    // 100 not divisible by 7
		{96, 6, false},    // 96 divisible by 6
		{10, 100, true},   // 10 not divisible by 100
		{100, 5, true},    // 100 not divisible by 5 (GridSize < 6)
		{90, 5, true},     // GridSize < 6
		{1024, 16, false}, // Default config values
	}
	for index, tc := range testCases {
		t.Run(fmt.Sprintf("Case%d_Size%d_Grid%d", index, tc.size, tc.gridSize), func(t *testing.T) {
			panics, _ := utils.AssertPanics(t, func() { NewCanvas(tc.size, tc.gridSize) }, fmt.Sprintf("- Code did not panic on index %d", index))
			if panics != tc.panics {
				t.Errorf("Panic expectation mismatch: Expected panic=%t, Got panic=%t", tc.panics, panics)
			}
			// If no panic expected, check basic properties
			if !tc.panics {
				canvas := NewCanvas(tc.size, tc.gridSize)
				expectedSize := tc.size
				expectedGridSize := tc.gridSize
				if expectedSize == 0 {
					expectedSize = utils.DefaultConfig().CanvasSize
				}
				if expectedGridSize == 0 {
					expectedGridSize = utils.DefaultConfig().GridSize
				}

				assert.Equal(t, expectedSize, canvas.CanvasSize)
				assert.Equal(t, expectedGridSize, canvas.GridSize)
				assert.Equal(t, expectedSize/expectedGridSize, canvas.CellSize)
				assert.NotNil(t, canvas.Grid)
				assert.Equal(t, expectedGridSize, len(canvas.Grid))
			}
		})
	}
}
