package cluster

func (ci *ClusterIdentity) AsKey() string {
	return ci.Kind + "/" + ci.Identity
}

//remove
func (ci *ClusterIdentity) ToShortString() string {
	return ci.Kind + "/" + ci.Identity
}

func NewClusterIdentity(identity string, kind string) *ClusterIdentity {
	return &ClusterIdentity{
		Identity: identity,
		Kind:     kind,
	}
}
