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

	// --- Capture Phasing State AT START of Tick from GameActor's cache ---
	// This cache is updated by BallStateUpdate from BallActor.
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
		ballActorPID := a.ballActors[id]      // Get PID for sending commands
		isPhasing := phasingStateThisTick[id] // Use captured phasing state for wall collision logic

		// Check Right Wall (Player 0)
		if ball.X+ball.Radius >= canvasSize {
			a.handleWallCollision(ctx, ball, ballActorPID, 0, isPhasing)
		}
		// Check Top Wall (Player 1)
		if ball.Y-ball.Radius <= 0 {
			a.handleWallCollision(ctx, ball, ballActorPID, 1, isPhasing)
		}
		// Check Left Wall (Player 2)
		if ball.X-ball.Radius <= 0 {
			a.handleWallCollision(ctx, ball, ballActorPID, 2, isPhasing)
		}
		// Check Bottom Wall (Player 3)
		if ball.Y+ball.Radius >= canvasSize {
			a.handleWallCollision(ctx, ball, ballActorPID, 3, isPhasing)
		}
	}

	// --- Ball-Paddle Collisions ---
	for id, ball := range a.balls {
		if ball == nil {
			continue
		}
		ballActorPID := a.ballActors[id] // Get PID
		// isPhasingForPaddle := phasingStateThisTick[id] // Phasing state doesn't change paddle collision mechanics

		for playerIndex, paddle := range a.paddles {
			if paddle == nil {
				continue
			}
			collisionKey := CollisionKey{Object1ID: id, Object2ID: playerIndex} // Use playerIndex as Object2ID for paddles

			if ball.BallInterceptPaddles(paddle) {
				isNewPaddleCollision := a.activeCollisions.BeginCollision(collisionKey)
				// Always mark as collided if intersecting, regardless of new or ongoing
				ball.Collided = true
				paddle.Collided = true // Also mark paddle as collided

				if isNewPaddleCollision {
					a.handlePaddleCollision(ctx, ball, ballActorPID, paddle, playerIndex) // Pass PID
				} else {
					// Ongoing collision: Ball is still intersecting paddle.
					// Physics (reflection, ownership) were handled on first contact.
					// No positional adjustment needed.
				}
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
		ballActorPID := a.ballActors[id]      // Get PID
		isPhasing := phasingStateThisTick[id] // Use captured state for this tick's brick collision logic

		// Get active brick collisions for this ball that might need ending
		activeBrickCollisions := a.activeCollisions.GetActiveCollisionsForKey1(id)
		collidedBrickIDsThisTick := make(map[int]bool) // Track bricks hit this tick

		// Iterate through potentially colliding grid cells around the ball
		minCol, maxCol, minRow, maxRow := a.getBallCollisionGridBounds(ball, gridSize)

		for r := minRow; r <= maxRow; r++ {
			for c := minCol; c <= maxCol; c++ {
				cell := &a.canvas.Grid[r][c] // Get pointer to cell
				if cell.Data == nil || cell.Data.Type != utils.Cells.Brick {
					continue // Skip empty or non-brick cells
				}

				brickID := makeBrickID(r, c, gridSize) // Unique ID for this brick cell
				collisionKey := CollisionKey{Object1ID: id, Object2ID: brickID}

				if ball.InterceptsIndex(c, r, cellSize) {
					collidedBrickIDsThisTick[brickID] = true // Mark as hit this tick
					isNewBrickCollision := a.activeCollisions.BeginCollision(collisionKey)

					// Phasing ball logic: Damage once per tick, no reflection, no phasing reset
					if isPhasing { // Use captured state from the start of the tick
						if isNewBrickCollision { // Damage only on first contact per phasing cycle with this brick
							a.damageBrick(ctx, ball, cell, r, c) // Damage brick, may trigger power-up
						}
						// Phasing balls that intersect a brick can also set their Collided flag
						// ball.Collided = true; // Optional: if phasing through should also set flag
					} else { // Non-phasing ball logic
						ball.Collided = true // Always mark as collided if intersecting and non-phasing

						if isNewBrickCollision { // Process collision only on first contact
							a.handleBrickCollision(ctx, ball, ballActorPID, cell, r, c) // Pass PID
						} else {
							// Ongoing collision: Ball is still intersecting non-phasing brick.
							// Damage/reflection handled on first contact.
							// No positional adjustment needed.
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
			// Check if the Object2ID corresponds to a brick (brick IDs are >= MaxPlayers)
			if key.Object2ID >= utils.MaxPlayers { // Assuming paddle indices are < MaxPlayers
				if _, hitThisTick := collidedBrickIDsThisTick[key.Object2ID]; !hitThisTick {
					a.activeCollisions.EndCollision(key)
				}
			}
		}
	} // end ball loop
}

// handleWallCollision processes ball hitting a wall.
// Phasing balls reflect but do not trigger scoring or phasing reset.
// Non-phasing balls hitting walls NO LONGER start phasing from this interaction.
// Direct position adjustments (e.g., ball.X = ...) are REMOVED.
func (a *GameActor) handleWallCollision(ctx bollywood.Context, ball *Ball, ballActorPID *bollywood.PID, wallIndex int, isPhasing bool) {
	if ball == nil || ballActorPID == nil {
		return
	}

	// 1. Reflect Velocity (no direct position adjustment)
	switch wallIndex {
	case 0: // Right
		if ball.Vx > 0 { // Only reflect if moving towards the wall
			ball.ReflectVelocity("X")
		}
	case 1: // Top
		if ball.Vy < 0 { // Only reflect if moving towards the wall
			ball.ReflectVelocity("Y")
		}
	case 2: // Left
		if ball.Vx < 0 { // Only reflect if moving towards the wall
			ball.ReflectVelocity("X")
		}
	case 3: // Bottom
		if ball.Vy > 0 { // Only reflect if moving towards the wall
			ball.ReflectVelocity("Y")
		}
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
	}
}

// handlePaddleCollision processes ball hitting a paddle.
// Phasing balls reflect and change owner, but do not reset phasing timer.
// Non-phasing balls hitting paddles NO LONGER start phasing from this interaction.
// Direct position adjustments (e.g., ball.X = ...) are REMOVED.
func (a *GameActor) handlePaddleCollision(ctx bollywood.Context, ball *Ball, ballActorPID *bollywood.PID, paddle *Paddle, playerIndex int) {
	if ball == nil || paddle == nil || ballActorPID == nil {
		return
	}

	// 1. Calculate Reflection (Complex - simplified example)
	speedFactor := a.cfg.BallHitPaddleSpeedFactor
	angleFactor := a.cfg.BallHitPaddleAngleFactor

	var relativeHitPos float64
	var paddleCenter float64
	var paddleSpan float64

	if paddle.Width > paddle.Height { // Horizontal paddle
		paddleCenter = float64(paddle.X + paddle.Width/2)
		relativeHitPos = float64(ball.X) - paddleCenter
		paddleSpan = float64(paddle.Width)
		// No positional adjustment here
	} else { // Vertical paddle
		paddleCenter = float64(paddle.Y + paddle.Height/2)
		relativeHitPos = float64(ball.Y) - paddleCenter
		paddleSpan = float64(paddle.Height)
		// No positional adjustment here
	}

	normalizedHitPos := (relativeHitPos / (paddleSpan / 2.0)) * 1.1
	normalizedHitPos = math.Max(-1.0, math.Min(1.0, normalizedHitPos))

	maxAngle := math.Pi / angleFactor
	reflectionAngle := normalizedHitPos * maxAngle

	if playerIndex == 0 || playerIndex == 1 {
		reflectionAngle = -reflectionAngle
	}

	var baseVx, baseVy float64
	switch playerIndex {
	case 0:
		baseVx, baseVy = -1, 0
	case 1:
		baseVx, baseVy = 0, 1
	case 2:
		baseVx, baseVy = 1, 0
	case 3:
		baseVx, baseVy = 0, -1
	}

	cosAngle := math.Cos(reflectionAngle)
	sinAngle := math.Sin(reflectionAngle)
	newVx := baseVx*cosAngle - baseVy*sinAngle
	newVy := baseVx*sinAngle + baseVy*cosAngle

	currentSpeed := math.Sqrt(float64(ball.Vx*ball.Vx + ball.Vy*ball.Vy))
	paddleVelComponent := 0.0
	if paddle.Width > paddle.Height {
		paddleVelComponent = float64(paddle.Vx) * newVx
	} else {
		paddleVelComponent = float64(paddle.Vy) * newVy
	}

	newSpeed := currentSpeed + (paddleVelComponent * speedFactor)
	newSpeed = math.Max(float64(a.cfg.MinBallVelocity), math.Min(float64(a.cfg.MaxBallVelocity)*1.2, newSpeed))

	finalVx := int(newVx * newSpeed)
	finalVy := int(newVy * newSpeed)

	if newSpeed > 0 {
		if finalVx == 0 {
			finalVx = int(math.Copysign(1.0, newVx))
		}
		if finalVy == 0 {
			finalVy = int(math.Copysign(1.0, newVy))
		}
	}

	// 2. Update Ball State in Cache
	ball.Vx = finalVx
	ball.Vy = finalVy
	ball.OwnerIndex = playerIndex

	// 3. Send Commands/Updates
	a.engine.Send(ballActorPID, SetVelocityCommand{Vx: finalVx, Vy: finalVy}, a.selfPID)
	a.addUpdate(&BallOwnershipChange{MessageType: "ballOwnerChanged", ID: ball.Id, NewOwnerIndex: playerIndex})
}

// handleBrickCollision processes non-phasing ball hitting a brick.
// It reflects the ball's velocity without direct position adjustment.
func (a *GameActor) handleBrickCollision(ctx bollywood.Context, ball *Ball, ballActorPID *bollywood.PID, cell *Cell, r, c int) {
	if ball == nil || cell == nil || cell.Data == nil || ballActorPID == nil {
		return
	}
	if ball.Phasing {
		return
	}

	// Determine hit face based on overlap or incoming velocity relative to brick center
	// This is a simplified approach. A more robust method would use the ball's previous position.
	cellCenterX := c*a.cfg.CellSize + a.cfg.CellSize/2
	cellCenterY := r*a.cfg.CellSize + a.cfg.CellSize/2

	// Calculate vector from brick center to ball center
	deltaX := ball.X - cellCenterX
	deltaY := ball.Y - cellCenterY

	// Determine primary axis of collision based on which side of the brick center the ball is
	// and how much it has penetrated along each axis relative to its velocity.
	// This is a complex problem to solve perfectly without sub-tick physics.
	// A common simplification is to check penetration depth.
	overlapX := float64(ball.Radius+a.cfg.CellSize/2) - math.Abs(float64(deltaX))
	overlapY := float64(ball.Radius+a.cfg.CellSize/2) - math.Abs(float64(deltaY))

	// Reflect based on the axis of shallower penetration, or prefer Y if equal.
	// This helps with corner hits.
	if overlapX > 0 && overlapY > 0 { // Intersecting
		if overlapX < overlapY { // Hit a vertical face more directly
			if (ball.Vx > 0 && deltaX < 0) || (ball.Vx < 0 && deltaX > 0) { // Moving towards brick center X
				ball.ReflectVelocity("X")
			}
		} else { // Hit a horizontal face more directly, or corner preferring Y
			if (ball.Vy > 0 && deltaY < 0) || (ball.Vy < 0 && deltaY > 0) { // Moving towards brick center Y
				ball.ReflectVelocity("Y")
			}
		}
		// If it's a corner and one reflection didn't stop penetration on the other axis,
		// it might require reflecting both. For simplicity, we reflect based on primary impact.
		// A more advanced system would check if after one reflection, it's still colliding.
	}

	// Send command to BallActor to update its internal velocity state
	a.engine.Send(ballActorPID, SetVelocityCommand{Vx: ball.Vx, Vy: ball.Vy}, a.selfPID)

	// 2. Damage Brick
	_ = a.damageBrick(ctx, ball, cell, r, c) // Handles scoring and power-ups
}

// damageBrick reduces brick life, handles destruction, scoring, and power-ups.
// Returns true if the brick was destroyed.
func (a *GameActor) damageBrick(ctx bollywood.Context, ball *Ball, cell *Cell, r, c int) bool {
	if cell == nil || cell.Data == nil || cell.Data.Life <= 0 {
		return false // Already destroyed or not a valid brick
	}

	cell.Data.Life--
	destroyed := false

	if cell.Data.Life <= 0 {
		destroyed = true
		brickLevel := cell.Data.Level // Store level before clearing data
		cell.Data.Type = utils.Cells.Empty
		cell.Data.Level = 0

		// Award score to ball owner
		scorerIndex := ball.OwnerIndex
		if scorerIndex >= 0 && scorerIndex < utils.MaxPlayers && a.players[scorerIndex] != nil && a.players[scorerIndex].IsConnected {
			newScore := a.players[scorerIndex].Score.Add(int32(brickLevel))
			a.addUpdate(&ScoreUpdate{MessageType: "scoreUpdate", Index: scorerIndex, Score: newScore})
		}

		// Trigger Power-up?
		if rand.Float64() < a.cfg.PowerUpChance {
			a.triggerRandomPowerUp(ctx, ball, r, c)
		}
	}
	return destroyed
}

// triggerRandomPowerUp selects and activates a power-up.
// Phasing is now one of the power-ups.
func (a *GameActor) triggerRandomPowerUp(ctx bollywood.Context, ball *Ball, brickRow, brickCol int) {
	if ball == nil {
		return
	}
	ballActorPID := a.ballActors[ball.Id] // Get PID
	if ballActorPID == nil {
		return // Cannot apply power-up if ball actor doesn't exist
	}

	// Define available power-up types
	const (
		powerUpSpawnBall = iota
		powerUpIncreaseMass
		powerUpIncreaseVelocity
		powerUpStartPhasing // New power-up type
		numPowerUpTypes     // Total number of power-up types
	)

	powerUpType := rand.Intn(numPowerUpTypes)

	switch powerUpType {
	case powerUpSpawnBall:
		spawnX := brickCol*a.cfg.CellSize + a.cfg.CellSize/2
		spawnY := brickRow*a.cfg.CellSize + a.cfg.CellSize/2
		spawnX += rand.Intn(a.cfg.CellSize/2) - a.cfg.CellSize/4
		spawnY += rand.Intn(a.cfg.CellSize/2) - a.cfg.CellSize/4
		// Spawn a new temporary, NON-PHASING ball
		a.spawnBall(ctx, ball.OwnerIndex, spawnX, spawnY, a.cfg.PowerUpSpawnBallExpiry, false, false)

	case powerUpIncreaseMass:
		a.engine.Send(ballActorPID, IncreaseMassCommand{Additional: a.cfg.PowerUpIncreaseMassAdd}, a.selfPID)
		ball.IncreaseMass(a.cfg, a.cfg.PowerUpIncreaseMassAdd)

	case powerUpIncreaseVelocity:
		a.engine.Send(ballActorPID, IncreaseVelocityCommand{Ratio: a.cfg.PowerUpIncreaseVelRatio}, a.selfPID)
		ball.IncreaseVelocity(a.cfg.PowerUpIncreaseVelRatio)

	case powerUpStartPhasing:
		// Apply phasing to the ball that broke the brick
		// Update GameActor's cache immediately and start its timer
		ball.Phasing = true
		a.startPhasingTimer(ball.Id) // This will stop existing timer and start new one
		// Send command to BallActor so its internal state matches
		a.engine.Send(ballActorPID, SetPhasingCommand{}, a.selfPID)
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

	// Stop existing timer for this ball, if any
	if timer, exists := a.phasingTimers[ballID]; exists && timer != nil {
		timer.Stop()
	}

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

	if timer, exists := a.phasingTimers[ballID]; exists && timer != nil {
		timer.Stop()
		delete(a.phasingTimers, ballID)
	}
}
