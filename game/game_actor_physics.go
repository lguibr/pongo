// File: game/game_actor_physics.go
package game

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
)

// --- Physics ---

// detectCollisions checks for collisions and generates specific update messages.
func (a *GameActor) detectCollisions(ctx bollywood.Context) {
	currentEngine := a.engine
	if currentEngine == nil {
		fmt.Printf("WARN: detectCollisions called on GameActor %s with nil engine.\n", a.selfPID)
		return
	}

	activeBalls := a.balls
	activePaddles := a.paddles

	cellSize := a.cfg.CellSize
	canvasSize := a.cfg.CanvasSize
	ballsToRemove := []int{}
	powerUpsToTrigger := []Ball{} // Store copies of ball state at the moment of power-up trigger
	positionAdjustmentBuffer := 1 // Small buffer to prevent sticking after adjustment
	minVelocityAwayFromWall := 2  // Minimum speed component away from wall after collision

	// Track score changes to send updates only once per tick per player
	scoreChanged := make(map[int]bool)

	// Iterate over a copy of ball IDs to avoid issues if balls are removed during iteration
	ballIDs := make([]int, 0, len(activeBalls))
	for id := range activeBalls {
		ballIDs = append(ballIDs, id)
	}

	for _, ballID := range ballIDs {
		// Re-fetch ball and actor PID inside the loop in case they were removed
		ball, ballExists := activeBalls[ballID]
		ballActorPID, pidExists := a.ballActors[ballID]

		if !ballExists || ball == nil || !pidExists || ballActorPID == nil {
			continue // Ball was removed during this tick's processing
		}

		shouldPhase := false
		reflectedX := false // Track if reflection already happened on this axis this tick
		reflectedY := false
		originalOwner := ball.OwnerIndex // Store owner before potential changes

		// 4.1 Wall Collisions
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
			newVx, newVy := ball.Vx, ball.Vy // Start with current velocity

			// Adjust position slightly inside the boundary
			switch hitWall {
			case 0: // Right wall
				ball.X = canvasSize - ball.Radius - positionAdjustmentBuffer
				if !reflectedX {
					newVx = -utils.Abs(ball.Vx); if utils.Abs(newVx) < minVelocityAwayFromWall { newVx = -minVelocityAwayFromWall }; reflectedX = true
				}
			case 1: // Top wall
				ball.Y = ball.Radius + positionAdjustmentBuffer
				if !reflectedY {
					newVy = utils.Abs(ball.Vy); if utils.Abs(newVy) < minVelocityAwayFromWall { newVy = minVelocityAwayFromWall }; reflectedY = true
				}
			case 2: // Left wall
				ball.X = ball.Radius + positionAdjustmentBuffer
				if !reflectedX {
					newVx = utils.Abs(ball.Vx); if utils.Abs(newVx) < minVelocityAwayFromWall { newVx = minVelocityAwayFromWall }; reflectedX = true
				}
			case 3: // Bottom wall
				ball.Y = canvasSize - ball.Radius - positionAdjustmentBuffer
				if !reflectedY {
					newVy = -utils.Abs(ball.Vy); if utils.Abs(newVy) < minVelocityAwayFromWall { newVy = -minVelocityAwayFromWall }; reflectedY = true
				}
			}

			// Send velocity update command if changed
			if newVx != ball.Vx || newVy != ball.Vy {
				currentEngine.Send(ballActorPID, SetVelocityCommand{Vx: newVx, Vy: newVy}, nil)
				ball.Vx = newVx; ball.Vy = newVy // Update local cache
			}

			ball.Collided = true // Mark ball as collided for this tick
			shouldPhase = true  // Trigger phasing

			// Scoring / Ball Removal Logic
			concederIndex := hitWall
			isPlayerWall := concederIndex >= 0 && concederIndex < len(a.players) && a.players[concederIndex] != nil && a.players[concederIndex].IsConnected

			if isPlayerWall {
				pInfo := a.players[concederIndex]
				scorerIndex := ball.OwnerIndex
				isScorerValid := scorerIndex >= 0 && scorerIndex < len(a.players) && a.players[scorerIndex] != nil && a.players[scorerIndex].IsConnected

				if isScorerValid && scorerIndex != concederIndex {
					scInfo := a.players[scorerIndex]; scInfo.Score.Add(1); pInfo.Score.Add(-1); scoreChanged[scorerIndex] = true; scoreChanged[concederIndex] = true
				} else if scorerIndex == -1 {
					pInfo.Score.Add(-1); scoreChanged[concederIndex] = true
				} else if scorerIndex == concederIndex {
					pInfo.Score.Add(-1); ball.OwnerIndex = -1; scoreChanged[concederIndex] = true
				}

				if ball.OwnerIndex != originalOwner && ball.OwnerIndex == -1 {
					ownerUpdate := &BallOwnershipChange{MessageType: "ballOwnerChanged", ID: ballID, NewOwnerIndex: -1}; a.addUpdate(ownerUpdate)
				}
			} else { // Hit an empty player slot's wall
				if !ball.IsPermanent {
					ballsToRemove = append(ballsToRemove, ballID); continue
				}
			}
		} // End wall collision check

		// 4.2 Paddle Collisions
		for paddleIndex, paddle := range activePaddles {
			if paddle == nil || !(paddleIndex >= 0 && paddleIndex < len(a.players) && a.players[paddleIndex] != nil && a.players[paddleIndex].IsConnected) {
				continue
			}

			if !ball.Phasing && ball.BallInterceptPaddles(paddle) {
				// --- Paddle Collision Physics ---
				vInX, vInY := float64(ball.Vx), float64(ball.Vy)
				speed := math.Sqrt(vInX*vInX + vInY*vInY); if speed < float64(a.cfg.MinBallVelocity) { speed = float64(a.cfg.MinBallVelocity) }
				paddleCenterX, paddleCenterY := float64(paddle.X+paddle.Width/2), float64(paddle.Y+paddle.Height/2)
				hitOffsetX, hitOffsetY := float64(ball.X)-paddleCenterX, float64(ball.Y)-paddleCenterY
				normOffset := 0.0
				if paddle.Index%2 == 0 { if paddle.Height > 0 { normOffset = hitOffsetY / (float64(paddle.Height) / 2.0) } } else { if paddle.Width > 0 { normOffset = hitOffsetX / (float64(paddle.Width) / 2.0) } }
				normOffset = math.Max(-1.0, math.Min(1.0, normOffset))
				vBaseX, vBaseY := vInX, vInY
				if paddle.Index%2 == 0 { if !reflectedX { vBaseX = -vInX; reflectedX = true } } else { if !reflectedY { vBaseY = -vInY; reflectedY = true } }
				maxAngleDeflection := math.Pi / a.cfg.BallHitPaddleAngleFactor; maxComponentChange := speed * math.Sin(maxAngleDeflection)
				vFinalX, vFinalY := vBaseX, vBaseY
				if paddle.Index%2 == 0 { vFinalY = vBaseY + normOffset*maxComponentChange } else { vFinalX = vBaseX + normOffset*maxComponentChange }
				finalDirLen := math.Sqrt(vFinalX*vFinalX + vFinalY*vFinalY)
				if finalDirLen > 0 { vFinalX /= finalDirLen; vFinalY /= finalDirLen } else { vFinalX, vFinalY = a.failsafeDirection(paddle.Index, hitOffsetX, hitOffsetY) }
				paddleVelAlongHit := float64(paddle.Vx)*vFinalX + float64(paddle.Vy)*vFinalY
				targetSpeed := speed + (paddleVelAlongHit * a.cfg.BallHitPaddleSpeedFactor); minSpeedAfterHit := float64(a.cfg.MinBallVelocity); if targetSpeed < minSpeedAfterHit { targetSpeed = minSpeedAfterHit }
				vFinalX *= targetSpeed; vFinalY *= targetSpeed
				finalVxInt, finalVyInt := a.ensureNonZeroIntVelocity(vFinalX, vFinalY)
				// --- End Paddle Collision Physics ---

				currentEngine.Send(ballActorPID, SetVelocityCommand{Vx: finalVxInt, Vy: finalVyInt}, nil)
				ball.Vx = finalVxInt; ball.Vy = finalVyInt // Update local cache

				if ball.OwnerIndex != paddleIndex {
					ball.OwnerIndex = paddleIndex
					ownerUpdate := &BallOwnershipChange{MessageType: "ballOwnerChanged", ID: ballID, NewOwnerIndex: paddleIndex}; a.addUpdate(ownerUpdate)
				}
				ball.Collided = true; paddle.Collided = true; shouldPhase = true
				goto nextBall // Skip brick check if paddle hit
			}
		} // End paddle loop

	nextBall: // Label for goto statement (moved before collidedCells declaration)

		// 4.3 Brick Collisions
		collidedCells := a.findCollidingCells(ball, cellSize)

		for _, cellPos := range collidedCells {
			col, row := cellPos[0], cellPos[1]
			cellCoord := [2]int{col, row}

			if a.canvas == nil || a.canvas.Grid == nil || row < 0 || row >= len(a.canvas.Grid) || col < 0 || col >= len(a.canvas.Grid[row]) {
				continue
			}

			cell := &a.canvas.Grid[row][col]
			if cell.Data == nil || cell.Data.Type != utils.Cells.Brick {
				continue
			}

			// --- Phasing Ball Logic ---
			if ball.Phasing {
				// Send command to BallActor to check and potentially apply damage
				currentEngine.Send(ballActorPID, DamageBrickCommand{Coord: cellCoord}, a.selfPID)
				// GameActor no longer directly damages or tracks phasing hits here.
				// It will wait for an ApplyBrickDamage message from BallActor if needed.
				// We still mark the ball as collided for visual feedback.
				ball.Collided = true
				// Continue checking other cells, phasing ball can hit multiple
			} else {
				// --- Non-Phasing Ball Logic ---
				brickLevel := cell.Data.Level
				cell.Data.Life--

				// Reflect
				dx := float64(ball.X - (col*cellSize + cellSize/2))
				dy := float64(ball.Y - (row*cellSize + cellSize/2))
				axisToReflect := ""
				if math.Abs(dx) > math.Abs(dy) { if !reflectedX { axisToReflect = "X"; reflectedX = true } } else { if !reflectedY { axisToReflect = "Y"; reflectedY = true } }
				if axisToReflect != "" {
					currentEngine.Send(ballActorPID, ReflectVelocityCommand{Axis: axisToReflect}, nil)
					if axisToReflect == "X" { ball.Vx = -ball.Vx } else { ball.Vy = -ball.Vy } // Update cache
				}

				ball.Collided = true
				shouldPhase = true // Trigger phasing for non-phasing ball

				// Handle destruction
				if cell.Data.Life <= 0 {
					cell.Data.Type = utils.Cells.Empty; cell.Data.Level = 0
					scorerIndex := ball.OwnerIndex
					if scorerIndex >= 0 && scorerIndex < len(a.players) && a.players[scorerIndex] != nil && a.players[scorerIndex].IsConnected {
						scInfo := a.players[scorerIndex]; scInfo.Score.Add(int32(brickLevel)); scoreChanged[scorerIndex] = true
					}
					if rand.Float64() < a.cfg.PowerUpChance {
						ballStateCopy := *ball; powerUpsToTrigger = append(powerUpsToTrigger, ballStateCopy)
					}
				}
				// Non-phasing ball processes only one brick collision per tick
				// Use continue to skip remaining cells for *this ball* this tick
				continue
			} // End phasing check
		} // End collided cells loop

		// Trigger phasing if needed (moved outside the brick loop)
		if shouldPhase && !ball.Phasing { // Only trigger phasing if not already phasing
			currentEngine.Send(ballActorPID, SetPhasingCommand{}, nil)
			ball.Phasing = true // Update local cache
		}
	} // End ball loop

	// --- Generate Score Updates ---
	for index, changed := range scoreChanged {
		if changed && a.players[index] != nil {
			scoreUpdate := &ScoreUpdate{MessageType: "scoreUpdate", Index: index, Score: a.players[index].Score.Load()}; a.addUpdate(scoreUpdate)
		}
	}

	// --- Handle Ball Removals ---
	pidsToStop := make([]*bollywood.PID, 0, len(ballsToRemove))
	if len(ballsToRemove) > 0 {
		for _, ballID := range ballsToRemove {
			if pid, ok := a.ballActors[ballID]; ok && pid != nil { pidsToStop = append(pidsToStop, pid) }
			delete(a.balls, ballID); delete(a.ballActors, ballID)
			removedUpdate := &BallRemoved{MessageType: "ballRemoved", ID: ballID}; a.addUpdate(removedUpdate)
		}
	}

	// --- Trigger Power-ups ---
	for _, ballState := range powerUpsToTrigger {
		// Pass the ball's X, Y, and OwnerIndex correctly
		a.triggerRandomPowerUp(ctx, ballState.X, ballState.Y, ballState.OwnerIndex)
	}

	// --- Stop Removed Ball Actors ---
	if len(pidsToStop) > 0 {
		for _, pid := range pidsToStop { currentEngine.Stop(pid) }
	}
}

// findCollidingCells checks which grid cells the ball overlaps with.
// Returns pairs of [column, row].
func (a *GameActor) findCollidingCells(ball *Ball, cellSize int) [][2]int {
	collided := [][2]int{}
	gridSize := a.cfg.GridSize
	if cellSize <= 0 || gridSize <= 0 || ball == nil {
		return collided
	}

	// Determine the range of grid cells the ball might overlap with
	minCol := (ball.X - ball.Radius) / cellSize
	maxCol := (ball.X + ball.Radius) / cellSize
	minRow := (ball.Y - ball.Radius) / cellSize
	maxRow := (ball.Y + ball.Radius) / cellSize

	// Clamp the range to valid grid indices
	minCol = utils.MaxInt(0, minCol)
	maxCol = utils.MinInt(gridSize-1, maxCol)
	minRow = utils.MaxInt(0, minRow)
	maxRow = utils.MinInt(gridSize-1, maxRow)

	// Check each cell in the potential range for actual intersection
	for c := minCol; c <= maxCol; c++ {
		for r := minRow; r <= maxRow; r++ {
			if ball.InterceptsIndex(c, r, cellSize) {
				collided = append(collided, [2]int{c, r}) // Append as [column, row]
			}
		}
	}
	return collided
}

// triggerRandomPowerUp handles the logic for activating a power-up.
// Takes ballX, ballY, ownerIndex as arguments.
func (a *GameActor) triggerRandomPowerUp(ctx bollywood.Context, ballX, ballY, ownerIndex int) {
	// Safety check for owner validity
	ownerValid := false
	if ownerIndex >= 0 && ownerIndex < len(a.players) && a.players[ownerIndex] != nil && a.players[ownerIndex].IsConnected {
		ownerValid = true
	}
	if !ownerValid {
		return
	}

	powerUpType := rand.Intn(3) // 0: SpawnBall, 1: IncreaseMass, 2: IncreaseVelocity

	// Find the original ball that caused the damage (needed for mass/velocity powerups)
	// This is slightly tricky now. We might need to pass the original ball ID through ApplyBrickDamage or the collision check.
	// For now, let's assume power-ups only spawn new balls if triggered by non-phasing damage.
	// TODO: Revisit if IncreaseMass/Velocity power-ups are desired from phasing damage.

	switch powerUpType {
	case 0: // SpawnBall
		// Send command to self (GameActor) to handle spawning
		a.engine.Send(a.selfPID, SpawnBallCommand{
			OwnerIndex:  ownerIndex,
			X:           ballX, // Spawn near the original ball's position at damage time
			Y:           ballY,
			ExpireIn:    a.cfg.PowerUpSpawnBallExpiry,
			IsPermanent: false,
		}, nil)
	case 1: // IncreaseMass
		// Requires BallID - currently not available here easily. Skip for now.
		fmt.Printf("WARN: IncreaseMass power-up triggered - currently not supported from brick break.\n")
	case 2: // IncreaseVelocity
		// Requires BallID - currently not available here easily. Skip for now.
		fmt.Printf("WARN: IncreaseVelocity power-up triggered - currently not supported from brick break.\n")
	}
}

// failsafeDirection provides a reasonable reflection direction if calculations result in zero vector.
func (a *GameActor) failsafeDirection(paddleIndex int, hitOffsetX, hitOffsetY float64) (float64, float64) {
	// Default to reflecting directly away from center (approximated)
	vFinalX := -hitOffsetX
	vFinalY := -hitOffsetY
	failsafeLen := math.Sqrt(vFinalX*vFinalX + vFinalY*vFinalY)

	if failsafeLen > 0 {
		vFinalX /= failsafeLen
		vFinalY /= failsafeLen
	} else {
		// If still zero (hit dead center?), reflect directly away from wall
		vFinalX, vFinalY = 0, 0
		switch paddleIndex {
		case 0: vFinalX = -1 // Right paddle -> reflect left
		case 1: vFinalY = 1  // Top paddle -> reflect down
		case 2: vFinalX = 1  // Left paddle -> reflect right
		case 3: vFinalY = -1 // Bottom paddle -> reflect up
		}
	}
	return vFinalX, vFinalY
}

// ensureNonZeroIntVelocity converts float velocities to int, ensuring non-zero if original float wasn't zero.
func (a *GameActor) ensureNonZeroIntVelocity(vxFloat, vyFloat float64) (int, int) {
	vxInt := int(vxFloat)
	vyInt := int(vyFloat)

	// If conversion to int resulted in zero, but float wasn't zero, set to +/- 1
	if vxInt == 0 && vxFloat != 0 {
		vxInt = int(math.Copysign(1.0, vxFloat))
	}
	if vyInt == 0 && vyFloat != 0 {
		vyInt = int(math.Copysign(1.0, vyFloat))
	}

	// Failsafe: If both are somehow still zero, but floats weren't, prioritize larger component or default
	if vxInt == 0 && vyInt == 0 && (vxFloat != 0 || vyFloat != 0) {
		if math.Abs(vxFloat) > math.Abs(vyFloat) {
			vxInt = int(math.Copysign(1.0, vxFloat))
			vyInt = 0
		} else if math.Abs(vyFloat) > math.Abs(vxFloat) {
			vxInt = 0
			vyInt = int(math.Copysign(1.0, vyFloat))
		} else { // Equal magnitude or both non-zero but small
			vxInt = int(math.Copysign(1.0, vxFloat)) // Keep both components +/- 1
			vyInt = int(math.Copysign(1.0, vyFloat))
		}
		// Final final failsafe: if somehow *still* zero (e.g., NaN floats?), set a default
		if vxInt == 0 && vyInt == 0 {
			vxInt = 1 // Default to moving right
		}
	}
	return vxInt, vyInt
}