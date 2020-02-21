package eventstream

import (
	"testing"

	cmap "github.com/orcaman/concurrent-map"
	"github.com/stretchr/testify/assert"
)

func TestEventStream_Subscribe(t *testing.T) {
	es := &EventStream{
		subscriptions: cmap.New(),
	}
	s := es.Subscribe(func(interface{}) {})
	assert.NotNil(t, s)
	assert.Equal(t, es.subscriptions.Count(), 1)
}

func TestEventStream_Unsubscribe(t *testing.T) {
	es := &EventStream{
		subscriptions: cmap.New(),
	}
	var c1, c2 int

	s1 := es.Subscribe(func(interface{}) { c1++ })
	s2 := es.Subscribe(func(interface{}) { c2++ })
	assert.Equal(t, es.subscriptions.Count(), 2)

	es.Unsubscribe(s2)
	assert.Equal(t, es.subscriptions.Count(), 1)

	es.Publish(1)
	assert.Equal(t, 1, c1)

	es.Unsubscribe(s1)
	assert.Equal(t, es.subscriptions.Count(), 0)

	es.Publish(1)
	assert.Equal(t, 1, c1)
	assert.Equal(t, 0, c2)
}

func TestEventStream_Publish(t *testing.T) {
	es := &EventStream{
		subscriptions: cmap.New(),
	}

	var v int
	es.Subscribe(func(m interface{}) { v = m.(int) })

	es.Publish(1)
	assert.Equal(t, 1, v)

	es.Publish(100)
	assert.Equal(t, 100, v)
}

func TestEventStream_Subscribe_WithPredicate_IsCalled(t *testing.T) {
	called := false
	es := &EventStream{
		subscriptions: cmap.New(),
	}
	es.Subscribe(func(interface{}) { called = true }).
		WithPredicate(func(m interface{}) bool { return true })
	es.Publish("")

	assert.True(t, called)
}

func TestEventStream_Subscribe_WithPredicate_IsNotCalled(t *testing.T) {
	called := false
	es := &EventStream{
		subscriptions: cmap.New(),
	}
	es.Subscribe(func(interface{}) { called = true }).
		WithPredicate(func(m interface{}) bool { return false })
	es.Publish("")

	assert.False(t, called)
}
