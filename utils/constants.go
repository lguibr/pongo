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

var CellTypes = map[string]string{
	"Empty": "Empty",
	"Brick": "Brick",
	"Block": "Block",
}
