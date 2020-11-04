package cluster

import (
	"time"

	"github.com/AsynkronIT/protoactor-go/remote"
)

type Config struct {
	Name                        string
	Address                     string
	ClusterProvider             ClusterProvider
	RemoteConfig                remote.Config
	TimeoutTime                 time.Duration
	InitialMemberStatusValue    MemberStatusValue
	MemberStatusValueSerializer MemberStatusValueSerializer
	MemberStrategyBuilder       func(kind string) MemberStrategy
}

func NewClusterConfig(name string, address string, clusterProvider ClusterProvider) *Config {
	return &Config{
		Name:                        name,
		Address:                     address,
		ClusterProvider:             clusterProvider,
		TimeoutTime:                 time.Second * 5,
		InitialMemberStatusValue:    nil,
		MemberStatusValueSerializer: &NilMemberStatusValueSerializer{},
		MemberStrategyBuilder:       newDefaultMemberStrategy,
	}
}

func (c *Config) WithRemoteConfig(config remote.Config) *Config {
	c.RemoteConfig = config
	return c
}

func (c *Config) WithTimeout(t time.Duration) *Config {
	c.TimeoutTime = t
	return c
}

func (c *Config) WithInitialMemberStatusValue(val MemberStatusValue) *Config {
	c.InitialMemberStatusValue = val
	return c
}

func (c *Config) WithMemberStatusValueSerializer(serializer MemberStatusValueSerializer) *Config {
	c.MemberStatusValueSerializer = serializer
	return c
}

func (c *Config) WithMemberStrategyBuilder(builder func(kind string) MemberStrategy) *Config {
	c.MemberStrategyBuilder = builder
	return c
}
