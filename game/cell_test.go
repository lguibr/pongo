package game

import (
	"testing"

	"github.com/lguibr/pongo/utils"
)

func TestNewBrickData(t *testing.T) {
	type NewBrickDataTestCase struct {
		typeOfCell utils.CellType
		life       int
		expected   *BrickData
	}

	testCases := []NewBrickDataTestCase{
		{typeOfCell: utils.Cells.Brick, life: 0, expected: &BrickData{Type: utils.Cells.Brick, Life: 1}},
		{typeOfCell: utils.Cells.Brick, life: 2, expected: &BrickData{Type: utils.Cells.Brick, Life: 2}},
		{typeOfCell: utils.Cells.Empty, life: 0, expected: &BrickData{Type: utils.Cells.Empty, Life: 0}},
		{typeOfCell: utils.Cells.Empty, life: 2, expected: &BrickData{Type: utils.Cells.Empty, Life: 0}},
	}

	for _, test := range testCases {
		result := NewBrickData(test.typeOfCell, test.life)
		if !result.Compare(test.expected) {
			t.Errorf("Expected %v for typeOfCell %s and life %d, got %v", test.expected, test.typeOfCell, test.life, result)
		}
	}
}

func TestBrickData_Compare(t *testing.T) {
	type CompareBrickDataTestCase struct {
		data         *BrickData
		comparedData *BrickData
		expected     bool
	}

	testCases := []CompareBrickDataTestCase{
		{&BrickData{Type: utils.Cells.Brick, Life: 1}, &BrickData{Type: utils.Cells.Brick, Life: 1}, true},
		{&BrickData{Type: utils.Cells.Empty, Life: 0}, &BrickData{Type: utils.Cells.Empty, Life: 0}, true},
		{&BrickData{Type: utils.Cells.Brick, Life: 2}, &BrickData{Type: utils.Cells.Brick, Life: 1}, false},
		{&BrickData{Type: utils.Cells.Empty, Life: 0}, &BrickData{Type: utils.Cells.Brick, Life: 0}, false},
	}

	for _, test := range testCases {
		result := test.data.Compare(test.comparedData)
		if result != test.expected {
			t.Errorf("Expected CompareBrickData(%v, %v) to return %v, got %v", test.data, test.comparedData, test.expected, result)
		}
	}
}

func TestCell_Compare(t *testing.T) {
	type CompareCellsTestCase struct {
		cell         Cell
		comparedCell Cell
		expected     bool
	}

	testCases := []CompareCellsTestCase{
		{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}, Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}, true},
		{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}, Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Empty, Life: 1}}, false},
		{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}, Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 2}}, false},
		{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Empty, Life: 0}}, Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Empty, Life: 0}}, true},
	}

	for _, test := range testCases {
		result := test.cell.Compare(test.comparedCell)
		if result != test.expected {
			t.Errorf("Expected CompareCells(%v, %v) to return %v, got %v", test.cell, test.comparedCell, test.expected, result)
		}
	}
}

func TestNewCell(t *testing.T) {
	type NewCellTestCase struct {
		x          int
		y          int
		life       int
		typeOfCell utils.CellType
		expected   Cell
	}

	testCases := []NewCellTestCase{
		{0, 0, 0, utils.Cells.Brick, Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}},
		{1, 2, 3, utils.Cells.Empty, Cell{X: 1, Y: 2, Data: &BrickData{Type: utils.Cells.Empty, Life: 0}}},
		{4, 5, 2, utils.Cells.Brick, Cell{X: 4, Y: 5, Data: &BrickData{Type: utils.Cells.Brick, Life: 2}}},
	}

	for _, test := range testCases {
		result := NewCell(test.x, test.y, test.life, test.typeOfCell)
		if !result.Compare(test.expected) {
			t.Errorf("Expected %v, got %v", test.expected, result)
		}
	}
}
