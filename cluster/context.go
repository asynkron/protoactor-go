package cluster

import "time"

// Context is an interface any cluster context needs to implement
type Context interface {
	Request(identity string, kind string, message interface{}, timeout ...time.Duration) (interface{}, error)
}
