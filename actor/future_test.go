package actor

import (
	"testing"
	"time"

	"github.com/otherview/protoactor-go/log"
	"github.com/stretchr/testify/assert"
)

func TestFuture_PipeTo_Message(t *testing.T) {
	a1, p1 := spawnMockProcess("a1")
	a2, p2 := spawnMockProcess("a2")
	a3, p3 := spawnMockProcess("a3")
	defer func() {
		removeMockProcess(a1)
		removeMockProcess(a2)
		removeMockProcess(a3)
	}()

	f := NewFuture(1 * time.Second)

	p1.On("SendUserMessage", a1, "hello")
	p2.On("SendUserMessage", a2, "hello")
	p3.On("SendUserMessage", a3, "hello")

	f.PipeTo(a1)
	f.PipeTo(a2)
	f.PipeTo(a3)

	ref, _ := ProcessRegistry.Get(f.pid)
	assert.IsType(t, &futureProcess{}, ref)
	fp, _ := ref.(*futureProcess)

	fp.SendUserMessage(f.pid, "hello")
	p1.AssertExpectations(t)
	p2.AssertExpectations(t)
	p3.AssertExpectations(t)
	assert.Empty(t, fp.pipes, "pipes were not cleared")
}

func TestFuture_PipeTo_TimeoutSendsError(t *testing.T) {
	a1, p1 := spawnMockProcess("a1")
	a2, p2 := spawnMockProcess("a2")
	a3, p3 := spawnMockProcess("a3")
	defer func() {
		removeMockProcess(a1)
		removeMockProcess(a2)
		removeMockProcess(a3)
	}()

	p1.On("SendUserMessage", a1, ErrTimeout)
	p2.On("SendUserMessage", a2, ErrTimeout)
	p3.On("SendUserMessage", a3, ErrTimeout)

	f := NewFuture(10 * time.Millisecond)
	ref, _ := ProcessRegistry.Get(f.pid)

	f.PipeTo(a1)
	f.PipeTo(a2)
	f.PipeTo(a3)

	err := f.Wait()
	assert.Error(t, err)

	assert.IsType(t, &futureProcess{}, ref)
	fp, _ := ref.(*futureProcess)

	p1.AssertExpectations(t)
	p2.AssertExpectations(t)
	p3.AssertExpectations(t)
	assert.Empty(t, fp.pipes, "pipes were not cleared")
}

func TestNewFuture_TimeoutNoRace(t *testing.T) {
	plog.SetLevel(log.OffLevel)
	future := NewFuture(1 * time.Microsecond)
	a := rootContext.Spawn(PropsFromFunc(func(context Context) {
		switch context.Message().(type) {
		case *Started:
			context.Send(future.PID(), EchoResponse{})
		}
	}))
	_ = rootContext.StopFuture(a).Wait()
	_, _ = future.Result()
}

func assertFutureSuccess(future *Future, t *testing.T) interface{} {
	res, err := future.Result()
	assert.NoError(t, err, "timed out")
	return res
}
