// File: game/canvas_test.go
package game

import (
	"fmt"
	"testing"

	"github.com/lguibr/pongo/utils"
	"github.com/stretchr/testify/assert"
)

// Removed duplicate Canvas struct, methods, and NewCanvas function definitions.
// The tests will now use the actual implementations from canvas.go.

func TestNewCanvas(t *testing.T) {
	type testCase struct {
		size, gridSize int
		panics         bool
	}
	testCases := []testCase{
		{0, 0, false},     // Will use defaults (900, 18) - Divisible
		{0, 8, true},      // Default size (900) NOT divisible by 8 -> Should panic
		{100, 7, true},    // 100 not divisible by 7
		{96, 6, false},    // 96 divisible by 6
		{10, 100, true},   // 10 not divisible by 100
		{100, 5, true},    // GridSize < 6
		{90, 5, true},     // GridSize < 6
		{1024, 16, false}, // Old default config values (still valid)
		{900, 18, false},  // Current default config values (900 / 18 = 50)
		{600, 12, false},  // Valid combination
	}
	for index, tc := range testCases {
		t.Run(fmt.Sprintf("Case%d_Size%d_Grid%d", index, tc.size, tc.gridSize), func(t *testing.T) {
			// Add logging before calling AssertPanics
			t.Logf("Running test case: Size=%d, GridSize=%d, ExpectPanic=%t", tc.size, tc.gridSize, tc.panics)

			panics, panicMsg := utils.AssertPanics(t, func() { NewCanvas(tc.size, tc.gridSize) }, fmt.Sprintf("- Code did not panic on index %d", index))

			// Add logging after AssertPanics
			t.Logf("AssertPanics result: panics=%t, panicMsg='%s'", panics, panicMsg)

			// Check if the panic status matches the expectation.
			// Relaxing the check for Case1 due to potential helper issues.
			assert.Equal(t, tc.panics, panics, "Panic expectation mismatch")

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
				// Check for zero gridSize before division
				if expectedGridSize != 0 {
					assert.Equal(t, expectedSize/expectedGridSize, canvas.CellSize)
				}
				assert.NotNil(t, canvas.Grid)
				assert.Equal(t, expectedGridSize, len(canvas.Grid))
			}
		})
	}
}