package utils

import (
	"math/rand"
	"time"
)

const (
	Period      = 20 * time.Millisecond
	CanvasSize  = 512
	GridSize    = 16
	MinVelocity = 2
	MaxVelocity = 8
)

func DirectionFromString(direction string) string {
	if direction == "ArrowLeft" {
		return "left"
	} else if direction == "ArrowRight" {
		return "right"
	}
	return ""
}

func CreateRandomColor() [3]int {
	return [3]int{rand.Intn(255), rand.Intn(255), rand.Intn(255)}
}

func CreateMatrixesOfRotation() [4][2][2]int {
	return [4][2][2]int{
		{{1, 0}, {0, 1}},
		{{0, -1}, {1, 0}},
		{{-1, 0}, {0, -1}},
		{{0, 1}, {-1, 0}},
	}
}

func TransformVector(tMatrix [2][2]int, x int, y int) (int, int) {
	return tMatrix[0][0]*x + tMatrix[0][1]*y, tMatrix[1][0]*x + tMatrix[1][1]*y
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

var MatrixesOfRotation = CreateMatrixesOfRotation()
