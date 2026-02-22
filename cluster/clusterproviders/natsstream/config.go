package natsstream

import (
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

const (
	defaultHeartbeatInterval = 2 * time.Second
	defaultHeartbeatTTL      = 5 * time.Second
	defaultMemberTimeout     = 8 * time.Second
	defaultCheckInterval     = 3 * time.Second
	defaultLeaderTTL         = 10 * time.Second
	defaultLockTTL           = 5 * time.Second
	defaultMaxConcurrency    = 200
	defaultReplicas          = 1
	defaultSubjectPrefix     = "cluster"
	defaultRetryInterval     = 1 * time.Second
	defaultMaxAge            = 1 * time.Hour
)

// RoleType represents the leadership role of a node in the cluster.
type RoleType int

const (
	Follower RoleType = iota
	Leader
)

func (r RoleType) String() string {
	switch r {
	case Leader:
		return "Leader"
	default:
		return "Follower"
	}
}

// RoleChangedListener receives notifications when the node's leadership role changes.
type RoleChangedListener interface {
	OnRoleChanged(RoleType)
}

type config struct {
	StreamName         string
	IdentityStreamName string
	SubjectPrefix      string
	Replicas           int
	Storage            jetstream.StorageType
	MaxAge             time.Duration
	HeartbeatInterval  time.Duration
	HeartbeatTTL       time.Duration
	MemberTimeout      time.Duration
	CheckInterval      time.Duration
	LeaderTTL          time.Duration
	LockTTL            time.Duration
	MaxConcurrency     int
	RetryInterval      time.Duration
	RoleChanged        RoleChangedListener
}

type Option func(*config)

func WithStreamName(name string) Option         { return func(c *config) { c.StreamName = name } }
func WithIdentityStreamName(name string) Option { return func(c *config) { c.IdentityStreamName = name } }
func WithSubjectPrefix(prefix string) Option    { return func(c *config) { c.SubjectPrefix = prefix } }
func WithReplicas(n int) Option                 { return func(c *config) { c.Replicas = n } }
func WithStorage(s jetstream.StorageType) Option {
	return func(c *config) { c.Storage = s }
}
func WithMaxAge(d time.Duration) Option            { return func(c *config) { c.MaxAge = d } }
func WithHeartbeatInterval(d time.Duration) Option { return func(c *config) { c.HeartbeatInterval = d } }
func WithHeartbeatTTL(d time.Duration) Option      { return func(c *config) { c.HeartbeatTTL = d } }
func WithMemberTimeout(d time.Duration) Option     { return func(c *config) { c.MemberTimeout = d } }
func WithCheckInterval(d time.Duration) Option     { return func(c *config) { c.CheckInterval = d } }
func WithLeaderTTL(d time.Duration) Option         { return func(c *config) { c.LeaderTTL = d } }
func WithLockTTL(d time.Duration) Option           { return func(c *config) { c.LockTTL = d } }
func WithMaxConcurrency(n int) Option              { return func(c *config) { c.MaxConcurrency = n } }
func WithRetryInterval(d time.Duration) Option     { return func(c *config) { c.RetryInterval = d } }
func WithRoleChangedListener(l RoleChangedListener) Option {
	return func(c *config) { c.RoleChanged = l }
}

func newDefaultConfig() *config {
	return &config{
		Replicas:          defaultReplicas,
		Storage:           jetstream.FileStorage,
		MaxAge:            defaultMaxAge,
		HeartbeatInterval: defaultHeartbeatInterval,
		HeartbeatTTL:      defaultHeartbeatTTL,
		MemberTimeout:     defaultMemberTimeout,
		CheckInterval:     defaultCheckInterval,
		LeaderTTL:         defaultLeaderTTL,
		LockTTL:           defaultLockTTL,
		MaxConcurrency:    defaultMaxConcurrency,
		SubjectPrefix:     defaultSubjectPrefix,
		RetryInterval:     defaultRetryInterval,
	}
}

func (c *config) streamName(clusterName string) string {
	if c.StreamName != "" {
		return c.StreamName
	}
	return "PROTOACTOR_" + clusterName
}

func (c *config) identityStreamName(clusterName string) string {
	if c.IdentityStreamName != "" {
		return c.IdentityStreamName
	}
	return "PROTOACTOR_" + clusterName + "_IDENTITIES"
}

func (c *config) subjectPrefix(clusterName string) string {
	if c.SubjectPrefix != defaultSubjectPrefix {
		return c.SubjectPrefix
	}
	return "cluster." + clusterName
}
