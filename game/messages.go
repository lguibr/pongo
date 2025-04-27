// File: game/messages.go
package game

import (
	"time"

	"golang.org/x/net/websocket" // Import websocket
)

// --- Messages TO GameActor ---

type PlayerConnectRequest struct {
	WsConn *websocket.Conn
}

type PlayerDisconnect struct {
	PlayerIndex int
	WsConn      *websocket.Conn
}

type ForwardedPaddleDirection struct {
	WsConn    *websocket.Conn
	Direction []byte
}

type DestroyExpiredBall struct {
	BallID int
}

// --- Messages Between GameActor and Child Actors ---

// (PaddlePositionMessage, BallPositionMessage defined in paddle.go/ball.go)

// --- Commands TO BallActor ---

type SetPhasingCommand struct {
	ExpireIn time.Duration // DEPRECATED: Duration now comes from config in GameActor physics
}

type IncreaseVelocityCommand struct {
	Ratio float64 // DEPRECATED: Ratio now comes from config in GameActor physics
}

type IncreaseMassCommand struct {
	Additional int // DEPRECATED: Amount now comes from config in GameActor physics
}

type ReflectVelocityCommand struct {
	Axis string // "X" or "Y"
}

type SetVelocityCommand struct {
	Vx int
	Vy int
}

type DestroyBallCommand struct{}

// --- Commands TO PaddleActor ---

// (PaddleDirectionMessage defined in paddle.go)

// --- Internal Actor Messages ---

type GameTick struct{}

type internalTick struct{}

// SpawnBallCommand tells GameActor to spawn a new ball
type SpawnBallCommand struct {
	OwnerIndex  int
	X, Y        int
	ExpireIn    time.Duration // Average duration, will be randomized in handler
	IsPermanent bool          // Add flag to indicate if the ball should be permanent
}
