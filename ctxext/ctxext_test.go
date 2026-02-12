package ctxext_test

import (
	"testing"

	"github.com/asynkron/protoactor-go/ctxext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testContextExtension is a minimal ContextExtension for testing.
type testContextExtension struct {
	id   ctxext.ContextExtensionID
	data string
}

func (e *testContextExtension) ExtensionID() ctxext.ContextExtensionID {
	return e.id
}

func TestNextContextExtensionID_Increments(t *testing.T) {
	id1 := ctxext.NextContextExtensionID()
	id2 := ctxext.NextContextExtensionID()
	id3 := ctxext.NextContextExtensionID()

	assert.Less(t, id1, id2, "second ID should be greater than first")
	assert.Less(t, id2, id3, "third ID should be greater than second")
	assert.Equal(t, id2-id1, ctxext.ContextExtensionID(1), "IDs should increment by 1")
}

func TestNewContextExtensions_ReturnsNonNil(t *testing.T) {
	ex := ctxext.NewContextExtensions()
	require.NotNil(t, ex)
}

func TestSetAndGet_RoundTrip(t *testing.T) {
	ex := ctxext.NewContextExtensions()
	id := ctxext.NextContextExtensionID()
	ext := &testContextExtension{id: id, data: "hello"}

	ex.Set(ext)

	got := ex.Get(id)
	require.NotNil(t, got)
	assert.Equal(t, ext, got)
	assert.Equal(t, "hello", got.(*testContextExtension).data)
}

func TestGet_UnsetID_ReturnsNil(t *testing.T) {
	ex := ctxext.NewContextExtensions()
	// Use an ID within the initial slice capacity (3 elements, indices 0-2)
	// NextContextExtensionID returns IDs starting from the global counter,
	// which may be beyond index 2, so we test with a known-safe index.
	// The slice is initialized to size 3, and index 0 is unused if no extension is set there.
	got := ex.Get(0)
	assert.Nil(t, got)
}

func TestSet_MultipleExtensions(t *testing.T) {
	ex := ctxext.NewContextExtensions()
	id1 := ctxext.NextContextExtensionID()
	id2 := ctxext.NextContextExtensionID()

	ext1 := &testContextExtension{id: id1, data: "first"}
	ext2 := &testContextExtension{id: id2, data: "second"}

	ex.Set(ext1)
	ex.Set(ext2)

	got1 := ex.Get(id1)
	got2 := ex.Get(id2)
	require.NotNil(t, got1)
	require.NotNil(t, got2)
	assert.Equal(t, "first", got1.(*testContextExtension).data)
	assert.Equal(t, "second", got2.(*testContextExtension).data)
}

func TestSet_GrowsSliceForLargeID(t *testing.T) {
	ex := ctxext.NewContextExtensions()
	// Create an extension with an ID that exceeds the initial slice capacity of 3.
	// We call NextContextExtensionID enough times to get a large-ish ID,
	// or we manually set a large ID. The Set method should grow the slice.
	// Since we cannot control atomic counter, let's just allocate several IDs.
	var lastID ctxext.ContextExtensionID
	for i := 0; i < 10; i++ {
		lastID = ctxext.NextContextExtensionID()
	}

	ext := &testContextExtension{id: lastID, data: "large-id"}
	// This should not panic even though the ID exceeds initial capacity.
	ex.Set(ext)

	got := ex.Get(lastID)
	require.NotNil(t, got)
	assert.Equal(t, "large-id", got.(*testContextExtension).data)
}

func TestSet_OverwritesSameID(t *testing.T) {
	ex := ctxext.NewContextExtensions()
	id := ctxext.NextContextExtensionID()

	ext1 := &testContextExtension{id: id, data: "original"}
	ext2 := &testContextExtension{id: id, data: "updated"}

	ex.Set(ext1)
	ex.Set(ext2)

	got := ex.Get(id)
	require.NotNil(t, got)
	assert.Equal(t, "updated", got.(*testContextExtension).data)
}
