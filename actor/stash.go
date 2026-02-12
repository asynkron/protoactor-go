package actor

// messageStack is a simple slice-based LIFO stack used for stashing messages.
type messageStack struct {
	items []interface{}
}

func newMessageStack() *messageStack {
	return &messageStack{}
}

func (s *messageStack) Push(item interface{}) {
	s.items = append(s.items, item)
}

func (s *messageStack) Pop() (interface{}, bool) {
	if len(s.items) == 0 {
		return nil, false
	}
	item := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return item, true
}

func (s *messageStack) Empty() bool {
	return len(s.items) == 0
}
