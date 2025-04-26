package game

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand" // Import net
	"sync/atomic"
	"time"

	"github.com/lguibr/pongo/bollywood"
	"github.com/lguibr/pongo/utils"
	// "golang.org/x/net/websocket" // No longer needed for type assertion
)

const maxPlayers = 4

// GameActor manages the overall game state and coordinates child actors.
type GameActor struct {
	canvas        *Canvas
	players       [maxPlayers]*playerInfo // Info about connected players
	paddles       [maxPlayers]*Paddle     // Live state of paddles (updated by messages)
	paddleActors  [maxPlayers]*bollywood.PID
	balls         map[int]*Ball // Live state of balls (updated by messages) - Keyed by Ball ID
	ballActors    map[int]*bollywood.PID
	engine        *bollywood.Engine // Reference to the engine
	ticker        *time.Ticker
	stopTickerCh  chan struct{}
	gameStateJSON atomic.Value   // Stores marshalled JSON for HTTP endpoint
	selfPID       *bollywood.PID // Store self PID for internal use
}

// playerInfo holds state associated with a connected player/websocket.
type playerInfo struct {
	Index       int
	ID          string
	Score       int
	Color       [3]int
	Ws          PlayerConnection // Use the interface type
	IsConnected bool
}

// NewGameActorProducer creates a producer for the GameActor.
func NewGameActorProducer(engine *bollywood.Engine) bollywood.Producer {
	return func() bollywood.Actor {
		canvas := NewCanvas(0, 0) // Default size
		canvas.Grid.Fill(0, 0, 0, 0)

		ga := &GameActor{
			canvas:       canvas,
			players:      [maxPlayers]*playerInfo{},
			paddles:      [maxPlayers]*Paddle{},
			paddleActors: [maxPlayers]*bollywood.PID{},
			balls:        make(map[int]*Ball),
			ballActors:   make(map[int]*bollywood.PID),
			engine:       engine,
			stopTickerCh: make(chan struct{}),
		}
		ga.updateGameStateJSON() // Initialize JSON state
		return ga
	}
}

func (a *GameActor) Receive(ctx bollywood.Context) {
	if a.selfPID == nil {
		a.selfPID = ctx.Self()
	}

	switch msg := ctx.Message().(type) {
	case bollywood.Started:
		fmt.Println("GameActor started.")
		a.selfPID = ctx.Self()
		a.ticker = time.NewTicker(utils.Period)
		go a.runTickerLoop()

	case *GameTick:
		a.detectCollisions(ctx) // Pass context
		a.broadcastGameState()
		a.updateGameStateJSON()

	case PlayerConnectRequest:
		a.handlePlayerConnect(ctx, msg.WsConn)

	case PlayerDisconnect:
		a.handlePlayerDisconnect(ctx, msg.PlayerIndex)

	case ForwardedPaddleDirection:
		a.handlePaddleDirection(ctx, msg.PlayerIndex, msg.Direction)

	case PaddlePositionMessage:
		a.handlePaddlePositionUpdate(ctx, msg.Paddle)

	case BallPositionMessage:
		a.handleBallPositionUpdate(ctx, msg.Ball)

	case SpawnBallCommand:
		a.spawnBall(ctx, msg.OwnerIndex, msg.X, msg.Y, msg.ExpireIn)

	case bollywood.Stopping:
		fmt.Println("GameActor stopping...")
		if a.ticker != nil {
			a.ticker.Stop()
			select {
			case <-a.stopTickerCh:
			default:
				close(a.stopTickerCh)
			}
		}
		for i := 0; i < maxPlayers; i++ {
			if a.paddleActors[i] != nil {
				a.engine.Stop(a.paddleActors[i])
			}
		}
		for _, pid := range a.ballActors {
			if pid != nil {
				a.engine.Stop(pid)
			}
		}
		for _, pInfo := range a.players {
			if pInfo != nil && pInfo.Ws != nil {
				_ = pInfo.Ws.Close()
			}
		}

	case bollywood.Stopped:
		fmt.Println("GameActor stopped.")

	default:
		fmt.Printf("GameActor received unknown message type: %T\n", msg)
	}
}

// --- Actor Logic Methods ---

func (a *GameActor) runTickerLoop() {
	fmt.Println("GameActor ticker loop started.")
	defer fmt.Println("GameActor ticker loop stopped.")
	if a.selfPID == nil {
		fmt.Println("ERROR: GameActor ticker loop cannot start, self PID not set.")
		return
	}
	for {
		select {
		case <-a.ticker.C:
			a.engine.Send(a.selfPID, &GameTick{}, nil)
		case <-a.stopTickerCh:
			return
		}
	}
}

func (a *GameActor) handlePlayerConnect(ctx bollywood.Context, ws PlayerConnection) {
	playerIndex := -1
	for i, p := range a.players {
		if p == nil {
			playerIndex = i
			break
		}
	}

	remoteAddr := "unknown"
	if ws != nil {
		remoteAddr = ws.RemoteAddr().String()
	}

	if playerIndex == -1 {
		fmt.Println("GameActor: Server full, rejecting connection from:", remoteAddr)
		if ws != nil {
			_ = ws.Close()
		}
		return
	}

	fmt.Printf("GameActor: Assigning player index %d to %s\n", playerIndex, remoteAddr)

	isFirstPlayer := true
	for _, p := range a.players {
		if p != nil {
			isFirstPlayer = false
			break
		}
	}
	if isFirstPlayer {
		fmt.Println("GameActor: First player joining, initializing grid.")
		a.canvas.Grid.Fill(0, 0, 0, 0)
	}

	player := &playerInfo{
		Index:       playerIndex,
		ID:          fmt.Sprintf("player%d", playerIndex),
		Score:       utils.InitialScore,
		Color:       utils.NewRandomColor(),
		Ws:          ws,
		IsConnected: true,
	}
	a.players[playerIndex] = player

	paddleData := NewPaddle(a.canvas.CanvasSize, playerIndex)
	a.paddles[playerIndex] = paddleData
	paddleProducer := NewPaddleActorProducer(*paddleData, ctx.Self())
	paddlePID := a.engine.Spawn(bollywood.NewProps(paddleProducer))
	a.paddleActors[playerIndex] = paddlePID
	fmt.Printf("GameActor: Spawned PaddleActor %s for player %d\n", paddlePID, playerIndex)

	a.spawnBall(ctx, playerIndex, 0, 0, 0)

	a.broadcastGameState()
}

func (a *GameActor) handlePlayerDisconnect(ctx bollywood.Context, playerIndex int) {
	if playerIndex < 0 || playerIndex >= maxPlayers || a.players[playerIndex] == nil {
		fmt.Printf("GameActor: Received disconnect for invalid or already disconnected player index %d\n", playerIndex)
		return
	}

	fmt.Printf("GameActor: Handling disconnect for player %d\n", playerIndex)

	if a.paddleActors[playerIndex] != nil {
		fmt.Printf("GameActor: Stopping PaddleActor %s for player %d\n", a.paddleActors[playerIndex], playerIndex)
		a.engine.Stop(a.paddleActors[playerIndex])
		a.paddleActors[playerIndex] = nil
	}

	ballsToStop := []*bollywood.PID{}
	ballsToRemoveFromState := []int{}
	for ballID, ball := range a.balls {
		if ball != nil && ball.OwnerIndex == playerIndex {
			if pid, ok := a.ballActors[ballID]; ok && pid != nil {
				ballsToStop = append(ballsToStop, pid)
			}
			ballsToRemoveFromState = append(ballsToRemoveFromState, ballID)
		}
	}
	for _, pid := range ballsToStop {
		fmt.Printf("GameActor: Stopping BallActor %s for disconnected player %d\n", pid, playerIndex)
		a.engine.Stop(pid)
	}
	for _, ballID := range ballsToRemoveFromState {
		delete(a.balls, ballID)
		delete(a.ballActors, ballID)
	}

	if a.players[playerIndex].Ws != nil {
		fmt.Printf("GameActor: Closing WebSocket for player %d\n", playerIndex)
		_ = a.players[playerIndex].Ws.Close()
	}

	a.players[playerIndex] = nil
	a.paddles[playerIndex] = nil

	fmt.Printf("GameActor: Player %d disconnected and cleaned up.\n", playerIndex)

	playersLeft := false
	for _, p := range a.players {
		if p != nil {
			playersLeft = true
			break
		}
	}
	if !playersLeft {
		fmt.Println("GameActor: Last player disconnected. Game inactive.")
	}

	a.broadcastGameState()
}

func (a *GameActor) handlePaddleDirection(ctx bollywood.Context, playerIndex int, directionData []byte) {
	if playerIndex < 0 || playerIndex >= maxPlayers || a.paddleActors[playerIndex] == nil {
		return
	}
	a.engine.Send(a.paddleActors[playerIndex], PaddleDirectionMessage{Direction: directionData}, ctx.Self())
}

func (a *GameActor) handlePaddlePositionUpdate(ctx bollywood.Context, paddleState *Paddle) {
	if paddleState == nil {
		return
	}
	idx := paddleState.Index
	if idx >= 0 && idx < maxPlayers {
		a.paddles[idx] = paddleState
	}
}

func (a *GameActor) handleBallPositionUpdate(ctx bollywood.Context, ballState *Ball) {
	if ballState == nil {
		return
	}
	a.balls[ballState.Id] = ballState
}

func (a *GameActor) spawnBall(ctx bollywood.Context, ownerIndex, x, y int, expireIn time.Duration) {
	ballID := time.Now().Nanosecond()
	ballData := NewBall(x, y, 0, a.canvas.CanvasSize, ownerIndex, ballID)
	a.balls[ballID] = ballData

	selfPID := a.selfPID
	if selfPID == nil && ctx != nil {
		selfPID = ctx.Self()
	}
	if selfPID == nil {
		fmt.Println("ERROR: GameActor cannot spawn ball, self PID is nil.")
		delete(a.balls, ballID)
		return
	}

	ballProducer := NewBallActorProducer(*ballData, selfPID)
	ballPID := a.engine.Spawn(bollywood.NewProps(ballProducer))
	if ballPID == nil {
		fmt.Printf("ERROR: GameActor failed to spawn BallActor for player %d\n", ownerIndex)
		delete(a.balls, ballID)
		return
	}
	a.ballActors[ballID] = ballPID
	fmt.Printf("GameActor: Spawned BallActor %s (ID: %d) for player %d\n", ballPID, ballID, ownerIndex)

	if expireIn > 0 {
		time.AfterFunc(expireIn, func() {
			if selfPID != nil {
				// TODO: Improve safety - send message to self instead of direct stop/delete
				a.engine.Stop(ballPID)
				// delete(a.balls, ballID) // Deletion should happen upon actor stop confirmation
				// delete(a.ballActors, ballID)
				fmt.Printf("GameActor: Timer expired, requested stop for BallActor %s (ID: %d)\n", ballPID, ballID)
			}
		})
	}
}

func (a *GameActor) broadcastGameState() {
	state := GameState{
		Canvas:  a.canvas,
		Players: [maxPlayers]*Player{},
		Paddles: a.paddles,
		Balls:   make([]*Ball, 0, len(a.balls)),
	}
	for i, pi := range a.players {
		if pi != nil {
			state.Players[i] = &Player{
				Index: pi.Index,
				Id:    pi.ID,
				Color: pi.Color,
				Score: pi.Score,
			}
		} else {
			state.Players[i] = nil
		}
	}
	// Filter out nil balls just in case
	validBalls := make([]*Ball, 0, len(a.balls))
	for _, b := range a.balls {
		if b != nil {
			validBalls = append(validBalls, b)
		}
	}
	state.Balls = validBalls

	stateJSON, err := json.Marshal(state)
	if err != nil {
		fmt.Println("GameActor: Error marshalling game state:", err)
		return
	}

	for _, pInfo := range a.players {
		if pInfo != nil && pInfo.IsConnected && pInfo.Ws != nil {
			// Use the Write method from the PlayerConnection interface
			_, err := pInfo.Ws.Write(stateJSON) // Write directly using the interface
			if err != nil {
				fmt.Printf("GameActor: Error writing state to player %d (%s): %v. Marking as disconnected.\n", pInfo.Index, pInfo.Ws.RemoteAddr(), err)
				pInfo.IsConnected = false
				if a.selfPID != nil {
					// Send disconnect message to self for cleanup
					a.engine.Send(a.selfPID, PlayerDisconnect{PlayerIndex: pInfo.Index}, nil)
				}
			}
		}
	}
}

func (a *GameActor) updateGameStateJSON() {
	state := GameState{
		Canvas:  a.canvas,
		Players: [maxPlayers]*Player{},
		Paddles: a.paddles,
		Balls:   make([]*Ball, 0, len(a.balls)),
	}
	for i, pi := range a.players {
		if pi != nil {
			state.Players[i] = &Player{Index: pi.Index, Id: pi.ID, Color: pi.Color, Score: pi.Score}
		} else {
			state.Players[i] = nil
		}
	}
	validBalls := make([]*Ball, 0, len(a.balls))
	for _, b := range a.balls {
		if b != nil {
			validBalls = append(validBalls, b)
		}
	}
	state.Balls = validBalls

	stateJSON, err := json.Marshal(state)
	if err != nil {
		fmt.Println("GameActor: Error marshalling game state for HTTP:", err)
		a.gameStateJSON.Store([]byte("{}"))
		return
	}
	a.gameStateJSON.Store(stateJSON)
}

func (a *GameActor) GetGameStateJSON() []byte {
	val := a.gameStateJSON.Load()
	if val == nil {
		return []byte("{}")
	}
	return val.([]byte)
}

// --- Collision Detection Logic ---

func (a *GameActor) detectCollisions(ctx bollywood.Context) { // Pass context
	cellSize := a.canvas.CellSize
	canvasSize := a.canvas.CanvasSize

	ballsToRemove := []int{}

	for ballID, ball := range a.balls {
		if ball == nil {
			continue
		}
		ballActorPID := a.ballActors[ballID]
		if ballActorPID == nil {
			continue
		}

		// 1. Wall Collisions
		hitWall := -1
		if ball.X+ball.Radius > canvasSize {
			hitWall = 0
		} else if ball.Y-ball.Radius < 0 {
			hitWall = 1
		} else if ball.X-ball.Radius < 0 {
			hitWall = 2
		} else if ball.Y+ball.Radius > canvasSize {
			hitWall = 3
		}

		if hitWall != -1 {
			if hitWall == 0 || hitWall == 2 {
				a.engine.Send(ballActorPID, ReflectVelocityCommand{Axis: "X"}, nil)
			} else {
				a.engine.Send(ballActorPID, ReflectVelocityCommand{Axis: "Y"}, nil)
			}
			a.engine.Send(ballActorPID, SetPhasingCommand{ExpireIn: 100 * time.Millisecond}, nil)

			scorerIndex := ball.OwnerIndex
			concederIndex := hitWall

			if concederIndex != scorerIndex && a.players[concederIndex] != nil {
				fmt.Printf("GameActor: Player %d scores against Player %d\n", scorerIndex, concederIndex)
				if a.players[scorerIndex] != nil {
					a.players[scorerIndex].Score++
				}
				a.players[concederIndex].Score--
			} else if concederIndex == scorerIndex {
				fmt.Printf("GameActor: Player %d hit their own wall.\n", scorerIndex)
			} else {
				fmt.Printf("GameActor: Ball %d hit empty wall %d. Removing.\n", ballID, hitWall)
				ballsToRemove = append(ballsToRemove, ballID)
			}
		}

		// 2. Paddle Collisions
		for paddleIndex, paddle := range a.paddles {
			if paddle == nil {
				continue
			}
			if ball.BallInterceptPaddles(paddle) {
				fmt.Printf("GameActor: Ball %d collided with Paddle %d\n", ballID, paddleIndex)
				if paddleIndex%2 == 0 {
					a.engine.Send(ballActorPID, ReflectVelocityCommand{Axis: "X"}, nil)
				} else {
					a.engine.Send(ballActorPID, ReflectVelocityCommand{Axis: "Y"}, nil)
				}
				ball.OwnerIndex = paddleIndex
				a.engine.Send(ballActorPID, SetPhasingCommand{ExpireIn: 100 * time.Millisecond}, nil)
				break
			}
		}

		// 3. Brick Collisions (only if not phasing)
		if !ball.Phasing {
			collidedCells := a.findCollidingCells(ball, cellSize)
			for _, cellPos := range collidedCells {
				col, row := cellPos[0], cellPos[1]
				if col < 0 || col >= a.canvas.GridSize || row < 0 || row >= a.canvas.GridSize {
					continue
				}
				cell := &a.canvas.Grid[col][row]
				if cell.Data.Type == utils.Cells.Brick {
					fmt.Printf("GameActor: Ball %d hit brick at [%d, %d]\n", ballID, col, row)
					brickLevel := cell.Data.Level
					cell.Data.Life--

					dx := float64(ball.X - (col*cellSize + cellSize/2))
					dy := float64(ball.Y - (row*cellSize + cellSize/2))
					if math.Abs(dx) > math.Abs(dy) {
						a.engine.Send(ballActorPID, ReflectVelocityCommand{Axis: "X"}, nil)
					} else {
						a.engine.Send(ballActorPID, ReflectVelocityCommand{Axis: "Y"}, nil)
					}

					if cell.Data.Life <= 0 {
						fmt.Printf("GameActor: Brick broken at [%d, %d]\n", col, row)
						cell.Data.Type = utils.Cells.Empty
						cell.Data.Level = 0

						scorerIndex := ball.OwnerIndex
						if a.players[scorerIndex] != nil {
							a.players[scorerIndex].Score += brickLevel
							fmt.Printf("GameActor: Player %d score +%d for breaking brick.\n", scorerIndex, brickLevel)
						}

						if rand.Intn(4) == 0 {
							a.triggerRandomPowerUp(ctx, ball) // Pass context
						}
					}
					a.engine.Send(ballActorPID, SetPhasingCommand{ExpireIn: 100 * time.Millisecond}, nil)
					break
				}
			}
		}
	} // End ball loop

	// Remove balls that went out of bounds
	for _, ballID := range ballsToRemove {
		if pid, ok := a.ballActors[ballID]; ok && pid != nil {
			fmt.Printf("GameActor: Ball %d went out of bounds, stopping actor %s\n", ballID, pid)
			a.engine.Stop(pid)
		}
		delete(a.balls, ballID)
		delete(a.ballActors, ballID)
	}
}

func (a *GameActor) findCollidingCells(ball *Ball, cellSize int) [][2]int {
	collided := [][2]int{}
	gridSize := a.canvas.GridSize
	minCol := (ball.X - ball.Radius) / cellSize
	maxCol := (ball.X + ball.Radius) / cellSize
	minRow := (ball.Y - ball.Radius) / cellSize
	maxRow := (ball.Y + ball.Radius) / cellSize

	minCol = utils.MaxInt(0, minCol)
	maxCol = utils.MinInt(gridSize-1, maxCol)
	minRow = utils.MaxInt(0, minRow)
	maxRow = utils.MinInt(gridSize-1, maxRow)

	for c := minCol; c <= maxCol; c++ {
		for r := minRow; r <= maxRow; r++ {
			if ball.InterceptsIndex(c, r, cellSize) {
				collided = append(collided, [2]int{c, r})
			}
		}
	}
	return collided
}

func (a *GameActor) triggerRandomPowerUp(ctx bollywood.Context, ball *Ball) { // Accept context
	powerUpType := rand.Intn(3)
	ballActorPID := a.ballActors[ball.Id]
	if ballActorPID == nil {
		return
	}

	switch powerUpType {
	case 0:
		fmt.Printf("GameActor: Triggering SpawnBall power-up from ball %d\n", ball.Id)
		a.spawnBall(ctx, ball.OwnerIndex, ball.X, ball.Y, time.Duration(rand.Intn(5)+5)*time.Second) // Pass context
	case 1:
		fmt.Printf("GameActor: Triggering IncreaseMass power-up for ball %d\n", ball.Id)
		a.engine.Send(ballActorPID, IncreaseMassCommand{Additional: 1}, nil)
	case 2:
		fmt.Printf("GameActor: Triggering IncreaseVelocity power-up for ball %d\n", ball.Id)
		a.engine.Send(ballActorPID, IncreaseVelocityCommand{Ratio: 1.1}, nil)
	}
}

// GameState struct for JSON marshalling (used in broadcast/updateJSON)
type GameState struct {
	Canvas  *Canvas    `json:"canvas"`
	Players [4]*Player `json:"players"` // Use Player struct for JSON
	Paddles [4]*Paddle `json:"paddles"`
	Balls   []*Ball    `json:"balls"`
}
