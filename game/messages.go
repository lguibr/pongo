// File: game/messages.go
package game

import (
	"io"
	"net"
	"time"
	// "golang.org/x/net/websocket" // No longer needed here
)

// PlayerConnection defines the interface needed by GameActor for a player connection.
// This allows using real websockets or mocks in tests.
type PlayerConnection interface {
	io.ReadWriteCloser // Includes Read, Write, Close methods
	RemoteAddr() net.Addr
}

// --- Messages TO GameActor ---

// PlayerConnectRequest signals a new player trying to connect.
type PlayerConnectRequest struct {
	WsConn PlayerConnection // Use the interface type
}

// PlayerDisconnect signals a player has disconnected.
// Can be triggered by write error (index known) or read error (conn known).
type PlayerDisconnect struct {
	PlayerIndex int              // -1 if triggered by readLoop/unknown index
	WsConn      PlayerConnection // Connection that disconnected (used if index is -1)
}

// ForwardedPaddleDirection carries direction input from a connection handler.
type ForwardedPaddleDirection struct {
	// PlayerIndex int // Removed - GameActor finds index via WsConn
	WsConn    PlayerConnection // The connection sending the direction
	Direction []byte           // Raw JSON bytes {"direction": "Arrow..."}
}

// DestroyExpiredBall is sent by the ball expiry timer to the GameActor.
type DestroyExpiredBall struct {
	BallID int
}

// --- Messages FROM GameActor ---

// GameStateUpdate sends the current game state to a connection handler/actor.
type GameStateUpdate struct {
	StateJSON []byte
}

// AssignPlayerIndex tells a connection handler which player index it got.
// DEPRECATED - Server no longer tracks index directly.
// type AssignPlayerIndex struct {
// 	Index int
// }

// RejectConnection tells a handler the connection was rejected (e.g., server full).
// DEPRECATED - GameActor closes connection directly.
// type RejectConnection struct {
// 	Reason string
// }

// --- Messages Between GameActor and Child Actors ---

// (PaddlePositionMessage, BallPositionMessage are already defined in paddle.go/ball.go)

// --- Commands TO BallActor ---

// SetPhasingCommand tells the ball actor to start phasing.
type SetPhasingCommand struct {
	ExpireIn time.Duration // Duration, not int seconds
}

// IncreaseVelocityCommand tells the ball to increase velocity.
type IncreaseVelocityCommand struct {
	Ratio float64
}

// IncreaseMassCommand tells the ball to increase mass.
type IncreaseMassCommand struct {
	Additional int
}

// ReflectVelocityCommand tells the ball to reflect velocity along an axis.
type ReflectVelocityCommand struct {
	Axis string // "X" or "Y"
}

// SetVelocityCommand tells the ball to set its velocity directly.
type SetVelocityCommand struct {
	Vx int
	Vy int
}

// DestroyBallCommand tells the ball actor it's being destroyed (e.g., out of bounds).
type DestroyBallCommand struct{}

// --- Commands TO PaddleActor ---
// (PaddleDirectionMessage is defined in paddle.go, but payload comes via ForwardedPaddleDirection)

// --- Internal Actor Messages ---

// GameTick triggers internal game logic update in GameActor.
type GameTick struct{}

// internalTick triggers internal logic update in BallActor and PaddleActor.
type internalTick struct{}

// CheckCollisions triggers collision detection (potentially internal to GameActor).
// DEPRECATED - Handled within GameTick
// type CheckCollisions struct{}

// SpawnBallCommand tells GameActor to spawn a new ball (e.g., from powerup)
type SpawnBallCommand struct {
	OwnerIndex int
	X, Y       int // Optional initial position override
	ExpireIn   time.Duration
}
