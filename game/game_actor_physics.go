package game

import (
	"math"
	"math/rand"

	// Keep time for power-up expiry
	"github.com/lguibr/bollywood" // Keep bollywood for context/engine access
	"github.com/lguibr/pongo/utils"
)

// detectCollisions checks for and handles collisions using the GameActor's cached state.
// Assumes called within the actor's message loop (no lock needed for main state).
func (a *GameActor) detectCollisions(ctx bollywood.Context) {
	currentEngine := a.engine
	if currentEngine == nil {
		return // Should not happen if actor is running
	}

	// --- Use local caches directly ---
	activeBalls := a.balls
	activePaddles := a.paddles // This is the cache [utils.MaxPlayers]*Paddle

	cellSize := a.cfg.CellSize
	canvasSize := a.cfg.CanvasSize
	ballsToRemove := []int{}
	powerUpsToTrigger := []Ball{} // Store copies of ball state for power-ups

	for ballID, ball := range activeBalls {
		// Ensure ball and its actor PID exist before proceeding
		ballActorPID, pidExists := a.ballActors[ballID]
		if ball == nil || !pidExists || ballActorPID == nil {
			// Clean up inconsistent state if necessary
			if ball == nil && pidExists {
				// fmt.Printf("WARN: GameActor %s: Ball state nil for existing actor PID %s (BallID %d). Removing actor.\n", a.selfPID, ballActorPID, ballID) // Reduce noise
				delete(a.ballActors, ballID)
				currentEngine.Stop(ballActorPID)
			} else if ball != nil && !pidExists {
				// fmt.Printf("WARN: GameActor %s: Ball state exists for non-existent actor PID (BallID %d). Removing state.\n", a.selfPID, ballID) // Reduce noise
				delete(a.balls, ballID)
			}
			continue
		}

		shouldPhase := false
		reflectedX := false
		reflectedY := false

		// 4.1 Wall Collisions
		hitWall := -1
		if ball.X+ball.Radius >= canvasSize {
			hitWall = 0 // Right wall
		} else if ball.Y-ball.Radius <= 0 {
			hitWall = 1 // Top wall
		} else if ball.X-ball.Radius <= 0 {
			hitWall = 2 // Left wall
		} else if ball.Y+ball.Radius >= canvasSize {
			hitWall = 3 // Bottom wall
		}

		if hitWall != -1 {
			axisToReflect := ""
			// Adjust position *before* reflecting velocity
			switch hitWall {
			case 0:
				ball.X = canvasSize - ball.Radius
				if !reflectedX {
					axisToReflect = "X"
					reflectedX = true
				}
			case 1:
				ball.Y = ball.Radius
				if !reflectedY {
					axisToReflect = "Y"
					reflectedY = true
				}
			case 2:
				ball.X = ball.Radius
				if !reflectedX {
					axisToReflect = "X"
					reflectedX = true
				}
			case 3:
				ball.Y = canvasSize - ball.Radius
				if !reflectedY {
					axisToReflect = "Y"
					reflectedY = true
				}
			}

			concederIndex := hitWall
			// Check if the wall belongs to an active player
			isPlayerWall := concederIndex >= 0 && concederIndex < len(a.players) && a.players[concederIndex] != nil && a.players[concederIndex].IsConnected

			if isPlayerWall {
				pInfo := a.players[concederIndex]
				if axisToReflect != "" {
					currentEngine.Send(ballActorPID, ReflectVelocityCommand{Axis: axisToReflect}, nil)
				}
				shouldPhase = true
				scorerIndex := ball.OwnerIndex
				isScorerValid := scorerIndex >= 0 && scorerIndex < len(a.players) && a.players[scorerIndex] != nil && a.players[scorerIndex].IsConnected

				if isScorerValid && scorerIndex != concederIndex {
					scInfo := a.players[scorerIndex]
					scInfo.Score.Add(1)
					pInfo.Score.Add(-1)
				} else if scorerIndex == -1 { // Ownerless ball
					pInfo.Score.Add(-1)
				} else if scorerIndex == concederIndex { // Hit own wall
					pInfo.Score.Add(-1)
				}
			} else { // Empty slot wall hit
				if ball.IsPermanent {
					if axisToReflect != "" {
						currentEngine.Send(ballActorPID, ReflectVelocityCommand{Axis: axisToReflect}, nil)
					}
					shouldPhase = true
				} else {
					ballsToRemove = append(ballsToRemove, ballID)
					continue // Skip other collision checks for this ball
				}
			}
		} // End wall collision check

		// 4.2 Paddle Collisions
		for paddleIndex, paddle := range activePaddles {
			// Check if paddle exists in cache and belongs to an active player
			if paddle == nil || !(paddleIndex >= 0 && paddleIndex < len(a.players) && a.players[paddleIndex] != nil && a.players[paddleIndex].IsConnected) {
				continue
			}

			if !ball.Phasing && ball.BallInterceptPaddles(paddle) {
				// --- Paddle Collision Physics (same as before) ---
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
				if paddle.Index%2 == 0 { // Vertical paddles (0, 2)
					if paddle.Height > 0 {
						normOffset = hitOffsetY / (float64(paddle.Height) / 2.0)
					}
				} else { // Horizontal paddles (1, 3)
					if paddle.Width > 0 {
						normOffset = hitOffsetX / (float64(paddle.Width) / 2.0)
					}
				}
				normOffset = math.Max(-1.0, math.Min(1.0, normOffset))

				vBaseX := vInX
				vBaseY := vInY
				if paddle.Index%2 == 0 { // Vertical paddles reflect X
					if !reflectedX {
						vBaseX = -vInX
						reflectedX = true
					}
				} else { // Horizontal paddles reflect Y
					if !reflectedY {
						vBaseY = -vInY
						reflectedY = true
					}
				}

				maxAngleDeflection := math.Pi / a.cfg.BallHitPaddleAngleFactor
				maxComponentChange := speed * math.Sin(maxAngleDeflection)

				vFinalX := vBaseX
				vFinalY := vBaseY
				if paddle.Index%2 == 0 {
					vFinalY = vBaseY + normOffset*maxComponentChange
				} else {
					vFinalX = vBaseX + normOffset*maxComponentChange
				}

				finalDirLen := math.Sqrt(vFinalX*vFinalX + vFinalY*vFinalY)
				if finalDirLen > 0 {
					vFinalX /= finalDirLen
					vFinalY /= finalDirLen
				} else {
					vFinalX, vFinalY = failsafeDirection(paddle.Index, hitOffsetX, hitOffsetY)
				}

				paddleVelAlongHit := float64(paddle.Vx)*vFinalX + float64(paddle.Vy)*vFinalY
				targetSpeed := speed + (paddleVelAlongHit * a.cfg.BallHitPaddleSpeedFactor)

				minSpeedAfterHit := float64(a.cfg.MinBallVelocity)
				if targetSpeed < minSpeedAfterHit {
					targetSpeed = minSpeedAfterHit
				}

				vFinalX *= targetSpeed
				vFinalY *= targetSpeed

				finalVxInt, finalVyInt := ensureNonZeroIntVelocity(vFinalX, vFinalY)
				// --- End Paddle Collision Physics ---

				// Send command to BallActor to update velocity
				currentEngine.Send(ballActorPID, SetVelocityCommand{Vx: finalVxInt, Vy: finalVyInt}, nil)
				// Update local cache immediately
				ball.Vx = finalVxInt
				ball.Vy = finalVyInt
				ball.OwnerIndex = paddleIndex // Update ball ownership in cache
				shouldPhase = true
				goto nextBall // Skip brick collision check if paddle hit occurred
			}
		} // End paddle loop

		// 4.3 Brick Collisions
		if !ball.Phasing {
			collidedCells := a.findCollidingCells(ball, cellSize)
			for _, cellPos := range collidedCells {
				col, row := cellPos[0], cellPos[1]
				if col < 0 || col >= a.cfg.GridSize || row < 0 || row >= a.cfg.GridSize {
					continue
				}
				// Check if grid and cell data exist before accessing
				if a.canvas == nil || a.canvas.Grid == nil || len(a.canvas.Grid) <= col || len(a.canvas.Grid[col]) <= row {
					continue // Grid structure invalid?
				}
				cell := &a.canvas.Grid[col][row] // Modify grid directly
				if cell.Data == nil {
					continue // Skip if cell data is nil
				}

				if cell.Data.Type == utils.Cells.Brick {
					brickLevel := cell.Data.Level
					cell.Data.Life--

					// Determine reflection axis
					dx := float64(ball.X - (col*cellSize + cellSize/2))
					dy := float64(ball.Y - (row*cellSize + cellSize/2))
					axisToReflect := ""
					if math.Abs(dx) > math.Abs(dy) {
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
					if axisToReflect != "" {
						// Send command to BallActor
						currentEngine.Send(ballActorPID, ReflectVelocityCommand{Axis: axisToReflect}, nil)
						// Update local cache immediately
						if axisToReflect == "X" {
							ball.Vx = -ball.Vx
						} else {
							ball.Vy = -ball.Vy
						}
					}

					if cell.Data.Life <= 0 {
						cell.Data.Type = utils.Cells.Empty
						cell.Data.Level = 0

						scorerIndex := ball.OwnerIndex
						if scorerIndex >= 0 && scorerIndex < len(a.players) && a.players[scorerIndex] != nil && a.players[scorerIndex].IsConnected {
							scInfo := a.players[scorerIndex]
							scInfo.Score.Add(int32(brickLevel))
						}

						if rand.Float64() < a.cfg.PowerUpChance {
							ballStateCopy := *ball // Create copy for power-up trigger
							powerUpsToTrigger = append(powerUpsToTrigger, ballStateCopy)
						}
					}
					shouldPhase = true
					goto nextBall // Only handle one brick collision per tick
				}
			} // End collided cells loop
		} // End brick collision check (!ball.Phasing)

	nextBall: // Label for goto statement
		if shouldPhase {
			// Send command to BallActor
			currentEngine.Send(ballActorPID, SetPhasingCommand{}, nil)
			// Update local cache immediately
			ball.Phasing = true
		}
	} // End ball loop

	// --- 5. Handle Ball Removals and Power-ups ---
	pidsToStop := make([]*bollywood.PID, 0, len(ballsToRemove))
	if len(ballsToRemove) > 0 {
		for _, ballID := range ballsToRemove {
			if pid, ok := a.ballActors[ballID]; ok && pid != nil {
				pidsToStop = append(pidsToStop, pid)
			}
			delete(a.balls, ballID) // Remove from cache
			delete(a.ballActors, ballID)
		}
	}
	// Trigger power-ups after processing all balls for the tick
	for _, ballState := range powerUpsToTrigger {
		// Pass copy of ball state to power-up function
		a.triggerRandomPowerUp(ctx, &ballState)
	}

	// --- 6. Stop Removed Ball Actors (outside the main state modification section) ---
	if len(pidsToStop) > 0 {
		for _, pid := range pidsToStop {
			currentEngine.Stop(pid)
		}
	}
}

// findCollidingCells remains the same.
func (a *GameActor) findCollidingCells(ball *Ball, cellSize int) [][2]int {
	collided := [][2]int{}
	gridSize := a.cfg.GridSize
	if cellSize <= 0 || gridSize <= 0 || ball == nil {
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

// triggerRandomPowerUp remains the same.
// Assumes called within the actor's message loop. Uses ball state copy.
func (a *GameActor) triggerRandomPowerUp(ctx bollywood.Context, ballState *Ball) {
	if ballState == nil || a.engine == nil || a.selfPID == nil {
		return
	}
	powerUpType := rand.Intn(3)

	// Check if the original ball actor still exists using the ID from the state copy
	ballActorPID, actorExists := a.ballActors[ballState.Id]
	_, stateExistsInCache := a.balls[ballState.Id] // Check if state still exists in cache

	if !actorExists || !stateExistsInCache || ballActorPID == nil {
		if powerUpType != 0 { // Only allow SpawnBall if original ball is gone
			return
		}
	}

	ownerIndex := ballState.OwnerIndex // Use owner from the state copy
	ownerValid := false
	if ownerIndex >= 0 && ownerIndex < len(a.players) && a.players[ownerIndex] != nil && a.players[ownerIndex].IsConnected {
		ownerValid = true
	}

	switch powerUpType {
	case 0: // SpawnBall
		if ownerValid {
			a.engine.Send(a.selfPID, SpawnBallCommand{
				OwnerIndex:  ownerIndex,
				X:           ballState.X, // Spawn near the triggering ball's location
				Y:           ballState.Y,
				ExpireIn:    a.cfg.PowerUpSpawnBallExpiry,
				IsPermanent: false,
			}, nil)
		}
	case 1: // IncreaseMass (Only if ball actor still exists)
		if actorExists && ballActorPID != nil {
			a.engine.Send(ballActorPID, IncreaseMassCommand{Additional: a.cfg.PowerUpIncreaseMassAdd}, nil)
		}
	case 2: // IncreaseVelocity (Only if ball actor still exists)
		if actorExists && ballActorPID != nil {
			a.engine.Send(ballActorPID, IncreaseVelocityCommand{Ratio: a.cfg.PowerUpIncreaseVelRatio}, nil)
		}
	}
}

// failsafeDirection remains the same.
func failsafeDirection(paddleIndex int, hitOffsetX, hitOffsetY float64) (float64, float64) {
	vFinalX := -hitOffsetX
	vFinalY := -hitOffsetY
	failsafeLen := math.Sqrt(vFinalX*vFinalX + vFinalY*vFinalY)
	if failsafeLen > 0 {
		vFinalX /= failsafeLen
		vFinalY /= failsafeLen
	} else {
		vFinalX, vFinalY = 0, 0
		switch paddleIndex {
		case 0:
			vFinalX = -1
		case 1:
			vFinalY = 1
		case 2:
			vFinalX = 1
		case 3:
			vFinalY = -1
		}
	}
	return vFinalX, vFinalY
}

// ensureNonZeroIntVelocity remains the same.
func ensureNonZeroIntVelocity(vxFloat, vyFloat float64) (int, int) {
	vxInt := int(vxFloat)
	vyInt := int(vyFloat)

	if vxInt == 0 && vxFloat != 0 {
		vxInt = int(math.Copysign(1.0, vxFloat))
	}
	if vyInt == 0 && vyFloat != 0 {
		vyInt = int(math.Copysign(1.0, vyFloat))
	}

	if vxInt == 0 && vyInt == 0 && (vxFloat != 0 || vyFloat != 0) {
		if math.Abs(vxFloat) > math.Abs(vyFloat) {
			vxInt = int(math.Copysign(1.0, vxFloat))
			vyInt = 0
		} else if math.Abs(vyFloat) > math.Abs(vxFloat) {
			vxInt = 0
			vyInt = int(math.Copysign(1.0, vyFloat))
		} else {
			vxInt = int(math.Copysign(1.0, vxFloat))
			vyInt = int(math.Copysign(1.0, vyFloat))
		}
		if vxInt == 0 && vyInt == 0 {
			vxInt = 1
		}
	}
	return vxInt, vyInt
}
