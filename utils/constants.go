package utils

import "time"

const (
	Period = 20 * time.Millisecond

	CanvasSize = 576 //INFO Must be divisible by GridSize
	GridSize   = 12  //INFO Must be divisible by 2

	CellSize    = CanvasSize / GridSize
	MinVelocity = CanvasSize / 200
	MaxVelocity = CanvasSize / 150

	NumberOfVectors       = GridSize * 2
	MaxVectorSize         = GridSize
	NumberOfRandomWalkers = GridSize / 4
	NumberOfRandomSteps   = GridSize / 2

	BallSize     = CellSize / 4
	PaddleLength = CellSize * 3
	PaddleWeight = CellSize / 2
)

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
