package actor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestLocalContext_SpawnNamed(t *testing.T) {
	pid, p := spawnMockProcess("foo/bar")
	defer removeMockProcess(pid)
	p.On("SendSystemMessage", matchPID(pid), mock.Anything)

	props := Props{
		spawner: func(id string, _ Props, _ *PID) *PID {
			assert.Equal(t, "foo/bar", id)
			return NewLocalPID(id)
		},
	}

	parent := &localContext{self: NewLocalPID("foo")}
	parent.SpawnNamed(props, "bar")
	mock.AssertExpectationsForObjects(t, p)
}

func BenchmarkLocalContext_Next(b *testing.B) {
	ctx := &localContext{actor: nullReceive}
	ctx.Become(nullReceive.Receive)
	for i := 0; i < b.N; i++ {
		ctx.Next()
	}
}
