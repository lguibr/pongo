// File: utils/config.go
package utils

import (
	"time"
)

// Config holds all configurable game parameters.
type Config struct {
	// Timing
	GameTickPeriod  time.Duration `json:"gameTickPeriod"`  // Time between game physics updates
	BroadcastRateHz int           `json:"broadcastRateHz"` // Target rate for sending state updates to clients (e.g., 30)

	// Score & Player
	InitialScore int `json:"initialScore"` // Starting score for players

	// Canvas & Grid
	CanvasSize int `json:"canvasSize"` // Pixel dimensions of the square canvas (must be divisible by GridSize)
	GridSize   int `json:"gridSize"`   // Number of cells along one dimension of the grid (must be divisible by 2)
	CellSize   int `json:"cellSize"`   // Calculated: CanvasSize / GridSize

	// Ball Physics & Properties
	MinBallVelocity          int           `json:"minBallVelocity"`          // Minimum speed component for a ball
	MaxBallVelocity          int           `json:"maxBallVelocity"`          // Maximum speed component for a ball (at spawn)
	BallMass                 int           `json:"ballMass"`                 // Default mass of a ball
	BallRadius               int           `json:"ballRadius"`               // Default radius of a ball
	BallPhasingTime          time.Duration `json:"ballPhasingTime"`          // How long a ball phases after collision
	BallHitPaddleSpeedFactor float64       `json:"ballHitPaddleSpeedFactor"` // Multiplier for paddle velocity influence on ball speed
	BallHitPaddleAngleFactor float64       `json:"ballHitPaddleAngleFactor"` // Multiplier for hit offset influence on angle (Pi / this value)

	// Paddle Properties
	PaddleLength   int `json:"paddleLength"`   // Length of the paddle along the wall
	PaddleWidth    int `json:"paddleWidth"`    // Thickness of the paddle
	PaddleVelocity int `json:"paddleVelocity"` // Base speed of the paddle movement

	// Grid Generation (Symmetrical)
	GridFillDensity       float64 `json:"gridFillDensity"`       // Probability (0.0 to 1.0) for a cell in the safe zone to become a brick
	GridClearCenterRadius int     `json:"gridClearCenterRadius"` // Radius around the absolute center to keep clear of bricks
	GridClearWallDistance int     `json:"gridClearWallDistance"` // Number of cells from each wall to keep clear of bricks
	GridBrickMinLife      int     `json:"gridBrickMinLife"`      // Minimum life for generated bricks (>= 1)
	GridBrickMaxLife      int     `json:"gridBrickMaxLife"`      // Maximum life for generated bricks

	// Power-ups
	PowerUpChance           float64       `json:"powerUpChance"`           // Chance (0.0 to 1.0) to trigger power-up on brick break
	PowerUpSpawnBallExpiry  time.Duration `json:"powerUpSpawnBallExpiry"`  // Duration after which spawned power-up balls expire (randomized around this)
	PowerUpIncreaseMassAdd  int           `json:"powerUpIncreaseMassAdd"`  // Mass added by power-up
	PowerUpIncreaseMassSize int           `json:"powerUpIncreaseMassSize"` // Radius added per mass point by power-up
	PowerUpIncreaseVelRatio float64       `json:"powerUpIncreaseVelRatio"` // Velocity multiplier for power-up
}

// DefaultConfig returns a Config struct with default values.
func DefaultConfig() Config {
	canvasSize := 900
	gridSize := 18 // Must be even
	cellSize := canvasSize / gridSize

	return Config{
		// Timing
		GameTickPeriod:  25 * time.Millisecond, // ~62.5 Hz physics updates (Adjusted for common refresh rates)
		BroadcastRateHz: 60,                    // INCREASED Target 60Hz network updates

		// Score & Player
		InitialScore: 0,

		// Canvas & Grid
		CanvasSize: canvasSize,
		GridSize:   gridSize,
		CellSize:   cellSize,

		// Ball Physics & Properties
		MinBallVelocity:          canvasSize / 180, // ~5.68 -> Adjusted to ~6
		MaxBallVelocity:          canvasSize / 90,  // ~11.37 -> Adjusted to ~13
		BallMass:                 1,
		BallRadius:               cellSize / 6, // ~16
		BallPhasingTime:          3000 * time.Millisecond,
		BallHitPaddleSpeedFactor: 0.3,
		BallHitPaddleAngleFactor: 2.8, // Max ~64 degrees deflection (Pi / 2.8)

		// Paddle Properties
		PaddleLength:   cellSize * 3, // 300
		PaddleWidth:    cellSize / 2, // 50
		PaddleVelocity: cellSize / 4, // 25

		// Grid Generation (Symmetrical)
		GridFillDensity:       0.55,
		GridClearCenterRadius: 1, // Clear 5x5 area in center (radius 2)
		GridClearWallDistance: 3, // Keep 3 cells clear from walls
		GridBrickMinLife:      1, // Bricks have 1-3 life
		GridBrickMaxLife:      7,

		// Power-ups
		PowerUpChance:           0.4,
		PowerUpSpawnBallExpiry:  12 * time.Second,
		PowerUpIncreaseMassAdd:  2,
		PowerUpIncreaseMassSize: 2,
		PowerUpIncreaseVelRatio: 1.09,
	}
}

// E2ETestConfig returns a config suitable for E2E tests, ensuring a playable grid.
func E2ETestConfig() Config {
	cfg := DefaultConfig()

	// Ensure a non-empty grid for basic tests
	cfg.GridFillDensity = 0.7     // Higher density
	cfg.GridClearCenterRadius = 1 // Smaller clear radius
	cfg.GridClearWallDistance = 2 // Smaller wall distance
	cfg.GridBrickMinLife = 1
	cfg.GridBrickMaxLife = 2 // Lower max life for faster testing

	// Slightly faster ticks for quicker test execution
	cfg.GameTickPeriod = 20 * time.Millisecond
	cfg.BroadcastRateHz = 50 // Slightly lower broadcast to reduce noise if needed

	return cfg
}

// FastGameConfig returns a config optimized for rapid game completion (used for testing).
func FastGameConfig() Config {
	cfg := DefaultConfig() // Start with defaults

	// Smaller grid, fewer bricks initially
	cfg.CanvasSize = 512                         // Must be divisible by GridSize
	cfg.GridSize = 8                             // Must be divisible by 2
	cfg.CellSize = cfg.CanvasSize / cfg.GridSize // 64

	// Symmetrical Grid Generation Params for Fast Config
	cfg.GridFillDensity = 0.35 // Lower density for faster clearing
	cfg.GridClearCenterRadius = 1
	cfg.GridClearWallDistance = 2
	cfg.GridBrickMinLife = 1 // Bricks have only 1 life
	cfg.GridBrickMaxLife = 1

	// Faster game loop
	cfg.GameTickPeriod = 16 * time.Millisecond // ~60 FPS physics
	cfg.BroadcastRateHz = 60                   // Keep broadcast rate high

	// Faster balls
	cfg.MinBallVelocity = cfg.CanvasSize / 60 // ~8.5
	cfg.MaxBallVelocity = cfg.CanvasSize / 40 // ~12.8
	cfg.BallRadius = cfg.CellSize / 4         // 16

	// Less phasing
	cfg.BallPhasingTime = 50 * time.Millisecond

	// Lower power-up chance to avoid too many balls complicating completion
	cfg.PowerUpChance = 0.1
	cfg.PowerUpSpawnBallExpiry = 5 * time.Second

	// Faster paddles (though not actively used by clients in this test)
	cfg.PaddleVelocity = cfg.CellSize / 2 // 32

	// Adjust paddle size relative to new cell size
	cfg.PaddleLength = cfg.CellSize * 2 // 128
	cfg.PaddleWidth = cfg.CellSize / 3  // ~21

	return cfg
}

// UltraFastGameConfig returns a config optimized for extremely rapid game completion.
func UltraFastGameConfig() Config {
	cfg := DefaultConfig() // Start with defaults

	// Tiny grid, very few bricks
	cfg.CanvasSize = 240                         // Divisible by 6
	cfg.GridSize = 6                             // Minimum allowed, even size
	cfg.CellSize = cfg.CanvasSize / cfg.GridSize // 40

	// Symmetrical Grid Generation Params for UltraFast Config
	cfg.GridFillDensity = 0.25 // Very low density
	cfg.GridClearCenterRadius = 1
	cfg.GridClearWallDistance = 1 // Minimal wall clearance
	cfg.GridBrickMinLife = 1      // Bricks have only 1 life
	cfg.GridBrickMaxLife = 1

	// Faster game loop
	cfg.GameTickPeriod = 16 * time.Millisecond // ~60 FPS physics
	cfg.BroadcastRateHz = 60                   // Keep broadcast rate high

	// Very fast balls
	cfg.MinBallVelocity = cfg.CanvasSize / 20 // 12
	cfg.MaxBallVelocity = cfg.CanvasSize / 15 // 16
	cfg.BallRadius = cfg.CellSize / 5         // 8 (Smaller radius)

	// Very short phasing
	cfg.BallPhasingTime = 20 * time.Millisecond

	// Moderate power-up chance, short expiry
	cfg.PowerUpChance = 0.25
	cfg.PowerUpSpawnBallExpiry = 3 * time.Second

	// Faster paddles
	cfg.PaddleVelocity = cfg.CellSize // 40

	// Adjust paddle size
	cfg.PaddleLength = cfg.CellSize * 2 // 80
	cfg.PaddleWidth = cfg.CellSize / 4  // 10

	return cfg
}

// BrickCollisionTestConfig returns a config for testing ball-brick collision behavior.
func BrickCollisionTestConfig() Config {
	cfg := DefaultConfig() // Start with defaults

	cfg.PowerUpChance = 0.0     // No power-ups to interfere
	cfg.GridBrickMinLife = 1000 // Bricks are effectively indestructible
	cfg.GridBrickMaxLife = 1000
	cfg.GridFillDensity = 0.8     // Dense brick field
	cfg.GridClearCenterRadius = 0 // Allow bricks in the center
	cfg.GridClearWallDistance = 1 // Minimal wall clearance for more brick area

	// Ball properties for robust collision detection
	// Ensure BallRadius is < CellSize / 2
	// Default CellSize = 900/18 = 50. Default BallRadius = 50/6 = ~8.
	// Let's make it slightly larger for this test.
	cfg.BallRadius = cfg.CellSize / 3 // e.g., 50/3 = ~16. (CellSize/2 = 25)

	// Faster game simulation for more interactions
	cfg.GameTickPeriod = 16 * time.Millisecond // ~60Hz physics
	cfg.BroadcastRateHz = 60                   // High broadcast rate

	// Ensure balls move at a reasonable speed
	cfg.MinBallVelocity = cfg.CanvasSize / 100 // e.g., 900/100 = 9
	cfg.MaxBallVelocity = cfg.CanvasSize / 70  // e.g., 900/70 = ~12

	return cfg
}
