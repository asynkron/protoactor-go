package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemberSet_Except(t *testing.T) {
	a := NewMemberSet(Members{
		{Id: "1", Host: "h1", Port: 1},
		{Id: "2", Host: "h2", Port: 2},
		{Id: "3", Host: "h3", Port: 3},
	})
	b := NewMemberSet(Members{
		{Id: "2", Host: "h2", Port: 2},
	})

	result := a.Except(b)
	assert.Equal(t, 2, result.Len())
	assert.True(t, result.ContainsID("1"))
	assert.True(t, result.ContainsID("3"))
	assert.False(t, result.ContainsID("2"))
}

func TestMemberSet_Except_NoOverlap(t *testing.T) {
	a := NewMemberSet(Members{
		{Id: "1", Host: "h1", Port: 1},
	})
	b := NewMemberSet(Members{
		{Id: "2", Host: "h2", Port: 2},
	})

	result := a.Except(b)
	assert.Equal(t, 1, result.Len())
	assert.True(t, result.ContainsID("1"))
}

func TestMemberSet_Except_AllOverlap(t *testing.T) {
	a := NewMemberSet(Members{
		{Id: "1", Host: "h1", Port: 1},
	})
	b := NewMemberSet(Members{
		{Id: "1", Host: "h1", Port: 1},
	})

	result := a.Except(b)
	assert.Equal(t, 0, result.Len())
}

func TestMemberSet_Union(t *testing.T) {
	a := NewMemberSet(Members{
		{Id: "1", Host: "h1", Port: 1},
	})
	b := NewMemberSet(Members{
		{Id: "2", Host: "h2", Port: 2},
	})

	result := a.Union(b)
	assert.Equal(t, 2, result.Len())
	assert.True(t, result.ContainsID("1"))
	assert.True(t, result.ContainsID("2"))
}

func TestMemberSet_Union_Overlapping(t *testing.T) {
	a := NewMemberSet(Members{
		{Id: "1", Host: "h1", Port: 1},
		{Id: "2", Host: "h2", Port: 2},
	})
	b := NewMemberSet(Members{
		{Id: "2", Host: "h2", Port: 2},
		{Id: "3", Host: "h3", Port: 3},
	})

	result := a.Union(b)
	assert.Equal(t, 3, result.Len())
	assert.True(t, result.ContainsID("1"))
	assert.True(t, result.ContainsID("2"))
	assert.True(t, result.ContainsID("3"))
}

func TestMemberSet_ExceptIds(t *testing.T) {
	ms := NewMemberSet(Members{
		{Id: "1", Host: "h1", Port: 1},
		{Id: "2", Host: "h2", Port: 2},
		{Id: "3", Host: "h3", Port: 3},
	})

	result := ms.ExceptIds([]string{"1", "3"})
	assert.Equal(t, 1, result.Len())
	assert.True(t, result.ContainsID("2"))
}

func TestMemberSet_ExceptIds_NoneMatch(t *testing.T) {
	ms := NewMemberSet(Members{
		{Id: "1", Host: "h1", Port: 1},
	})

	result := ms.ExceptIds([]string{"99"})
	assert.Equal(t, 1, result.Len())
	assert.True(t, result.ContainsID("1"))
}

func TestMemberSet_Equals(t *testing.T) {
	a := NewMemberSet(Members{
		{Id: "1", Host: "h1", Port: 1},
	})
	b := NewMemberSet(Members{
		{Id: "1", Host: "h1", Port: 1},
	})

	assert.True(t, a.Equals(b))
}

func TestMemberSet_Equals_DifferentMembers(t *testing.T) {
	a := NewMemberSet(Members{
		{Id: "1", Host: "h1", Port: 1},
	})
	b := NewMemberSet(Members{
		{Id: "2", Host: "h2", Port: 2},
	})

	assert.False(t, a.Equals(b))
}

func TestMemberSet_Equals_SameIdsDifferentOrder(t *testing.T) {
	a := NewMemberSet(Members{
		{Id: "1", Host: "h1", Port: 1},
		{Id: "2", Host: "h2", Port: 2},
	})
	b := NewMemberSet(Members{
		{Id: "2", Host: "h2", Port: 2},
		{Id: "1", Host: "h1", Port: 1},
	})

	assert.True(t, a.Equals(b))
}

func TestMemberSet_Empty(t *testing.T) {
	ms := NewMemberSet(Members{})
	assert.Equal(t, 0, ms.Len())
}

func TestMemberSet_ContainsID(t *testing.T) {
	ms := NewMemberSet(Members{
		{Id: "1", Host: "h1", Port: 1},
	})

	assert.True(t, ms.ContainsID("1"))
	assert.False(t, ms.ContainsID("2"))
}

func TestMemberSet_GetMemberById(t *testing.T) {
	ms := NewMemberSet(Members{
		{Id: "1", Host: "h1", Port: 1},
		{Id: "2", Host: "h2", Port: 2},
	})

	m := ms.GetMemberById("1")
	assert.NotNil(t, m)
	assert.Equal(t, "h1", m.Host)
	assert.Equal(t, int32(1), m.Port)

	assert.Nil(t, ms.GetMemberById("nonexistent"))
}

func TestMemberSet_TopologyHash(t *testing.T) {
	a := NewMemberSet(Members{
		{Id: "1", Host: "h1", Port: 1},
		{Id: "2", Host: "h2", Port: 2},
	})
	b := NewMemberSet(Members{
		{Id: "2", Host: "h2", Port: 2},
		{Id: "1", Host: "h1", Port: 1},
	})

	// Same members in different order should produce the same topology hash
	assert.Equal(t, a.TopologyHash(), b.TopologyHash())
	assert.NotEqual(t, uint64(0), a.TopologyHash())
}

func TestMemberSet_Members(t *testing.T) {
	ms := NewMemberSet(Members{
		{Id: "1", Host: "h1", Port: 1},
		{Id: "2", Host: "h2", Port: 2},
	})

	members := ms.Members()
	assert.Equal(t, 2, len(members))
}
