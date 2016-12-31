package streams

import "github.com/AsynkronIT/gam/actor"

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
	s.pid.Stop()
	close(s.c)
}

func NewUntypedStream() *UntypedStream {
	c := make(chan interface{})
	props := actor.FromFunc(func(ctx actor.Context) {
		switch msg := ctx.Message().(type) {
		case actor.AutoReceiveMessage:
		case actor.SystemMessage: //ignore terminate
		default:
			c <- msg
		}
	})
	pid := actor.Spawn(props)

	stream := &UntypedStream{
		c:   c,
		pid: pid,
	}
	return stream
}
