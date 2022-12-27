package game

type Paddle struct {
	Starts    int     `json:"starts"`
	Ends      int     `json:"ends"`
	Direction string  `json:"direction"`
	Velocity  int     `json:"velocity"`
	OwnerId   string  `json:"ownerId"`
	Canvas    *Canvas `json:"canvas"`
}

func (p *Paddle) Move() {
	if p.Direction == "left" && p.Starts-p.Velocity >= 0 {
		p.Starts -= p.Velocity
		p.Ends -= p.Velocity
	} else if p.Direction == "right" && p.Ends+p.Velocity <= p.Canvas.Width {
		p.Starts += p.Velocity
		p.Ends += p.Velocity
	}
}

func CreatePaddle(ownerId string, canvas *Canvas) *Paddle {

	starts := canvas.Width/2 - 50
	ends := canvas.Width/2 + 50

	return &Paddle{
		Starts:    starts,
		Ends:      ends,
		Direction: "",
		Velocity:  10,
		OwnerId:   ownerId,
		Canvas:    canvas,
	}
}
