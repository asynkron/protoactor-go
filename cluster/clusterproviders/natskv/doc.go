// Package natskv provides a NATS JetStream KV-based cluster provider
// for Proto.Actor. It uses KeyValue buckets for member registration,
// health checking via key TTL, leader election via atomic Create, and
// an integrated identity lookup for virtual actor activation.
package natskv
