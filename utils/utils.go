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
