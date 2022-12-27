package utils

import (
	"math/rand"
	"time"
)

const Period = 50 * time.Millisecond

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
