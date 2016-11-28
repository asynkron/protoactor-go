package streams

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReceiveFromStream(t *testing.T) {
	s := NewUntypedStream()
	go s.PID().Tell("hello")
	res := <-s.C()
	assert.Equal(t, "hello", res.(string))
}
