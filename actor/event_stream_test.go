package actor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEventStream_Subscribe(t *testing.T) {
	es := &eventStream{}
	s := es.Subscribe(func(interface{}) {})
	assert.NotNil(t, s)
	assert.Len(t, es.subscriptions, 1)
}

func TestEventStream_Unsubscribe(t *testing.T) {
	es := &eventStream{}
	var c1, c2 int

	s1 := es.Subscribe(func(interface{}) { c1++ })
	s2 := es.Subscribe(func(interface{}) { c2++ })
	assert.Len(t, es.subscriptions, 2)

	es.Unsubscribe(s2)
	assert.Len(t, es.subscriptions, 1)

	es.Publish(1)
	assert.Equal(t, 1, c1)

	es.Unsubscribe(s1)
	assert.Empty(t, es.subscriptions)

	es.Publish(1)
	assert.Equal(t, 1, c1)
	assert.Equal(t, 0, c2)
}

func TestEventStream_Publish(t *testing.T) {
	es := &eventStream{}

	var v int
	es.Subscribe(func(m interface{}) { v = m.(int) })

	es.Publish(1)
	assert.Equal(t, 1, v)

	es.Publish(100)
	assert.Equal(t, 100, v)
}
