// File: utils/config.go
package utils

import "time"

// Config holds all configurable game parameters.
type Config struct {
	// Timing
	GameTickPeriod time.Duration `json:"gameTickPeriod"` // Time between game state updates

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

	// Grid Generation (Procedural)
	GridFillVectors    int `json:"gridFillVectors"`    // Number of vectors for grid generation per quarter
	GridFillVectorSize int `json:"gridFillVectorSize"` // Max length of vectors for grid generation
	GridFillWalkers    int `json:"gridFillWalkers"`    // Number of random walkers per quarter
	GridFillSteps      int `json:"gridFillSteps"`      // Number of steps per random walker

	// Power-ups
	PowerUpChance           float64       `json:"powerUpChance"`           // Chance (0.0 to 1.0) to trigger power-up on brick break
	PowerUpSpawnBallExpiry  time.Duration `json:"powerUpSpawnBallExpiry"`  // Duration after which spawned power-up balls expire (randomized around this)
	PowerUpIncreaseMassAdd  int           `json:"powerUpIncreaseMassAdd"`  // Mass added by power-up
	PowerUpIncreaseMassSize int           `json:"powerUpIncreaseMassSize"` // Radius added per mass point by power-up
	PowerUpIncreaseVelRatio float64       `json:"powerUpIncreaseVelRatio"` // Velocity multiplier for power-up
}

// DefaultConfig returns a Config struct with default values.
func DefaultConfig() Config {
	canvasSize := 1024
	gridSize := 16
	cellSize := canvasSize / gridSize

	return Config{
		// Timing
		GameTickPeriod: 24 * time.Millisecond,

		// Score & Player
		InitialScore: 0,

		// Canvas & Grid
		CanvasSize: canvasSize,
		GridSize:   gridSize,
		CellSize:   cellSize,

		// Ball Physics & Properties
		MinBallVelocity:          canvasSize / 180, // ~5.68
		MaxBallVelocity:          canvasSize / 90,  // ~11.37
		BallMass:                 1,
		BallRadius:               cellSize / 6, // ~10.6
		BallPhasingTime:          100 * time.Millisecond,
		BallHitPaddleSpeedFactor: 0.3,
		BallHitPaddleAngleFactor: 2.8, // Max ~64 degrees deflection (Pi / 2.8)

		// Paddle Properties
		PaddleLength:   cellSize * 3, // 192
		PaddleWidth:    cellSize / 2, // 32
		PaddleVelocity: cellSize / 4, // 16

		// Grid Generation
		GridFillVectors:    gridSize * 2,
		GridFillVectorSize: gridSize,
		GridFillWalkers:    gridSize / 4,
		GridFillSteps:      gridSize / 2,

		// Power-ups
		PowerUpChance:           0.6,
		PowerUpSpawnBallExpiry:  9 * time.Second,
		PowerUpIncreaseMassAdd:  1,
		PowerUpIncreaseMassSize: 2,
		PowerUpIncreaseVelRatio: 1.1,
	}
}

// FastGameConfig returns a config optimized for rapid game completion (used for testing).
func FastGameConfig() Config {
	cfg := DefaultConfig() // Start with defaults

	// Smaller grid, fewer bricks initially
	cfg.CanvasSize = 512                         // Must be divisible by GridSize
	cfg.GridSize = 8                             // Must be divisible by 2
	cfg.CellSize = cfg.CanvasSize / cfg.GridSize // 64

	// Fewer generation steps -> less dense grid
	cfg.GridFillVectors = cfg.GridSize / 2    // 4
	cfg.GridFillVectorSize = cfg.GridSize / 2 // 4
	cfg.GridFillWalkers = cfg.GridSize / 4    // 2
	cfg.GridFillSteps = cfg.GridSize / 4      // 2

	// Faster game loop
	cfg.GameTickPeriod = 16 * time.Millisecond // ~60 FPS physics

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
