// File: game/game_actor_physics.go
package game

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
)

// detectCollisions checks for and handles collisions between balls, walls, paddles, and bricks.
// NOTE: This method assumes it's called with the GameActor's mutex locked.
func (a *GameActor) detectCollisions(ctx bollywood.Context) {
	cellSize := a.cfg.CellSize
	canvasSize := a.cfg.CanvasSize

	ballsToRemove := []int{}
	powerUpsToTrigger := []Ball{}

	for ballID, ball := range a.balls {
		if ball == nil {
			continue
		}
		ballActorPID := a.ballActors[ballID]
		if ballActorPID == nil {
			fmt.Printf("WARN: No actor PID found for ball ID %d during collision check.\n", ballID)
			delete(a.balls, ballID) // Clean up inconsistent state
			// No need to delete from ballActors here as it's already missing
			continue
		}

		originalOwner := ball.OwnerIndex
		shouldPhase := false
		reflectedX := false
		reflectedY := false

		// 1. Wall Collisions
		hitWall := -1
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
			axisToReflect := ""
			if hitWall == 0 || hitWall == 2 {
				if !reflectedX {
					axisToReflect = "X"
					reflectedX = true
				}
			} else {
				if !reflectedY {
					axisToReflect = "Y"
					reflectedY = true
				}
			}

			concederIndex := hitWall
			if a.players[concederIndex] != nil && a.players[concederIndex].IsConnected {
				if axisToReflect != "" {
					a.engine.Send(ballActorPID, ReflectVelocityCommand{Axis: axisToReflect}, nil)
				}
				shouldPhase = true

				scorerIndex := originalOwner
				if concederIndex != scorerIndex {
					if a.players[scorerIndex] != nil && a.players[scorerIndex].IsConnected {
						a.players[scorerIndex].Score++
					}
					a.players[concederIndex].Score--
				}
			} else {
				if ball.IsPermanent {
					// fmt.Printf("GameActor: Permanent Ball %d hit empty wall %d. Reflecting.\n", ballID, hitWall) // Reduce noise
					if axisToReflect != "" {
						a.engine.Send(ballActorPID, ReflectVelocityCommand{Axis: axisToReflect}, nil)
					}
					shouldPhase = true
				} else {
					// fmt.Printf("GameActor: Temporary Ball %d hit empty wall %d. Removing.\n", ballID, hitWall) // Reduce noise
					ballsToRemove = append(ballsToRemove, ballID)
					continue
				}
			}
		}

		// 2. Paddle Collisions
		for paddleIndex, paddle := range a.paddles {
			if paddle == nil || a.players[paddleIndex] == nil || !a.players[paddleIndex].IsConnected {
				continue
			}
			if ball.BallInterceptPaddles(paddle) {
				vInX := float64(ball.Vx)
				vInY := float64(ball.Vy)
				speed := math.Sqrt(vInX*vInX + vInY*vInY)
				if speed < float64(a.cfg.MinBallVelocity) {
					speed = float64(a.cfg.MinBallVelocity)
				}

				paddleCenterX := float64(paddle.X + paddle.Width/2)
				paddleCenterY := float64(paddle.Y + paddle.Height/2)
				hitOffsetX := float64(ball.X) - paddleCenterX
				hitOffsetY := float64(ball.Y) - paddleCenterY

				normOffset := 0.0
				if paddle.Index%2 == 0 { // Vertical paddles
					if paddle.Height > 0 {
						normOffset = hitOffsetY / (float64(paddle.Height) / 2.0)
					}
				} else { // Horizontal paddles
					if paddle.Width > 0 {
						normOffset = hitOffsetX / (float64(paddle.Width) / 2.0)
					}
				}
				normOffset = math.Max(-1.0, math.Min(1.0, normOffset)) // Clamp to [-1, 1]

				// Base reflection (reflect perpendicular to paddle face)
				vBaseX := vInX
				vBaseY := vInY
				if paddle.Index%2 == 0 { // Vertical paddle -> reflect X
					if !reflectedX {
						vBaseX = -vInX
						reflectedX = true
					}
				} else { // Horizontal paddle -> reflect Y
					if !reflectedY {
						vBaseY = -vInY
						reflectedY = true
					}
				}

				// Angle deflection based on hit offset
				maxAngleDeflection := math.Pi / a.cfg.BallHitPaddleAngleFactor
				maxComponentChange := speed * math.Sin(maxAngleDeflection) // Max change in the parallel component

				vFinalX := vBaseX
				vFinalY := vBaseY

				if paddle.Index%2 == 0 { // Vertical paddle -> change Y based on offset
					vyChange := normOffset * maxComponentChange
					vFinalY = vBaseY + vyChange
				} else { // Horizontal paddle -> change X based on offset
					vxChange := normOffset * maxComponentChange
					vFinalX = vBaseX + vxChange
				}

				// Normalize the deflected direction vector
				finalDirLen := math.Sqrt(vFinalX*vFinalX + vFinalY*vFinalY)
				if finalDirLen > 0 {
					vFinalX /= finalDirLen
					vFinalY /= finalDirLen
				} else {
					// Failsafe: if somehow direction became zero, use reflection direction
					baseLen := math.Sqrt(vBaseX*vBaseX + vBaseY*vBaseY)
					if baseLen > 0 {
						vFinalX = vBaseX / baseLen
						vFinalY = vBaseY / baseLen
					} else { // Even more failsafe: direct away from paddle center
						vFinalX = -hitOffsetX
						vFinalY = -hitOffsetY
						failsafeLen := math.Sqrt(vFinalX*vFinalX + vFinalY*vFinalY)
						if failsafeLen > 0 {
							vFinalX /= failsafeLen
							vFinalY /= failsafeLen
						} else { // Absolute failsafe: default direction based on paddle index
							vFinalX = 0
							vFinalY = 0
							switch paddle.Index {
							case 0:
								vFinalX = -1 // Right paddle -> move left
							case 1:
								vFinalY = 1 // Top paddle -> move down
							case 2:
								vFinalX = 1 // Left paddle -> move right
							case 3:
								vFinalY = -1 // Bottom paddle -> move up
							}
						}
					}
				}

				// Adjust speed based on paddle movement
				paddleVelFactor := a.cfg.BallHitPaddleSpeedFactor
				// Project paddle velocity onto the ball's final direction vector
				paddleVelAlongHit := float64(paddle.Vx)*vFinalX + float64(paddle.Vy)*vFinalY
				targetSpeed := speed + (paddleVelAlongHit * paddleVelFactor)

				// Clamp speed to min/max (optional max clamping)
				minSpeedAfterHit := float64(a.cfg.MinBallVelocity)
				if targetSpeed < minSpeedAfterHit {
					targetSpeed = minSpeedAfterHit
				}
				// Optional: Clamp max speed
				// maxSpeedAfterHit := float64(a.cfg.MaxBallVelocity) * 1.5 // Example max
				// if targetSpeed > maxSpeedAfterHit {
				// 	targetSpeed = maxSpeedAfterHit
				// }

				// Final velocity vector
				vFinalX *= targetSpeed
				vFinalY *= targetSpeed

				// *** Ensure final components are not zero ***
				finalVxInt := int(vFinalX)
				finalVyInt := int(vFinalY)

				if finalVxInt == 0 {
					finalVxInt = int(math.Copysign(1.0, vFinalX)) // Use sign of float, default to 1 if float was also 0
					if finalVxInt == 0 {
						finalVxInt = 1
					}
				}
				if finalVyInt == 0 {
					finalVyInt = int(math.Copysign(1.0, vFinalY)) // Use sign of float, default to 1 if float was also 0
					if finalVyInt == 0 {
						finalVyInt = 1
					}
				}

				// Send command to BallActor
				a.engine.Send(ballActorPID, SetVelocityCommand{Vx: finalVxInt, Vy: finalVyInt}, nil)

				ball.OwnerIndex = paddleIndex // Update owner
				shouldPhase = true
				goto nextBall // Skip brick collision check after paddle hit
			}
		}

		// 3. Brick Collisions
		if !ball.Phasing {
			collidedCells := a.findCollidingCells(ball, cellSize)
			for _, cellPos := range collidedCells {
				col, row := cellPos[0], cellPos[1]
				if col < 0 || col >= a.cfg.GridSize || row < 0 || row >= a.cfg.GridSize {
					continue
				}
				cell := &a.canvas.Grid[col][row] // Get pointer to cell

				if cell.Data.Type == utils.Cells.Brick {
					brickLevel := cell.Data.Level
					cell.Data.Life--

					// Determine reflection axis based on relative position
					// (Simplified: reflect based on which side is closer)
					dx := float64(ball.X - (col*cellSize + cellSize/2))
					dy := float64(ball.Y - (row*cellSize + cellSize/2))
					if math.Abs(dx) > math.Abs(dy) { // Hit was more horizontal
						if !reflectedX {
							a.engine.Send(ballActorPID, ReflectVelocityCommand{Axis: "X"}, nil)
							reflectedX = true
						}
					} else { // Hit was more vertical
						if !reflectedY {
							a.engine.Send(ballActorPID, ReflectVelocityCommand{Axis: "Y"}, nil)
							reflectedY = true
						}
					}

					if cell.Data.Life <= 0 {
						// fmt.Printf("GameActor: Brick broken at [%d, %d]\n", col, row) // Reduce noise
						cell.Data.Type = utils.Cells.Empty
						cell.Data.Level = 0

						scorerIndex := ball.OwnerIndex
						if a.players[scorerIndex] != nil && a.players[scorerIndex].IsConnected {
							a.players[scorerIndex].Score += brickLevel
						}

						if rand.Float64() < a.cfg.PowerUpChance {
							powerUpsToTrigger = append(powerUpsToTrigger, *ball) // Copy ball state at time of break
						}
					}

					shouldPhase = true
					goto nextBall // Only handle one brick collision per tick
				}
			}
		}

	nextBall: // Label for skipping to next ball iteration
		if shouldPhase {
			a.engine.Send(ballActorPID, SetPhasingCommand{ExpireIn: a.cfg.BallPhasingTime}, nil)
		}

	} // End ball loop

	// --- Post-Loop Actions ---
	if len(ballsToRemove) > 0 {
		pidsToStop := make([]*bollywood.PID, 0, len(ballsToRemove))
		// Lock is already held here
		for _, ballID := range ballsToRemove {
			if pid, ok := a.ballActors[ballID]; ok && pid != nil {
				pidsToStop = append(pidsToStop, pid)
			}
			// Clean up state maps immediately while lock is held
			delete(a.balls, ballID)
			delete(a.ballActors, ballID)
		}

		// Stop actors outside lock
		a.mu.Unlock() // Release main lock temporarily
		for _, pid := range pidsToStop {
			a.engine.Stop(pid)
		}
		a.mu.Lock() // Re-acquire lock
	}

	// Trigger power-ups (lock is held)
	for _, ballState := range powerUpsToTrigger {
		a.triggerRandomPowerUp(ctx, &ballState)
	}
}

// findCollidingCells checks which grid cells the ball might be overlapping with.
// NOTE: Assumes GameActor mutex is held.
func (a *GameActor) findCollidingCells(ball *Ball, cellSize int) [][2]int {
	collided := [][2]int{}
	gridSize := a.cfg.GridSize
	if cellSize <= 0 || gridSize <= 0 {
		return collided
	}

	// Calculate the bounding box of the ball in terms of grid cells
	minCol := (ball.X - ball.Radius) / cellSize
	maxCol := (ball.X + ball.Radius) / cellSize
	minRow := (ball.Y - ball.Radius) / cellSize
	maxRow := (ball.Y + ball.Radius) / cellSize

	// Clamp to grid boundaries
	minCol = utils.MaxInt(0, minCol)
	maxCol = utils.MinInt(gridSize-1, maxCol)
	minRow = utils.MaxInt(0, minRow)
	maxRow = utils.MinInt(gridSize-1, maxRow)

	// Check intersection for cells within the bounding box
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
// NOTE: Assumes GameActor mutex is held. Uses config values.
func (a *GameActor) triggerRandomPowerUp(ctx bollywood.Context, ball *Ball) {
	powerUpType := rand.Intn(3) // 0: SpawnBall, 1: IncreaseMass, 2: IncreaseVelocity

	ballActorPID := a.ballActors[ball.Id] // Get PID based on the ball that broke the brick
	selfPID := a.selfPID

	// Check if the original ball still exists (it might have been removed simultaneously)
	if _, ok := a.balls[ball.Id]; !ok || ballActorPID == nil {
		// fmt.Printf("GameActor: Skipping power-up trigger; original ball %d no longer exists.\n", ball.Id) // Reduce noise
		return
	}

	if selfPID == nil {
		fmt.Printf("ERROR: GameActor cannot trigger power-up, self PID is nil.\n")
		return
	}

	switch powerUpType {
	case 0: // SpawnBall
		a.engine.Send(selfPID, SpawnBallCommand{
			OwnerIndex:  ball.OwnerIndex,
			X:           ball.X, // Spawn near the broken brick
			Y:           ball.Y,
			ExpireIn:    a.cfg.PowerUpSpawnBallExpiry,
			IsPermanent: false,
		}, nil)
	case 1: // IncreaseMass (apply to the ball that broke the brick)
		a.engine.Send(ballActorPID, IncreaseMassCommand{Additional: a.cfg.PowerUpIncreaseMassAdd}, nil)
	case 2: // IncreaseVelocity (apply to the ball that broke the brick)
		a.engine.Send(ballActorPID, IncreaseVelocityCommand{Ratio: a.cfg.PowerUpIncreaseVelRatio}, nil)
	}
}
