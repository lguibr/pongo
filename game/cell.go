package game

import "github.com/lguibr/pongo/utils"

type BrickData struct {
	Type utils.CellType `json:"type"`
	Life int            `json:"life"`
}
type Cell struct {
	X    int        `json:"x"`
	Y    int        `json:"y"`
	Data *BrickData `json:"data"`
}

func NewCell(x, y, life int, typeOfCell utils.CellType) Cell {
	return Cell{X: x, Y: y, Data: NewBrickData(typeOfCell, life)}
}

func NewBrickData(typeOfCell utils.CellType, life int) *BrickData {
	if typeOfCell == utils.Cells.Brick && life == 0 {
		life = 1
	}
	if typeOfCell == utils.Cells.Empty {
		life = 0
	}
	return &BrickData{Type: typeOfCell, Life: life}
}

func (cell *Cell) Compare(comparedCell Cell) bool {
	if cell.Data.Type != comparedCell.Data.Type {
		return false
	}
	if cell.Data.Life != comparedCell.Data.Life {
		return false
	}
	return true
}

func (data *BrickData) Compare(comparedData *BrickData) bool {
	if data.Type != comparedData.Type {
		return false
	}
	if data.Life != comparedData.Life {
		return false
	}
	return true
}
