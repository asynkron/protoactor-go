package cluster

import (
	"time"

	"github.com/otherview/protoactor-go/remote"
)

type ClusterConfig struct {
	Name                        string
	Address                     string
	ClusterProvider             ClusterProvider
	RemotingOption              []remote.RemotingOption
	TimeoutTime                 time.Duration
	InitialMemberStatusValue    MemberStatusValue
	MemberStatusValueSerializer MemberStatusValueSerializer
	MemberStrategyBuilder       func(kind string) MemberStrategy
}

func NewClusterConfig(name string, address string, clusterProvider ClusterProvider) *ClusterConfig {
	return &ClusterConfig{
		Name:                        name,
		Address:                     address,
		ClusterProvider:             clusterProvider,
		TimeoutTime:                 time.Second * 5,
		InitialMemberStatusValue:    nil,
		MemberStatusValueSerializer: &NilMemberStatusValueSerializer{},
		MemberStrategyBuilder:       newDefaultMemberStrategy,
	}
}

func (c *ClusterConfig) WithRemotingOption(remotingOption []remote.RemotingOption) *ClusterConfig {
	c.RemotingOption = remotingOption
	return c
}

func (c *ClusterConfig) WithTimeout(t time.Duration) *ClusterConfig {
	c.TimeoutTime = t
	return c
}

func (c *ClusterConfig) WithInitialMemberStatusValue(val MemberStatusValue) *ClusterConfig {
	c.InitialMemberStatusValue = val
	return c
}

func (c *ClusterConfig) WithMemberStatusValueSerializer(serializer MemberStatusValueSerializer) *ClusterConfig {
	c.MemberStatusValueSerializer = serializer
	return c
}

func (c *ClusterConfig) WithMemberStrategyBuilder(builder func(kind string) MemberStrategy) *ClusterConfig {
	c.MemberStrategyBuilder = builder
	return c
}
