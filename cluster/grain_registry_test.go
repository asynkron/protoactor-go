package cluster

import (
	"testing"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockIdentityLookup implements IdentityLookup but NOT GrainEnumerator.
type mockIdentityLookup struct{}

func (m *mockIdentityLookup) Get(_ *ClusterIdentity) *actor.PID            { return nil }
func (m *mockIdentityLookup) RemovePid(_ *ClusterIdentity, _ *actor.PID)   {}
func (m *mockIdentityLookup) Setup(_ *Cluster, _ []string, _ bool)         {}
func (m *mockIdentityLookup) Shutdown()                                    {}
func (m *mockIdentityLookup) Peek(_ *ClusterIdentity) (*PeekResult, error) { return nil, nil }

func TestGrainRegistry_Count_ReturnsVirtualActorCount(t *testing.T) {
	c := newTestCluster()
	c.grainReg = &GrainRegistry{cluster: c}

	assert.Equal(t, 0, c.GrainRegistry().Count())
}

func TestGrainRegistry_CountByKind_ReturnsPerKindCounts(t *testing.T) {
	c := newTestCluster()
	c.grainReg = &GrainRegistry{cluster: c}

	kind1 := NewKind("kindA", actor.PropsFromProducer(func() actor.Actor {
		return &testActor{}
	}))
	kind2 := NewKind("kindB", actor.PropsFromProducer(func() actor.Actor {
		return &testActor{}
	}))

	err := c.RegisterKind(kind1)
	require.NoError(t, err)
	err = c.RegisterKind(kind2)
	require.NoError(t, err)

	counts := c.GrainRegistry().CountByKind()
	assert.Contains(t, counts, "kindA")
	assert.Contains(t, counts, "kindB")
	// Fresh kinds have zero activations.
	assert.Equal(t, 0, counts["kindA"])
	assert.Equal(t, 0, counts["kindB"])
}

func TestGrainRegistry_All_ReturnsErrorWhenNotSupported(t *testing.T) {
	c := newTestCluster()
	c.grainReg = &GrainRegistry{cluster: c}
	c.IdentityLookup = &mockIdentityLookup{}

	grains, err := c.GrainRegistry().All()
	assert.ErrorIs(t, err, ErrEnumerationNotSupported)
	assert.Nil(t, grains)
}

func TestGrainRegistry_ByKind_ReturnsErrorWhenNotSupported(t *testing.T) {
	c := newTestCluster()
	c.grainReg = &GrainRegistry{cluster: c}
	c.IdentityLookup = &mockIdentityLookup{}

	grains, err := c.GrainRegistry().ByKind("someKind")
	assert.ErrorIs(t, err, ErrEnumerationNotSupported)
	assert.Nil(t, grains)
}

func TestGrainRegistry_ByMember_ReturnsErrorWhenNotSupported(t *testing.T) {
	c := newTestCluster()
	c.grainReg = &GrainRegistry{cluster: c}
	c.IdentityLookup = &mockIdentityLookup{}

	grains, err := c.GrainRegistry().ByMember("member1")
	assert.ErrorIs(t, err, ErrEnumerationNotSupported)
	assert.Nil(t, grains)
}

func TestGrainRegistry_Get_ReturnsErrorWhenNotSupported(t *testing.T) {
	c := newTestCluster()
	c.grainReg = &GrainRegistry{cluster: c}
	c.IdentityLookup = &mockIdentityLookup{}

	info, found, err := c.GrainRegistry().Get("id1", "kind1")
	assert.ErrorIs(t, err, ErrEnumerationNotSupported)
	assert.False(t, found)
	assert.Nil(t, info)
}

func TestParseStoredActivationInfoKey(t *testing.T) {
	kind, identity := ParseStoredActivationInfoKey("myKind/myIdentity")
	assert.Equal(t, "myKind", kind)
	assert.Equal(t, "myIdentity", identity)

	kind, identity = ParseStoredActivationInfoKey("noSlash")
	assert.Equal(t, "noSlash", kind)
	assert.Equal(t, "", identity)
}

func TestParseDotSeparatedKey(t *testing.T) {
	kind, identity := ParseDotSeparatedKey("myKind.myIdentity")
	assert.Equal(t, "myKind", kind)
	assert.Equal(t, "myIdentity", identity)

	kind, identity = ParseDotSeparatedKey("noDot")
	assert.Equal(t, "noDot", kind)
	assert.Equal(t, "", identity)
}

func TestStoredActivationInfoToGrainInfo(t *testing.T) {
	info := &StoredActivationInfo{
		Identity: "grain1",
		Kind:     "myKind",
		Pid:      "127.0.0.1:8080/actor1",
		MemberID: "member1",
	}

	gi := StoredActivationInfoToGrainInfo(info)
	assert.Equal(t, "grain1", gi.Identity)
	assert.Equal(t, "myKind", gi.Kind)
	assert.Equal(t, "member1", gi.MemberID)
	require.NotNil(t, gi.PID)
	assert.Equal(t, "127.0.0.1:8080", gi.PID.Address)
	assert.Equal(t, "actor1", gi.PID.Id)
}

func TestStoredActivationInfoToGrainInfo_EmptyPid(t *testing.T) {
	info := &StoredActivationInfo{
		Identity: "grain1",
		Kind:     "myKind",
		Pid:      "",
		MemberID: "member1",
	}

	gi := StoredActivationInfoToGrainInfo(info)
	assert.Nil(t, gi.PID)
}
