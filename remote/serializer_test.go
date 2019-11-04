package remote

import (
	"testing"

	"github.com/otherview/protoactor-go/actor"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/stretchr/testify/assert"
)

func TestJsonSerializer_round_trip(t *testing.T) {
	m := &ActorPidRequest{
		Kind: "abc",
		Name: "def",
	}
	b, typeName, _ := Serialize(m, 1)
	res, err := Deserialize(b, typeName, 1)

	assert.Nil(t, err)

	typed := res.(*ActorPidRequest)

	assert.Equal(t, "remote.ActorPidRequest", typeName)
	assert.Equal(t, m, typed)
}

func TestJsonSerializer_Serialize_PID_raw(t *testing.T) {
	m, _ := rootContext.SpawnNamed(actor.PropsFromFunc(func(ctx actor.Context) {}), "actorpid")
	ser := jsonpb.Marshaler{}
	res, _ := ser.MarshalToString(m)
	assert.Equal(t, "{\"Address\":\"nonhost\",\"Id\":\"actorpid\"}", res)
}

func TestJsonSerializer_Serialize_PID(t *testing.T) {
	m := actor.NewLocalPID("foo")
	b, typeName, _ := Serialize(m, 1)
	res, err := Deserialize(b, typeName, 1)

	assert.Nil(t, err)

	typed := res.(*actor.PID)

	assert.Equal(t, "actor.PID", typeName)
	assert.Equal(t, m, typed)
}
