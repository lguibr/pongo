package utils

import "time"

const (
	Period = 20 * time.Millisecond

	CanvasSize = 576 //INFO Must be divisible by GridSize
	GridSize   = 12  //INFO Must be divisible by 2

	CellSize    = CanvasSize / GridSize
	MinVelocity = CanvasSize / 200
	MaxVelocity = CanvasSize / 150

	NumberOfVectors       = 2 * GridSize / 3
	MaxVectorSize         = 2 * GridSize / 6
	NumberOfRandomWalkers = 2 * GridSize
	NumberOfRandomSteps   = 2 * GridSize

	BallSize     = CellSize / 3
	PaddleLength = CellSize * 3
	PaddleWeight = CellSize / 2
)

var CellTypes = map[string]string{
	"Empty": "Empty",
	"Brick": "Brick",
	"Block": "Block",
}
