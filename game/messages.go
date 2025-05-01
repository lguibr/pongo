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

// PositionUpdateMessage is sent by child actors (Ball, Paddle) to GameActor
// after their position/state is updated.
type PositionUpdateMessage struct {
	PID      *bollywood.PID // PID of the sender (BallActor or PaddleActor)
	ActorID  int            // Specific ID (BallID or PaddleIndex)
	IsPaddle bool           // True if sender is PaddleActor, false if BallActor
	X        int
	Y        int
	Vx       int
	Vy       int
	// Ball specific
	Radius  int
	Phasing bool
	// Paddle specific
	Width    int
	Height   int
	IsMoving bool
}

// --- Messages Between GameActor and Child Actors ---
type UpdatePositionCommand struct{}

// GetPositionRequest might still be useful for debugging or specific scenarios, keep for now.
type GetPositionRequest struct{}

// PositionResponse simplified, potentially deprecated if GetPositionRequest is removed later.
type PositionResponse struct {
	X int
	Y int
	// Other fields removed as they are now sent via PositionUpdateMessage
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

// GameOverMessage is sent by GameActor via BroadcasterActor when the game ends.
type GameOverMessage struct {
	WinnerIndex int      `json:"winnerIndex"` // Index of the winning player (-1 if draw/no winner)
	FinalScores [4]int32 `json:"finalScores"` // Final scores of all players
	Reason      string   `json:"reason"`      // e.g., "All bricks destroyed"
	RoomPID     string   `json:"roomPid"`     // PID of the game room that ended
}

// --- Specific Message TO Client ---

// PlayerAssignmentMessage informs a client of their assigned index.
// Sent directly from GameActor to the specific client's WebSocket upon connection.
type PlayerAssignmentMessage struct {
	PlayerIndex int `json:"playerIndex"`
}

// --- Internal Actor Messages ---

type GameTick struct{}      // Sent to GameActor by its physics ticker
type BroadcastTick struct{} // Sent to GameActor by its broadcast ticker

// --- Internal Connection Handler Messages ---
// Exported types for use in server package
type InternalReadLoopMsg struct{ Payload []byte }
