package cluster

import "golang.org/x/net/context"

// KeyValueStore is a distributed key value store
type KeyValueStore[T any] interface {
	// Set the value for the given key.
	Set(ctx context.Context, key string, value T) error
	// Get the value for the given key..
	Get(ctx context.Context, key string) (T, error)
	// Clear the value for the given key.
	Clear(ctx context.Context, key string) error
}

// EmptyKeyValueStore is a key value store that does nothing.
type EmptyKeyValueStore[T any] struct{}

// Set discards the key and value and always returns nil.
func (e *EmptyKeyValueStore[T]) Set(_ context.Context, _ string, _ T) error { return nil }

// Get always returns the zero value for T and no error.
func (e *EmptyKeyValueStore[T]) Get(_ context.Context, _ string) (T, error) {
	var r T
	return r, nil
}

// Clear is a no-op that always returns nil.
func (e *EmptyKeyValueStore[T]) Clear(_ context.Context, _ string) error { return nil }
