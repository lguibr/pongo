// File: game/messages.go
package game

import (
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

// --- Common Message Header ---
type MessageHeader struct {
	MessageType string `json:"messageType"`
}

// --- Brick State Update (for FullGridUpdate) ---
// Contains essential info needed for rendering state for a single brick.
type BrickStateUpdate struct {
	X    float64        `json:"x"`    // R3F X coordinate (centered)
	Y    float64        `json:"y"`    // R3F Y coordinate (centered, Y-up)
	Life int            `json:"life"` // Current life
	Type utils.CellType `json:"type"` // Current type
}

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
type GetRoomListRequest struct{}
type RoomListResponse struct {
	Rooms map[string]int
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
	OwnerIndex       int
	X, Y             int
	ExpireIn         time.Duration
	IsPermanent      bool
	SetInitialPhasing bool // New field to force phasing on spawn for testing
}
// Message from BallActor telling GameActor to apply damage
type ApplyBrickDamage struct {
	BallID int      // ID of the ball that caused the damage
	Coord  [2]int   // [col, row] of the brick to damage
	BallX  int      // Ball's X position at time of damage (for power-up spawn)
	BallY  int      // Ball's Y position at time of damage (for power-up spawn)
	OwnerIndex int  // Ball's owner at time of damage (for scoring/power-up)
}

// --- State Updates FROM Child Actors TO GameActor ---
type PaddleStateUpdate struct {
	PID       *bollywood.PID
	Index     int
	Direction string
}
type BallStateUpdate struct {
	PID     *bollywood.PID
	ID      int
	Vx      int
	Vy      int
	Radius  int
	Mass    int
	Phasing bool
}

// --- Commands TO Child Actors ---
type SetPhasingCommand struct{}
type IncreaseVelocityCommand struct{ Ratio float64 }
type IncreaseMassCommand struct{ Additional int }
type ReflectVelocityCommand struct{ Axis string }
type SetVelocityCommand struct{ Vx, Vy int }
type DestroyBallCommand struct{}
type PaddleDirectionMessage struct{ Direction []byte }
// Message from GameActor telling BallActor to check brick damage
type DamageBrickCommand struct {
	Coord [2]int // [col, row] of the brick intersected
}


// --- Messages TO/FROM BroadcasterActor ---
type AddClient struct {
	Conn *websocket.Conn
}
type RemoveClient struct {
	Conn *websocket.Conn
}
type BroadcastUpdatesCommand struct {
	Updates []interface{}
}

// --- Specific Messages TO Client (via Broadcaster or directly) ---
type PlayerAssignmentMessage struct {
	MessageType string `json:"messageType"` // "playerAssignment"
	PlayerIndex int    `json:"playerIndex"`
}

// --- Initial State Message Structures (Modified) ---

// InitialPaddleState includes core Paddle data and its initial R3F coordinates.
type InitialPaddleState struct {
	Paddle // Embed core Paddle data
	R3fX   float64 `json:"r3fX"` // Initial R3F X coordinate
	R3fY   float64 `json:"r3fY"` // Initial R3F Y coordinate
}

// InitialBallState includes core Ball data and its initial R3F coordinates.
type InitialBallState struct {
	Ball // Embed core Ball data
	R3fX float64 `json:"r3fX"` // Initial R3F X coordinate
	R3fY float64 `json:"r3fY"` // Initial R3F Y coordinate
}

// InitialPlayersAndBallsState sends the state of existing entities to a newly joined player.
// Uses the new structs that include R3F coordinates.
type InitialPlayersAndBallsState struct {
	MessageType string                `json:"messageType"` // "initialPlayersAndBallsState"
	Players     []*Player             `json:"players"`     // Array of existing Player data
	Paddles     []InitialPaddleState `json:"paddles"`     // Array of existing Paddle data WITH R3F coords
	Balls       []InitialBallState    `json:"balls"`       // Array of existing Ball data WITH R3F coords
	// Add CanvasSize if needed by frontend for calculations (e.g., scaling)
	// CanvasSize int `json:"canvasSize"`
}

// --- End Initial State Message Structures ---

type GameOverMessage struct {
	MessageType string   `json:"messageType"` // "gameOver"
	WinnerIndex int      `json:"winnerIndex"`
	FinalScores [4]int32 `json:"finalScores"`
	Reason      string   `json:"reason"`
	RoomPID     string   `json:"roomPid"`
}

// --- Atomic Update Messages (Batched for Broadcast) ---
type GameUpdatesBatch struct {
	MessageType string        `json:"messageType"` // "gameUpdates"
	Updates     []interface{} `json:"updates"`
}
type BallPositionUpdate struct {
	MessageType string  `json:"messageType"` // "ballPositionUpdate"
	ID          int     `json:"id"`
	X           int     `json:"x"` // Original X
	Y           int     `json:"y"` // Original Y
	R3fX        float64 `json:"r3fX"` // Centered R3F X
	R3fY        float64 `json:"r3fY"` // Centered R3F Y (Y-up)
	Vx          int     `json:"vx"`
	Vy          int     `json:"vy"`
	Collided    bool    `json:"collided"`
}
type PaddlePositionUpdate struct {
	MessageType string  `json:"messageType"` // "paddlePositionUpdate"
	Index       int     `json:"index"`
	X           int     `json:"x"` // Original X
	Y           int     `json:"y"` // Original Y
	R3fX        float64 `json:"r3fX"` // Centered R3F X of center
	R3fY        float64 `json:"r3fY"` // Centered R3F Y of center (Y-up)
	Width       int     `json:"width"` // Original Width
	Height      int     `json:"height"` // Original Height
	Vx          int     `json:"vx"`
	Vy          int     `json:"vy"`
	IsMoving    bool    `json:"isMoving"`
	Collided    bool    `json:"collided"`
}

// FullGridUpdate sends the state of ALL grid cells as a flat list with R3F coordinates.
type FullGridUpdate struct {
	MessageType string             `json:"messageType"` // "fullGridUpdate"
	CellSize    int                `json:"cellSize"`    // Include cell size for geometry scaling
	Bricks      []BrickStateUpdate `json:"bricks"`      // List containing state and R3F coords for ALL cells
}

type ScoreUpdate struct {
	MessageType string `json:"messageType"` // "scoreUpdate"
	Index       int    `json:"index"`
	Score       int32  `json:"score"`
}
type BallOwnershipChange struct {
	MessageType   string `json:"messageType"` // "ballOwnerChanged"
	ID            int    `json:"id"`
	NewOwnerIndex int    `json:"newOwnerIndex"`
}
type BallSpawned struct {
	MessageType string  `json:"messageType"` // "ballSpawned"
	Ball        Ball    `json:"ball"`        // Original Ball struct
	R3fX        float64 `json:"r3fX"`        // Initial R3F X
	R3fY        float64 `json:"r3fY"`        // Initial R3F Y
}
type BallRemoved struct {
	MessageType string `json:"messageType"` // "ballRemoved"
	ID          int    `json:"id"`
}
type PlayerJoined struct {
	MessageType string  `json:"messageType"` // "playerJoined"
	Player      Player  `json:"player"`      // Original Player struct
	Paddle      Paddle  `json:"paddle"`      // Original Paddle struct
	R3fX        float64 `json:"r3fX"`        // Initial Paddle R3F X (center)
	R3fY        float64 `json:"r3fY"`        // Initial Paddle R3F Y (center)
}
type PlayerLeft struct {
	MessageType string `json:"messageType"` // "playerLeft"
	Index       int    `json:"index"`
}

// --- Internal Actor Messages ---
type GameTick struct{}
type BroadcastTick struct{}
type InternalReadLoopMsg struct{ Payload []byte }