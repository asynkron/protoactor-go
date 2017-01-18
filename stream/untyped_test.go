package stream

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReceiveFromStream(t *testing.T) {
	s := NewUntypedStream()
	go func() {
		s.PID().Tell("hello")
		s.PID().Tell("you")
	}()
	res := <-s.C()
	res2 := <-s.C()
	assert.Equal(t, "hello", res.(string))
	assert.Equal(t, "you", res2.(string))
}
