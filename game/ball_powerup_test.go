package game

import (
	"testing"

	"github.com/lguibr/pongo/utils"
)

func TestIncreaseVelocityScalesAndKeepsBallMoving(t *testing.T) {
	cases := []struct {
		name           string
		vx, vy         int
		ratio          float64
		wantVx, wantVy int
	}{
		{"scales and floors", 10, -10, 1.09, 10, -11},
		{"a slowed component keeps magnitude one", 1, -1, 0.5, 1, -1},
		{"a zero component stays zero", 0, 4, 2, 0, 8},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ball := &Ball{Vx: tc.vx, Vy: tc.vy}
			ball.IncreaseVelocity(tc.ratio)
			if ball.Vx != tc.wantVx || ball.Vy != tc.wantVy {
				t.Fatalf("velocity (%d, %d), want (%d, %d)", ball.Vx, ball.Vy, tc.wantVx, tc.wantVy)
			}
		})
	}
}

func TestIncreaseMassGrowsRadiusAndKeepsItPositive(t *testing.T) {
	cfg := utils.DefaultConfig()
	ball := &Ball{Mass: 1, Radius: 8}
	ball.IncreaseMass(cfg, 2)
	if ball.Mass != 3 || ball.Radius != 8+2*cfg.PowerUpIncreaseMassSize {
		t.Fatalf("mass %d radius %d after +2", ball.Mass, ball.Radius)
	}
	ball.IncreaseMass(cfg, -100)
	if ball.Radius != 1 {
		t.Fatalf("radius %d, want the minimum of 1", ball.Radius)
	}
}
