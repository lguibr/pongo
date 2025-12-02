package game

import (
	"github.com/lguibr/bollywood"
)

// RoomCleanupTimeout is an internal message sent when the empty room grace period expires.
type RoomCleanupTimeout struct{}

// PlayerLeftRoom is sent by GameActor to RoomManager when a player disconnects.
type PlayerLeftRoom struct {
	RoomPID *bollywood.PID
}
