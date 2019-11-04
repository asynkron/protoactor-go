package stream

import "github.com/otherview/protoactor-go/actor"

type UntypedStream struct {
	c   chan interface{}
	pid *actor.PID
}

func (s *UntypedStream) C() <-chan interface{} {
	return s.c
}

func (s *UntypedStream) PID() *actor.PID {
	return s.pid
}

func (s *UntypedStream) Close() {
	actor.EmptyRootContext.Stop(s.pid)
	close(s.c)
}

func NewUntypedStream() *UntypedStream {
	c := make(chan interface{})

	props := actor.PropsFromFunc(func(ctx actor.Context) {
		switch msg := ctx.Message().(type) {
		case actor.AutoReceiveMessage, actor.SystemMessage:
		// ignore terminate
		default:
			c <- msg
		}
	})
	pid := actor.EmptyRootContext.Spawn(props)

	return &UntypedStream{
		c:   c,
		pid: pid,
	}
}
