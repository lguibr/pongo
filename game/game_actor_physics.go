// File: game/game_actor_physics.go
package game

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/lguibr/pongo/bollywood"
	"github.com/lguibr/pongo/utils"
)

// detectCollisions checks for and handles collisions between balls, walls, paddles, and bricks.
// NOTE: This method assumes it's called with the GameActor's mutex locked.
func (a *GameActor) detectCollisions(ctx bollywood.Context) {
	cellSize := a.canvas.CellSize
	canvasSize := a.canvas.CanvasSize

	ballsToRemove := []int{}      // Store IDs of balls to remove after iteration
	powerUpsToTrigger := []Ball{} // Store balls that broke bricks for powerups

	for ballID, ball := range a.balls {
		if ball == nil {
			continue // Skip nil balls if any exist temporarily
		}
		ballActorPID := a.ballActors[ballID]
		if ballActorPID == nil {
			fmt.Printf("WARN: No actor PID found for ball ID %d during collision check.\n", ballID)
			delete(a.balls, ballID) // Clean up inconsistent state
			delete(a.ballActors, ballID)
			continue
		}

		// --- Create copies for modification checks ---
		originalOwner := ball.OwnerIndex
		shouldPhase := false
		reflectedX := false
		reflectedY := false

		// 1. Wall Collisions
		hitWall := -1 // 0: Right, 1: Top, 2: Left, 3: Bottom
		if ball.X+ball.Radius >= canvasSize {
			hitWall = 0
		} else if ball.Y-ball.Radius <= 0 {
			hitWall = 1
		} else if ball.X-ball.Radius <= 0 {
			hitWall = 2
		} else if ball.Y+ball.Radius >= canvasSize {
			hitWall = 3
		}

		if hitWall != -1 {
			// Reflect velocity command
			if hitWall == 0 || hitWall == 2 { // Hit side walls
				a.engine.Send(ballActorPID, ReflectVelocityCommand{Axis: "X"}, nil)
				reflectedX = true // Mark as reflected this tick
			} else { // Hit top/bottom walls
				a.engine.Send(ballActorPID, ReflectVelocityCommand{Axis: "Y"}, nil)
				reflectedY = true // Mark as reflected this tick
			}
			shouldPhase = true

			// Score update logic
			scorerIndex := originalOwner
			concederIndex := hitWall

			// Check if the wall belongs to an active player
			if a.players[concederIndex] != nil {
				if concederIndex != scorerIndex {
					// fmt.Printf("GameActor: Player %d scores against Player %d\n", scorerIndex, concederIndex) // Reduce noise
					if a.players[scorerIndex] != nil { // Check scorer still exists
						a.players[scorerIndex].Score++
					}
					a.players[concederIndex].Score--
				} else {
					// Player hit their own wall - no score change
					// fmt.Printf("GameActor: Player %d hit their own wall.\n", scorerIndex) // Reduce noise
				}
			} else {
				// Wall belongs to an empty slot - remove the ball
				fmt.Printf("GameActor: Ball %d hit empty wall %d. Removing.\n", ballID, hitWall)
				ballsToRemove = append(ballsToRemove, ballID)
				continue // Skip other collision checks for this ball
			}
		}

		// 2. Paddle Collisions
		for paddleIndex, paddle := range a.paddles {
			if paddle == nil {
				continue
			}
			if ball.BallInterceptPaddles(paddle) {
				// fmt.Printf("GameActor: Ball %d collided with Paddle %d\n", ballID, paddleIndex) // Reduce noise
				if paddleIndex%2 == 0 { // Vertical paddles (0, 2) reflect X
					if !reflectedX { // Avoid double reflection if wall hit same tick
						a.engine.Send(ballActorPID, ReflectVelocityCommand{Axis: "X"}, nil)
						reflectedX = true
					}
				} else { // Horizontal paddles (1, 3) reflect Y
					if !reflectedY { // Avoid double reflection
						a.engine.Send(ballActorPID, ReflectVelocityCommand{Axis: "Y"}, nil)
						reflectedY = true
					}
				}
				ball.OwnerIndex = paddleIndex // Update ownership IN GAME ACTOR STATE
				shouldPhase = true
				goto nextBall // Ball can only hit one paddle per tick
			}
		}

		// 3. Brick Collisions (only if not phasing)
		if !ball.Phasing {
			collidedCells := a.findCollidingCells(ball, cellSize)
			for _, cellPos := range collidedCells {
				col, row := cellPos[0], cellPos[1]
				// Basic bounds check (should be redundant if findCollidingCells is correct)
				if col < 0 || col >= a.canvas.GridSize || row < 0 || row >= a.canvas.GridSize {
					continue
				}
				cell := &a.canvas.Grid[col][row] // Get pointer to modify grid directly

				if cell.Data.Type == utils.Cells.Brick {
					// fmt.Printf("GameActor: Ball %d hit brick at [%d, %d] (Life: %d)\n", ballID, col, row, cell.Data.Life) // Reduce noise
					brickLevel := cell.Data.Level // Store original level for scoring
					cell.Data.Life--

					// Reflect velocity based on relative position (simple approach)
					dx := float64(ball.X - (col*cellSize + cellSize/2))
					dy := float64(ball.Y - (row*cellSize + cellSize/2))
					if math.Abs(dx) > math.Abs(dy) { // Hit more horizontally
						if !reflectedX {
							a.engine.Send(ballActorPID, ReflectVelocityCommand{Axis: "X"}, nil)
							reflectedX = true
						}
					} else { // Hit more vertically
						if !reflectedY {
							a.engine.Send(ballActorPID, ReflectVelocityCommand{Axis: "Y"}, nil)
							reflectedY = true
						}
					}

					// Handle brick destruction
					if cell.Data.Life <= 0 {
						fmt.Printf("GameActor: Brick broken at [%d, %d]\n", col, row)
						cell.Data.Type = utils.Cells.Empty
						cell.Data.Level = 0 // Reset level as well

						// Award score
						scorerIndex := ball.OwnerIndex
						if a.players[scorerIndex] != nil {
							a.players[scorerIndex].Score += brickLevel
							// fmt.Printf("GameActor: Player %d score +%d for breaking brick.\n", scorerIndex, brickLevel) // Reduce noise
						}

						// Trigger power-up?
						if rand.Intn(4) == 0 { // ~25% chance
							powerUpsToTrigger = append(powerUpsToTrigger, *ball) // Store ball state for triggering later
						}
					}

					shouldPhase = true
					goto nextBall // Ball only interacts with one brick per tick
				}
			}
		}

	nextBall:
		// Send phasing command if needed
		if shouldPhase {
			a.engine.Send(ballActorPID, SetPhasingCommand{ExpireIn: 100 * time.Millisecond}, nil)
		}

	} // End ball loop

	// --- Post-Loop Actions ---

	// Remove balls marked for removal (e.g., hit empty walls)
	if len(ballsToRemove) > 0 {
		// Using Option 1: Stop actors first, then lock and remove from maps.
		pidsToStop := make([]*bollywood.PID, 0, len(ballsToRemove))
		// Need RLock here as we are reading ballActors map
		// This was called within the main lock, so no extra lock needed here.
		for _, ballID := range ballsToRemove {
			if pid, ok := a.ballActors[ballID]; ok && pid != nil {
				pidsToStop = append(pidsToStop, pid)
			}
		}

		// Stop actors outside lock (release main lock temporarily)
		a.mu.Unlock()
		for _, pid := range pidsToStop {
			// fmt.Printf("GameActor: Ball went out of bounds, stopping actor %s\n", pid) // Reduce noise
			a.engine.Stop(pid)
		}
		a.mu.Lock() // Re-acquire lock

		// Remove from maps now that actors are stopping
		for _, ballID := range ballsToRemove {
			delete(a.balls, ballID)
			delete(a.ballActors, ballID)
		}
	}

	// Trigger power-ups (still within the main lock)
	for _, ballState := range powerUpsToTrigger {
		a.triggerRandomPowerUp(ctx, &ballState) // Pass context and ball state
	}
}

// findCollidingCells checks which grid cells the ball might be overlapping with.
// NOTE: Assumes GameActor mutex is held.
func (a *GameActor) findCollidingCells(ball *Ball, cellSize int) [][2]int {
	collided := [][2]int{}
	gridSize := a.canvas.GridSize
	if cellSize <= 0 || gridSize <= 0 {
		return collided // Avoid division by zero or invalid grid
	}

	// Determine the range of grid cells the ball's bounding box overlaps
	minCol := (ball.X - ball.Radius) / cellSize
	maxCol := (ball.X + ball.Radius) / cellSize
	minRow := (ball.Y - ball.Radius) / cellSize
	maxRow := (ball.Y + ball.Radius) / cellSize

	// Clamp the range to valid grid indices
	minCol = utils.MaxInt(0, minCol)
	maxCol = utils.MinInt(gridSize-1, maxCol)
	minRow = utils.MaxInt(0, minRow)
	maxRow = utils.MinInt(gridSize-1, maxRow)

	// Check intersection for each cell in the potential range
	for c := minCol; c <= maxCol; c++ {
		for r := minRow; r <= maxRow; r++ {
			if ball.InterceptsIndex(c, r, cellSize) {
				collided = append(collided, [2]int{c, r})
			}
		}
	}
	return collided
}

// triggerRandomPowerUp sends a command for a power-up effect.
// NOTE: Assumes GameActor mutex is held.
func (a *GameActor) triggerRandomPowerUp(ctx bollywood.Context, ball *Ball) {
	powerUpType := rand.Intn(3) // 0: SpawnBall, 1: IncreaseMass, 2: IncreaseVelocity

	ballActorPID := a.ballActors[ball.Id] // Already locked, safe to access
	selfPID := a.selfPID

	if ballActorPID == nil {
		// fmt.Printf("WARN: Cannot trigger power-up for ball %d, actor PID not found (likely already removed).\n", ball.Id) // Reduce noise
		return
	}
	if selfPID == nil {
		fmt.Printf("WARN: Cannot trigger power-up for ball %d, GameActor selfPID is nil.\n", ball.Id)
		return
	}

	switch powerUpType {
	case 0: // SpawnBall
		// fmt.Printf("GameActor: Triggering SpawnBall power-up from ball %d\n", ball.Id) // Reduce noise
		// Send command to self to spawn the ball
		a.engine.Send(selfPID, SpawnBallCommand{
			OwnerIndex: ball.OwnerIndex,
			X:          ball.X, // Spawn near the breaking ball
			Y:          ball.Y,
			ExpireIn:   time.Duration(rand.Intn(5)+5) * time.Second, // Random expiry 5-9s
		}, nil) // Sender is nil or selfPID
	case 1: // IncreaseMass
		// fmt.Printf("GameActor: Triggering IncreaseMass power-up for ball %d\n", ball.Id) // Reduce noise
		a.engine.Send(ballActorPID, IncreaseMassCommand{Additional: 1}, nil)
	case 2: // IncreaseVelocity
		// fmt.Printf("GameActor: Triggering IncreaseVelocity power-up for ball %d\n", ball.Id) // Reduce noise
		a.engine.Send(ballActorPID, IncreaseVelocityCommand{Ratio: 1.1}, nil)
	}
}
