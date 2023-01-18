package utils

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"
)

// DEV Matrix
func NewMatrixesOfRotation() [4][2][2]int {
	return [4][2][2]int{
		{{1, 0}, {0, 1}},
		{{0, 1}, {-1, 0}},
		{{-1, 0}, {0, -1}},
		{{0, -1}, {1, 0}},
	}
}

// DEV Matrix
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

// DEV Matrix
var MatrixesOfRotation = NewMatrixesOfRotation()

// DEV Vector
func TransformVector(tMatrix [2][2]int, x int, y int) (int, int) {
	return tMatrix[0][0]*x + tMatrix[0][1]*y, tMatrix[1][0]*x + tMatrix[1][1]*y
}

// DEV Vector
func RotateVector(index int, x int, y int, canvasWidth int, canvasHeight int) (int, int) {
	return TransformVector(MatrixesOfRotation[index], x, y)
}

// DEV Vector
func NewPositiveRandomVector(vectorMaxLen int) [2]int {
	maxCoordinateSize := int(math.Max(float64(vectorMaxLen)/(2*math.Sqrt(2)), 1.0))
	x := rand.Intn(maxCoordinateSize)
	rand.Seed(time.Now().UnixNano())
	y := rand.Intn(maxCoordinateSize)

	return [2]int{x, y}
}

// DEV Vector
func NewRandomVector(vectorMaxLen int) [2]int {
	maxCoordinateSize := int((math.Max(float64(vectorMaxLen)/2*math.Sqrt(2), 1.0)))
	x := rand.Intn(maxCoordinateSize)*2 - maxCoordinateSize
	rand.Seed(time.Now().UnixNano())
	y := rand.Intn(maxCoordinateSize)*2 - maxCoordinateSize
	return [2]int{x, y}
}

// DEV Vector
func CheckPointWithinBounds(x int, y int, topSide [2]int, bottomOppositeSide [2]int) bool {
	return x >= topSide[0] && x <= bottomOppositeSide[0] && y >= topSide[1] && y <= bottomOppositeSide[1]
}

// DEV Vector
func SubtractVectors(vectorA [2]int, vectorB [2]int) [2]int {
	return [2]int{vectorA[0] - vectorB[0], vectorA[1] - vectorB[1]}
}

// DEV Vector
func SumVectors(vectorA [2]int, vectorB [2]int) [2]int {
	return [2]int{vectorA[0] + vectorB[0], vectorA[1] + vectorB[1]}
}

// DEV Vector
func MultiplyVectorByScalar(vectorA [2]int, scalar int) [2]int {
	return [2]int{vectorA[0] * scalar, vectorA[1] * scalar}
}

// DEV Vector
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

// DEV Vector
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

// DEV Vector
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

// DEV Vector
func SwapVectorCoordinates(vector [2]int) [2]int {
	return [2]int{vector[1], vector[0]}
}

// DEV Vector
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

// DEV Vector
func Distance(x1, y1, x2, y2 int) float64 {
	deltaX := x2 - x1
	deltaY := y2 - y1

	return math.Sqrt(math.Pow(float64(deltaX), 2) + math.Pow(float64(deltaY), 2))
}

// DEV Number
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

// DEV Number
func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// DEV string
func DirectionFromString(direction string) string {
	if direction == "ArrowLeft" {
		return "left"
	} else if direction == "ArrowRight" {
		return "right"
	}
	return ""
}

// DEV color
func NewRandomColor() [3]int {
	return [3]int{rand.Intn(255), rand.Intn(255), rand.Intn(255)}
}

func AssertPanics(t *testing.T, f func(), message string) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf(fmt.Sprint("The code did not panic", message))
		}
	}()
	f()
}
