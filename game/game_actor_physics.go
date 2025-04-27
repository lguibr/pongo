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
	cellSize := a.cfg.CellSize     // Use config
	canvasSize := a.cfg.CanvasSize // Use config

	ballsToRemove := []int{}      // Store IDs of balls to remove after iteration
	powerUpsToTrigger := []Ball{} // Store balls that broke bricks for powerups

	for ballID, ball := range a.balls {
		if ball == nil {
			continue
		}
		ballActorPID := a.ballActors[ballID]
		if ballActorPID == nil {
			fmt.Printf("WARN: No actor PID found for ball ID %d during collision check.\n", ballID)
			delete(a.balls, ballID)
			delete(a.ballActors, ballID)
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
			if hitWall == 0 || hitWall == 2 { // Hit side walls
				if !reflectedX {
					axisToReflect = "X"
					reflectedX = true
				}
			} else { // Hit top/bottom walls
				if !reflectedY {
					axisToReflect = "Y"
					reflectedY = true
				}
			}

			// Check if the wall belongs to an active player
			concederIndex := hitWall
			if a.players[concederIndex] != nil && a.players[concederIndex].IsConnected {
				// Wall belongs to an active player
				if axisToReflect != "" {
					a.engine.Send(ballActorPID, ReflectVelocityCommand{Axis: axisToReflect}, nil)
				}
				shouldPhase = true

				// Score update logic
				scorerIndex := originalOwner
				if concederIndex != scorerIndex {
					if a.players[scorerIndex] != nil && a.players[scorerIndex].IsConnected {
						a.players[scorerIndex].Score++
					}
					a.players[concederIndex].Score--
				}
			} else {
				// Wall belongs to an empty slot
				if ball.IsPermanent {
					// Reflect permanent balls instead of removing them
					fmt.Printf("GameActor: Permanent Ball %d hit empty wall %d. Reflecting.\n", ballID, hitWall)
					if axisToReflect != "" {
						a.engine.Send(ballActorPID, ReflectVelocityCommand{Axis: axisToReflect}, nil)
					}
					shouldPhase = true
				} else {
					// Remove temporary balls
					fmt.Printf("GameActor: Temporary Ball %d hit empty wall %d. Removing.\n", ballID, hitWall)
					ballsToRemove = append(ballsToRemove, ballID)
					continue // Skip other collision checks for this ball
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
				if speed < float64(a.cfg.MinBallVelocity) { // Use config
					speed = float64(a.cfg.MinBallVelocity)
				}

				paddleCenterX := float64(paddle.X + paddle.Width/2)
				paddleCenterY := float64(paddle.Y + paddle.Height/2)
				hitOffsetX := float64(ball.X) - paddleCenterX
				hitOffsetY := float64(ball.Y) - paddleCenterY

				normOffset := 0.0
				if paddle.Index%2 == 0 {
					if paddle.Height > 0 {
						normOffset = hitOffsetY / (float64(paddle.Height) / 2.0)
					}
				} else {
					if paddle.Width > 0 {
						normOffset = hitOffsetX / (float64(paddle.Width) / 2.0)
					}
				}
				normOffset = math.Max(-1.0, math.Min(1.0, normOffset))

				vBaseX := vInX
				vBaseY := vInY
				if paddle.Index%2 == 0 {
					if !reflectedX {
						vBaseX = -vInX
						reflectedX = true
					}
				} else {
					if !reflectedY {
						vBaseY = -vInY
						reflectedY = true
					}
				}

				// Use config for angle deflection
				maxAngleDeflection := math.Pi / a.cfg.BallHitPaddleAngleFactor
				maxComponentChange := speed * math.Sin(maxAngleDeflection)

				vFinalX := vBaseX
				vFinalY := vBaseY

				if paddle.Index%2 == 0 {
					vyChange := normOffset * maxComponentChange
					vFinalY = vBaseY + vyChange
				} else {
					vxChange := normOffset * maxComponentChange
					vFinalX = vBaseX + vxChange
				}

				finalDirLen := math.Sqrt(vFinalX*vFinalX + vFinalY*vFinalY)
				if finalDirLen > 0 {
					vFinalX /= finalDirLen
					vFinalY /= finalDirLen
				} else {
					baseLen := math.Sqrt(vBaseX*vBaseX + vBaseY*vBaseY)
					if baseLen > 0 {
						vFinalX = vBaseX / baseLen
						vFinalY = vBaseY / baseLen
					} else {
						vFinalX = -hitOffsetX
						vFinalY = -hitOffsetY
						failsafeLen := math.Sqrt(vFinalX*vFinalX + vFinalY*vFinalY)
						if failsafeLen > 0 {
							vFinalX /= failsafeLen
							vFinalY /= failsafeLen
						} else {
							vFinalX = 0
							vFinalY = 0
							if paddle.Index == 1 {
								vFinalY = 1
							}
							if paddle.Index == 3 {
								vFinalY = -1
							}
							if paddle.Index == 0 {
								vFinalX = -1
							}
							if paddle.Index == 2 {
								vFinalX = 1
							}
						}
					}
				}

				// Use config for paddle speed influence
				paddleVelFactor := a.cfg.BallHitPaddleSpeedFactor
				paddleVelAlongHit := float64(paddle.Vx)*vFinalX + float64(paddle.Vy)*vFinalY
				targetSpeed := speed + (paddleVelAlongHit * paddleVelFactor)

				minSpeedAfterHit := float64(a.cfg.MinBallVelocity) // Use config
				if targetSpeed < minSpeedAfterHit {
					targetSpeed = minSpeedAfterHit
				}

				vFinalX *= targetSpeed
				vFinalY *= targetSpeed

				a.engine.Send(ballActorPID, SetVelocityCommand{Vx: int(vFinalX), Vy: int(vFinalY)}, nil)

				ball.OwnerIndex = paddleIndex
				shouldPhase = true
				goto nextBall
			}
		}

		// 3. Brick Collisions
		if !ball.Phasing {
			collidedCells := a.findCollidingCells(ball, cellSize)
			for _, cellPos := range collidedCells {
				col, row := cellPos[0], cellPos[1]
				if col < 0 || col >= a.cfg.GridSize || row < 0 || row >= a.cfg.GridSize { // Use config
					continue
				}
				cell := &a.canvas.Grid[col][row]

				if cell.Data.Type == utils.Cells.Brick {
					brickLevel := cell.Data.Level
					cell.Data.Life--

					dx := float64(ball.X - (col*cellSize + cellSize/2))
					dy := float64(ball.Y - (row*cellSize + cellSize/2))
					if math.Abs(dx) > math.Abs(dy) {
						if !reflectedX {
							a.engine.Send(ballActorPID, ReflectVelocityCommand{Axis: "X"}, nil)
							reflectedX = true
						}
					} else {
						if !reflectedY {
							a.engine.Send(ballActorPID, ReflectVelocityCommand{Axis: "Y"}, nil)
							reflectedY = true
						}
					}

					if cell.Data.Life <= 0 {
						fmt.Printf("GameActor: Brick broken at [%d, %d]\n", col, row)
						cell.Data.Type = utils.Cells.Empty
						cell.Data.Level = 0

						scorerIndex := ball.OwnerIndex
						if a.players[scorerIndex] != nil && a.players[scorerIndex].IsConnected {
							a.players[scorerIndex].Score += brickLevel
						}

						// Use config for power-up chance
						if rand.Float64() < a.cfg.PowerUpChance {
							powerUpsToTrigger = append(powerUpsToTrigger, *ball)
						}
					}

					shouldPhase = true
					goto nextBall
				}
			}
		}

	nextBall:
		if shouldPhase {
			// Use config for phasing time
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
		}

		// Stop actors outside lock
		a.mu.Unlock() // Release main lock temporarily
		for _, pid := range pidsToStop {
			a.engine.Stop(pid)
		}
		a.mu.Lock() // Re-acquire lock

		// Remove from maps
		for _, ballID := range ballsToRemove {
			delete(a.balls, ballID)
			delete(a.ballActors, ballID)
		}
	}

	// Trigger power-ups
	for _, ballState := range powerUpsToTrigger {
		a.triggerRandomPowerUp(ctx, &ballState)
	}
}

// findCollidingCells checks which grid cells the ball might be overlapping with.
// NOTE: Assumes GameActor mutex is held.
func (a *GameActor) findCollidingCells(ball *Ball, cellSize int) [][2]int {
	collided := [][2]int{}
	gridSize := a.cfg.GridSize // Use config
	if cellSize <= 0 || gridSize <= 0 {
		return collided
	}

	minCol := (ball.X - ball.Radius) / cellSize
	maxCol := (ball.X + ball.Radius) / cellSize
	minRow := (ball.Y - ball.Radius) / cellSize
	maxRow := (ball.Y + ball.Radius) / cellSize

	minCol = utils.MaxInt(0, minCol)
	maxCol = utils.MinInt(gridSize-1, maxCol)
	minRow = utils.MaxInt(0, minRow)
	maxRow = utils.MinInt(gridSize-1, maxRow)

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

	ballActorPID := a.ballActors[ball.Id]
	selfPID := a.selfPID

	if ballActorPID == nil || selfPID == nil {
		return
	}

	switch powerUpType {
	case 0: // SpawnBall
		// Send command to self to spawn the ball (temporary ball)
		a.engine.Send(selfPID, SpawnBallCommand{
			OwnerIndex:  ball.OwnerIndex,
			X:           ball.X,
			Y:           ball.Y,
			ExpireIn:    a.cfg.PowerUpSpawnBallExpiry, // Use config (will be randomized in spawnBall)
			IsPermanent: false,                        // Power-up balls are temporary
		}, nil)
	case 1: // IncreaseMass
		// Use config for mass amount
		a.engine.Send(ballActorPID, IncreaseMassCommand{Additional: a.cfg.PowerUpIncreaseMassAdd}, nil)
	case 2: // IncreaseVelocity
		// Use config for velocity ratio
		a.engine.Send(ballActorPID, IncreaseVelocityCommand{Ratio: a.cfg.PowerUpIncreaseVelRatio}, nil)
	}
}
