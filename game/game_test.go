package game

import (
	"encoding/json"
	"testing"
	"time"

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
		Canvas: &Canvas{
			Grid: Grid{
				{Cell{X: 0, Y: 0, Data: &BrickData{Type: "Brick", Life: 1}}, Cell{X: 0, Y: 1, Data: &BrickData{Type: "Brick", Life: 2}}},
				{Cell{X: 1, Y: 0, Data: &BrickData{Type: "Empty", Life: 0}}, Cell{X: 1, Y: 1, Data: &BrickData{Type: "Brick", Life: 1}}},
			},
		},
		Players: [4]*Player{
			{Id: "player0",
				Paddle: &Paddle{X: 10, Y: 20, Width: 30, Height: 40},
				Balls: []*Ball{
					{X: 0, Y: 0, Index: 0},
					{X: 1, Y: 1, Index: 0},
					{X: 2, Y: 2, Index: 0},
				}},
			{
				Id:     "player1",
				Paddle: &Paddle{X: 15, Y: 25, Width: 35, Height: 45},
				Balls: []*Ball{
					{X: 3, Y: 3, Index: 1},
					{X: 4, Y: 4, Index: 1},
				}},
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

func TestGame_SubscribeBall(t *testing.T) {
	// create a game and a player
	game := StartGame()
	_, player := game.SubscribePlayer()

	// Create a test case for each ball in player
	for i, ball := range player.Balls {
		testCases := []struct {
			playerIndex int
			shouldBreak bool
		}{
			{i, false},
			{i + 1, true},
		}

		for _, tc := range testCases {
			// set the player index and break flag
			ball.Index = tc.playerIndex
			breakFlag := false

			// override the function to check if it breaks as expected
			game.SubscribeBall(ball)
			go func() {
				time.Sleep(time.Millisecond * 10)
				if game.Players[ball.Index] == nil && !breakFlag {
					breakFlag = true
				}
			}()
			time.Sleep(time.Millisecond * 50)
			if breakFlag != tc.shouldBreak {
				t.Errorf("Expected SubscribeBall to break %v but got %v for player index %d", tc.shouldBreak, breakFlag, tc.playerIndex)
			}
		}
	}
}

func TestGame_SubscribePaddle(t *testing.T) {
	game := StartGame()
	paddle := &Paddle{X: 10, Y: 20, Width: 30, Height: 40, Velocity: 5, Index: 3}
	game.SubscribePaddle(paddle)

	// Test case when player is connected
	if paddle.X != 10 || paddle.Y != 20 {
		t.Errorf("Expected paddle to remain at (10, 20) but got (%d, %d)", paddle.X, paddle.Y)
	}

	// Test case when player is disconnected
	game.Players[3] = nil
	time.Sleep(utils.Period)
	if paddle.X != 10 || paddle.Y != 20 {
		t.Errorf("Expected paddle to remain at (10, 20) after player disconnected, but got (%d, %d)", paddle.X, paddle.Y)
	}
}

func TestGame_SubscribePlayer(t *testing.T) {
	game := StartGame()
	playerCount := 0
	for i := 0; i < 4; i++ {
		unsubscribe, _ := game.SubscribePlayer()
		playerCount++
		if game.GetNextIndex() != playerCount {
			t.Errorf("Expected next index to be %d but got %d", playerCount, game.GetNextIndex())
		}
		if game.HasPlayer() != true {
			t.Errorf("Expected game to have a player but got %v", game.HasPlayer())
		}
		unsubscribe()
		playerCount--
		if game.GetNextIndex() != playerCount {
			t.Errorf("Expected next index to be %d but got %d", playerCount, game.GetNextIndex())
		}
	}
	if game.HasPlayer() != false {
		t.Errorf("Expected game to have no players but got %v", game.HasPlayer())
	}
}

func TestGame_UnSubscribePlayer(t *testing.T) {
	game := StartGame()
	player := NewPlayer(game.Canvas, 0, "player0")
	game.Players[0] = player
	game.UnSubscribePlayer(0)
	if game.Players[0] != nil {
		t.Errorf("Expected player to be unsubscribed but got %v", game.Players[0])
	}
	if player.Balls != nil {
		t.Errorf("Expected player balls to be set to nil but got %v", player.Balls)
	}
	if player.Paddle != nil {
		t.Errorf("Expected player paddle to be set to nil but got %v", player.Paddle)
	}
}
