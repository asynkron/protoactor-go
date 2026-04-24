package remote

import "github.com/awevoke/protoactor-go/actor"

// Kind is the configuration for a kind
type Kind struct {
	Kind  string
	Props *actor.Props
}

// NewKind creates a new kind configuration
func NewKind(kind string, props *actor.Props) *Kind {
	return &Kind{
		Kind:  kind,
		Props: props,
	}
}
