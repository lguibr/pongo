package bollywood

type Actor struct {
	Mailbox     *Mailbox
	State       interface{}
	Performance func()
	Alive       bool
}

func NewActor(mailboxConfig []MailboxConfig, State interface{}, performance func()) *Actor {
	return &Actor{
		Mailbox:     NewMailbox(mailboxConfig),
		Performance: performance,
		Alive:       true,
	}
}

func (a *Actor) Perform() {
	go func() {
		for a.Alive {
			a.Performance()
		}
	}()
}

func (a *Actor) Die() {
	a.Mailbox.Deactivate()
	a.Alive = false
}
