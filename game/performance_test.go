package game

import (
	"github.com/lguibr/pongo/internal/actor"
	"github.com/lguibr/pongo/utils"
	"testing"
)

// BenchmarkRoomBroadcastStableGrid measures room-side snapshot work, excluding
// encoding and socket IO (covered by the protocol-correct multiroom load test).
func BenchmarkRoomBroadcastStableGrid(b *testing.B) {
	e := actor.NewEngine()
	a := NewGameActorProducer(e, utils.DefaultConfig(), nil)().(*GameActor)
	a.selfPID = &actor.PID{ID: "room"}
	a.broadcasterPID = &actor.PID{ID: "sink"}
	a.canvas.Grid.FillSymmetrical(a.cfg)
	for i := 0; i < 4; i++ {
		a.paddles[i] = NewPaddle(a.cfg, i)
		a.balls[i] = NewBall(a.cfg, 300+i*20, 300, i, i, true)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.generatePositionUpdates()
		a.handleBroadcastTick(nil)
	}
}
func BenchmarkRoomPhysics(b *testing.B) {
	e := actor.NewEngine()
	cfg := utils.DefaultConfig()
	cfg.PowerUpChance = 0
	a := NewGameActorProducer(e, cfg, nil)().(*GameActor)
	a.selfPID = &actor.PID{ID: "room"}
	a.canvas.Grid.FillSymmetrical(cfg)
	for i := 0; i < 4; i++ {
		a.paddles[i] = NewPaddle(cfg, i)
		a.balls[i] = NewBall(cfg, 300+i*20, 300, i, i, true)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.moveEntities()
		a.detectCollisions(nil)
		a.generatePositionUpdates()
		a.resetPerTickCollisionFlags()
		a.pendingUpdates = a.pendingUpdates[:0]
	}
}
