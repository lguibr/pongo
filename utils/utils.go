// File: utils/utils.go
package utils

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"testing"
	"time"
)

// Matrix operations
func NewMatrixesOfRotation() [4][2][2]int {
	return [4][2][2]int{
		{{1, 0}, {0, 1}},
		{{0, 1}, {-1, 0}},
		{{-1, 0}, {0, -1}},
		{{0, -1}, {1, 0}},
	}
}

func TransformMatrix(matrix [2][2]int, tMatrix [2][2]int) [2][2]int {
	var transformedMatrix [2][2]int
	for i := range matrix {
		var vector [2]int
		x, y := TransformVector(tMatrix, matrix[i][0], matrix[i][1])
		vector = [2]int{x, y}
		transformedMatrix[i] = vector
	}
	return transformedMatrix
}

var MatrixesOfRotation = NewMatrixesOfRotation()

// Vector operations
func TransformVector(tMatrix [2][2]int, x int, y int) (int, int) {
	return tMatrix[0][0]*x + tMatrix[0][1]*y, tMatrix[1][0]*x + tMatrix[1][1]*y
}

func RotateVector(index int, x int, y int, canvasWidth int, canvasHeight int) (int, int) {
	return TransformVector(MatrixesOfRotation[index], x, y)
}

func NewPositiveRandomVector(vectorMaxLen int) [2]int {
	maxCoordinateSize := int(math.Max(float64(vectorMaxLen)/(2*math.Sqrt(2)), 1.0))
	x := rand.Intn(maxCoordinateSize)
	rand.Seed(time.Now().UnixNano())
	y := rand.Intn(maxCoordinateSize)

	return [2]int{x, y}
}

func NewRandomVector(vectorMaxLen int) [2]int {
	maxCoordinateSize := int((math.Max(float64(vectorMaxLen)/2*math.Sqrt(2), 1.0)))
	x := rand.Intn(maxCoordinateSize)*2 - maxCoordinateSize
	rand.Seed(time.Now().UnixNano())
	y := rand.Intn(maxCoordinateSize)*2 - maxCoordinateSize
	return [2]int{x, y}
}

func CheckPointWithinBounds(x int, y int, topSide [2]int, bottomOppositeSide [2]int) bool {
	return x >= topSide[0] && x <= bottomOppositeSide[0] && y >= topSide[1] && y <= bottomOppositeSide[1]
}

func SubtractVectors(vectorA [2]int, vectorB [2]int) [2]int {
	return [2]int{vectorA[0] - vectorB[0], vectorA[1] - vectorB[1]}
}

func SumVectors(vectorA [2]int, vectorB [2]int) [2]int {
	return [2]int{vectorA[0] + vectorB[0], vectorA[1] + vectorB[1]}
}

func MultiplyVectorByScalar(vectorA [2]int, scalar int) [2]int {
	return [2]int{vectorA[0] * scalar, vectorA[1] * scalar}
}

func DotProduct(vectorA, vectorB []int) int {
	if len(vectorA) != len(vectorB) || len(vectorA) == 0 {
		panic("vectors must have the same length")
	}
	var result int
	for i := range vectorA {
		result += vectorA[i] * vectorB[i]
	}
	return result
}

func Equal(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func CrossProduct(vectorA, vectorB []int) []int {
	if len(vectorA) != 3 || len(vectorB) != 3 {
		panic("vectors must have length 3")
	}
	return []int{
		vectorA[1]*vectorB[2] - vectorA[2]*vectorB[1],
		vectorA[2]*vectorB[0] - vectorA[0]*vectorB[2],
		vectorA[0]*vectorB[1] - vectorA[1]*vectorB[0],
	}
}

func SwapVectorCoordinates(vector [2]int) [2]int {
	return [2]int{vector[1], vector[0]}
}

func NewRandomPositiveVectors(numberOfVectors, maxVectorSize int) [][2]int {
	seedVectors := make([][2]int, numberOfVectors)
	for index := range seedVectors {
		currentLength := rand.Intn(maxVectorSize)
		if currentLength == 0 || currentLength > maxVectorSize {
			currentLength = maxVectorSize
		}
		seedVectors[index] = NewPositiveRandomVector(currentLength)
	}
	return seedVectors
}

func Distance(x1, y1, x2, y2 int) float64 {
	deltaX := x2 - x1
	deltaY := y2 - y1

	return math.Sqrt(math.Pow(float64(deltaX), 2) + math.Pow(float64(deltaY), 2))
}

// Number operations
func RandomNumber(amplitude int) int {
	return rand.Intn(amplitude*2) - amplitude
}

var randomNumberN func(amplitude int) int

func RandomNumberN(amplitude int) int {
	randomNumberN = func(amplitude int) int {
		value := rand.Intn(amplitude*2) - amplitude
		if value == 0 {
			value = RandomNumberN(amplitude)
		}
		return value
	}
	return randomNumberN(amplitude)
}

func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// MaxInt returns the greater of two integers.
func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// MinInt returns the smaller of two integers.
func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// String conversion
// DirectionFromString converts frontend direction strings ("ArrowLeft", "ArrowRight", "Stop")
// to internal representations ("left", "right", "").
func DirectionFromString(direction string) string {
	switch direction {
	case "ArrowLeft":
		return "left"
	case "ArrowRight":
		return "right"
	case "Stop": // Explicitly handle "Stop"
		return "" // Map "Stop" to empty string to halt movement
	default:
		return "" // Default to empty string (no movement) for unknown inputs
	}
}

// Color generation
func NewRandomColor() [3]int {
	return [3]int{rand.Intn(255), rand.Intn(255), rand.Intn(255)}
}

// Testing helpers
func AssertPanics(t *testing.T, testingFunction func(), message string) (panics bool, errorMessage string) {

	panics = false
	errorMessage = ""

	// Define the defer function
	deferFunc := func() {
		if r := recover(); r != nil {
			panics = true
			// Try to convert recover() result to string
			switch v := r.(type) {
			case string:
				errorMessage = v
			case error:
				errorMessage = v.Error()
			default:
				errorMessage = fmt.Sprintf("%v", v)
			}
		}
	}

	// Anonymous function to execute the test function with the defer
	func() {
		defer deferFunc() // Correct: Call the defer function
		testingFunction()
	}()

	return panics, errorMessage
}

// Logging helpers
type JSONable interface {
	ToJson() []byte
}

func JsonLogger(filePath string, data interface{}) error {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(data); err != nil {
		return err
	}

	return nil
}

func Logger(filePath string, data string) error {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()
	if _, err := file.Write([]byte(data)); err != nil {
		return fmt.Errorf("failed to write to log file: %w", err)
	}

	return nil
}
