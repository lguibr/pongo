package game

import "github.com/lguibr/pongo/utils"

type BrickData struct {
	Type  utils.CellType `json:"type"`
	Life  int            `json:"life"`
	Level int            `json:"level"`
}
type Cell struct {
	X    int        `json:"x"`
	Y    int        `json:"y"`
	Data *BrickData `json:"data"`
}

func (c *Cell) GetX() int           { return c.X }
func (c *Cell) GetY() int           { return c.Y }
func (c *Cell) GetData() *BrickData { return c.Data }
func (c *Cell) GetType() utils.CellType {
	if c.Data == nil {
		return utils.Cells.Empty // Treat nil Data as Empty
	}
	return c.Data.Type
}

func (b *BrickData) GetLife() int  { return b.Life }
func (b *BrickData) GetLevel() int { return b.Level }

func NewCell(x, y, life int, typeOfCell utils.CellType) Cell {
	return Cell{X: x, Y: y, Data: NewBrickData(typeOfCell, life)}
}

func NewBrickData(typeOfCell utils.CellType, life int) *BrickData {
	if typeOfCell == utils.Cells.Brick && life <= 0 {
		life = 1 // Bricks must have at least 1 life initially
	}
	if typeOfCell == utils.Cells.Empty {
		life = 0 // Empty cells always have 0 life
	}
	return &BrickData{Type: typeOfCell, Life: life, Level: life} // Level usually equals initial life
}

func (cell *Cell) Compare(comparedCell Cell) bool {
	// Handle nil Data pointers
	if cell.Data == nil && comparedCell.Data == nil {
		return true // Both nil, considered equal
	}
	if cell.Data == nil || comparedCell.Data == nil {
		return false // One is nil, the other isn't
	}
	// If both are non-nil, compare the data
	return cell.Data.Compare(comparedCell.Data)
}

func (data *BrickData) Compare(comparedData *BrickData) bool {
	if data == nil && comparedData == nil {
		return true
	}
	if data == nil || comparedData == nil {
		return false
	}
	if data.Type != comparedData.Type {
		return false
	}
	if data.Life != comparedData.Life {
		return false
	}
	if data.Level != comparedData.Level {
		return false
	}
	return true
}
