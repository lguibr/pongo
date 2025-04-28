// File: game/game_actor_physics.go
package game

import (
	"errors" // Import errors
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync" // Import atomic
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
)

// AskTimeout defines how long GameActor waits for a response from child actors.
const AskTimeout = 5 * time.Millisecond // Short timeout for position queries

// detectCollisions checks for and handles collisions. Queries child state first using Ask.
// Assumes called within the actor's message loop (no lock needed for main state).
func (a *GameActor) detectCollisions(ctx bollywood.Context) {
	currentEngine := a.engine
	if currentEngine == nil {
		return
	}

	// --- 1. Get PIDs of active children ---
	// No lock needed - reading actor state within actor loop
	paddlePIDs := make(map[int]*bollywood.PID)
	ballPIDs := make(map[int]*bollywood.PID)
	for i, pid := range a.paddleActors {
		if i >= 0 && i < len(a.players) && a.players[i] != nil && a.players[i].IsConnected && pid != nil {
			paddlePIDs[i] = pid
		}
	}
	for id, pid := range a.ballActors {
		if pid != nil {
			ballPIDs[id] = pid
		}
	}

	// --- 2. Query current state from all children using Ask ---
	var wg sync.WaitGroup
	positionResponses := sync.Map{} // Stores PositionResponse keyed by "paddle-idx" or "ball-id"
	queryErrors := sync.Map{}       // Stores errors keyed by PID string

	// Query Paddles
	for index, pid := range paddlePIDs {
		wg.Add(1)
		go func(idx int, targetPID *bollywood.PID) {
			defer wg.Done()
			reply, err := currentEngine.Ask(targetPID, GetPositionRequest{}, AskTimeout)
			if err != nil {
				if !errors.Is(err, bollywood.ErrTimeout) && !strings.Contains(err.Error(), "not found") {
					queryErrors.Store(targetPID.String(), err)
				}
				return
			}
			if resp, ok := reply.(PositionResponse); ok {
				positionResponses.Store(fmt.Sprintf("paddle-%d", idx), resp)
			} else {
				queryErrors.Store(targetPID.String(), fmt.Errorf("unexpected reply type: %T", reply))
			}
		}(index, pid)
	}

	// Query Balls
	for id, pid := range ballPIDs {
		wg.Add(1)
		go func(ballID int, targetPID *bollywood.PID) {
			defer wg.Done()
			reply, err := currentEngine.Ask(targetPID, GetPositionRequest{}, AskTimeout)
			if err != nil {
				if !errors.Is(err, bollywood.ErrTimeout) && !strings.Contains(err.Error(), "not found") {
					queryErrors.Store(targetPID.String(), err)
				}
				return
			}
			if resp, ok := reply.(PositionResponse); ok {
				positionResponses.Store(fmt.Sprintf("ball-%d", ballID), resp)
			} else {
				queryErrors.Store(targetPID.String(), fmt.Errorf("unexpected reply type: %T", reply))
			}
		}(id, pid)
	}

	wg.Wait() // Wait for all queries to complete or timeout

	queryErrors.Range(func(key, value interface{}) bool {
		fmt.Printf("WARN: GameActor %s: Error querying child %s: %v\n", a.selfPID, key.(string), value.(error))
		return true
	})

	// --- 3. Update GameActor's internal state cache based on successful responses ---
	// No lock needed - modifying actor state within actor loop
	activeBalls := make(map[int]*Ball)
	activePaddles := make(map[int]*Paddle)

	positionResponses.Range(func(key, value interface{}) bool {
		keyStr := key.(string)
		resp := value.(PositionResponse)

		if strings.HasPrefix(keyStr, "ball-") {
			var ballID int
			fmt.Sscanf(keyStr, "ball-%d", &ballID)
			if ball, ok := a.balls[ballID]; ok && ball != nil {
				ball.X, ball.Y, ball.Vx, ball.Vy = resp.X, resp.Y, resp.Vx, resp.Vy
				ball.Radius, ball.Phasing = resp.Radius, resp.Phasing
				activeBalls[ballID] = ball
			}
		} else if strings.HasPrefix(keyStr, "paddle-") {
			var paddleIndex int
			fmt.Sscanf(keyStr, "paddle-%d", &paddleIndex)
			if paddleIndex >= 0 && paddleIndex < len(a.paddles) && a.paddles[paddleIndex] != nil {
				paddle := a.paddles[paddleIndex]
				if paddleIndex >= 0 && paddleIndex < len(a.players) && a.players[paddleIndex] != nil && a.players[paddleIndex].IsConnected {
					paddle.X, paddle.Y, paddle.Vx, paddle.Vy = resp.X, resp.Y, resp.Vx, resp.Vy
					paddle.Width, paddle.Height, paddle.IsMoving = resp.Width, resp.Height, resp.IsMoving
					activePaddles[paddleIndex] = paddle
				}
			}
		}
		return true
	})

	// --- 4. Perform Collision Detection using updated state ---
	// No lock needed - modifying actor state within actor loop
	cellSize := a.cfg.CellSize
	canvasSize := a.cfg.CanvasSize
	ballsToRemove := []int{}
	powerUpsToTrigger := []Ball{}

	for ballID, ball := range activeBalls {
		ballActorPID, pidExists := a.ballActors[ballID]
		if !pidExists || ballActorPID == nil {
			continue
		}

		shouldPhase := false
		reflectedX := false
		reflectedY := false

		// 4.1 Wall Collisions
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
			if concederIndex >= 0 && concederIndex < len(a.players) && a.players[concederIndex] != nil && a.players[concederIndex].IsConnected {
				pInfo := a.players[concederIndex]
				if axisToReflect != "" {
					currentEngine.Send(ballActorPID, ReflectVelocityCommand{Axis: axisToReflect}, nil)
				}
				shouldPhase = true
				scorerIndex := ball.OwnerIndex
				if scorerIndex >= 0 && scorerIndex < len(a.players) && a.players[scorerIndex] != nil && a.players[scorerIndex].IsConnected && scorerIndex != concederIndex {
					scInfo := a.players[scorerIndex]
					scInfo.Score.Add(1)
					pInfo.Score.Add(-1)
				} else if scorerIndex == -1 {
					pInfo.Score.Add(-1)
				}
			} else {
				if ball.IsPermanent {
					if axisToReflect != "" {
						currentEngine.Send(ballActorPID, ReflectVelocityCommand{Axis: axisToReflect}, nil)
					}
					shouldPhase = true
				} else {
					ballsToRemove = append(ballsToRemove, ballID)
					continue
				}
			}
		}

		// 4.2 Paddle Collisions
		for paddleIndex, paddle := range activePaddles {
			if paddle == nil {
				continue
			}
			if !(paddleIndex >= 0 && paddleIndex < len(a.players) && a.players[paddleIndex] != nil && a.players[paddleIndex].IsConnected) {
				continue
			}

			if !ball.Phasing && ball.BallInterceptPaddles(paddle) {
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

				currentEngine.Send(ballActorPID, SetVelocityCommand{Vx: finalVxInt, Vy: finalVyInt}, nil)
				ball.OwnerIndex = paddleIndex
				shouldPhase = true
				goto nextBall
			}
		}

		// 4.3 Brick Collisions
		if !ball.Phasing {
			collidedCells := a.findCollidingCells(ball, cellSize)
			for _, cellPos := range collidedCells {
				col, row := cellPos[0], cellPos[1]
				if col < 0 || col >= a.cfg.GridSize || row < 0 || row >= a.cfg.GridSize {
					continue
				}
				cell := &a.canvas.Grid[col][row] // Modify grid directly

				if cell.Data.Type == utils.Cells.Brick {
					brickLevel := cell.Data.Level
					cell.Data.Life--

					dx := float64(ball.X - (col*cellSize + cellSize/2))
					dy := float64(ball.Y - (row*cellSize + cellSize/2))

					if math.Abs(dx) > math.Abs(dy) {
						if !reflectedX {
							currentEngine.Send(ballActorPID, ReflectVelocityCommand{Axis: "X"}, nil)
							reflectedX = true
						}
					} else {
						if !reflectedY {
							currentEngine.Send(ballActorPID, ReflectVelocityCommand{Axis: "Y"}, nil)
							reflectedY = true
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
							ballStateCopy := *ball
							powerUpsToTrigger = append(powerUpsToTrigger, ballStateCopy)
						}
					}
					shouldPhase = true
					goto nextBall
				}
			}
		}

	nextBall:
		if shouldPhase {
			currentEngine.Send(ballActorPID, SetPhasingCommand{}, nil)
		}
	} // End ball loop

	// --- 5. Handle Ball Removals and Power-ups ---
	pidsToStop := make([]*bollywood.PID, 0, len(ballsToRemove))
	if len(ballsToRemove) > 0 {
		for _, ballID := range ballsToRemove {
			if pid, ok := a.ballActors[ballID]; ok && pid != nil {
				pidsToStop = append(pidsToStop, pid)
			}
			delete(a.balls, ballID)
			delete(a.ballActors, ballID)
		}
	}
	for _, ballState := range powerUpsToTrigger {
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
// Assumes called within the actor's message loop.
func (a *GameActor) triggerRandomPowerUp(ctx bollywood.Context, ball *Ball) {
	if ball == nil || a.engine == nil || a.selfPID == nil {
		return
	}
	powerUpType := rand.Intn(3)
	ballActorPID, actorExists := a.ballActors[ball.Id]
	_, stateExists := a.balls[ball.Id]
	if !actorExists || !stateExists || ballActorPID == nil {
		return
	}
	ownerIndex := ball.OwnerIndex
	ownerValid := false
	if ownerIndex >= 0 && ownerIndex < len(a.players) && a.players[ownerIndex] != nil && a.players[ownerIndex].IsConnected {
		ownerValid = true
	}

	switch powerUpType {
	case 0: // SpawnBall
		if ownerValid {
			a.engine.Send(a.selfPID, SpawnBallCommand{
				OwnerIndex: ownerIndex, X: ball.X, Y: ball.Y,
				ExpireIn: a.cfg.PowerUpSpawnBallExpiry, IsPermanent: false,
			}, nil)
		}
	case 1: // IncreaseMass
		a.engine.Send(ballActorPID, IncreaseMassCommand{Additional: a.cfg.PowerUpIncreaseMassAdd}, nil)
	case 2: // IncreaseVelocity
		a.engine.Send(ballActorPID, IncreaseVelocityCommand{Ratio: a.cfg.PowerUpIncreaseVelRatio}, nil)
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
