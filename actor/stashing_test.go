package actor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type stashingActor struct {
	captured        *CapturedContext
	stashed         bool
	processed       *[]string
	self            *PID
	sender          *PID
	selfPreserved   bool
	senderPreserved bool
}

func newStashingActor(processed *[]string) *stashingActor {
	return &stashingActor{processed: processed}
}

func (a *stashingActor) Receive(ctx Context) {
	switch msg := ctx.Message().(type) {
	case string:
		if msg == "unstash" {
			if a.captured != nil {
				a.captured.Receive()
				a.captured = nil
			}
			return
		}

		if !a.stashed {
			a.self = ctx.Self()
			a.sender = ctx.Sender()
			a.captured = ctx.Capture()
			a.stashed = true
			return
		}

		if msg == "first" {
			a.selfPreserved = a.self == ctx.Self()
			a.senderPreserved = a.sender == ctx.Sender()
		}

		*a.processed = append(*a.processed, msg)
		ctx.Respond(msg)
	}
}

type applyActor struct {
	captured         *CapturedContext
	observed         *[]string
	contextCorrupted bool
}

func newApplyActor(observed *[]string) *applyActor {
	return &applyActor{observed: observed}
}

func (a *applyActor) Receive(ctx Context) {
	switch msg := ctx.Message().(type) {
	case string:
		switch msg {
		case "stash":
			a.captured = ctx.Capture()
		case "apply":
			current := ctx.Capture()
			if a.captured != nil {
				a.captured.Apply()
			}
			*a.observed = append(*a.observed, ctx.Message().(string))
			current.Apply()
			if ctx.Message().(string) != current.MessageEnvelope.Message {
				a.contextCorrupted = true
			}
			*a.observed = append(*a.observed, ctx.Message().(string))
			ctx.Respond("ok")
		}
	}
}

// Test that a captured message can be processed later and retains context information.
func TestCapturedContext_ReplayedMessageIsProcessedOnce(t *testing.T) {
	t.Parallel()

	processed := make([]string, 0, 2)
	var act *stashingActor
	props := PropsFromProducer(func() Actor {
		act = newStashingActor(&processed)
		return act
	})
	pid := rootContext.Spawn(props)
	defer rootContext.Stop(pid)

	f1 := rootContext.RequestFuture(pid, "first", time.Second)

	// trigger processing of the stashed message
	rootContext.Send(pid, "unstash")

	f2 := rootContext.RequestFuture(pid, "second", time.Second)

	r1, err1 := f1.Result()
	assert.NoError(t, err1)
	assert.Equal(t, "first", r1.(string))

	r2, err2 := f2.Result()
	assert.NoError(t, err2)
	assert.Equal(t, "second", r2.(string))

	assert.Equal(t, []string{"first", "second"}, processed)
	assert.True(t, act.selfPreserved)
	assert.True(t, act.senderPreserved)
}

// Test that Apply restores a captured message without corrupting current context.
func TestCapturedContext_ApplyRestoresMessage(t *testing.T) {
	t.Parallel()

	observed := make([]string, 0, 2)
	var act *applyActor
	props := PropsFromProducer(func() Actor {
		act = newApplyActor(&observed)
		return act
	})
	pid := rootContext.Spawn(props)
	defer rootContext.Stop(pid)

	rootContext.Send(pid, "stash")
	f := rootContext.RequestFuture(pid, "apply", time.Second)
	_, err := f.Result()
	assert.NoError(t, err)

	assert.Equal(t, []string{"stash", "apply"}, observed)
	assert.False(t, act.contextCorrupted)
}
