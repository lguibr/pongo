// File: game/reader.go
package game

import (
	"fmt"
	// "math/rand" // No longer used in this file
	// "time" // No longer used in this file
	// "github.com/lguibr/pongo/utils" // No longer used in this file
)

// ReadBallChannel processes messages originating from a specific Ball instance.
// TODO: This logic should move into the GameActor's Receive method.
// DEPRECATED: Ball logic is now intended to be handled by BallActor sending
// position updates to GameActor, and GameActor performing collisions/reactions.
func (g *Game) ReadBallChannel(ownerIndex int, ball *Ball) {
	fmt.Printf("ReadBallChannel for Ball %d is DEPRECATED and should not be called.\n", ball.Id)
	/* // Old logic removed:
	ballChannel := ball.Channel // Removed field

	for {
		message, ok := <-ballChannel
		if !ok {
			fmt.Printf("Ball channel closed for ball ID %d (owner %d)\n", ball.Id, ownerIndex)
			return // Channel closed, exit goroutine
		}

		// fmt.Printf("Game received message from ball %d: %T\n", ball.Id, message) // Debug log

		switch payload := message.(type) {
		case BallPositionMessage:
			// This message type might be redundant if the GameActor polls or the BallActor sends updates
			currentBall := payload.Ball // Get the ball state from the message
			// Perform collision checks (this logic belongs in the GameActor)
			currentBall.CollidePaddles(g.Paddles) // Needs access to paddles (GameActor state)
			currentBall.CollideCells(g.Canvas.Grid, g.Canvas.CellSize) // Needs access to grid (GameActor state)
			currentBall.CollideWalls() // Ball checks its own boundaries

		case WallCollisionMessage:
			collidingBall := payload.Ball
			wallIndex := payload.Index
			fmt.Printf("Ball %d (owner %d) hit wall %d\n", collidingBall.Id, collidingBall.OwnerIndex, wallIndex)

			// If the wall hit corresponds to an opponent's side, adjust scores
			// This requires checking if the opponent player exists.
			if wallIndex != collidingBall.OwnerIndex && g.Players[wallIndex] != nil {
				fmt.Printf("Player %d scores point against Player %d\n", collidingBall.OwnerIndex, wallIndex)
				// Send score messages (should be sent to the PlayerActors or GameActor)
				if g.Players[collidingBall.OwnerIndex] != nil {
					g.Players[collidingBall.OwnerIndex].channel <- PlayerScore{1} // Send to scorer
				}
				g.Players[wallIndex].channel <- PlayerScore{-1} // Send to player who conceded
			} else if wallIndex == collidingBall.OwnerIndex {
				// Player hit their own wall (optional: penalty or just ignore)
				fmt.Printf("Player %d hit their own wall.\n", wallIndex)
			} else {
				// Hit a wall corresponding to an empty player slot
				fmt.Printf("Ball hit wall %d (empty slot).\n", wallIndex)
			}

		case BreakBrickMessage:
			level := payload.Level
			breakingBall := payload.BallPayload
			playerIndex := breakingBall.OwnerIndex
			fmt.Printf("Ball %d (owner %d) broke brick level %d\n", breakingBall.Id, playerIndex, level)

			if g.Players[playerIndex] != nil {
				// Award score to the player
				g.Players[playerIndex].channel <- PlayerScore{level}

				// Randomly trigger a power-up/event (logic belongs in GameActor)
				random := rand.Intn(4)
				switch random {
				case 0: // Add new ball
					fmt.Println("Triggering AddBall event")
					newBall := NewBall(
						// NewBallChannel(), // Removed
						breakingBall.X, breakingBall.Y, utils.BallSize,
						utils.CanvasSize, playerIndex, time.Now().Nanosecond(),
					)
					// Send message to GameActor to add the ball
					g.channel <- AddBall{BallPayload: newBall, ExpireIn: rand.Intn(2) + 1}
					// Start the new ball's engine (will be handled by actor spawn)
					// go g.ReadBallChannel(playerIndex, newBall) // Removed
					// go newBall.Engine() // Removed
				case 1: // Increase ball mass
					fmt.Println("Triggering IncreaseBallMass event")
					g.channel <- IncreaseBallMass{BallPayload: breakingBall, Additional: 1}
				case 2: // Increase ball velocity
					fmt.Println("Triggering IncreaseBallVelocity event")
					g.channel <- IncreaseBallVelocity{BallPayload: breakingBall, Ratio: 1.1}
				case 3: // Ball phasing
					fmt.Println("Triggering BallPhasing event")
					g.channel <- BallPhasing{BallPayload: breakingBall, ExpireIn: 1}
				}
			} else {
				fmt.Printf("Player %d not found for score update after brick break.\n", playerIndex)
			}

		default:
			fmt.Printf("Game received unknown message type from ball %d: %T\n", ball.Id, message)
			continue
		}
	}
	*/
}

// ReadPaddleChannel is DEPRECATED. Logic moved to PaddleActor.
func (playerPaddle *Paddle) ReadPaddleChannel(paddleChannel chan PaddleMessage) {
	fmt.Printf("ReadPaddleChannel for Paddle %d is DEPRECATED and should not be called.\n", playerPaddle.Index)
	// Drain channel to prevent goroutine leak if it was somehow started and channel wasn't closed
	for range paddleChannel {
	}
	fmt.Printf("Paddle channel drained/closed for paddle %d\n", playerPaddle.Index)
}

// ReadPlayerChannel processes messages related to a specific player's lifecycle and score.
// TODO: This logic should move into the GameActor's Receive method.
// It currently still relies on the deprecated game.channel for some actions.
func (g *Game) ReadPlayerChannel(
	index int,
	playerChannel chan PlayerMessage,
	paddle *Paddle, // Reference to initial paddle state (might not be needed long term)
	ball *Ball,     // Reference to initial ball state (might not be needed long term)
) {
	fmt.Printf("Starting ReadPlayerChannel for player index %d\n", index)
	for message := range playerChannel {
		// fmt.Printf("Game logic received message for player %d: %T\n", index, message) // Debug log
		switch payload := message.(type) {
		case PlayerConnectMessage:
			player := payload.PlayerPayload
			fmt.Printf("Processing PlayerConnect for index %d, ID %s\n", index, player.Id)
			// Add player and paddle state to game (should be done by GameActor)
			g.AddPlayer(index, player, paddle)
			// Add the initial ball state to game (should be done by GameActor)
			g.channel <- AddBall{BallPayload: ball, ExpireIn: 0} // Send to old game channel

		case PlayerDisconnectMessage:
			fmt.Printf("Processing PlayerDisconnect for index %d\n", index)
			// Remove player state (should be done by GameActor)
			g.RemovePlayer(index)

		case PlayerScore:
			scoreChange := payload.Score
			// Lock needed here until GameActor handles state
			g.mu.Lock()
			if g.Players[index] != nil {
				g.Players[index].Score += scoreChange
				fmt.Printf("Player %d score updated by %d to %d\n", index, scoreChange, g.Players[index].Score)
			} else {
				fmt.Printf("Attempted to update score for disconnected player %d\n", index)
			}
			g.mu.Unlock()

		default:
			fmt.Printf("Game logic received unknown message type for player %d: %T\n", index, message)
			continue
		}
	}
	fmt.Printf("Player channel closed for player index %d\n", index)
}

// ReadGameChannel processes messages sent to the main game loop/actor (DEPRECATED).
// TODO: This logic should move into the GameActor's Receive method.
func (g *Game) ReadGameChannel() {
	fmt.Println("Starting ReadGameChannel loop (DEPRECATED)")
	for message := range g.channel { // Reads from deprecated channel
		// fmt.Printf("Main game channel received message: %T\n", message) // Debug log
		switch msg := message.(type) {
		case AddBall:
			ball := msg.BallPayload
			expire := msg.ExpireIn
			fmt.Printf("Game adding ball ID %d (owner %d), expires in %d sec (DEPRECATED - Use GameActor)\n", ball.Id, ball.OwnerIndex, expire)
			g.AddBall(ball, expire) // Add ball state to game list

		case RemoveBall:
			id := msg.Id
			fmt.Printf("Game removing ball ID %d (DEPRECATED - Use GameActor)\n", id)
			g.RemoveBall(id) // Remove ball state from game list

		case IncreaseBallVelocity:
			ball := msg.BallPayload
			ratio := msg.Ratio
			fmt.Printf("Game increasing velocity for ball ID %d by %.2f (DEPRECATED - Use message to BallActor)\n", ball.Id, ratio)
			// TODO: Need to find BallActor PID and send IncreaseVelocityCommand
			// ball.IncreaseVelocity(ratio) // Incorrect - modifies potentially shared state

		case IncreaseBallMass:
			ball := msg.BallPayload
			additional := msg.Additional
			fmt.Printf("Game increasing mass for ball ID %d by %d (DEPRECATED - Use message to BallActor)\n", ball.Id, additional)
			// TODO: Need to find BallActor PID and send IncreaseMassCommand
			// ball.IncreaseMass(additional) // Incorrect

		case BallPhasing:
			ball := msg.BallPayload
			expireIn := msg.ExpireIn
			fmt.Printf("Game setting phasing for ball ID %d for %d sec (DEPRECATED - Use message to BallActor)\n", ball.Id, expireIn)
			// TODO: Need to find BallActor PID and send SetPhasingCommand
			// ball.SetBallPhasing(expireIn) // Removed

		default:
			fmt.Printf("Main game channel received unknown message type: %T\n", message)
			continue
		}
	}
	fmt.Println("Main game channel closed")
}
