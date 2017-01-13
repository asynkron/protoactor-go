package actor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestActorCell_SpawnNamed(t *testing.T) {
	pid, p := spawnMockProcess("foo/bar")
	defer removeMockProcess(pid)
	p.On("SendSystemMessage", pid, mock.Anything)

	props := Props{
		spawner: func(id string, _ Props, _ *PID) *PID {
			assert.Equal(t, "foo/bar", id)
			return NewLocalPID(id)
		},
	}

	parent := &actorCell{self: NewLocalPID("foo")}
	parent.SpawnNamed(props, "bar")
	mock.AssertExpectationsForObjects(t, p)
}

func BenchmarkActorCell_Next(b *testing.B) {
	ac := &actorCell{actor: nullReceive}
	ac.Become(nullReceive.Receive)
	for i := 0; i < b.N; i++ {
		ac.Next()
	}
}
