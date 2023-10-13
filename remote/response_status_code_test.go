package remote

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusCode_String(t *testing.T) {
	assert := assert.New(t)
	for i := 0; i < int(ResponseStatusCodeMAX); i++ {
		code := ResponseStatusCode(i)
		assert.NotEmpty(code.String())
	}
	s := ResponseStatusCode(100).String()
	assert.Equal(s, "ResponseStatusCode-100")
}

func TestStatusCode_Error(t *testing.T) {
	assert := assert.New(t)
	for i := 0; i < int(ResponseStatusCodeMAX); i++ {
		var err error = nil
		err = &ResponseError{ResponseStatusCode(i)}
		assert.Error(err)
	}
}
