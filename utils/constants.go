// File: utils/constants.go
package utils

import "time"

// MaxPlayers remains a fundamental constant of the game structure.
const MaxPlayers = 4

// Deprecated constants below. Use values from config.DefaultConfig() instead.

const (
	// Deprecated: Use config.DefaultConfig().GameTickPeriod
	Period = 24 * time.Millisecond

	// Deprecated: Use config.DefaultConfig().InitialScore
	InitialScore = 100

	// Deprecated: Use config.DefaultConfig().CanvasSize
	CanvasSize = 576 // Must be divisible by GridSize
	// Deprecated: Use config.DefaultConfig().GridSize
	GridSize = 12 // Must be divisible by 2

	// Deprecated: Use config.DefaultConfig().CellSize
	CellSize = CanvasSize / GridSize
	// Deprecated: Use config.DefaultConfig().MinBallVelocity
	MinVelocity = CanvasSize / 200
	// Deprecated: Use config.DefaultConfig().MaxBallVelocity
	MaxVelocity = CanvasSize / 150

	// Deprecated: Use config.DefaultConfig().GridFillVectors
	NumberOfVectors = GridSize * 2
	// Deprecated: Use config.DefaultConfig().GridFillVectorSize
	MaxVectorSize = GridSize
	// Deprecated: Use config.DefaultConfig().GridFillWalkers
	NumberOfRandomWalkers = GridSize / 4
	// Deprecated: Use config.DefaultConfig().GridFillSteps
	NumberOfRandomSteps = GridSize / 2

	// Deprecated: Use config.DefaultConfig().BallMass
	BallMass = 1
	// Deprecated: Use config.DefaultConfig().BallRadius
	BallSize = CellSize / 4 // Ball Radius
	// Deprecated: Use config.DefaultConfig().PaddleLength
	PaddleLength = CellSize * 3
	// Deprecated: Use config.DefaultConfig().PaddleWidth
	PaddleWeight = CellSize / 2 // Paddle Width/Thickness
)

// CellType remains as it defines fundamental grid states.
type CellType int64

const (
	brick CellType = iota
	block
	empty
)

type cellTypes struct {
	Brick CellType
	Block CellType
	Empty CellType
}

var Cells = cellTypes{
	Brick: brick,
	Block: block,
	Empty: empty,
}

func (cellType CellType) String() string {
	switch cellType {
	case brick:
		return "Brick"
	case block:
		return "Block"
	case empty:
		return "Empty"
	default:
		return "Unknown"
	}
}
