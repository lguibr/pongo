// File: game/game_actor_physics.go
package game

import (
	"math"
	"math/rand"
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
)

// --- Collision Detection & Physics ---

// detectCollisions checks for and handles collisions between balls, paddles, walls, and bricks.
// It updates the game state (scores, grid, ball/paddle properties) in the GameActor's cache
// and sends commands to child actors (BallActor, PaddleActor) to modify their internal state (velocity, phasing).
// It also generates atomic update messages (ScoreUpdate, BallOwnershipChange, etc.) and adds them to the pending buffer.
func (a *GameActor) detectCollisions(ctx bollywood.Context) {
	if a.canvas == nil || a.canvas.Grid == nil {
		return // Cannot detect collisions without a grid
	}
	gridSize := a.cfg.GridSize
	cellSize := a.cfg.CellSize
	canvasSize := a.cfg.CanvasSize
	// selfPIDStr := "unknown" // Removed unused variable
	// if a.selfPID != nil {
	// 	selfPIDStr = a.selfPID.String()
	// }

	// --- Capture Phasing State AT START of Tick ---
	phasingStateThisTick := make(map[int]bool)
	for id, ball := range a.balls {
		if ball != nil {
			phasingStateThisTick[id] = ball.Phasing
		}
	}
	// --- End Capture ---

	// --- Ball-Wall Collisions ---
	for id, ball := range a.balls {
		if ball == nil {
			continue
		}
		ballActorPID := a.ballActors[id] // Get PID for sending commands

		// Check Right Wall (Player 0)
		if ball.X+ball.Radius >= canvasSize {
			// REMOVED: Phasing check here - handler deals with it
			a.handleWallCollision(ctx, ball, ballActorPID, 0) // Pass PID
		}
		// Check Top Wall (Player 1)
		if ball.Y-ball.Radius <= 0 {
			// REMOVED: Phasing check here - handler deals with it
			a.handleWallCollision(ctx, ball, ballActorPID, 1) // Pass PID
		}
		// Check Left Wall (Player 2)
		if ball.X-ball.Radius <= 0 {
			// REMOVED: Phasing check here - handler deals with it
			a.handleWallCollision(ctx, ball, ballActorPID, 2) // Pass PID
		}
		// Check Bottom Wall (Player 3)
		if ball.Y+ball.Radius >= canvasSize {
			// REMOVED: Phasing check here - handler deals with it
			a.handleWallCollision(ctx, ball, ballActorPID, 3) // Pass PID
		}
	}

	// --- Ball-Paddle Collisions ---
	for id, ball := range a.balls {
		if ball == nil {
			continue
		}
		ballActorPID := a.ballActors[id] // Get PID

		for playerIndex, paddle := range a.paddles {
			if paddle == nil {
				continue
			}
			collisionKey := CollisionKey{Object1ID: id, Object2ID: playerIndex} // Use playerIndex as Object2ID for paddles

			if ball.BallInterceptPaddles(paddle) {
				// Collision tracker prevents rapid re-triggering during continuous contact
				if a.activeCollisions.BeginCollision(collisionKey) {
					// Handle collision handles phasing internally now
					a.handlePaddleCollision(ctx, ball, ballActorPID, paddle, playerIndex) // Pass PID
				}
				// If already colliding (BeginCollision false), do nothing this tick
			} else {
				// Not intersecting, end any active collision tracking
				a.activeCollisions.EndCollision(collisionKey)
			}
		}
	}

	// --- Ball-Brick Collisions ---
	for id, ball := range a.balls {
		if ball == nil {
			continue
		}
		ballActorPID := a.ballActors[id] // Get PID
		isPhasing := phasingStateThisTick[id] // Use captured state for this tick

		// Get active brick collisions for this ball that might need ending
		activeBrickCollisions := a.activeCollisions.GetActiveCollisionsForKey1(id)
		collidedBrickIDsThisTick := make(map[int]bool) // Track bricks hit this tick

		// Iterate through potentially colliding grid cells around the ball
		minCol, maxCol, minRow, maxRow := a.getBallCollisionGridBounds(ball, gridSize)

		// logPrefix := fmt.Sprintf("PhysicsTick %s Ball %d (PhasingCaptured: %t, Actual: %t, Pos: %d,%d, Vel: %d,%d): ", selfPIDStr, id, isPhasing, ball.Phasing, ball.X, ball.Y, ball.Vx, ball.Vy) // Removed log

		for r := minRow; r <= maxRow; r++ {
			for c := minCol; c <= maxCol; c++ {
				cell := &a.canvas.Grid[r][c] // Get pointer to cell
				if cell.Data == nil || cell.Data.Type != utils.Cells.Brick {
					continue // Skip empty or non-brick cells
				}

				brickID := makeBrickID(r, c, gridSize) // Unique ID for this brick cell
				collisionKey := CollisionKey{Object1ID: id, Object2ID: brickID}
				// brickLifeBefore := cell.Data.Life // Removed unused variable

				if ball.InterceptsIndex(c, r, cellSize) {
					// fmt.Printf("%s Intersects Brick [%d,%d] (ID: %d, Life: %d)\n", logPrefix, r, c, brickID, brickLifeBefore) // Removed log

					collidedBrickIDsThisTick[brickID] = true // Mark as hit this tick
					isNewCollision := a.activeCollisions.BeginCollision(collisionKey)

					// fmt.Printf("%s -> Check Phasing (Captured): %t. IsNewCollision: %t\n", logPrefix, isPhasing, isNewCollision) // Removed log

					// Phasing ball logic: Damage once per tick, no reflection, no phasing reset
					if isPhasing { // Use captured state
						if isNewCollision {
							// fmt.Printf("%s -> Phasing Path: Damaging brick.\n", logPrefix) // Removed log
							a.damageBrick(ctx, ball, cell, r, c)
						} else {
							// fmt.Printf("%s -> Phasing Path: Already damaged this brick this phase.\n", logPrefix) // Removed log
						}
						// Do NOT reflect, do NOT send SetPhasingCommand, do NOT reset timer
					} else { // Use captured state
						// Non-phasing ball logic: Reflect, damage, start phasing
						// fmt.Printf("%s -> Non-Phasing Path. IsNewCollision: %t\n", logPrefix, isNewCollision) // Removed log
						if isNewCollision {
							a.handleBrickCollision(ctx, ball, ballActorPID, cell, r, c) // Pass PID
						}
					}
				} else {
					// If not intersecting this tick, ensure any active collision is ended
					if a.activeCollisions.IsColliding(collisionKey) {
						a.activeCollisions.EndCollision(collisionKey)
					}
				}
			} // end col loop
		} // end row loop

		// End tracking for any previously active brick collisions that weren't hit this tick
		for _, key := range activeBrickCollisions {
			// Check if the Object2ID corresponds to a brick
			if key.Object2ID >= utils.MaxPlayers {
				if _, hitThisTick := collidedBrickIDsThisTick[key.Object2ID]; !hitThisTick {
					a.activeCollisions.EndCollision(key)
				}
			}
		}
	} // end ball loop
}

// handleWallCollision processes ball hitting a wall.
// Phasing balls reflect but do not trigger scoring or phasing reset.
func (a *GameActor) handleWallCollision(ctx bollywood.Context, ball *Ball, ballActorPID *bollywood.PID, wallIndex int) {
	if ball == nil || ballActorPID == nil {
		return
	}
	isPhasing := ball.Phasing // Check current phasing state

	// 1. Adjust Position (ensure ball is inside boundary)
	switch wallIndex {
	case 0: // Right
		ball.X = a.cfg.CanvasSize - ball.Radius
		ball.ReflectVelocity("X") // Reflect in cache immediately
	case 1: // Top
		ball.Y = ball.Radius
		ball.ReflectVelocity("Y")
	case 2: // Left
		ball.X = ball.Radius
		ball.ReflectVelocity("X")
	case 3: // Bottom
		ball.Y = a.cfg.CanvasSize - ball.Radius
		ball.ReflectVelocity("Y")
	}
	ball.Collided = true // Set collision flag for broadcast

	// 2. Send Command to BallActor to update its internal velocity state
	a.engine.Send(ballActorPID, SetVelocityCommand{Vx: ball.Vx, Vy: ball.Vy}, a.selfPID)

	// 3. Handle Scoring and Ownership (ONLY if NOT phasing)
	if !isPhasing {
		concederIndex := wallIndex
		scorerIndex := ball.OwnerIndex // Player who last hit the ball

		// Check if the wall belongs to an active player
		concederPlayer := a.players[concederIndex]
		isConcederActive := concederPlayer != nil && concederPlayer.IsConnected

		if isConcederActive {
			// Conceder loses points
			newScore := concederPlayer.Score.Add(-1)
			a.addUpdate(&ScoreUpdate{MessageType: "scoreUpdate", Index: concederIndex, Score: newScore})

			// Scorer gains points (if valid, connected, and not the same as conceder)
			scorerPlayer := (*playerInfo)(nil)
			isScorerValid := scorerIndex >= 0 && scorerIndex < utils.MaxPlayers
			if isScorerValid {
				scorerPlayer = a.players[scorerIndex]
			}
			isScorerActive := isScorerValid && scorerPlayer != nil && scorerPlayer.IsConnected

			if isScorerActive && scorerIndex != concederIndex {
				newScore := scorerPlayer.Score.Add(1)
				a.addUpdate(&ScoreUpdate{MessageType: "scoreUpdate", Index: scorerIndex, Score: newScore})
			}

			// Handle hitting own wall -> lose ownership
			if ball.OwnerIndex == concederIndex {
				ball.OwnerIndex = -1 // Ball becomes ownerless
				a.addUpdate(&BallOwnershipChange{MessageType: "ballOwnerChanged", ID: ball.Id, NewOwnerIndex: -1})
			}
		} else {
			// Wall belongs to an empty slot
			if !ball.IsPermanent {
				// Remove temporary ball
				a.handleDestroyExpiredBall(ctx, ball.Id) // Reuse expiry logic for removal
				return                                   // Exit early as ball is gone
			}
			// Permanent balls just reflect (velocity already handled)
		}

		// 4. Start Phasing (ONLY if NOT already phasing)
		ball.Phasing = true
		a.startPhasingTimer(ball.Id) // Log is inside this function
		a.engine.Send(ballActorPID, SetPhasingCommand{}, a.selfPID)
	}
	// If already phasing, we just reflect (handled above) and do nothing else.
}

// handlePaddleCollision processes ball hitting a paddle.
// Phasing balls reflect and change owner, but do not reset phasing timer.
func (a *GameActor) handlePaddleCollision(ctx bollywood.Context, ball *Ball, ballActorPID *bollywood.PID, paddle *Paddle, playerIndex int) {
	if ball == nil || paddle == nil || ballActorPID == nil {
		return
	}
	isPhasing := ball.Phasing // Check current phasing state

	// 1. Calculate Reflection (Complex - simplified example)
	// Factors for reflection calculation
	speedFactor := a.cfg.BallHitPaddleSpeedFactor
	angleFactor := a.cfg.BallHitPaddleAngleFactor // Smaller value = wider angle range

	// Calculate collision point relative to paddle center
	var relativeHitPos float64
	var paddleCenter float64
	var paddleSpan float64 // The length/width dimension the ball hits

	if paddle.Width > paddle.Height { // Horizontal paddle
		paddleCenter = float64(paddle.X + paddle.Width/2)
		relativeHitPos = float64(ball.X) - paddleCenter
		paddleSpan = float64(paddle.Width)
	} else { // Vertical paddle
		paddleCenter = float64(paddle.Y + paddle.Height/2)
		relativeHitPos = float64(ball.Y) - paddleCenter
		paddleSpan = float64(paddle.Height)
	}

	// Normalize hit position (-1 to 1)
	normalizedHitPos := (relativeHitPos / (paddleSpan / 2.0)) * 1.1 // Add slight amplification
	normalizedHitPos = math.Max(-1.0, math.Min(1.0, normalizedHitPos)) // Clamp

	// Calculate reflection angle based on hit position
	// Max reflection angle based on angleFactor (e.g., Pi/3 for angleFactor=3)
	maxAngle := math.Pi / angleFactor
	reflectionAngle := normalizedHitPos * maxAngle

	// Determine base reflection vector based on paddle orientation
	var baseVx, baseVy float64
	switch playerIndex {
	case 0: baseVx, baseVy = -1, 0 // Reflect left from right paddle
	case 1: baseVx, baseVy = 0, 1  // Reflect down from top paddle
	case 2: baseVx, baseVy = 1, 0  // Reflect right from left paddle
	case 3: baseVx, baseVy = 0, -1 // Reflect up from bottom paddle
	}

	// Rotate base reflection vector by reflectionAngle
	cosAngle := math.Cos(reflectionAngle)
	sinAngle := math.Sin(reflectionAngle)
	newVx := baseVx*cosAngle - baseVy*sinAngle
	newVy := baseVx*sinAngle + baseVy*cosAngle

	// Calculate new speed: base speed + influence from paddle movement
	currentSpeed := math.Sqrt(float64(ball.Vx*ball.Vx + ball.Vy*ball.Vy))
	paddleVelComponent := 0.0
	if paddle.Width > paddle.Height { // Horizontal paddle
		paddleVelComponent = float64(paddle.Vx) * newVx // Use Vx if horizontal
	} else { // Vertical paddle
		paddleVelComponent = float64(paddle.Vy) * newVy // Use Vy if vertical
	}

	// Combine base speed and paddle influence
	newSpeed := currentSpeed + (paddleVelComponent * speedFactor)
	// Clamp speed to configured min/max velocity range
	newSpeed = math.Max(float64(a.cfg.MinBallVelocity), math.Min(float64(a.cfg.MaxBallVelocity)*1.2, newSpeed)) // Allow slight boost over max

	// Apply new speed to the reflection vector components
	finalVx := int(newVx * newSpeed)
	finalVy := int(newVy * newSpeed)

	// Ensure velocity components are not zero if speed is non-zero
	if newSpeed > 0 {
		if finalVx == 0 { finalVx = int(math.Copysign(1.0, newVx)) }
		if finalVy == 0 { finalVy = int(math.Copysign(1.0, newVy)) }
	}

	// 2. Update Ball State in Cache
	ball.Vx = finalVx
	ball.Vy = finalVy
	ball.OwnerIndex = playerIndex
	ball.Collided = true // Set collision flag for broadcast
	paddle.Collided = true

	// 3. Send Commands/Updates
	a.engine.Send(ballActorPID, SetVelocityCommand{Vx: finalVx, Vy: finalVy}, a.selfPID)
	a.addUpdate(&BallOwnershipChange{MessageType: "ballOwnerChanged", ID: ball.Id, NewOwnerIndex: playerIndex})

	// 4. Start Phasing (ONLY if NOT already phasing)
	if !isPhasing {
		ball.Phasing = true
		a.startPhasingTimer(ball.Id) // Log is inside this function
		a.engine.Send(ballActorPID, SetPhasingCommand{}, a.selfPID)
	}
	// If already phasing, we just reflect and change owner (handled above).
}

// handleBrickCollision processes non-phasing ball hitting a brick.
func (a *GameActor) handleBrickCollision(ctx bollywood.Context, ball *Ball, ballActorPID *bollywood.PID, cell *Cell, r, c int) {
	if ball == nil || cell == nil || cell.Data == nil || ballActorPID == nil {
		return
	}
	// This function is only called for non-phasing balls by detectCollisions
	if ball.Phasing {
		// selfPIDStr := "unknown"; if a.selfPID != nil { selfPIDStr = a.selfPID.String() } // Removed unused variable
		// fmt.Printf("PhysicsTick %s Ball %d: Re-check Skipping brick collision handler (Phasing: %t)\n", selfPIDStr, ball.Id, ball.Phasing) // Removed log
		return
	}
	// selfPIDStr := "unknown" // Removed unused variable
	// if a.selfPID != nil {
	// 	selfPIDStr = a.selfPID.String()
	// }
	// logPrefix := fmt.Sprintf("PhysicsTick %s Ball %d: ", selfPIDStr, ball.Id) // Removed log

	// fmt.Printf("%s HandleBrickCollision for Brick [%d,%d]\n", logPrefix, r, c) // Removed log

	// 1. Reflect Ball Velocity (Simplified: Assume axis based on relative position)
	cellCenterX := c*a.cfg.CellSize + a.cfg.CellSize/2
	cellCenterY := r*a.cfg.CellSize + a.cfg.CellSize/2
	deltaX := ball.X - cellCenterX
	deltaY := ball.Y - cellCenterY

	// originalVx, originalVy := ball.Vx, ball.Vy // Removed unused variable
	// reflectAxis := "" // Removed unused variable
	if math.Abs(float64(deltaX)) > math.Abs(float64(deltaY)) {
		ball.ReflectVelocity("X") // Reflect horizontally
		// reflectAxis = "X" // Removed unused variable
	} else {
		ball.ReflectVelocity("Y") // Reflect vertically
		// reflectAxis = "Y" // Removed unused variable
	}
	ball.Collided = true // Set collision flag

	// fmt.Printf("%s Reflected Vel (Axis: %s): (%d, %d) -> (%d, %d)\n", logPrefix, reflectAxis, originalVx, originalVy, ball.Vx, ball.Vy) // Removed log

	// Send command to BallActor
	a.engine.Send(ballActorPID, SetVelocityCommand{Vx: ball.Vx, Vy: ball.Vy}, a.selfPID)

	// 2. Damage Brick
	destroyed := a.damageBrick(ctx, ball, cell, r, c) // Handles scoring and power-ups

	// 3. Start Phasing (only if brick wasn't destroyed, otherwise ball might phase through next ball spawn)
	if !destroyed {
		ball.Phasing = true
		a.startPhasingTimer(ball.Id) // Log is inside this function
		a.engine.Send(ballActorPID, SetPhasingCommand{}, a.selfPID)
	} else {
		// fmt.Printf("%s Not starting phasing (Destroyed: %t, Already Phasing: %t)\n", logPrefix, destroyed, ball.Phasing) // Removed log
	}
}

// damageBrick reduces brick life, handles destruction, scoring, and power-ups.
// Returns true if the brick was destroyed.
func (a *GameActor) damageBrick(ctx bollywood.Context, ball *Ball, cell *Cell, r, c int) bool {
	if cell == nil || cell.Data == nil || cell.Data.Life <= 0 {
		return false // Already destroyed or not a valid brick
	}
	// selfPIDStr := "unknown" // Removed unused variable
	// if a.selfPID != nil {
	// 	selfPIDStr = a.selfPID.String()
	// }
	// logPrefix := fmt.Sprintf("PhysicsTick %s Ball %d: ", selfPIDStr, ball.Id) // Removed log

	// lifeBefore := cell.Data.Life // Removed unused variable
	cell.Data.Life--
	destroyed := false

	// fmt.Printf("%s Damaging Brick [%d,%d]. Life: %d -> %d\n", logPrefix, r, c, lifeBefore, cell.Data.Life) // Removed log

	if cell.Data.Life <= 0 {
		destroyed = true
		brickLevel := cell.Data.Level // Store level before clearing data
		cell.Data.Type = utils.Cells.Empty
		cell.Data.Level = 0

		// fmt.Printf("%s Brick [%d,%d] Destroyed. Level: %d\n", logPrefix, r, c, brickLevel) // Removed log

		// Award score to ball owner
		scorerIndex := ball.OwnerIndex
		if scorerIndex >= 0 && scorerIndex < utils.MaxPlayers && a.players[scorerIndex] != nil && a.players[scorerIndex].IsConnected {
			newScore := a.players[scorerIndex].Score.Add(int32(brickLevel))
			a.addUpdate(&ScoreUpdate{MessageType: "scoreUpdate", Index: scorerIndex, Score: newScore})
			// fmt.Printf("%s Awarded %d points to Player %d (New Score: %d)\n", logPrefix, brickLevel, scorerIndex, newScore) // Removed log
		}

		// Trigger Power-up?
		if rand.Float64() < a.cfg.PowerUpChance {
			// fmt.Printf("%s Triggering PowerUp for Brick [%d,%d]\n", logPrefix, r, c) // Removed log
			a.triggerRandomPowerUp(ctx, ball, r, c)
		}
	}

	// Note: Grid updates are now handled by the periodic FullGridUpdate,
	// so we don't add an individual BrickStateUpdate here.

	return destroyed
}

// triggerRandomPowerUp selects and activates a power-up.
func (a *GameActor) triggerRandomPowerUp(ctx bollywood.Context, ball *Ball, brickRow, brickCol int) {
	if ball == nil {
		return
	}
	ballActorPID := a.ballActors[ball.Id] // Get PID
	if ballActorPID == nil {
		return // Cannot apply power-up if ball actor doesn't exist
	}
	// selfPIDStr := "unknown" // Removed unused variable
	// if a.selfPID != nil {
	// 	selfPIDStr = a.selfPID.String()
	// }
	// logPrefix := fmt.Sprintf("PhysicsTick %s Ball %d: ", selfPIDStr, ball.Id) // Removed log

	powerUpType := rand.Intn(3) // 0: Spawn Ball, 1: Increase Mass, 2: Increase Velocity

	switch powerUpType {
	case 0: // Spawn Ball
		spawnX := brickCol*a.cfg.CellSize + a.cfg.CellSize/2
		spawnY := brickRow*a.cfg.CellSize + a.cfg.CellSize/2
		// Spawn slightly offset from the brick center
		spawnX += rand.Intn(a.cfg.CellSize/2) - a.cfg.CellSize/4
		spawnY += rand.Intn(a.cfg.CellSize/2) - a.cfg.CellSize/4

		// fmt.Printf("%s PowerUp: Spawning new ball for owner %d at (%d, %d)\n", logPrefix, ball.OwnerIndex, spawnX, spawnY) // Removed log
		// Spawn a new temporary ball owned by the player who broke the brick
		// Set initial phasing for the spawned ball to avoid immediate re-collision
		a.spawnBall(ctx, ball.OwnerIndex, spawnX, spawnY, a.cfg.PowerUpSpawnBallExpiry, false, true) // isPermanent=false, setInitialPhasing=true

	case 1: // Increase Mass
		// fmt.Printf("%s PowerUp: Increasing mass by %d\n", logPrefix, a.cfg.PowerUpIncreaseMassAdd) // Removed log
		// Send command to the *original* ball that broke the brick
		a.engine.Send(ballActorPID, IncreaseMassCommand{Additional: a.cfg.PowerUpIncreaseMassAdd}, a.selfPID)
		// Update cache immediately (BallActor will confirm via BallStateUpdate later)
		ball.IncreaseMass(a.cfg, a.cfg.PowerUpIncreaseMassAdd)

	case 2: // Increase Velocity
		// fmt.Printf("%s PowerUp: Increasing velocity by ratio %.2f\n", logPrefix, a.cfg.PowerUpIncreaseVelRatio) // Removed log
		// Send command to the *original* ball that broke the brick
		a.engine.Send(ballActorPID, IncreaseVelocityCommand{Ratio: a.cfg.PowerUpIncreaseVelRatio}, a.selfPID)
		// Update cache immediately (BallActor will confirm via BallStateUpdate later)
		ball.IncreaseVelocity(a.cfg.PowerUpIncreaseVelRatio)
	}
}

// getBallCollisionGridBounds calculates the min/max grid indices the ball might overlap with.
func (a *GameActor) getBallCollisionGridBounds(ball *Ball, gridSize int) (minCol, maxCol, minRow, maxRow int) {
	if ball == nil || a.cfg.CellSize <= 0 {
		return 0, 0, 0, 0
	}
	cellSize := a.cfg.CellSize
	radius := ball.Radius

	// Calculate potential grid range based on ball's bounding box
	minCol = (ball.X - radius) / cellSize
	maxCol = (ball.X + radius) / cellSize
	minRow = (ball.Y - radius) / cellSize
	maxRow = (ball.Y + radius) / cellSize

	// Clamp to grid boundaries
	minCol = utils.MaxInt(0, utils.MinInt(gridSize-1, minCol))
	maxCol = utils.MaxInt(0, utils.MinInt(gridSize-1, maxCol))
	minRow = utils.MaxInt(0, utils.MinInt(gridSize-1, minRow))
	maxRow = utils.MaxInt(0, utils.MinInt(gridSize-1, maxRow))

	return minCol, maxCol, minRow, maxRow
}

// makeBrickID creates a unique integer ID for a brick based on its row and column.
// Assumes gridSize is positive.
func makeBrickID(row, col, gridSize int) int {
	// Offset by MaxPlayers to avoid collision with paddle indices (0-3)
	// Ensure row/col are within bounds just in case
	r := utils.MaxInt(0, utils.MinInt(gridSize-1, row))
	c := utils.MaxInt(0, utils.MinInt(gridSize-1, col))
	return utils.MaxPlayers + r*gridSize + c
}

// --- Phasing Timer Management ---

// startPhasingTimer starts a timer for a ball's phasing duration.
func (a *GameActor) startPhasingTimer(ballID int) {
	a.phasingTimersMu.Lock()
	defer a.phasingTimersMu.Unlock()
	// selfPIDStr := "unknown" // Removed unused variable
	// if a.selfPID != nil {
	// 	selfPIDStr = a.selfPID.String()
	// }

	// Stop existing timer for this ball, if any
	if timer, exists := a.phasingTimers[ballID]; exists && timer != nil {
		// fmt.Printf("PhasingTimer %s Ball %d: Stopping existing timer.\n", selfPIDStr, ballID) // Removed log
		timer.Stop()
	}

	// fmt.Printf("PhasingTimer %s Ball %d: Starting new timer for %v.\n", selfPIDStr, ballID, a.cfg.BallPhasingTime) // Removed log

	// Create new timer
	timer := time.AfterFunc(a.cfg.BallPhasingTime, func() {
		// Send message back to self to handle timer expiry in actor context
		if a.engine != nil && a.selfPID != nil {
			a.engine.Send(a.selfPID, stopPhasingTimerMsg{BallID: ballID}, nil)
		}
	})
	a.phasingTimers[ballID] = timer
}

// stopPhasingTimer stops and removes the phasing timer for a ball.
func (a *GameActor) stopPhasingTimer(ballID int) {
	a.phasingTimersMu.Lock()
	defer a.phasingTimersMu.Unlock()
	// selfPIDStr := "unknown" // Removed unused variable
	// if a.selfPID != nil {
	// 	selfPIDStr = a.selfPID.String()
	// }

	if timer, exists := a.phasingTimers[ballID]; exists && timer != nil {
		// fmt.Printf("PhasingTimer %s Ball %d: Explicitly stopping and removing timer.\n", selfPIDStr, ballID) // Removed log
		timer.Stop()
		delete(a.phasingTimers, ballID)
	}
}