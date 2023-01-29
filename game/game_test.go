package game

import (
	"encoding/json"
	"testing"

	"github.com/lguibr/pongo/utils"
)

func TestGame_HasPlayer(t *testing.T) {
	testCases := []struct {
		players   [4]*Player
		hasPlayer bool
	}{
		{[4]*Player{{Id: "player1"}, nil, nil}, true},
		{[4]*Player{nil, nil, nil}, false},
		{[4]*Player{{Id: "player1"}, {Id: "player2"}, nil}, true},
		{[4]*Player{nil, nil, {Id: "player3"}}, true},
	}

	for _, tc := range testCases {
		game := Game{Players: tc.players}
		result := game.HasPlayer()
		if result != tc.hasPlayer {
			t.Errorf("Game.HasPlayer() = %v, want %v", result, tc.hasPlayer)
		}
	}
}

func TestGame_GetNextIndex(t *testing.T) {
	testCases := []struct {
		players   [4]*Player
		nextIndex int
	}{
		{[4]*Player{nil, nil, nil}, 0},
		{[4]*Player{{Id: "player1"}, nil, nil}, 1},
		{[4]*Player{{Id: "player1"}, {Id: "player2"}, nil}, 2},
		{[4]*Player{{Id: "player1"}, {Id: "player2"}, {Id: "player3"}}, 3},
		{[4]*Player{{Id: "player1"}, {Id: "player2"}, {Id: "player3"}, {Id: "player4"}}, 0},
	}

	for _, tc := range testCases {
		game := Game{Players: tc.players}
		result := game.GetNextIndex()
		if result != tc.nextIndex {
			t.Errorf("Game.GetNextIndex() = %v, want %v", result, tc.nextIndex)
		}
	}
}

func TestStartGame(t *testing.T) {
	game := StartGame()
	if game.Canvas == nil {
		t.Errorf("Expected game to have a canvas, but got nil")
	}
	if game.Canvas.Grid == nil {
		t.Errorf("Expected game to have a grid, but got nil")
	}
	if len(game.Players) != 4 {
		t.Errorf("Expected game to have 4 players, but got %d", len(game.Players))
	}
	for _, player := range game.Players {
		if player != nil {
			t.Errorf("Expected all players to be nil, but got %v", player)
		}
	}
}

func TestGame_ToJson(t *testing.T) {
	game := &Game{
		Balls: []*Ball{
			{X: 0, Y: 0, Index: 0},
			{X: 1, Y: 1, Index: 0},
			{X: 2, Y: 2, Index: 0},
		},
		Paddles: [4]*Paddle{{Index: 0, X: 10, Y: 20, Width: 30, Height: 40}, {X: 15, Y: 25, Width: 35, Height: 45}},
		Canvas: &Canvas{
			Grid: Grid{
				{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}, Cell{X: 0, Y: 1, Data: &BrickData{Type: utils.Cells.Brick, Life: 2}}},
				{Cell{X: 1, Y: 0, Data: &BrickData{Type: utils.Cells.Empty, Life: 0}}, Cell{X: 1, Y: 1, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}},
			},
		},
		Players: [4]*Player{
			{Id: "player0"},
			{Id: "player1"},
			nil,
			nil,
		},
	}

	gameBytes, err := json.Marshal(game)

	if err != nil {
		t.Errorf("Error marshalling game: %v", err)
	}

	result := game.ToJson()

	if string(result) != string(gameBytes) {
		t.Errorf("Expected %s, got %s", string(gameBytes), string(result))
	}
}
