package cluster

import (
	"fmt"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/ctxext"
)

// ValidateIdentity checks that a grain Identity string is acceptable. An
// empty identity produces an unparseable key (the "kind/" form has no value
// after the kind|identity boundary), and there is no sensible default actor
// for an unspecified identity, so empty identities are rejected.
//
// Identities may contain '/' (kinds may not — see ValidateKindName) so a
// single forbidden character is empty-string only. Other ill-formed
// identities still surface as ErrInvalidKey from the storage backend.
func ValidateIdentity(identity string) error {
	if identity == "" {
		return fmt.Errorf("identity must not be empty")
	}
	return nil
}

// AsKey formats the identity as "kind/identity".
func (ci *ClusterIdentity) AsKey() string {
	return ci.Kind + "/" + ci.Identity
}

var ciExtensionId = ctxext.NextContextExtensionID()

// ToShortString returns a compact string representation of the identity.
func (ci *ClusterIdentity) ToShortString() string {
	return ci.Kind + "/" + ci.Identity
}

// NewClusterIdentity constructs a new ClusterIdentity value.
func NewClusterIdentity(identity string, kind string) *ClusterIdentity {
	return &ClusterIdentity{
		Identity: identity,
		Kind:     kind,
	}
}

// ExtensionID implements ctxext.Extension and returns the extension identifier.
func (ci *ClusterIdentity) ExtensionID() ctxext.ContextExtensionID {
	return ciExtensionId
}

// GetClusterIdentity retrieves the ClusterIdentity from the context.
func GetClusterIdentity(ctx actor.ExtensionContext) *ClusterIdentity {
	if ext := ctx.Get(ciExtensionId); ext != nil {
		if ci, ok := ext.(*ClusterIdentity); ok {
			return ci
		}
	}
	return nil
}

// SetClusterIdentity stores the ClusterIdentity on the context.
func SetClusterIdentity(ctx actor.ExtensionContext, ci *ClusterIdentity) {
	ctx.Set(ci)
}
