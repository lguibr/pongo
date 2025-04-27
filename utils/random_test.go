// File: utils/random_test.go
package utils

import (
	"math"
	"testing"
)

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
		t.Run(tc.name, func(t *testing.T) {
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
		})
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
		t.Run("RandomNumber", func(t *testing.T) {
			result := RandomNumber(test.amplitude)
			if result < test.expectedMin || result > test.expectedMax {
				t.Errorf("Expected random number between %d and %d for amplitude %d, got %d", test.expectedMin, test.expectedMax, test.amplitude, result)
			}
		})
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
		t.Run("RandomNumberN", func(t *testing.T) {
			for i := 0; i < 100; i++ {
				// Call the function and save the result
				result := RandomNumberN(test.amplitude)

				// Check that the result is within the expected range
				if result < test.min || result > test.max {
					t.Errorf("Expected a number between %d and %d, got %d", test.min, test.max, result)
				}
				if result == 0 {
					t.Errorf("Expected a non-zero number, got %d", result)
				}
			}
		})
	}
}
