package game

import (
	"encoding/json"
	"fmt"
)

type Canvas struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

func CreateCanvas(size int) *Canvas {
	return &Canvas{
		Width:  size,
		Height: size,
	}
}

func (c *Canvas) ToJson() []byte {
	canvas, err := json.Marshal(c)
	if err != nil {
		fmt.Println(err)
		return []byte{}
	}
	return canvas
}
