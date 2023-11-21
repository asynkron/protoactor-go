package cluster

// Context is an interface any cluster context needs to implement
type Context interface {
	Request(identity string, kind string, message interface{}, opts ...GrainCallOption) (interface{}, error)
}
