// Package etcd contains options for configuring an etcd-based cluster provider.
package etcd

import (
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	defaultKeepAliveTTL  = 3 * time.Second
	defaultRetryInterval = 1 * time.Second
)

// RoleChangedListener receives notifications when the node role changes.
type RoleChangedListener interface {
	OnRoleChanged(RoleType)
}

// Option configures the etcd provider.
type Option func(*config)

// WithBaseKey sets the base key used for all etcd entries.
func WithBaseKey(baseKey string) Option {
	return func(o *config) {
		o.BaseKey = baseKey
	}
}

// WithEtcdConfig sets a custom etcd client configuration.
func WithEtcdConfig(cfg clientv3.Config) Option {
	return func(o *config) {
		o.cfg = cfg
	}
}

// WithRoleChangedListener sets a callback for role changes.
func WithRoleChangedListener(l RoleChangedListener) Option {
	return func(o *config) {
		o.RoleChanged = l
	}
}

// WithKeepAliveTTL sets the grant TTL for node leases.
func WithKeepAliveTTL(ttl time.Duration) Option {
	return func(o *config) {
		o.KeepAliveTTL = ttl
	}
}

// WithRetryInterval sets the interval between retries for etcd operations.
func WithRetryInterval(interval time.Duration) Option {
	return func(o *config) {
		o.RetryInterval = interval
	}
}

// WithEtcdClient sets a custom etcd client instance.
func WithEtcdClient(client *clientv3.Client) Option {
	return func(o *config) {
		o.client = client
	}
}

type config struct {
	BaseKey       string
	cfg           clientv3.Config
	RoleChanged   RoleChangedListener
	KeepAliveTTL  time.Duration
	RetryInterval time.Duration
	client        *clientv3.Client
}

func defaultConfig() *config {
	return &config{
		KeepAliveTTL:  defaultKeepAliveTTL,
		RetryInterval: defaultRetryInterval,
	}
}
