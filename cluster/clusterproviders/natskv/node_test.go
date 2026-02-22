package natskv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNode(t *testing.T) {
	kinds := []string{"kind1", "kind2"}
	n := NewNode("node1", "127.0.0.1", 8080, kinds)

	assert.Equal(t, "node1", n.ID)
	assert.Equal(t, "node1", n.Name)
	assert.Equal(t, "127.0.0.1", n.Host)
	assert.Equal(t, 8080, n.Port)
	assert.Equal(t, kinds, n.Kinds)
	assert.True(t, n.Alive)
}

func TestNode_Serialize_Deserialize(t *testing.T) {
	original := NewNode("node1", "127.0.0.1", 8080, []string{"kind1", "kind2"})

	data, err := original.Serialize()
	require.NoError(t, err)

	restored := &Node{}
	err = restored.Deserialize(data)
	require.NoError(t, err)

	assert.Equal(t, original.ID, restored.ID)
	assert.Equal(t, original.Name, restored.Name)
	assert.Equal(t, original.Host, restored.Host)
	assert.Equal(t, original.Port, restored.Port)
	assert.Equal(t, original.Kinds, restored.Kinds)
	assert.Equal(t, original.Alive, restored.Alive)
}

func TestNewNodeFromBytes_Invalid(t *testing.T) {
	_, err := NewNodeFromBytes([]byte("not valid json"))
	assert.Error(t, err)
}

func TestNode_MemberStatus(t *testing.T) {
	n := NewNode("node1", "127.0.0.1", 8080, []string{"kind1", "kind2"})
	m := n.MemberStatus()

	assert.Equal(t, "node1", m.Id)
	assert.Equal(t, "127.0.0.1", m.Host)
	assert.Equal(t, int32(8080), m.Port)
	assert.Equal(t, []string{"kind1", "kind2"}, m.Kinds)
}

func TestNode_MemberStatus_NilKinds(t *testing.T) {
	n := NewNode("node1", "127.0.0.1", 8080, nil)
	// Kinds should be nil initially
	assert.Nil(t, n.Kinds)

	m := n.MemberStatus()
	// MemberStatus should convert nil kinds to empty slice
	assert.NotNil(t, m.Kinds)
	assert.Empty(t, m.Kinds)
}

func TestNode_Equal(t *testing.T) {
	n1 := NewNode("node1", "127.0.0.1", 8080, nil)
	n2 := NewNode("node1", "127.0.0.2", 9090, nil)
	n3 := NewNode("node2", "127.0.0.1", 8080, nil)

	// Same ID = equal
	assert.True(t, n1.Equal(n2))

	// Different ID = not equal
	assert.False(t, n1.Equal(n3))

	// Nil = not equal
	assert.False(t, n1.Equal(nil))
	assert.False(t, (*Node)(nil).Equal(n1))
	assert.False(t, (*Node)(nil).Equal(nil))

	// Self = equal
	assert.True(t, n1.Equal(n1))
}

func TestNode_AliveFlag(t *testing.T) {
	n := NewNode("node1", "127.0.0.1", 8080, nil)

	// Default is alive
	assert.True(t, n.IsAlive())

	// Set to dead
	n.SetAlive(false)
	assert.False(t, n.IsAlive())

	// Set back to alive
	n.SetAlive(true)
	assert.True(t, n.IsAlive())
}
