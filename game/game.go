// File: game/game.go
package game

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"runtime/debug"
	"sync"
	"time"

	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

// GameMessage defines messages sent internally, primarily to the old game channel.
// TODO: Refactor this for actor-based communication (messages to GameActor).
type GameMessage interface{}

// AddBall message used by old ReadPlayerChannel/ReadBallChannel.
// TODO: Replace with direct spawning or message to GameActor.
type AddBall struct {
	BallPayload *Ball
	ExpireIn    int
}
// RemoveBall message used by old game logic.
// TODO: Replace with message to GameActor.
type RemoveBall struct {
	Id int
}

// IncreaseVelocity message used by old ReadBallChannel.
// TODO: Replace with message to BallActor.
type IncreaseBallVelocity struct {
	BallPayload *Ball
	Ratio       float64
}
// IncreaseBallMass message used by old ReadBallChannel.
// TODO: Replace with message to BallActor.
type IncreaseBallMass struct {
	BallPayload *Ball
	Additional  int
}
// BallPhasing message used by old ReadBallChannel.
// TODO: Replace with message to BallActor.
type BallPhasing struct {
	BallPayload *Ball
	ExpireIn    int
}

// Game struct holds the overall game state.
// TODO: This state should be managed *within* the GameActor.
type Game struct {
	Canvas  *Canvas    `json:"canvas"`
	Players [4]*Player `json:"players"`
	Paddles [4]*Paddle `json:"paddles"` // Paddles are now just state data
	Balls   []*Ball    `json:"balls"`   // Balls are now just state data
	channel chan GameMessage // DEPRECATED: Main game logic channel (will become GameActor mailbox)
	mu      sync.RWMutex     // Mutex for protecting concurrent access (temporary)
	// TODO: Add reference to bollywood.Engine and GameActor PID
}

// StartGame initializes a new game with empty state.
func StartGame() *Game {
	rand.Seed(time.Now().UnixNano())

	canvas := NewCanvas(0, 0)
	canvas.Grid.Fill(0, 0, 0, 0)
	players := [4]*Player{}

	game := Game{
		Canvas:  canvas,
		Players: players,
		channel: make(chan GameMessage, 100), // DEPRECATED: Buffered channel for old game events
	}

	// TODO: Initialize and start the main bollywood.Engine here?
	// TODO: Spawn the main GameActor here?

	return &game
}

// ToJson marshals the current game state.
// TODO: This should be a method on the GameActor or requested via message.
func (game *Game) ToJson() []byte {
	// Panic Recovery
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC recovered during ToJson: %v\nStack trace:\n%s\n", r, string(debug.Stack()))
		}
	}()

	// Temporary read lock for safety until actor refactor
	game.mu.RLock()
	defer game.mu.RUnlock()

	// Create a copy of the state for marshalling
	stateCopy := struct {
		Canvas  *Canvas    `json:"canvas"`
		Players [4]*Player `json:"players"`
		Paddles [4]*Paddle `json:"paddles"`
		Balls   []*Ball    `json:"balls"`
	}{
		Canvas:  game.Canvas,
		Players: game.Players,
		Paddles: game.Paddles, // Copy paddle state
		Balls:   make([]*Ball, len(game.Balls)), // Need to deep copy ball state pointers? Or just pointers?
	}
    // Shallow copy of ball pointers is likely sufficient for JSON marshal,
    // as long as the GameActor ensures exclusive write access during updates.
    copy(stateCopy.Balls, game.Balls)


	gameBytes, err := json.Marshal(stateCopy)
	if err != nil {
		fmt.Println("Error Marshaling the game state:", err)
		return []byte("{}")
	}
	return gameBytes
}

// GetNextIndex finds the first available player slot.
func (game *Game) GetNextIndex() int {
	game.mu.RLock() // Still needs locking if accessed concurrently before GameActor
	defer game.mu.RUnlock()

	for i, player := range game.Players {
		if player == nil {
			return i
		}
	}
	return -1 // No slot available
}

// HasPlayer checks if any player slots are occupied.
func (game *Game) HasPlayer() bool {
	game.mu.RLock() // Still needs locking
	defer game.mu.RUnlock()

	for _, player := range game.Players {
		if player != nil {
			return true
		}
	}
	return false
}

// WriteGameState sends the game state to the client periodically.
// TODO: This should ideally be triggered by the GameActor sending state updates,
// rather than running its own timer loop per connection. The GameActor would
// likely broadcast the state to all connected client handlers/actors.
func (game *Game) WriteGameState(ws *websocket.Conn, stopCh <-chan struct{}) {
	ticker := time.NewTicker(utils.Period)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Reading game state needs protection until GameActor takes over
			gameState := game.ToJson()
			if len(gameState) <= 2 { // Avoid sending empty "{}"
				continue
			}
			_, err := ws.Write(gameState)
			if err != nil {
				// Connection likely closed, stop writing for this client
				// fmt.Printf("WriteGameState: Error writing to %s: %v\n", ws.RemoteAddr(), err)
				return
			}
		case <-stopCh:
			// Signal received to stop writing for this client
			// fmt.Printf("WriteGameState: Stop signal received for %s\n", ws.RemoteAddr())
			return
		}
	}
}

// RemovePlayer removes a player and signals removal of their balls.
// TODO: This should be handled by sending a message to the GameActor.
func (game *Game) RemovePlayer(playerIndex int) {
	fmt.Printf("Removing player state for index %d\n", playerIndex)

	// Acquire write lock to update state (temporary, actor will handle concurrency)
	game.mu.Lock()
	// Validate index
	if playerIndex < 0 || playerIndex >= len(game.Players) {
		fmt.Printf("Invalid player index %d for removal\n", playerIndex)
		game.mu.Unlock()
		return
	}

	// Close player channel if exists (part of old player logic)
	if game.Players[playerIndex] != nil && game.Players[playerIndex].channel != nil {
		// Ensure channel close is idempotent
		select {
		case _, ok := <-game.Players[playerIndex].channel:
			if ok { close(game.Players[playerIndex].channel) }
		default: close(game.Players[playerIndex].channel)
		}
		game.Players[playerIndex].channel = nil
	}
	// Remove player reference
	game.Players[playerIndex] = nil

	// Remove paddle reference (PaddleActor should be stopped separately via engine)
	game.Paddles[playerIndex] = nil

	// Collect balls owned by this player
	ballsToRemove := []int{}
	for _, ball := range game.Balls {
		if ball != nil && ball.OwnerIndex == playerIndex {
			ballsToRemove = append(ballsToRemove, ball.Id)
		}
	}

	// Release lock before potentially sending messages
	game.mu.Unlock()

	// Signal removal of each ball (to GameActor via game channel for now - DEPRECATED)
	for _, ballId := range ballsToRemove {
		// Non-blocking send to game channel
		select {
		case game.channel <- RemoveBall{Id: ballId}: // Send message to old channel
		default:
			fmt.Printf("Game channel full, could not send RemoveBall for ball %d\n", ballId)
		}
	}
	// TODO: Also need to tell the Engine to Stop the PaddleActor for this playerIndex
	// engine.Stop(paddlePID) // Need paddlePID associated with playerIndex
	// TODO: Tell Engine to Stop all BallActors associated with this playerIndex
	// for _, ballId := range ballsToRemove { engine.Stop(ballPIDs[ballId]) }
}

// AddPlayer adds a player and paddle state to the game.
// TODO: This should be handled by sending a message to the GameActor.
func (g *Game) AddPlayer(index int, player *Player, playerPaddle *Paddle) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if index < 0 || index >= len(g.Players) {
		fmt.Printf("AddPlayer: Invalid index %d\n", index)
		return
	}
	if g.Players[index] != nil {
		fmt.Printf("AddPlayer: Index %d already occupied\n", index)
		return
	}
	g.Players[index] = player
	g.Paddles[index] = playerPaddle // Store the initial paddle state
}

// AddBall adds a ball state object to the game's list and manages expiration via the old channel.
// TODO: This should be handled by sending a message to the GameActor,
// which would then likely spawn a BallActor. The GameActor would manage timers/expiration.
func (game *Game) AddBall(ball *Ball, expire int) {
	game.mu.Lock()
	// Check if ball already exists (could happen with concurrent AddBall messages)
	exists := false
	for _, b := range game.Balls {
		if b != nil && b.Id == ball.Id {
			exists = true
			break
		}
	}
	if exists {
		fmt.Printf("AddBall: Ball %d already exists, skipping add.\n", ball.Id)
		game.mu.Unlock()
		return
	}

	game.Balls = append(game.Balls, ball)
	fmt.Printf("Ball %d added. Total balls: %d\n", ball.Id, len(game.Balls))
	game.mu.Unlock() // Unlock before starting goroutine

	if expire > 0 {
		// Start timer using the old game channel for removal signal
		go func(ballId int, duration time.Duration) {
			time.Sleep(duration)
			// Send message non-blockingly to old channel
			select {
			case game.channel <- RemoveBall{Id: ballId}:
			default:
				fmt.Printf("Game channel full, could not send timed RemoveBall for %d\n", ballId)
			}
		}(ball.Id, time.Duration(expire)*time.Second)
	}
}

// RemoveBall removes a ball state object from the game's list.
// TODO: This should be handled by sending a message to the GameActor,
// which would then tell the Engine to Stop the corresponding BallActor.
func (game *Game) RemoveBall(id int) {
	game.mu.Lock()
	defer game.mu.Unlock()

	// var ballToClose *Ball // No longer needed - Actor manages itself
	found := false
	newBalls := []*Ball{} // Create new slice efficiently
	for _, ball := range game.Balls {
		if ball != nil {
			if ball.Id == id {
				// Ball state found, mark for removal from list
				// ballToClose = ball
				found = true
			} else {
				newBalls = append(newBalls, ball) // Keep other balls
			}
		}
	}

	if !found {
		fmt.Printf("RemoveBall: Ball %d not found.\n", id)
		return
	}

	game.Balls = newBalls // Replace with the new slice (without the removed ball)
	fmt.Printf("Ball %d state removed from list. Total balls: %d\n", id, len(game.Balls))

	// TODO: GameActor needs to tell the Engine to Stop the BallActor with this id.
	// engine.Stop(ballActorPID) // Need mapping from ball.Id to ballActorPID
}

