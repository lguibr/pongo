// File: game/game.go
package game

// Game struct is largely deprecated. State is managed by GameActor.
type Game struct {
}

// StartGame is DEPRECATED. Initialization happens in GameActor producer.
func StartGame() *Game {
	// fmt.Println("WARNING: game.StartGame() is deprecated. GameActor initializes the game.")
	return nil
}

// ToJson is DEPRECATED. GameActor handles state marshalling.
func (game *Game) ToJson() []byte {
	// fmt.Println("WARNING: game.ToJson() is deprecated. Use GameActor state.")
	return []byte("{}")
}

// GetNextIndex is DEPRECATED. GameActor manages player slots.
func (game *Game) GetNextIndex() int {
	// fmt.Println("WARNING: game.GetNextIndex() is deprecated. Use GameActor logic.")
	return -1 // Indicate error or unavailability
}

// HasPlayer is DEPRECATED. GameActor manages player state.
func (game *Game) HasPlayer() bool {
	// fmt.Println("WARNING: game.HasPlayer() is deprecated. Use GameActor logic.")
	return false
}

// WriteGameState is DEPRECATED. GameActor broadcasts state.
// Remove websocket import from signature.
func (game *Game) WriteGameState( /* ws *websocket.Conn, */ stopCh <-chan struct{}) {
	// fmt.Println("WARNING: game.WriteGameState() is deprecated. GameActor broadcasts state.")
	// Drain stopCh to prevent goroutine leak if called somehow
	<-stopCh
}

// RemovePlayer is DEPRECATED. Send PlayerDisconnect message to GameActor.
func (game *Game) RemovePlayer(playerIndex int) {
	// fmt.Printf("WARNING: game.RemovePlayer(%d) is deprecated. Send PlayerDisconnect message to GameActor.\n", playerIndex)
}

// AddPlayer is DEPRECATED. GameActor handles PlayerConnectRequest.
func (g *Game) AddPlayer(index int, player *Player, playerPaddle *Paddle) {
	// fmt.Printf("WARNING: game.AddPlayer(%d) is deprecated. GameActor handles PlayerConnectRequest.\n", index)
}

// AddBall is DEPRECATED. Send SpawnBallCommand message to GameActor.
func (game *Game) AddBall(ball *Ball, expire int) {
	// fmt.Printf("WARNING: game.AddBall(%d) is deprecated. Send SpawnBallCommand message to GameActor.\n", ball.Id)
}

// RemoveBall is DEPRECATED. GameActor stops the BallActor.
func (game *Game) RemoveBall(id int) {
	// fmt.Printf("WARNING: game.RemoveBall(%d) is deprecated. GameActor stops the BallActor.\n", id)
}
