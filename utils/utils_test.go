package utils

import (
	"fmt"
	"math"
	"testing"
)

func TestDirectionFromString(t *testing.T) {
	testCases := map[string]string{
		"ArrowLeft":  "left",
		"ArrowRight": "right",
		"ArrowUp":    "",
		"":           "",
	}

	for input, expected := range testCases {
		result := DirectionFromString(input)
		if result != expected {
			t.Errorf("DirectionFromString(%s) = %s, want %s", input, result, expected)
		}
	}
}

func TestNewRandomColor(t *testing.T) {
	// Test that all elements of the returned array are between 0 and 255 inclusive
	for i := 0; i < 100; i++ {
		color := NewRandomColor()
		for i := range color {
			if color[i] < 0 || color[i] > 255 {
				t.Errorf("NewRandomColor() returned an invalid color value: %d", color[i])
			}
		}
	}
}

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
		x, y := TransformVector(tc.tMatrix, tc.vector[0], tc.vector[1])
		result := [2]int{x, y}
		if result != tc.expected {
			t.Errorf("TransformVector(%v, %v) = %v, want %v in test case %s", tc.tMatrix, tc.vector, result, tc.expected, tc.name)
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
		{[2]int{1, 0}, 1, [2]int{0, -1}},
		{[2]int{1, 0}, 2, [2]int{-1, 0}},
		{[2]int{1, 0}, 3, [2]int{0, 1}},
	}
	for caseIndex, testCase := range testCases {
		x, y := RotateVector(testCase.index, testCase.Vector[0], testCase.Vector[1], 100, 100)
		if x != testCase.expectedVector[0] || y != testCase.expectedVector[1] {
			t.Error("Expected ", testCase.expectedVector, " got ", [2]int{x, y}, " for case ", caseIndex)
		}
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
		x, y := RotateVector(tc.index, tc.vector[0], tc.vector[1], tc.canvasSize, tc.canvasSize)
		result := [2]int{x, y}
		if result != tc.expected {
			t.Errorf("RotateVector(%d, %v, %d, %d) = %v, want %v on test case %s",
				tc.index, tc.vector, tc.canvasSize, tc.canvasSize, result, tc.expected, tc.name)
		}
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

func TestNewPositiveRandomVector(t *testing.T) {
	size := 10
	vector := NewPositiveRandomVector(size)
	if vector[0] < 0 || vector[1] < 0 {
		t.Errorf("NewPositiveRandomVector(%d) = %v, want positive values", size, vector)
	}
}

func TestNewRandomVector(t *testing.T) {
	size := 10
	// Call the function multiple times and check if the returned vector is within bounds
	for i := 0; i < 100; i++ {
		vector := NewRandomVector(size)
		if math.Abs(float64(vector[0])) > float64(size) || math.Abs(float64(vector[1])) > float64(size) {
			t.Errorf("Expected vector to be within bounds, got %v", vector)
		}
	}
}

func TestCheckPointWithinBounds(t *testing.T) {
	type CheckPointWithinBoundsTestCase struct {
		x                  int
		y                  int
		topSide            [2]int
		bottomOppositeSide [2]int
		expected           bool
	}

	testCases := []CheckPointWithinBoundsTestCase{
		{5, 5, [2]int{0, 0}, [2]int{10, 10}, true},
		{15, 15, [2]int{0, 0}, [2]int{10, 10}, false},
		{-5, -5, [2]int{-10, -10}, [2]int{0, 0}, true},
		{0, 0, [2]int{-10, -10}, [2]int{0, 0}, true},
	}

	for _, test := range testCases {
		result := CheckPointWithinBounds(test.x, test.y, test.topSide, test.bottomOppositeSide)
		if result != test.expected {
			t.Errorf("Expected %v for point %d,%d within bounds %v,%v, got %v", test.expected, test.x, test.y, test.topSide, test.bottomOppositeSide, result)
		}
	}
}

func TestAbs(t *testing.T) {
	testCases := []struct {
		x        int
		expected int
		name     string
	}{
		{1, 1, "Positive value"},
		{-1, 1, "Negative value"},
		{0, 0, "Zero value"},
	}
	for _, tc := range testCases {
		result := Abs(tc.x)
		if result != tc.expected {
			t.Errorf("Abs(%d) = %d, want %d", tc.x, result, tc.expected)
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
		result := SubtractVectors(tc.vectorA, tc.vectorB)
		if result != tc.expected {
			t.Errorf("SubtractVectors(%v, %v) = %v, want %v", tc.vectorA, tc.vectorB, result, tc.expected)
		}
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
		result := SumVectors(tc.vectorA, tc.vectorB)
		if result != tc.expected {
			t.Errorf("SumVectors(%v, %v) = %v, want %v", tc.vectorA, tc.vectorB, result, tc.expected)
		}
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
		result := MultiplyVectorByScalar(test.vectorA, test.scalar)
		if result != test.expected {
			t.Errorf("Expected %v for vector %v multiplied by scalar %d, got %v", test.expected, test.vectorA, test.scalar, result)
		}
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
		result := Equal(test.a, test.b)
		if result != test.expected {
			t.Errorf("Expected %v for vectors %v and %v, got %v", test.expected, test.a, test.b, result)
		}
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
		result := SwapVectorCoordinates(test.vector)
		if result != test.expected {
			t.Errorf("Expected %v for vector %v, got %v", test.expected, test.vector, result)
		}
	}
}

func TestNewRandomPositiveVectors(t *testing.T) {
	testCases := []struct {
		n      int
		size   int
		panics bool
		name   string
	}{
		{3, 10, false, "3 positive random vectors of size 10"},
		{5, 20, false, "5 positive random vectors of size 20"},
		{2, 5, false, "2 positive random vectors of size 5"},
		{100, 500, false, "100 positive random vectors of size 500"},
		{1, 0, true, "100 positive random vectors of size 0 should panics"},
		{0, 0, false, "0 positive random vectors of size 0 should panics"},
	}
	for _, tc := range testCases {
		if tc.panics {
			panics, err := AssertPanics(t, func() { NewRandomPositiveVectors(tc.n, tc.size) }, "")
			if !panics {
				t.Errorf("Expected panic for %s, got %v", tc.name, err)
			}
		} else {

			result := NewRandomPositiveVectors(tc.n, tc.size)
			if len(result) != tc.n {
				t.Errorf("NewRandomPositiveVectors(%d, %d) = %v, want %d vectors", tc.n, tc.size, result, tc.n)
			}

			for _, vector := range result {
				if vector[0] < 0 || vector[1] < 0 {
					t.Errorf("NewRandomPositiveVectors(%d, %d) = %v, want positive values", tc.n, tc.size, result)
					break
				}
			}
		}
	}
}

func TestDistance(t *testing.T) {
	type DistanceTestCase struct {
		x1, y1, x2, y2 int
		expected       float64
	}

	testCases := []DistanceTestCase{
		{1, 1, 2, 2, 1.4142135623730951},
		{-1, 2, 4, -1, 5.830951894845301},
		{0, 0, 3, 4, 5.0},
		{-2, -3, 2, 4, 8.06225774829855},
		{-2, -3, -2, -3, 0},
		{1, 1, 1, 1, 0},
	}

	for _, test := range testCases {
		result := Distance(test.x1, test.y1, test.x2, test.y2)
		if result != test.expected {
			t.Errorf("Expected %v for point1(%d,%d) and point2(%d,%d), got %v", test.expected, test.x1, test.y1, test.x2, test.y2, result)
		}
	}
}

func TestRandomNumber(t *testing.T) {
	type RandomNumberTestCase struct {
		amplitude   int
		expectedMin int
		expectedMax int
	}

	testCases := []RandomNumberTestCase{
		{10, -10, 10},
		{5, -5, 5},
		{7, -7, 7},
	}

	for _, test := range testCases {
		result := RandomNumber(test.amplitude)
		if result < test.expectedMin || result > test.expectedMax {
			t.Errorf("Expected random number between %d and %d for amplitude %d, got %d", test.expectedMin, test.expectedMax, test.amplitude, result)
		}
	}
}

func TestRandomNumberN(t *testing.T) {
	// Set up test cases
	testCases := []struct {
		amplitude int
		min       int
		max       int
	}{
		{1, -1, 1},
		{2, -2, 2},
		{3, -3, 3},
	}

	// Iterate over test cases
	for _, test := range testCases {
		for i := 0; i < 100; i++ {
			// Call the function and save the result
			result := RandomNumberN(test.amplitude)

			// Check that the result is within the expected range
			if result < test.min || result > test.max {
				t.Errorf("Expected a number between %d and %d, got %d", test.min, test.max, result)
			}
		}
	}
}
func TestAssertPanics(t *testing.T) {
	t.Run("Panicking function", func(t *testing.T) {
		// Function that is expected to panic
		shouldPanic := func() { panic("Panic occurred") }
		// Call our AssertPanics function with the above function
		panics, err := AssertPanics(t, shouldPanic, " - PosMessage")
		if !panics {
			t.Errorf("Expected panic, got %v", err)
		}
	})
	t.Run("Non-panicking function", func(t *testing.T) {
		// Function that is NOT expected to panic
		shouldNotPanic := func() { fmt.Println("Hello, world") }
		// Call our AssertPanics function with the above function
		// and wrap it with a defer function to catch a panic if it happens
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("Recovered:", r)
			}
		}()
		panics, err := AssertPanics(t, shouldNotPanic, "Hello, world")
		if panics {
			t.Errorf("Expected no panic, got %v", err)
		}
	})
}
