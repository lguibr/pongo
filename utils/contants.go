package utils

import (
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
