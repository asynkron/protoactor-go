package cluster

type ClusterContext interface {
	Request(identity string, kind string, message interface{}) (interface{}, error)
}
