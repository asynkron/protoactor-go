// Package etcd contains options for configuring an etcd-based cluster provider.
package etcd

import (
	clientv3 "go.etcd.io/etcd/client/v3"
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

type config struct {
	BaseKey     string
	cfg         clientv3.Config
	RoleChanged RoleChangedListener
}

func defaultConfig() *config {
	return &config{}
}
