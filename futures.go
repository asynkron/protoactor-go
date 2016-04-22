package gam

import "time"
import "fmt"

func FuturePID() (*PID,*Ask) {
	ref := &Ask{
		channel: make(chan interface{}),
	}
	pid := registerPID(ref)
	return pid,ref
}

type Ask struct {
	channel chan interface{}
}

func (ref *Ask) Tell(message interface{}) {
	ref.channel <- message
}

func (ref *Ask) ResultChannel() <-chan interface{} {
	return ref.channel
}

func (ref *Ask) ResultOrTimeout(timeout time.Duration) (interface{}, error) {
	select {
	case res := <-ref.channel:
		return res, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("Timeout")
	}
}

func (ref *Ask) Result() interface{} {
	return <-ref.channel
}

func (ref *Ask) SendSystemMessage(message SystemMessage) {
}

func (ref *Ask) Stop() {
	close(ref.channel)
}
