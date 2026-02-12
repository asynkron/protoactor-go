package extensions_test

import (
	"testing"

	"github.com/asynkron/protoactor-go/extensions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testExtension is a minimal Extension implementation for testing.
type testExtension struct {
	id   extensions.ExtensionID
	name string
}

func (e *testExtension) ExtensionID() extensions.ExtensionID {
	return e.id
}

func TestNextExtensionID_Increments(t *testing.T) {
	id1 := extensions.NextExtensionID()
	id2 := extensions.NextExtensionID()
	id3 := extensions.NextExtensionID()

	assert.Less(t, id1, id2, "second ID should be greater than first")
	assert.Less(t, id2, id3, "third ID should be greater than second")
	assert.Equal(t, id2-id1, extensions.ExtensionID(1), "IDs should increment by 1")
	assert.Equal(t, id3-id2, extensions.ExtensionID(1), "IDs should increment by 1")
}

func TestNewExtensions_ReturnsNonNil(t *testing.T) {
	ex := extensions.NewExtensions()
	require.NotNil(t, ex)
}

func TestRegisterAndGet_RoundTrip(t *testing.T) {
	ex := extensions.NewExtensions()
	id := extensions.NextExtensionID()
	ext := &testExtension{id: id, name: "test-ext"}

	ex.Register(ext)

	got := ex.Get(id)
	require.NotNil(t, got)
	assert.Equal(t, ext, got)
	assert.Equal(t, "test-ext", got.(*testExtension).name)
}

func TestGet_UnregisteredID_ReturnsNil(t *testing.T) {
	ex := extensions.NewExtensions()
	id := extensions.NextExtensionID()

	got := ex.Get(id)
	assert.Nil(t, got)
}

func TestRegister_MultipleExtensions(t *testing.T) {
	ex := extensions.NewExtensions()
	id1 := extensions.NextExtensionID()
	id2 := extensions.NextExtensionID()

	ext1 := &testExtension{id: id1, name: "ext-1"}
	ext2 := &testExtension{id: id2, name: "ext-2"}

	ex.Register(ext1)
	ex.Register(ext2)

	got1 := ex.Get(id1)
	got2 := ex.Get(id2)
	require.NotNil(t, got1)
	require.NotNil(t, got2)
	assert.Equal(t, "ext-1", got1.(*testExtension).name)
	assert.Equal(t, "ext-2", got2.(*testExtension).name)
}

func TestRegister_OverwritesSameID(t *testing.T) {
	ex := extensions.NewExtensions()
	id := extensions.NextExtensionID()

	ext1 := &testExtension{id: id, name: "original"}
	ext2 := &testExtension{id: id, name: "replacement"}

	ex.Register(ext1)
	ex.Register(ext2)

	got := ex.Get(id)
	require.NotNil(t, got)
	assert.Equal(t, "replacement", got.(*testExtension).name)
}
