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
