package game

import (
	"fmt"
	"testing"

	"github.com/lguibr/pongo/utils"
)

func TestNewCanvas(t *testing.T) {
	type testCase struct {
		size, gridSize int
		panics         bool
	}
	testCases := []testCase{
		{0, 0, false},
		{100, 0, false},
		{0, 8, false},
		{100, 7, true},
		{90, 6, false},
	}
	for index, tc := range testCases {
		if tc.panics {
			utils.AssertPanics(t, func() { NewCanvas(tc.size, tc.gridSize) }, fmt.Sprintf("Code did not panic on index %d", index))
		}
	}
}
