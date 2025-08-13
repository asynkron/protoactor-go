// Package etcd contains options for configuring an etcd-based cluster provider.
package etcd

import (
	clientv3 "go.etcd.io/etcd/client/v3"
)

type RoleChangedListener interface {
	OnRoleChanged(RoleType)
}

type Option func(*config)

func WithBaseKey(baseKey string) Option {
	return func(o *config) {
		o.BaseKey = baseKey
	}
}

func WithEtcdConfig(cfg clientv3.Config) Option {
	return func(o *config) {
		o.cfg = cfg
	}
}

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
