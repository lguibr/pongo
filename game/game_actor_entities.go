package game

import (
	"encoding/json"
	"log/slog"
	"math/rand"
	"time"

	"github.com/lguibr/pongo/internal/actor"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

// handlePaddleDirection sets the direction of the sending player's paddle.
func (a *GameActor) handlePaddleDirection(ctx actor.Context, wsConn *websocket.Conn, directionData []byte) {
	if wsConn == nil {
		return
	}
	playerIndex, playerFound := a.connToIndex[wsConn]
	if !playerFound || playerIndex < 0 || playerIndex >= utils.MaxPlayers || a.players[playerIndex] == nil || !a.players[playerIndex].IsConnected {
		return
	}
	var direction Direction
	if json.Unmarshal(directionData, &direction) != nil {
		return
	}
	if paddle := a.paddles[playerIndex]; paddle != nil {
		paddle.Direction = utils.DirectionFromString(direction.Direction)
	}
}

// spawnBall adds a ball to the room and queues its BallSpawned update including R3F coords.
// The setInitialPhasing flag determines if the ball starts phasing (used by power-ups).
func (a *GameActor) spawnBall(ctx actor.Context, ownerIndex, x, y int, expireIn time.Duration, isPermanent bool, setInitialPhasing bool) {
	if ownerIndex < -1 || ownerIndex >= utils.MaxPlayers {
		slog.Warn("ball spawn with invalid owner ignored", "room", a.selfPID, "owner", ownerIndex)
		return
	}
	if ownerIndex >= 0 && (a.players[ownerIndex] == nil || !a.players[ownerIndex].IsConnected) {
		return
	}
	a.nextBallID++
	ballID := a.nextBallID
	// Test fixtures may use fixed IDs; never replace an existing entity.
	for a.balls[ballID] != nil {
		a.nextBallID++
		ballID = a.nextBallID
	}
	ball := NewBall(a.cfg, x, y, ownerIndex, ballID, isPermanent)
	a.balls[ballID] = ball
	if setInitialPhasing {
		ball.Phasing = true
		a.startPhasingTimer(ballID)
	}
	rx, ry := mapToR3FCoords(ball.X, ball.Y, a.cfg.CanvasSize)
	a.addUpdate(&BallSpawned{MessageType: "ballSpawned", Ball: *ball, R3fX: rx, R3fY: ry})
	if !isPermanent && expireIn > 0 {
		offset := time.Duration(rand.Intn(4000)-2000) * time.Millisecond
		duration := expireIn + offset
		if duration <= 0 {
			duration = 500 * time.Millisecond
		}
		if a.expiryTimers == nil {
			a.expiryTimers = make(map[int]*time.Timer)
		}
		engine, self := a.engine, a.selfPID
		a.expiryTimers[ballID] = time.AfterFunc(duration, func() { engine.Send(self, DestroyExpiredBall{BallID: ballID}, nil) })
	}
}

// handleDestroyExpiredBall removes a temporary ball and everything that tracks it.
func (a *GameActor) handleDestroyExpiredBall(ctx actor.Context, ballID int) {
	ball := a.balls[ballID]
	if ball == nil || ball.IsPermanent {
		return
	}
	delete(a.balls, ballID)
	if timer := a.expiryTimers[ballID]; timer != nil {
		timer.Stop()
		delete(a.expiryTimers, ballID)
	}
	delete(a.phasingGeneration, ballID)
	a.stopPhasingTimer(ballID)
	for _, key := range a.activeCollisions.GetActiveCollisionsForKey1(ballID) {
		a.activeCollisions.EndCollision(key)
	}
	a.addUpdate(&BallRemoved{MessageType: "ballRemoved", ID: ballID})
}

// handleStopPhasingTimerMsg ends a ball's phasing.
func (a *GameActor) handleStopPhasingTimerMsg(ctx actor.Context, ballID int) {
	a.stopPhasingTimer(ballID)
	if ball := a.balls[ballID]; ball != nil {
		ball.Phasing = false
	}
}
