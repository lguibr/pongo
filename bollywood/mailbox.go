package bollywood

type Mailbox struct {
	Addressees map[string]*Address
	Activated  bool
}

type MailboxConfig struct {
	Size int
	Id   string
}

func NewAddresses(config []MailboxConfig) map[string]*Address {
	Addressees := make(map[string]*Address)
	for _, c := range config {
		Addressees[c.Id] = NewAddress(c.Id, c.Size)
	}
	return Addressees
}

func NewMailbox(config []MailboxConfig) *Mailbox {
	return &Mailbox{
		Addressees: NewAddresses(config),
	}
}

func (mailbox *Mailbox) Send(id string, msg interface{}) {
	if !mailbox.Activated {
		return
	}
	mailbox.Addressees[id].Send(msg)
}

func (m *Mailbox) Receive(id string) interface{} {
	if !m.Activated {
		return nil
	}
	return m.Addressees[id].Receive()
}

func (m *Mailbox) Close(id string) { m.Addressees[id].Close() }

func (m *Mailbox) Open(id string) { m.Addressees[id].Open() }

func (m *Mailbox) OpenAll() {
	for _, a := range m.Addressees {
		a.Open()
	}
}

func (m *Mailbox) CloseAll() {
	for _, a := range m.Addressees {
		a.Close()
	}
}

func (m *Mailbox) Get(id string) *Address {
	return m.Addressees[id]
}

func (m *Mailbox) Deactivate() {
	m.CloseAll()
	m.Activated = false
}
