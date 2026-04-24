package cluster

import "github.com/awevoke/protoactor-go/actor"

// Context is an interface any cluster context needs to implement
type Context interface {
	Request(identity string, kind string, message any, opts ...GrainCallOption) (any, error)
	RequestFuture(identity string, kind string, message any, opts ...GrainCallOption) (actor.Future, error)
}
