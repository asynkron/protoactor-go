package cluster

import (
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"

	"github.com/AsynkronIT/protoactor-go/remote"
)

type Config struct {
	Name                  string
	Address               string
	ClusterProvider       ClusterProvider
	Identitylookup        IdentityLookup
	RemoteConfig          remote.Config
	RequestTimeoutTime    time.Duration
	MemberStrategyBuilder func(kind string) MemberStrategy
	Kinds                 map[string]*actor.Props
}

func Configure(clusterName string, clusterProvider ClusterProvider, identityLookup IdentityLookup, remoteConfig remote.Config, kinds ...*Kind) *Config {
	config := &Config{
		Name:                  clusterName,
		ClusterProvider:       clusterProvider,
		Identitylookup:        identityLookup,
		RequestTimeoutTime:    time.Second * 5,
		MemberStrategyBuilder: newDefaultMemberStrategy,
		RemoteConfig:          remoteConfig,
		Kinds:                 make(map[string]*actor.Props),
	}

	for _, kind := range kinds {
		config.Kinds[kind.Kind] = kind.Props
	}

	return config
}

func (c *Config) WithRequestTimeout(t time.Duration) *Config {
	c.RequestTimeoutTime = t
	return c
}

type Kind struct {
	Kind  string
	Props *actor.Props
}

func NewKind(kind string, props *actor.Props) *Kind {
	return &Kind{
		Kind:  kind,
		Props: props,
	}
}
