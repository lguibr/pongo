// File: game/player.go
package game

import (
	"fmt"

	"github.com/lguibr/pongo/utils"
)

// Player struct now primarily for holding state data used in JSON marshalling.
type Player struct {
	Index int    `json:"index"`
	Id    string `json:"id"`
	Color [3]int `json:"color"`
	Score int32  `json:"score"` // Changed to int32 for atomic operations
}

// NewPlayerChannel is DEPRECATED.
func NewPlayerChannel() /* chan PlayerMessage */ interface{} { // Return interface{} to avoid type error
	fmt.Println("WARNING: NewPlayerChannel() is deprecated.")
	return nil
}

// NewPlayer creates the Player data struct.
func NewPlayer(canvas *Canvas, index int) *Player {
	cfg := utils.DefaultConfig() // Get default config for initial score
	return &Player{
		Index: index,
		Id:    "player" + fmt.Sprint(index),
		Color: utils.NewRandomColor(),
		Score: int32(cfg.InitialScore), // Use initial score from config
	}
}

// Connect is DEPRECATED. GameActor handles connection logic.
func (player *Player) Connect() {
	fmt.Printf("WARNING: player.Connect() for player %d is deprecated. GameActor handles connection.\n", player.Index)
}

// Disconnect is DEPRECATED. Connection handler sends PlayerDisconnect to GameActor.
func (player *Player) Disconnect() {
	fmt.Printf("WARNING: player.Disconnect() for player %d is deprecated. Connection handler sends message.\n", player.Index)
}
