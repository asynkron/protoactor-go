package actor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalMessageGivesEmptyMessageHeaders(t *testing.T) {
	t.Parallel()

	props := PropsFromFunc(func(ctx Context) {
		if _, ok := ctx.Message().(string); ok {
			l := len(ctx.MessageHeader().Keys())
			ctx.Respond(l)
		}
	})
	a := rootContext.Spawn(props)

	defer func() {
		_ = rootContext.StopFuture(a).Wait()
	}()

	f := rootContext.RequestFuture(a, "hello", testTimeout)

	res, _ := assertFutureSuccess(f, t).(int)
	assert.Equal(t, 0, res)
}
