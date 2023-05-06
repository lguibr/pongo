package bollywood

type Address struct {
	Size    int
	IsOpen  bool
	Id      string
	Channel chan interface{}
}

func NewAddress(id string, size int) *Address {
	return &Address{
		Channel: make(chan interface{}, size),
		IsOpen:  false,
	}
}

func (a *Address) Send(msg interface{}) {
	if !a.IsOpen {
		return
	}
	a.Channel <- msg
}

func (a *Address) Receive() interface{} {
	if !a.IsOpen {
		return nil
	}
	return <-a.Channel
}

func (a *Address) Close() {
	a.IsOpen = false
}

func (a *Address) Open() {
	a.IsOpen = true
}
