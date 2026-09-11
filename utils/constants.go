package utils

// MaxPlayers is the number of player slots in a room.
const MaxPlayers = 4

// CellType is the state of a grid cell.
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
