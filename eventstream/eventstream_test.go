package eventstream_test

import (
	"testing"

	"github.com/asynkron/protoactor-go/eventstream"
	"github.com/stretchr/testify/assert"
)

func TestEventStream_Subscribe(t *testing.T) {
	es := &eventstream.EventStream{}
	s := es.Subscribe(func(interface{}) {})
	assert.NotNil(t, s)
	assert.Equal(t, es.Length(), int32(1))
}

func TestEventStream_Unsubscribe(t *testing.T) {
	es := &eventstream.EventStream{}
	var c1, c2 int

	s1 := es.Subscribe(func(interface{}) { c1++ })
	s2 := es.Subscribe(func(interface{}) { c2++ })
	assert.Equal(t, es.Length(), int32(2))

	es.Unsubscribe(s2)
	assert.Equal(t, es.Length(), int32(1))

	es.Publish(1)
	assert.Equal(t, 1, c1)

	es.Unsubscribe(s1)
	assert.Equal(t, es.Length(), int32(0))

	es.Publish(1)
	assert.Equal(t, 1, c1)
	assert.Equal(t, 0, c2)
}

func TestEventStream_Publish(t *testing.T) {
	es := &eventstream.EventStream{}

	var v int
	es.Subscribe(func(m interface{}) { v = m.(int) })

	es.Publish(1)
	assert.Equal(t, 1, v)

	es.Publish(100)
	assert.Equal(t, 100, v)
}

func TestEventStream_Subscribe_WithPredicate_IsCalled(t *testing.T) {
	called := false
	es := &eventstream.EventStream{}
	es.SubscribeWithPredicate(
		func(interface{}) { called = true },
		func(m interface{}) bool { return true },
	)
	es.Publish("")

	assert.True(t, called)
}

func TestEventStream_Subscribe_WithPredicate_IsNotCalled(t *testing.T) {
	called := false
	es := &eventstream.EventStream{}
	es.SubscribeWithPredicate(
		func(interface{}) { called = true },
		func(m interface{}) bool { return false },
	)
	es.Publish("")

	assert.False(t, called)
}

type Event struct {
	i int
}

func BenchmarkEventStream(b *testing.B) {
	es := eventstream.NewEventStream()
	subs := make([]*eventstream.Subscription, 10)
	for i := 0; i < b.N; i++ {
		for j := 0; j < 10; j++ {
			sub := es.Subscribe(func(evt interface{}) {
				if e := evt.(*Event); e.i != i {
					b.Fatalf("expected i to be %d but its value is %d", i, e.i)
				}
			})
			subs[j] = sub
		}

		es.Publish(&Event{i: i})
		for j := range subs {
			es.Unsubscribe(subs[j])
			if subs[j].IsActive() {
				b.Fatal("subscription should not be active")
			}
		}
	}
}
