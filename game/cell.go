package game

type BrickData struct {
	Type string `json:"type"`
	Life int    `json:"life"`
}
type Cell struct {
	X    int        `json:"x"`
	Y    int        `json:"y"`
	Data *BrickData `json:"data"`
}

func NewCell(x, y, life int, typeOfCell string) Cell {
	return Cell{X: x, Y: y, Data: NewBrickData(typeOfCell, life)}
}

func NewBrickData(typeOfCell string, life int) *BrickData {
	if typeOfCell == "Brick" && life == 0 {
		life = 1
	}
	if typeOfCell == "Empty" {
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
