package natsstream

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNode(t *testing.T) {
	n := NewNode("member1", "127.0.0.1", 8080, []string{"MyKind"})
	assert.Equal(t, "member1", n.ID)
	assert.Equal(t, "member1", n.Name)
	assert.Equal(t, "127.0.0.1", n.Host)
	assert.Equal(t, 8080, n.Port)
	assert.Equal(t, []string{"MyKind"}, n.Kinds)
	assert.True(t, n.Alive)
}

func TestNode_Serialize_Deserialize(t *testing.T) {
	n := NewNode("member1", "127.0.0.1", 8080, []string{"MyKind"})
	data, err := n.Serialize()
	require.NoError(t, err)

	n2, err := NewNodeFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, n.ID, n2.ID)
	assert.Equal(t, n.Host, n2.Host)
	assert.Equal(t, n.Port, n2.Port)
	assert.Equal(t, n.Kinds, n2.Kinds)
	assert.Equal(t, n.Alive, n2.Alive)
}

func TestNode_MemberStatus(t *testing.T) {
	n := NewNode("member1", "127.0.0.1", 8080, []string{"MyKind"})
	m := n.MemberStatus()
	assert.Equal(t, "member1", m.Id)
	assert.Equal(t, "127.0.0.1", m.Host)
	assert.Equal(t, int32(8080), m.Port)
	assert.Equal(t, []string{"MyKind"}, m.Kinds)
}

func TestNode_MemberStatus_NilKinds(t *testing.T) {
	n := NewNode("member1", "127.0.0.1", 8080, nil)
	m := n.MemberStatus()
	assert.Equal(t, []string{}, m.Kinds)
}

func TestNode_Equal(t *testing.T) {
	n1 := NewNode("member1", "127.0.0.1", 8080, nil)
	n2 := NewNode("member1", "10.0.0.1", 9090, nil)
	n3 := NewNode("member2", "127.0.0.1", 8080, nil)

	assert.True(t, n1.Equal(n2))
	assert.False(t, n1.Equal(n3))
	assert.False(t, n1.Equal(nil))

	var nilNode *Node
	assert.False(t, nilNode.Equal(n1))
}

func TestNode_AliveFlag(t *testing.T) {
	n := NewNode("member1", "127.0.0.1", 8080, nil)
	assert.True(t, n.IsAlive())
	n.SetAlive(false)
	assert.False(t, n.IsAlive())
}
