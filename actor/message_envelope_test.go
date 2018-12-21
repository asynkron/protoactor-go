package actor

import "testing"
import "github.com/stretchr/testify/assert"

func TestNormalMessageGivesEmptyMessageHeaders(t *testing.T) {
	props := PropsFromFunc(func(ctx Context) {
		switch ctx.Message().(type) {
		case string:
			l := len(ctx.MessageHeader().Keys())
			ctx.Respond(l)
		}
	})
	a, _ := EmptyRootContext.Spawn(props)
	defer a.GracefulStop()
	f := EmptyRootContext.RequestFuture(a, "hello", testTimeout)
	res := assertFutureSuccess(f, t).(int)
	assert.Equal(t, 0, res)
}
