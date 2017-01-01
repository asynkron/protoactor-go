package messages

func (m *Ping) HashBy() string {
	return m.User
}
