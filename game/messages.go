// File: game/messages.go
package game

import (
	"time"

	"github.com/lguibr/bollywood"
	"golang.org/x/net/websocket"
)

// --- Messages TO/FROM RoomManager ---
type FindRoomRequest struct {
	ReplyTo *bollywood.PID
}
type AssignRoomResponse struct {
	RoomPID *bollywood.PID
}
type GameRoomEmpty struct {
	RoomPID *bollywood.PID
}
type GetRoomListRequest struct {
	// ReplyTo is implicit when using Ask
}
type RoomListResponse struct {
	Rooms map[string]int // Key is Room PID string
}

// --- Messages TO GameActor ---
type AssignPlayerToRoom struct {
	WsConn *websocket.Conn
}
type PlayerDisconnect struct {
	WsConn *websocket.Conn
}
type ForwardedPaddleDirection struct {
	WsConn    *websocket.Conn
	Direction []byte
}
type DestroyExpiredBall struct {
	BallID int
}
type SpawnBallCommand struct {
	OwnerIndex  int
	X, Y        int
	ExpireIn    time.Duration
	IsPermanent bool
}

// --- Messages Between GameActor and Child Actors ---
type UpdatePositionCommand struct{}
type GetPositionRequest struct{}
type PositionResponse struct {
	X        int
	Y        int
	Vx       int
	Vy       int
	Radius   int
	Width    int
	Height   int
	Phasing  bool
	IsMoving bool
}

// --- Commands TO BallActor ---
type SetPhasingCommand struct{}
type IncreaseVelocityCommand struct{ Ratio float64 }
type IncreaseMassCommand struct{ Additional int }
type ReflectVelocityCommand struct{ Axis string }
type SetVelocityCommand struct{ Vx, Vy int }
type DestroyBallCommand struct{}

// --- Commands TO PaddleActor ---
type PaddleDirectionMessage struct{ Direction []byte }

// --- Messages TO/FROM BroadcasterActor ---

// AddClient tells the BroadcasterActor to start sending updates to a connection.
type AddClient struct {
	Conn *websocket.Conn
}

// RemoveClient tells the BroadcasterActor to stop sending updates to a connection.
type RemoveClient struct {
	Conn *websocket.Conn
}

// BroadcastStateCommand carries the state snapshot to be broadcasted.
type BroadcastStateCommand struct {
	State GameState // Changed from StateJSON []byte
}

// --- Internal Actor Messages ---

type GameTick struct{}      // Sent to GameActor by its physics ticker
type BroadcastTick struct{} // Sent to GameActor by its broadcast ticker

// --- Internal Connection Handler Messages ---
// Exported types for use in server package
type InternalReadLoopMsg struct{ Payload []byte }
