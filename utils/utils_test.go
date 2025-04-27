// File: utils/utils_test.go
package utils

import (
	"fmt"
	"testing"
)

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
		t.Run(tc.name, func(t *testing.T) {
			result := Abs(tc.x)
			if result != tc.expected {
				t.Errorf("Abs(%d) = %d, want %d", tc.x, result, tc.expected)
			}
		})
	}
}

func TestDirectionFromString(t *testing.T) {
	testCases := map[string]string{
		"ArrowLeft":  "left",
		"ArrowRight": "right",
		"ArrowUp":    "",
		"":           "",
	}

	for input, expected := range testCases {
		t.Run("DirectionFromString_"+input, func(t *testing.T) {
			result := DirectionFromString(input)
			if result != expected {
				t.Errorf("DirectionFromString(%s) = %s, want %s", input, result, expected)
			}
		})
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

// Note: Tests for JSONLogger and Logger would require file system interaction
// and are often skipped in standard unit tests or handled with mocks/temp files.
