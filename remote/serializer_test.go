package remote

import (
	"testing"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

func TestJsonSerializer_round_trip(t *testing.T) {
	m := &ActorPidRequest{
		Kind: "abc",
		Name: "def",
	}
	b, typeName, _ := serialize(m, 1)
	res := deserialize(b, typeName, 1)
	typed := res.(*ActorPidRequest)

	assert.Equal(t, "remote.ActorPidRequest", typeName)
	assert.Equal(t, m, typed)
}

func TestJsonSerializer_Serialize_PID(t *testing.T) {
	m := actor.NewLocalPID("foo")
	b, typeName, _ := serialize(m, 1)
	res := deserialize(b, typeName, 1)
	typed := res.(*actor.PID)

	assert.Equal(t, "actor.PID", typeName)
	assert.Equal(t, m, typed)
}
