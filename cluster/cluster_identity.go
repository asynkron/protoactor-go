package cluster

import (
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/ctxext"
)

func (ci *ClusterIdentity) AsKey() string {
	return ci.Kind + "/" + ci.Identity
}

var ciExtensionId = ctxext.NextContextExtensionID()

// remove
func (ci *ClusterIdentity) ToShortString() string {
	return ci.Kind + "/" + ci.Identity
}

func NewClusterIdentity(identity string, kind string) *ClusterIdentity {
	return &ClusterIdentity{
		Identity: identity,
		Kind:     kind,
	}
}

func (ci *ClusterIdentity) ExtensionID() ctxext.ContextExtensionID {
	return ciExtensionId
}

func GetClusterIdentity(ctx actor.ExtensionContext) *ClusterIdentity {
	return ctx.Get(ciExtensionId).(*ClusterIdentity)
}

func SetClusterIdentity(ctx actor.ExtensionContext, ci *ClusterIdentity) {
	ctx.Set(ci)
}
