package cluster

func (ci *ClusterIdentity) AsKey() string {
	return ci.Kind + "/" + ci.Identity
}
