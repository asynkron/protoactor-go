package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogger_With(t *testing.T) {
	base := New(DebugLevel, "", Field{key: "first"})
	l := base.With(Field{key: "second"})

	assert.Equal(t, []Field{{key: "first"}, {key: "second"}}, l.context)
}

func Benchmark_OffLevel_TwoFields(b *testing.B) {
	l := New(MinLevel, "")
	for i := 0; i < b.N; i++ {
		l.Debug("foo", Int("bar", 32), Bool("fum", false))
	}
}

func Benchmark_OffLevel_OnlyContext(b *testing.B) {
	l := New(MinLevel, "", Int("bar", 32), Bool("fum", false))
	for i := 0; i < b.N; i++ {
		l.Debug("foo")
	}
}

func Benchmark_DebugLevel_OnlyContext_OneSubscriber(b *testing.B) {
	Unsubscribe(sub)
	s1 := Subscribe(func(Event) {})

	l := New(DebugLevel, "", Int("bar", 32), Bool("fum", false))
	for i := 0; i < b.N; i++ {
		l.Debug("foo")
	}
	Unsubscribe(s1)
}

func Benchmark_DebugLevel_OnlyContext_MultipleSubscribers(b *testing.B) {
	Unsubscribe(sub)
	s1 := Subscribe(func(Event) {})
	s2 := Subscribe(func(Event) {})

	l := New(DebugLevel, "", Int("bar", 32), Bool("fum", false))
	for i := 0; i < b.N; i++ {
		l.Debug("foo")
	}

	Unsubscribe(s1)
	Unsubscribe(s2)
}
