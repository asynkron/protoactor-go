package messages

func (m *Ping) Hash() string {
	return m.User
}
