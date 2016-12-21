package actor

import (
	"io/ioutil"
	"log"
)

type receiveFn func(Context)

func (fn receiveFn) Receive(ctx Context) {
	fn(ctx)
}

var nullReceive receiveFn = func(Context) {}

func init() {
	// discard all logging in tests
	log.SetOutput(ioutil.Discard)
}
