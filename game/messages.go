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

// PaddleStateUpdate is sent by PaddleActor to GameActor when its internal state changes due to a command.
type PaddleStateUpdate struct {
	PID       *bollywood.PID // PID of the sender (PaddleActor)
	Index     int            // Paddle Index (0-3)
	Direction string         // The new internal direction ("left", "right", "")
	// Position/Velocity/IsMoving are NOT sent, GameActor calculates these
}

// BallStateUpdate is sent by BallActor to GameActor when its internal state changes due to a command.
type BallStateUpdate struct {
	PID     *bollywood.PID // PID of the sender (BallActor)
	ID      int            // Ball ID
	Vx      int
	Vy      int
	Radius  int // Send radius in case it changed (e.g., IncreaseMass)
	Mass    int // Send mass in case it changed
	Phasing bool
	// Position is NOT sent, GameActor calculates this
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

// BroadcastStateCommand carries the dynamic state snapshot to be broadcasted.
// The grid is sent separately via InitialGridStateMessage.
type BroadcastStateCommand struct {
	State GameState // Contains the full dynamic state snapshot
}

// GameOverMessage is sent by GameActor via BroadcasterActor when the game ends.
type GameOverMessage struct {
	WinnerIndex int      `json:"winnerIndex"` // Index of the winning player (-1 if draw/no winner)
	FinalScores [4]int32 `json:"finalScores"` // Final scores of all players
	Reason      string   `json:"reason"`      // e.g., "All bricks destroyed"
	RoomPID     string   `json:"roomPid"`     // PID of the game room that ended
	MessageType string   `json:"messageType"` // Added for client-side differentiation
}

// --- Specific Messages TO Client ---

// PlayerAssignmentMessage informs a client of their assigned index.
// Sent directly from GameActor to the specific client's WebSocket upon connection.
type PlayerAssignmentMessage struct {
	PlayerIndex int    `json:"playerIndex"`
	MessageType string `json:"messageType"` // Added for client-side differentiation
}

// InitialGridStateMessage sends the static grid layout and canvas dimensions.
// Sent directly from GameActor to the specific client's WebSocket upon connection,
// after the PlayerAssignmentMessage.
type InitialGridStateMessage struct {
	CanvasWidth  int    `json:"canvasWidth"`
	CanvasHeight int    `json:"canvasHeight"`
	GridSize     int    `json:"gridSize"`
	CellSize     int    `json:"cellSize"`
	Grid         Grid   `json:"grid"`        // Contains the initial brick layout
	MessageType  string `json:"messageType"` // To help client distinguish messages
}

// --- Internal Actor Messages ---

type GameTick struct{}      // Sent to GameActor by its physics ticker
type BroadcastTick struct{} // Sent to GameActor by its broadcast ticker

// --- Internal Connection Handler Messages ---
// Exported types for use in server package
type InternalReadLoopMsg struct{ Payload []byte }

// --- Debug/Test Messages ---
// Removed GetInternalStateDebug and InternalStateResponse