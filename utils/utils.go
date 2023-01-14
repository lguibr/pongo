package utils

import (
	"math/rand"
	"time"
)

const (
	Period = 250 * time.Millisecond

	CanvasSize = 576 //INFO Must be divisible by GridSize
	GridSize   = 12  //INFO Must be divisible by 2

	CellSize    = CanvasSize / GridSize
	MinVelocity = CanvasSize / 200
	MaxVelocity = CanvasSize / 150

	NumberOfVectors       = 2 * GridSize / 3
	MaxVectorSize         = 2 * GridSize / 6
	NumberOfRandomWalkers = 2 * GridSize
	NumberOfRandomSteps   = 2 * GridSize

	BallSize     = CellSize / 3
	PaddleLength = CellSize * 3
	PaddleWeight = CellSize / 3
)

var CellTypes = map[string]string{
	"Empty": "Empty",
	"Brick": "Brick",
	"Block": "Block",
}

func DirectionFromString(direction string) string {
	if direction == "ArrowLeft" {
		return "left"
	} else if direction == "ArrowRight" {
		return "right"
	}
	return ""
}

func NewRandomColor() [3]int {
	return [3]int{rand.Intn(255), rand.Intn(255), rand.Intn(255)}
}

func NewMatrixesOfRotation() [4][2][2]int {
	return [4][2][2]int{
		{{1, 0}, {0, 1}},
		{{0, 1}, {-1, 0}},
		{{-1, 0}, {0, -1}},
		{{0, -1}, {1, 0}},
	}
}

func TransformVector(tMatrix [2][2]int, x int, y int) (int, int) {
	return tMatrix[0][0]*x + tMatrix[0][1]*y, tMatrix[1][0]*x + tMatrix[1][1]*y
}

func RotateVector(index int, x int, y int, canvasWidth int, canvasHeight int) (int, int) {
	return TransformVector(MatrixesOfRotation[index], x, y)
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

func NewPositiveRandomVector(size int) [2]int {
	x := rand.Intn(size)
	rand.Seed(time.Now().UnixNano())
	y := rand.Intn(size)

	return [2]int{x, y}
}
func NewRandomVector(size int) [2]int {
	x := rand.Intn(size)*2 - size
	rand.Seed(time.Now().UnixNano())
	y := rand.Intn(size)*2 - size
	return [2]int{x, y}
}

func CheckPointWithinBounds(x int, y int, topSide [2]int, bottomOppositeSide [2]int) bool {
	return x >= topSide[0] && x <= bottomOppositeSide[0] && y >= topSide[1] && y <= bottomOppositeSide[1]
}

func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
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
	if len(vectorA) != len(vectorB) {
		panic("vectors must have the same length")
	}
	var result int
	for i := range vectorA {
		result += vectorA[i] * vectorB[i]
	}
	return result
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

func MultiplyVectors(vectorA [2]int, vectorB [2]int) [2]int {
	return [2]int{vectorA[0] * vectorB[0], vectorA[1] * vectorB[1]}
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
