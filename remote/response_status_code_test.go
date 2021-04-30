package remote

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusCode_String(t *testing.T) {
	assert := assert.New(t)
	for i := 0; i < int(ResponseStatusCodeMAX); i++ {
		code := ResponseStatusCode(i)
		assert.NotEmpty(fmt.Sprintf("%s", code))
	}
	s := fmt.Sprintf("%s", ResponseStatusCode(100))
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
