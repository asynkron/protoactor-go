package gam

import "time"
import "fmt"

func FuturePID() (*PID, *FutureResult) {
	ref := &FutureResult{
		channel: make(chan interface{}),
	}
	pid := registerPID(ref)
	return pid, ref
}

type FutureResult struct {
	channel chan interface{}
}

func (ref *FutureResult) Tell(message interface{}) {
	ref.channel <- message
}

func (ref *FutureResult) ResultChannel() <-chan interface{} {
	return ref.channel
}

func (ref *FutureResult) ResultOrTimeout(timeout time.Duration) (interface{}, error) {
	select {
	case res := <-ref.channel:
		return res, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("Timeout")
	}
}

func (ref *FutureResult) Result() interface{} {
	return <-ref.channel
}

func (ref *FutureResult) SendSystemMessage(message SystemMessage) {
}

func (ref *FutureResult) Stop() {
	close(ref.channel)
}
