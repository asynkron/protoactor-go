package actor

type behaviorStack []ActorFunc

func (b *behaviorStack) Clear() {
	if len(*b) == 0 {
		return
	}

	for i := range *b {
		(*b)[i] = nil
	}
	*b = (*b)[:0]
}

func (b *behaviorStack) Peek() (v ActorFunc, ok bool) {
	l := b.Len()
	if l > 0 {
		ok = true
		v = (*b)[l-1]
	}
	return
}

func (b *behaviorStack) Push(v ActorFunc) {
	*b = append(*b, v)
}

func (b *behaviorStack) Pop() (v ActorFunc, ok bool) {
	l := b.Len()
	if l > 0 {
		l--
		ok = true
		v = (*b)[l]
		(*b)[l] = nil
		*b = (*b)[:l]
	}
	return
}

func (b *behaviorStack) Len() int {
	return len(*b)
}
