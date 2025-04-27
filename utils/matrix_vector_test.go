// File: utils/matrix_vector_test.go
package utils

import (
	"testing"
)

func TestNewMatrixesOfRotation(t *testing.T) {
	matrixes := NewMatrixesOfRotation()
	expectedMatrixes := [4][2][2]int{
		{{1, 0}, {0, 1}},
		{{0, 1}, {-1, 0}},
		{{-1, 0}, {0, -1}},
		{{0, -1}, {1, 0}},
	}
	for i := range matrixes {
		for j := range matrixes[i] {
			for k := range matrixes[i][j] {
				if matrixes[i][j][k] != expectedMatrixes[i][j][k] {
					t.Errorf("NewMatrixesOfRotation() returned an invalid matrix at index [%d][%d][%d]: %d, want %d", i, j, k, matrixes[i][j][k], expectedMatrixes[i][j][k])
				}
			}
		}
	}
}

func TestTransformVector(t *testing.T) {
	cardinalX := [2]int{1, 0}
	expectedValues := [][2]int{{1, 0}, {0, -1}, {-1, 0}, {0, 1}}
	for index, matrixOfRotation := range MatrixesOfRotation {
		x, y := TransformVector(matrixOfRotation, cardinalX[0], cardinalX[1])
		if x != expectedValues[index][0] || y != expectedValues[index][1] {
			t.Error("Expected ", expectedValues[index], " got ", [2]int{x, y}, "on index", index)
		}
	}
}

func TestTransformVector2(t *testing.T) {
	testCases := []struct {
		tMatrix  [2][2]int
		vector   [2]int
		expected [2]int
		name     string
	}{
		{[2][2]int{{1, 0}, {0, 1}}, [2]int{1, 1}, [2]int{1, 1}, "Identity transform"},
		{[2][2]int{{0, -1}, {1, 0}}, [2]int{1, 1}, [2]int{-1, 1}, "90 degrees rotation"},
		{[2][2]int{{-1, 0}, {0, -1}}, [2]int{1, 1}, [2]int{-1, -1}, "180 degrees rotation"},
		{[2][2]int{{0, 1}, {-1, 0}}, [2]int{1, 1}, [2]int{1, -1}, "270 degrees rotation"},
		{[2][2]int{{2, 0}, {0, 2}}, [2]int{1, 1}, [2]int{2, 2}, "Diagonal scaling"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			x, y := TransformVector(tc.tMatrix, tc.vector[0], tc.vector[1])
			result := [2]int{x, y}
			if result != tc.expected {
				t.Errorf("TransformVector(%v, %v) = %v, want %v in test case %s", tc.tMatrix, tc.vector, result, tc.expected, tc.name)
			}
		})
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
		{[2]int{1, 0}, 1, [2]int{0, -1}},
		{[2]int{1, 0}, 2, [2]int{-1, 0}},
		{[2]int{1, 0}, 3, [2]int{0, 1}},
	}
	for caseIndex, testCase := range testCases {
		t.Run("RotateVector", func(t *testing.T) {
			x, y := RotateVector(testCase.index, testCase.Vector[0], testCase.Vector[1], 100, 100)
			if x != testCase.expectedVector[0] || y != testCase.expectedVector[1] {
				t.Error("Expected ", testCase.expectedVector, " got ", [2]int{x, y}, " for case ", caseIndex)
			}
		})
	}
}

func TestRotateVector2(t *testing.T) {
	testCases := []struct {
		index      int
		vector     [2]int
		canvasSize int
		expected   [2]int
		name       string
	}{
		{0, [2]int{1, 1}, 2, [2]int{1, 1}, "0 degrees rotation"},
		{1, [2]int{1, 1}, 2, [2]int{1, -1}, "90 degrees rotation"},
		{2, [2]int{1, 1}, 2, [2]int{-1, -1}, "180 degrees rotation"},
		{3, [2]int{1, 1}, 2, [2]int{-1, 1}, "270 degrees rotation"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			x, y := RotateVector(tc.index, tc.vector[0], tc.vector[1], tc.canvasSize, tc.canvasSize)
			result := [2]int{x, y}
			if result != tc.expected {
				t.Errorf("RotateVector(%d, %v, %d, %d) = %v, want %v on test case %s",
					tc.index, tc.vector, tc.canvasSize, tc.canvasSize, result, tc.expected, tc.name)
			}
		})
	}
}
func TestTransformMatrix(t *testing.T) {
	matrix := [2][2]int{{1, 2}, {3, 4}}
	tMatrix := [2][2]int{{2, 0}, {0, 2}}
	expected := [2][2]int{{2, 4}, {6, 8}}
	result := TransformMatrix(matrix, tMatrix)
	for i := range matrix {
		for j := range matrix[i] {
			if result[i][j] != expected[i][j] {
				t.Errorf("TransformMatrix(%v, %v) = %v, want %v", matrix, tMatrix, result, expected)
			}
		}
	}
}

func TestSubtractVectors(t *testing.T) {
	testCases := []struct {
		vectorA  [2]int
		vectorB  [2]int
		expected [2]int
		name     string
	}{
		{[2]int{1, 1}, [2]int{1, 1}, [2]int{0, 0}, "Subtracting same vectors"},
		{[2]int{1, 2}, [2]int{2, 3}, [2]int{-1, -1}, "Subtracting different vectors"},
		{[2]int{-1, -1}, [2]int{1, 1}, [2]int{-2, -2}, "Subtracting negative vectors"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := SubtractVectors(tc.vectorA, tc.vectorB)
			if result != tc.expected {
				t.Errorf("SubtractVectors(%v, %v) = %v, want %v", tc.vectorA, tc.vectorB, result, tc.expected)
			}
		})
	}
}

func TestSumVectors(t *testing.T) {
	testCases := []struct {
		vectorA  [2]int
		vectorB  [2]int
		expected [2]int
		name     string
	}{
		{[2]int{1, 1}, [2]int{1, 1}, [2]int{2, 2}, "Summing same vectors"},
		{[2]int{1, 2}, [2]int{2, 3}, [2]int{3, 5}, "Summing different vectors"},
		{[2]int{-1, -1}, [2]int{1, 1}, [2]int{0, 0}, "Summing negative vectors"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := SumVectors(tc.vectorA, tc.vectorB)
			if result != tc.expected {
				t.Errorf("SumVectors(%v, %v) = %v, want %v", tc.vectorA, tc.vectorB, result, tc.expected)
			}
		})
	}
}

func TestMultiplyVectorByScalar(t *testing.T) {

	type MultiplyVectorByScalarTestCase struct {
		vectorA  [2]int
		scalar   int
		expected [2]int
	}

	testCases := []MultiplyVectorByScalarTestCase{
		{[2]int{1, 1}, 2, [2]int{2, 2}},
		{[2]int{-1, -1}, 2, [2]int{-2, -2}},
		{[2]int{1, 2}, -1, [2]int{-1, -2}},
		{[2]int{0, 0}, 2, [2]int{0, 0}},
	}

	for _, test := range testCases {
		t.Run("MultiplyScalar", func(t *testing.T) {
			result := MultiplyVectorByScalar(test.vectorA, test.scalar)
			if result != test.expected {
				t.Errorf("Expected %v for vector %v multiplied by scalar %d, got %v", test.expected, test.vectorA, test.scalar, result)
			}
		})
	}
}

func TestDotProduct(t *testing.T) {
	type DotProductTestCase struct {
		vectorA  []int
		vectorB  []int
		expected int
		panics   bool
	}

	testCases := []DotProductTestCase{
		{[]int{1, 2}, []int{2, 3}, 8, false},
		{[]int{1, 1}, []int{1, 1}, 2, false},
		{[]int{-1, -2}, []int{2, -1}, 0, false},
		{[]int{0, 0, 0}, []int{0, 0, 0}, 0, false},
		{[]int{0, 1, 2}, []int{1, 2, 3}, 8, false},
		{[]int{1, 2}, []int{2}, 0, true},
		{[]int{}, []int{2, 3}, 0, true},
		{[]int{}, []int{}, 0, true},
	}

	for _, test := range testCases {
		t.Run("DotProduct", func(t *testing.T) {
			if test.panics {
				panics, _ := AssertPanics(t, func() { DotProduct(test.vectorA, test.vectorB) }, "")
				if !panics {
					t.Errorf("Expected panic for vectors %v and %v", test.vectorA, test.vectorB)
				}
			} else {
				result := DotProduct(test.vectorA, test.vectorB)
				if result != test.expected {
					t.Errorf("Expected %v for vectors %v and %v, got %v", test.expected, test.vectorA, test.vectorB, result)
				}
			}
		})
	}
}

func TestEqual(t *testing.T) {
	type EqualTestCase struct {
		a        []int
		b        []int
		expected bool
	}
	testCases := []EqualTestCase{
		{[]int{1, 2, 3}, []int{1, 2, 3}, true},
		{[]int{-1, 2, 1}, []int{4, -1, 2}, false},
		{[]int{3, 1, 2}, []int{3, 1, 2}, true},
		{[]int{1, 1, 1}, []int{1, 1, 1, 1}, false},
	}

	for _, test := range testCases {
		t.Run("Equal", func(t *testing.T) {
			result := Equal(test.a, test.b)
			if result != test.expected {
				t.Errorf("Expected %v for vectors %v and %v, got %v", test.expected, test.a, test.b, result)
			}
		})
	}
}

func TestCrossProduct(t *testing.T) {
	type CrossProductTestCase struct {
		vectorA  []int
		vectorB  []int
		expected []int
		panics   bool
	}

	testCases := []CrossProductTestCase{
		{[]int{1, 0, 0}, []int{0, 1, 0}, []int{0, 0, 1}, false},
		{[]int{0, 1, 0}, []int{0, 0, 1}, []int{1, 0, 0}, false},
		{[]int{0, 0, 1}, []int{1, 0, 0}, []int{0, 1, 0}, false},
		{[]int{1, 2, 3}, []int{4, 5}, nil, true},
		{[]int{}, []int{}, nil, true},
	}

	for _, test := range testCases {
		t.Run("CrossProduct", func(t *testing.T) {
			if test.panics {
				panics, _ := AssertPanics(t, func() { CrossProduct(test.vectorA, test.vectorB) }, "")
				if !panics {
					t.Errorf("Expected panic for vectors %v and %v", test.vectorA, test.vectorB)
				}
			} else {
				result := CrossProduct(test.vectorA, test.vectorB)
				if !Equal(result, test.expected) {
					t.Errorf("Expected %v for vectors %v and %v, got %v", test.expected, test.vectorA, test.vectorB, result)
				}
			}
		})
	}
}

func TestSwapVectorCoordinates(t *testing.T) {

	type SwapVectorCoordinatesTestCase struct {
		vector   [2]int
		expected [2]int
	}

	testCases := []SwapVectorCoordinatesTestCase{
		{[2]int{1, 2}, [2]int{2, 1}},
		{[2]int{-1, 2}, [2]int{2, -1}},
		{[2]int{3, 0}, [2]int{0, 3}},
		{[2]int{-1, -1}, [2]int{-1, -1}},
	}

	for _, test := range testCases {
		t.Run("SwapCoords", func(t *testing.T) {
			result := SwapVectorCoordinates(test.vector)
			if result != test.expected {
				t.Errorf("Expected %v for vector %v, got %v", test.expected, test.vector, result)
			}
		})
	}
}
