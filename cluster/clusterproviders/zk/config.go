// Package zk provides ZooKeeper-based cluster provider configuration.
package zk

import (
	"time"
)

const baseKey = `/protoactor`

// Option configures the ZooKeeper provider.
type Option func(*config)

// WithAuth set zk auth
func WithAuth(scheme string, credential string) Option {
	return func(o *config) {
		o.Auth = authConfig{Scheme: scheme, Credential: credential}
	}
}

// WithBaseKey set actors base key
func WithBaseKey(key string) Option {
	return func(o *config) {
		if isStrBlank(key) {
			o.BaseKey = baseKey
		} else {
			o.BaseKey = formatBaseKey(key)
		}
	}
}

// WithSessionTimeout set zk session timeout
func WithSessionTimeout(tm time.Duration) Option {
	return func(o *config) {
		o.SessionTimeout = tm
	}
}

// WithRoleChangedListener triggered on self role changed
// WithRoleChangedListener registers a callback invoked when the node's role changes.
func WithRoleChangedListener(l RoleChangedListener) Option {
	return func(o *config) {
		o.RoleChanged = l
	}
}

// WithRoleChangedFunc registers a function to be called on role changes.
func WithRoleChangedFunc(f OnRoleChangedFunc) Option {
	return func(o *config) {
		o.RoleChanged = f
	}
}

func withEndpoints(e []string) Option {
	return func(o *config) {
		o.Endpoints = e
	}
}

type authConfig struct {
	Scheme     string
	Credential string
}

func (za authConfig) isEmpty() bool {
	return za.Scheme == "" && za.Credential == ""
}

// RoleChangedListener is notified whenever the node's leadership role changes.
type RoleChangedListener interface {
	OnRoleChanged(RoleType)
}

// OnRoleChangedFunc is an adapter to allow the use of ordinary functions as listeners.
type OnRoleChangedFunc func(RoleType)

// OnRoleChanged calls f(rt).
func (fn OnRoleChangedFunc) OnRoleChanged(rt RoleType) {
	fn(rt)
}

type config struct {
	BaseKey        string
	Endpoints      []string
	SessionTimeout time.Duration
	Auth           authConfig
	RoleChanged    RoleChangedListener
}

func defaultConfig() *config {
	return &config{
		BaseKey:        baseKey,
		SessionTimeout: time.Second * 10,
	}
}
