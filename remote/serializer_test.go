package remote

import (
	"testing"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

//func TestJsonSerializer_round_trip(t *testing.T) {
//	m := &ActorPidRequest{
//		Kind: "abc",
//		Name: "def",
//	}
//	b, typeName, _ := Serialize(m, 1)
//	res, err := Deserialize(b, typeName, 1)
//
//	assert.Nil(t, err)
//
//	var typed = res.(*ActorPidRequest)
//	assert.Equal(t, "remote.ActorPidRequest", typeName)
//	assert.Equal(t, m, typed)
//}
//
//func TestJsonSerializer_Serialize_PID_raw(t *testing.T) {
//	system := actor.NewActorSystem()
//	m, _ := system.Root.SpawnNamed(actor.PropsFromFunc(func(ctx actor.Context) {}), "actorpid")
//	var ser = jsonpb.Marshaler{}
//	res, _ := ser.MarshalToString(m)
//	assert.Equal(t, "{\"Address\":\"nonhost\",\"Id\":\"actorpid\"}", res)
//}
//
//func TestJsonSerializer_Serialize_PID(t *testing.T) {
//	system := actor.NewActorSystem()
//	m := system.NewLocalPID("foo")
//	b, typeName, _ := Serialize(m, 1)
//	res, err := Deserialize(b, typeName, 1)
//
//	assert.Nil(t, err)
//
//	var typed = res.(*actor.PID)
//	assert.Equal(t, "actor.PID", typeName)
//	assert.Equal(t, m, typed)
//}

func TestProtobufSerializer_Serialize_PID(t *testing.T) {
	system := actor.NewActorSystem()
	m := system.NewLocalPID("foo")
	b, typeName, _ := Serialize(m, 0)
	res, err := Deserialize(b, typeName, 0)

	assert.Nil(t, err)

	typed := res.(*actor.PID)
	assert.Equal(t, "actor.PID", typeName)
	assert.True(t, m.Equal(typed))
}
