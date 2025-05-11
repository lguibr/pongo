// File: game/messages.go
package game

import (
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

// --- Message Header ---
// Used for identifying message types after unmarshalling from JSON
type MessageHeader struct {
	MessageType string `json:"messageType"`
}

// --- WebSocket Messages (Client <-> Server) ---

// PlayerAssignmentMessage informs the client of their assigned index.
type PlayerAssignmentMessage struct {
	MessageType string `json:"messageType"` // "playerAssignment"
	PlayerIndex int    `json:"playerIndex"`
}

// InitialPaddleState includes R3F coordinates for initial state messages.
type InitialPaddleState struct {
	Paddle         // Embed original data
	R3fX   float64 `json:"r3fX"`
	R3fY   float64 `json:"r3fY"`
}

// InitialBallState includes R3F coordinates for initial state messages.
type InitialBallState struct {
	Ball         // Embed original data
	R3fX float64 `json:"r3fX"`
	R3fY float64 `json:"r3fY"`
}

// InitialPlayersAndBallsState sends the initial state of all players and balls to a new client.
// Includes pre-calculated R3F coordinates.
type InitialPlayersAndBallsState struct {
	MessageType string               `json:"messageType"` // "initialPlayersAndBallsState"
	Players     []*Player            `json:"players"`
	Paddles     []InitialPaddleState `json:"paddles"` // Now includes R3F coords
	Balls       []InitialBallState   `json:"balls"`   // Now includes R3F coords
}

// GameUpdatesBatch bundles multiple atomic updates for efficient network transmission.
type GameUpdatesBatch struct {
	MessageType string        `json:"messageType"` // "gameUpdates"
	Updates     []interface{} `json:"updates"`     // Slice of different update message types
}

// GameOverMessage signals the end of the game.
type GameOverMessage struct {
	MessageType string                  `json:"messageType"` // "gameOver"
	WinnerIndex int                     `json:"winnerIndex"` // -1 for a tie or no winner
	FinalScores [utils.MaxPlayers]int32 `json:"finalScores"`
	Reason      string                  `json:"reason"`
	RoomPID     string                  `json:"roomPID"` // PID of the game room that ended
}

// --- Atomic Update Messages (Included in GameUpdatesBatch) ---

// PlayerJoined signals a new player has joined the room. Includes R3F coords.
type PlayerJoined struct {
	MessageType string  `json:"messageType"` // "playerJoined"
	Player      Player  `json:"player"`      // Player data (value type)
	Paddle      Paddle  `json:"paddle"`      // Initial paddle state (value type)
	R3fX        float64 `json:"r3fX"`        // R3F X coordinate for paddle center
	R3fY        float64 `json:"r3fY"`        // R3F Y coordinate for paddle center
}

// PlayerLeft signals a player has left the room.
type PlayerLeft struct {
	MessageType string `json:"messageType"` // "playerLeft"
	Index       int    `json:"index"`
}

// BallSpawned signals a new ball has been added. Includes R3F coords.
type BallSpawned struct {
	MessageType string  `json:"messageType"` // "ballSpawned"
	Ball        Ball    `json:"ball"`        // Ball data (value type)
	R3fX        float64 `json:"r3fX"`        // R3F X coordinate for ball center
	R3fY        float64 `json:"r3fY"`        // R3F Y coordinate for ball center
}

// BallRemoved signals a ball has been removed.
type BallRemoved struct {
	MessageType string `json:"messageType"` // "ballRemoved"
	ID          int    `json:"id"`
}

// BallPositionUpdate provides the latest position and state of a ball. Includes R3F coords.
type BallPositionUpdate struct {
	MessageType string  `json:"messageType"` // "ballPositionUpdate"
	ID          int     `json:"id"`
	X           int     `json:"x"`    // Original X
	Y           int     `json:"y"`    // Original Y
	R3fX        float64 `json:"r3fX"` // R3F X coordinate
	R3fY        float64 `json:"r3fY"` // R3F Y coordinate
	Vx          int     `json:"vx"`
	Vy          int     `json:"vy"`
	Collided    bool    `json:"collided"` // Collision flag
	Phasing     bool    `json:"phasing"`  // Phasing state
}

// PaddlePositionUpdate provides the latest position and state of a paddle. Includes R3F coords.
type PaddlePositionUpdate struct {
	MessageType string  `json:"messageType"` // "paddlePositionUpdate"
	Index       int     `json:"index"`
	X           int     `json:"x"`     // Original X
	Y           int     `json:"y"`     // Original Y
	R3fX        float64 `json:"r3fX"`  // R3F X coordinate for paddle center
	R3fY        float64 `json:"r3fY"`  // R3F Y coordinate for paddle center
	Width       int     `json:"width"` // Include dimensions for frontend geometry
	Height      int     `json:"height"`
	Vx          int     `json:"vx"`
	Vy          int     `json:"vy"`
	IsMoving    bool    `json:"isMoving"` // Movement state
	Collided    bool    `json:"collided"` // Collision flag
}

// BrickStateUpdate represents the state of a single brick cell. Includes R3F coords.
type BrickStateUpdate struct {
	// MessageType string `json:"messageType"` // "brickStateUpdate" - Removed, part of FullGridUpdate
	X    float64        `json:"x"`    // R3F X coordinate for cell center
	Y    float64        `json:"y"`    // R3F Y coordinate for cell center
	Life int            `json:"life"` // Remaining life
	Type utils.CellType `json:"type"` // Type (Brick, Empty, etc.)
}

// FullGridUpdate sends the state of ALL grid cells. Includes R3F coords.
type FullGridUpdate struct {
	MessageType string             `json:"messageType"` // "fullGridUpdate"
	CellSize    int                `json:"cellSize"`    // Cell size for geometry scaling
	Bricks      []BrickStateUpdate `json:"bricks"`      // Flat list of all brick states
}

// ScoreUpdate signals a change in a player's score.
type ScoreUpdate struct {
	MessageType string `json:"messageType"` // "scoreUpdate"
	Index       int    `json:"index"`
	Score       int32  `json:"score"`
}

// BallOwnershipChange signals that a ball's owner has changed.
type BallOwnershipChange struct {
	MessageType   string `json:"messageType"` // "ballOwnerChanged"
	ID            int    `json:"id"`
	NewOwnerIndex int    `json:"newOwnerIndex"` // -1 for ownerless
}

// --- Actor Messages (Internal Communication) ---

// --- RoomManagerActor Messages ---

// FindRoomRequest asks the RoomManager to find or create a room.
type FindRoomRequest struct {
	ReplyTo *bollywood.PID // PID of the actor requesting the room (ConnectionHandlerActor)
}

// AssignRoomResponse is the reply from RoomManager with the assigned GameActor PID.
type AssignRoomResponse struct {
	RoomPID *bollywood.PID // nil if no room could be assigned
}

// GameRoomEmpty notifies the RoomManager that a GameActor is finished or empty.
type GameRoomEmpty struct {
	RoomPID *bollywood.PID
}

// GetRoomListRequest asks the RoomManager for the list of active rooms (used by HTTP handler via Ask).
type GetRoomListRequest struct{}

// RoomListResponse contains the map of active rooms and player counts.
type RoomListResponse struct {
	Rooms map[string]int // Map of Room PID string to player count
}

// --- ConnectionHandlerActor Messages ---

// InternalReadLoopMsg wraps data read from WebSocket for processing by the actor.
type InternalReadLoopMsg struct {
	Payload []byte
}

// --- GameActor Messages ---

// AssignPlayerToRoom tells the GameActor to add a player associated with a WebSocket connection.
type AssignPlayerToRoom struct {
	WsConn *websocket.Conn
}

// PlayerDisconnect tells the GameActor that a player's connection was lost.
type PlayerDisconnect struct {
	WsConn *websocket.Conn
}

// ForwardedPaddleDirection carries paddle input from ConnectionHandler to GameActor.
type ForwardedPaddleDirection struct {
	WsConn    *websocket.Conn // Corrected type: Use *websocket.Conn
	Direction []byte          // Raw JSON payload {"direction": "..."}
}

// GameTick signals the GameActor to perform a physics update.
type GameTick struct{}

// BroadcastTick signals the GameActor to broadcast the current state.
type BroadcastTick struct{}

// PaddleStateUpdate sent from PaddleActor to GameActor when internal state changes.
type PaddleStateUpdate struct {
	PID       *bollywood.PID
	Index     int
	Direction string // Internal direction ("left", "right", "")
}

// BallStateUpdate sent from BallActor to GameActor when internal state changes.
type BallStateUpdate struct {
	PID     *bollywood.PID
	ID      int
	Vx      int
	Vy      int
	Radius  int
	Mass    int
	Phasing bool
}

// SpawnBallCommand tells GameActor to create a new ball.
type SpawnBallCommand struct {
	OwnerIndex        int
	X, Y              int           // Optional initial position (0,0 for default near owner)
	ExpireIn          time.Duration // 0 for permanent balls
	IsPermanent       bool
	SetInitialPhasing bool // Flag to make the ball phase immediately on spawn
}

// DestroyExpiredBall tells GameActor to remove a ball that reached its expiry time.
type DestroyExpiredBall struct {
	BallID int
}

// stopPhasingTimerMsg is an internal message sent by time.AfterFunc when a ball's phasing ends.
type stopPhasingTimerMsg struct {
	BallID int
}

// --- BroadcasterActor Messages ---

// AddClient tells the Broadcaster to start sending updates to a new connection.
type AddClient struct {
	Conn *websocket.Conn
}

// RemoveClient tells the Broadcaster to stop sending updates to a connection.
type RemoveClient struct {
	Conn *websocket.Conn
}

// BroadcastUpdatesCommand sends a batch of updates from GameActor to BroadcasterActor.
type BroadcastUpdatesCommand struct {
	Updates []interface{}
}

// --- PaddleActor Messages ---

// PaddleDirectionMessage carries the raw direction payload to the PaddleActor.
type PaddleDirectionMessage struct {
	Direction []byte // Raw JSON payload {"direction": "..."}
}

// --- BallActor Messages ---

// ReflectVelocityCommand tells the BallActor to reverse velocity on an axis.
type ReflectVelocityCommand struct {
	Axis string // "X" or "Y"
}

// SetVelocityCommand tells the BallActor to set a specific velocity.
type SetVelocityCommand struct {
	Vx, Vy int
}

// SetPhasingCommand tells the BallActor to enter the phasing state.
type SetPhasingCommand struct{}

// StopPhasingCommand tells the BallActor to exit the phasing state.
type StopPhasingCommand struct{}

// IncreaseVelocityCommand tells the BallActor to increase its speed.
type IncreaseVelocityCommand struct {
	Ratio float64
}

// IncreaseMassCommand tells the BallActor to increase its mass and radius.
type IncreaseMassCommand struct {
	Additional int
}

// DestroyBallCommand tells the BallActor to stop itself.
type DestroyBallCommand struct{}

// --- Internal Test Messages ---

// internalAddBallTestMsg allows tests to directly add a ball and its actor PID to GameActor state.
type internalAddBallTestMsg struct {
	Ball *Ball
	PID  *bollywood.PID
}

// internalStartTickersTestMsg allows tests to trigger ticker start in GameActor.
type internalStartTickersTestMsg struct{}

// internalTestingAddPlayerAndStart allows tests to add a player and start the game loop.
type internalTestingAddPlayerAndStart struct {
	PlayerIndex int
}

// internalGetBallRequest asks GameActor for a specific ball's state (used via Ask).
type internalGetBallRequest struct {
	BallID int
}

// internalGetBallResponse is the reply containing the ball state.
type internalGetBallResponse struct {
	Ball   *Ball // Pointer to a copy of the ball state
	Exists bool
}

// internalGetBrickRequest asks GameActor for a specific brick's state (used via Ask).
type internalGetBrickRequest struct {
	Row, Col int
}

// internalGetBrickResponse is the reply containing the brick state.
type internalGetBrickResponse struct {
	Life    int
	Type    utils.CellType
	Exists  bool // Does the cell exist in the grid?
	IsBrick bool // Is the cell currently a brick?
}

// internalTriggerStartPhasingPowerUp tells GameActor to activate phasing for a ball.
type internalTriggerStartPhasingPowerUp struct {
	BallID int
}

// internalConfirmPhasingRequest asks GameActor to confirm a ball's phasing state.
type internalConfirmPhasingRequest struct {
	BallID int
}

// internalConfirmPhasingResponse is the reply to internalConfirmPhasingRequest.
type internalConfirmPhasingResponse struct {
	IsPhasing bool
	Exists    bool
}
