package utils

import (
	"testing"
)

func TestTransformVector(t *testing.T) {
	cardinalX := [2]int{1, 0}
	expectedValues := [][2]int{{1, 0}, {0, 1}, {-1, 0}, {0, -1}}
	for index, transformation := range MatrixesOfRotation {
		x, y := TransformVector(transformation, cardinalX[0], cardinalX[1])
		if x != expectedValues[index][0] || y != expectedValues[index][1] {
			t.Error("Expected ", expectedValues[index], " got ", [2]int{x, y}, "on index", index)
		}
	}
}

func TestRotateVector(t *testing.T) {
	type TestRotateVectorCase struct {
		Vector         [2]int
		index          int
		expectedVector [2]int
	}
	testCases := []TestRotateVectorCase{
		{[2]int{1, 0}, 0, [2]int{1, 0}},
		{[2]int{1, 0}, 1, [2]int{0, 1}},
		{[2]int{1, 0}, 2, [2]int{-1, 0}},
		{[2]int{1, 0}, 3, [2]int{0, -1}},
	}
	for caseIndex, testCase := range testCases {
		x, y := RotateVector(testCase.index, testCase.Vector[0], testCase.Vector[1], 100, 100)
		if x != testCase.expectedVector[0] || y != testCase.expectedVector[1] {
			t.Error("Expected ", testCase.expectedVector, " got ", [2]int{x, y}, " for case ", caseIndex)
		}
	}
}
