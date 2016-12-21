package actor

type receiveFn func(Context)

func (fn receiveFn) Receive(ctx Context) {
	fn(ctx)
}

var nullReceive receiveFn = func(Context) {}

