package actor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

	p1.On("SendUserMessage", a1, "hello", nilPID)
	p2.On("SendUserMessage", a2, "hello", nilPID)
	p3.On("SendUserMessage", a3, "hello", nilPID)

	f.PipeTo(a1)
	f.PipeTo(a2)
	f.PipeTo(a3)

	ref, _ := ProcessRegistry.Get(f.pid)
	assert.IsType(t, &futureProcess{}, ref)
	fp, _ := ref.(*futureProcess)

	fp.SendUserMessage(f.pid, "hello", nil)
	mock.AssertExpectationsForObjects(t, p1, p2, p3)
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

	p1.On("SendUserMessage", a1, ErrTimeout, nilPID)
	p2.On("SendUserMessage", a2, ErrTimeout, nilPID)
	p3.On("SendUserMessage", a3, ErrTimeout, nilPID)

	f := NewFuture(10 * time.Millisecond)
	ref, _ := ProcessRegistry.Get(f.pid)

	f.PipeTo(a1)
	f.PipeTo(a2)
	f.PipeTo(a3)

	err := f.Wait()
	assert.Error(t, err)

	assert.IsType(t, &futureProcess{}, ref)
	fp, _ := ref.(*futureProcess)

	mock.AssertExpectationsForObjects(t, p1, p2, p3)
	assert.Empty(t, fp.pipes, "pipes were not cleared")
}

func assertFutureSuccess(future *Future, t *testing.T) interface{} {
	res, err := future.Result()
	assert.NoError(t, err, "timed out")
	return res
}