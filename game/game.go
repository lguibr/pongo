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

type GameMessage interface{}

type AddBall struct {
	BallPayload *Ball
	ExpireIn    int
}
type RemoveBall struct {
	Id int
}

type IncreaseBallVelocity struct {
	BallPayload *Ball
	Ratio       float64
}
type IncreaseBallMass struct {
	BallPayload *Ball
	Additional  int
}
type BallPhasing struct {
	BallPayload *Ball
	ExpireIn    int
}

type Game struct {
	Canvas  *Canvas    `json:"canvas"`
	Players [4]*Player `json:"players"`
	Paddles [4]*Paddle `json:"paddles"`
	Balls   []*Ball    `json:"balls"`
	channel chan GameMessage // Main game logic channel (will become GameActor mailbox)
	mu      sync.RWMutex     // Mutex for protecting concurrent access (temporary)
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
		channel: make(chan GameMessage, 100), // Buffered channel for game events
	}

	return &game
}

// ToJson marshals the current game state. Added panic recovery and temporary locking.
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

	stateCopy := struct {
		Canvas  *Canvas    `json:"canvas"`
		Players [4]*Player `json:"players"`
		Paddles [4]*Paddle `json:"paddles"`
		Balls   []*Ball    `json:"balls"`
	}{
		Canvas:  game.Canvas,
		Players: game.Players,
		Paddles: game.Paddles,
		Balls:   game.Balls,
	}

	gameBytes, err := json.Marshal(stateCopy)
	if err != nil {
		fmt.Println("Error Marshaling the game state:", err)
		return []byte("{}")
	}
	return gameBytes
}

// GetNextIndex finds the first available player slot.
func (game *Game) GetNextIndex() int {
	game.mu.RLock()
	defer game.mu.RUnlock()

	for i, player := range game.Players {
		if player == nil {
			return i
		}
	}
	return -1
}

// HasPlayer checks if any player slots are occupied.
func (game *Game) HasPlayer() bool {
	game.mu.RLock()
	defer game.mu.RUnlock()

	for _, player := range game.Players {
		if player != nil {
			return true
		}
	}
	return false
}

// WriteGameState sends the game state to the client periodically.
func (game *Game) WriteGameState(ws *websocket.Conn, stopCh <-chan struct{}) {
	ticker := time.NewTicker(utils.Period)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			gameState := game.ToJson()
			if len(gameState) <= 2 {
				continue
			}
			_, err := ws.Write(gameState)
			if err != nil {
				return
			}
		case <-stopCh:
			return
		}
	}
}

// RemovePlayer removes a player and signals removal of their balls.
func (game *Game) RemovePlayer(playerIndex int) {
	fmt.Printf("Removing player state for index %d\n", playerIndex)

	// Acquire write lock to update state
	game.mu.Lock()
	// Validate index
	if playerIndex < 0 || playerIndex >= len(game.Players) {
		fmt.Printf("Invalid player index %d for removal\n", playerIndex)
		game.mu.Unlock()
		return
	}

	// Close player channel if exists
	if game.Players[playerIndex] != nil && game.Players[playerIndex].channel != nil {
		close(game.Players[playerIndex].channel)
		game.Players[playerIndex].channel = nil
	}
	// Remove player
	game.Players[playerIndex] = nil

	// Close paddle channel if exists
	if game.Paddles[playerIndex] != nil && game.Paddles[playerIndex].channel != nil {
		close(game.Paddles[playerIndex].channel)
		game.Paddles[playerIndex].channel = nil
	}
	// Remove paddle
	game.Paddles[playerIndex] = nil

	// Collect balls to remove
	ballsToRemove := []int{}
	for _, ball := range game.Balls {
		if ball != nil && ball.OwnerIndex == playerIndex {
			ballsToRemove = append(ballsToRemove, ball.Id)
		}
	}

	// Release lock before sending messages
	game.mu.Unlock()

	// Signal removal of each ball
	for _, ballId := range ballsToRemove {
		select {
		case game.channel <- RemoveBall{Id: ballId}:
		default:
			fmt.Printf("Game channel full, could not send RemoveBall for ball %d\n", ballId)
		}
	}
}

// AddPlayer adds a player and paddle to the game.
func (g *Game) AddPlayer(index int, player *Player, playerPaddle *Paddle) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if index < 0 || index >= len(g.Players) {
		return
	}
	if g.Players[index] != nil {
		return
	}
	g.Players[index] = player
	g.Paddles[index] = playerPaddle
}

// AddBall adds a ball to the game state and manages expiration.
func (game *Game) AddBall(ball *Ball, expire int) {
	game.mu.Lock()
	defer game.mu.Unlock()
	game.Balls = append(game.Balls, ball)
	if expire > 0 {
		go func(ballId int, duration time.Duration) {
			time.Sleep(duration)
			select {
			case game.channel <- RemoveBall{Id: ballId}:
			default:
			}
		}(ball.Id, time.Duration(expire)*time.Second)
	}
}

// RemoveBall removes a ball from the game state and stops its engine.
func (game *Game) RemoveBall(id int) {
	game.mu.Lock()
	defer game.mu.Unlock()

	var ballToClose *Ball
	newBalls := []*Ball{}
	for _, ball := range game.Balls {
		if ball != nil && ball.Id == id {
			ball.open = false
			ballToClose = ball
		} else {
			newBalls = append(newBalls, ball)
		}
	}
	game.Balls = newBalls
	if ballToClose != nil && ballToClose.Channel != nil {
		close(ballToClose.Channel)
		ballToClose.Channel = nil
	}
}
