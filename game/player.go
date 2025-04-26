// File: game/player.go
package game

import (
	"fmt"
	// "io" // No longer needed here

	"github.com/lguibr/pongo/utils"
	// "golang.org/x/net/websocket" // No longer needed here
)

type PlayerMessage interface{}

type PlayerConnectMessage struct {
	PlayerPayload *Player
}
type PlayerDisconnectMessage struct{}
type PlayerScore struct {
	Score int
}

type Player struct {
	Index   int     `json:"index"`
	Id      string  `json:"id"`
	Canvas  *Canvas `json:"canvas"`
	Color   [3]int  `json:"color"`
	Score   int     `json:"score"`
	channel chan PlayerMessage
}

func NewPlayerChannel() chan PlayerMessage {
	return make(chan PlayerMessage)
}

func NewPlayer(canvas *Canvas, index int, channel chan PlayerMessage) *Player {
	return &Player{
		Index:   index,
		Id:      "player" + fmt.Sprint(index),
		Canvas:  canvas,
		Color:   utils.NewRandomColor(),
		channel: channel,
		Score:   utils.InitialScore,
	}
}

func (player *Player) Connect() {
	// Send connect message to the game logic (e.g., GameActor)
	select {
	case player.channel <- PlayerConnectMessage{PlayerPayload: player}:
		fmt.Printf("Player %d sent connect message\n", player.Index)
	default:
		fmt.Printf("Player %d could not send connect message (channel full/closed?)\n", player.Index)
	}
}

// Disconnect sends a message to the game logic actor to clean up player state.
func (player *Player) Disconnect() {
	// Use a non-blocking send in case the channel is closed or full during shutdown
	select {
	case player.channel <- PlayerDisconnectMessage{}:
		fmt.Printf("Player %d sent disconnect message to game logic\n", player.Index)
	default:
		fmt.Printf("Player %d could not send disconnect message (channel closed/full?)\n", player.Index)
	}
}

