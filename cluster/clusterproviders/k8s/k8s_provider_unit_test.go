package k8s

import (
	"log/slog"
	"testing"

	"github.com/asynkron/protoactor-go/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Compile-time check: Provider must implement KindUpdater.
var _ cluster.KindUpdater = (*Provider)(nil)

func TestProvider_UpdateKinds_UpdatesKnownKinds(t *testing.T) {
	p := &Provider{
		knownKinds: []string{"kindA"},
	}

	err := p.UpdateKinds([]string{"kindA", "kindB"})
	// We expect an error because there's no K8s client, but knownKinds should be updated.
	_ = err
	assert.ElementsMatch(t, []string{"kindA", "kindB"}, p.knownKinds)
}

func TestProvider_New_FieldDefaults(t *testing.T) {
	p := &Provider{
		clusterPods: make(map[types.UID]*v1.Pod),
		namespace:   "default",
		clusterName: "test-cluster",
	}
	assert.NotNil(t, p)
	assert.Equal(t, "default", p.namespace)
	assert.Equal(t, "test-cluster", p.clusterName)
	assert.NotNil(t, p.clusterPods)
	assert.Empty(t, p.clusterPods)
}

func TestProvider_ClusterPodsStorage(t *testing.T) {
	p := &Provider{
		clusterName: "test-cluster",
		clusterPods: make(map[types.UID]*v1.Pod),
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:  types.UID("test-uid-1"),
			Name: "test-pod-1",
			Labels: map[string]string{
				LabelCluster:  "test-cluster",
				LabelPort:     "8080",
				LabelMemberID: "member-1",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	p.clusterPods[pod.UID] = pod
	assert.Equal(t, 1, len(p.clusterPods))
	assert.Equal(t, "test-pod-1", p.clusterPods[types.UID("test-uid-1")].Name)

	// Add a second pod
	pod2 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:  types.UID("test-uid-2"),
			Name: "test-pod-2",
		},
	}
	p.clusterPods[pod2.UID] = pod2
	assert.Equal(t, 2, len(p.clusterPods))

	// Delete first pod
	delete(p.clusterPods, pod.UID)
	assert.Equal(t, 1, len(p.clusterPods))
}

func TestMapPodsToMembers_SingleRunningPod(t *testing.T) {
	logger := slog.Default()
	clusterPods := make(map[types.UID]*v1.Pod)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:  types.UID("uid-1"),
			Name: "pod-1",
			Labels: map[string]string{
				LabelCluster:               "test-cluster",
				LabelPort:                  "8080",
				LabelMemberID:              "member-1",
				LabelKind + "-grainType1":  "true",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			PodIPs: []v1.PodIP{
				{IP: "10.0.0.1"},
			},
			PodIP: "10.0.0.1",
			ContainerStatuses: []v1.ContainerStatus{
				{Ready: true, Name: "main"},
			},
		},
	}
	clusterPods[pod.UID] = pod

	members := mapPodsToMembers(clusterPods, logger)

	require.Len(t, members, 1)
	assert.Equal(t, "member-1", members[0].Id)
	assert.Equal(t, "10.0.0.1", members[0].Host)
	assert.Equal(t, int32(8080), members[0].Port)
	assert.Contains(t, members[0].Kinds, "grainType1")
}

func TestMapPodsToMembers_MultipleRunningPods(t *testing.T) {
	logger := slog.Default()
	clusterPods := make(map[types.UID]*v1.Pod)

	for i, tc := range []struct {
		uid  string
		name string
		ip   string
		port string
		mid  string
	}{
		{"uid-1", "pod-1", "10.0.0.1", "8080", "member-1"},
		{"uid-2", "pod-2", "10.0.0.2", "8081", "member-2"},
		{"uid-3", "pod-3", "10.0.0.3", "8082", "member-3"},
	} {
		_ = i
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				UID:  types.UID(tc.uid),
				Name: tc.name,
				Labels: map[string]string{
					LabelCluster:  "test-cluster",
					LabelPort:     tc.port,
					LabelMemberID: tc.mid,
				},
			},
			Status: v1.PodStatus{
				Phase: v1.PodRunning,
				PodIPs: []v1.PodIP{
					{IP: tc.ip},
				},
				PodIP: tc.ip,
				ContainerStatuses: []v1.ContainerStatus{
					{Ready: true, Name: "main"},
				},
			},
		}
		clusterPods[pod.UID] = pod
	}

	members := mapPodsToMembers(clusterPods, logger)
	assert.Len(t, members, 3)

	// Collect member IDs to verify all are present (order may vary due to map iteration)
	memberIDs := make(map[string]bool)
	for _, m := range members {
		memberIDs[m.Id] = true
	}
	assert.True(t, memberIDs["member-1"])
	assert.True(t, memberIDs["member-2"])
	assert.True(t, memberIDs["member-3"])
}

func TestMapPodsToMembers_SkipsNonRunningPods(t *testing.T) {
	logger := slog.Default()
	clusterPods := make(map[types.UID]*v1.Pod)

	// Pod in Pending phase
	pendingPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:  types.UID("uid-pending"),
			Name: "pending-pod",
			Labels: map[string]string{
				LabelCluster:  "test-cluster",
				LabelPort:     "8080",
				LabelMemberID: "member-pending",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodPending,
			PodIPs: []v1.PodIP{
				{IP: "10.0.0.1"},
			},
		},
	}
	clusterPods[pendingPod.UID] = pendingPod

	// Pod in Succeeded phase (completed)
	succeededPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:  types.UID("uid-succeeded"),
			Name: "succeeded-pod",
			Labels: map[string]string{
				LabelCluster:  "test-cluster",
				LabelPort:     "8081",
				LabelMemberID: "member-succeeded",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodSucceeded,
			PodIPs: []v1.PodIP{
				{IP: "10.0.0.2"},
			},
		},
	}
	clusterPods[succeededPod.UID] = succeededPod

	members := mapPodsToMembers(clusterPods, logger)
	assert.Empty(t, members)
}

func TestMapPodsToMembers_SkipsPodsWithNoPodIPs(t *testing.T) {
	logger := slog.Default()
	clusterPods := make(map[types.UID]*v1.Pod)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:  types.UID("uid-no-ip"),
			Name: "no-ip-pod",
			Labels: map[string]string{
				LabelCluster:  "test-cluster",
				LabelPort:     "8080",
				LabelMemberID: "member-no-ip",
			},
		},
		Status: v1.PodStatus{
			Phase:  v1.PodRunning,
			PodIPs: []v1.PodIP{}, // empty PodIPs
		},
	}
	clusterPods[pod.UID] = pod

	members := mapPodsToMembers(clusterPods, logger)
	assert.Empty(t, members)
}

func TestMapPodsToMembers_SkipsPodsWithUnreadyContainers(t *testing.T) {
	logger := slog.Default()
	clusterPods := make(map[types.UID]*v1.Pod)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:  types.UID("uid-unready"),
			Name: "unready-pod",
			Labels: map[string]string{
				LabelCluster:  "test-cluster",
				LabelPort:     "8080",
				LabelMemberID: "member-unready",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			PodIPs: []v1.PodIP{
				{IP: "10.0.0.1"},
			},
			PodIP: "10.0.0.1",
			ContainerStatuses: []v1.ContainerStatus{
				{Ready: true, Name: "main"},
				{Ready: false, Name: "sidecar"}, // one container not ready
			},
		},
	}
	clusterPods[pod.UID] = pod

	members := mapPodsToMembers(clusterPods, logger)
	assert.Empty(t, members)
}

func TestMapPodsToMembers_InvalidPortLabel(t *testing.T) {
	logger := slog.Default()
	clusterPods := make(map[types.UID]*v1.Pod)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:  types.UID("uid-bad-port"),
			Name: "bad-port-pod",
			Labels: map[string]string{
				LabelCluster:  "test-cluster",
				LabelPort:     "not-a-number",
				LabelMemberID: "member-bad-port",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			PodIPs: []v1.PodIP{
				{IP: "10.0.0.1"},
			},
			PodIP: "10.0.0.1",
			ContainerStatuses: []v1.ContainerStatus{
				{Ready: true, Name: "main"},
			},
		},
	}
	clusterPods[pod.UID] = pod

	members := mapPodsToMembers(clusterPods, logger)
	assert.Empty(t, members, "pods with invalid port labels should be skipped")
}

func TestMapPodsToMembers_MultipleKinds(t *testing.T) {
	logger := slog.Default()
	clusterPods := make(map[types.UID]*v1.Pod)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:  types.UID("uid-multi-kind"),
			Name: "multi-kind-pod",
			Labels: map[string]string{
				LabelCluster:              "test-cluster",
				LabelPort:                 "8080",
				LabelMemberID:             "member-multi",
				LabelKind + "-grainA":     "true",
				LabelKind + "-grainB":     "true",
				LabelKind + "-grainC":     "true",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			PodIPs: []v1.PodIP{
				{IP: "10.0.0.1"},
			},
			PodIP: "10.0.0.1",
			ContainerStatuses: []v1.ContainerStatus{
				{Ready: true, Name: "main"},
			},
		},
	}
	clusterPods[pod.UID] = pod

	members := mapPodsToMembers(clusterPods, logger)
	require.Len(t, members, 1)
	assert.Len(t, members[0].Kinds, 3)
	assert.Contains(t, members[0].Kinds, "grainA")
	assert.Contains(t, members[0].Kinds, "grainB")
	assert.Contains(t, members[0].Kinds, "grainC")
}

func TestMapPodsToMembers_EmptyCluster(t *testing.T) {
	logger := slog.Default()
	clusterPods := make(map[types.UID]*v1.Pod)

	members := mapPodsToMembers(clusterPods, logger)
	assert.Empty(t, members)
	assert.NotNil(t, members, "should return empty slice, not nil")
}

func TestMapPodsToMembers_MixedReadyAndNotReady(t *testing.T) {
	logger := slog.Default()
	clusterPods := make(map[types.UID]*v1.Pod)

	// Ready pod
	readyPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:  types.UID("uid-ready"),
			Name: "ready-pod",
			Labels: map[string]string{
				LabelCluster:  "test-cluster",
				LabelPort:     "8080",
				LabelMemberID: "member-ready",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			PodIPs: []v1.PodIP{
				{IP: "10.0.0.1"},
			},
			PodIP: "10.0.0.1",
			ContainerStatuses: []v1.ContainerStatus{
				{Ready: true, Name: "main"},
			},
		},
	}
	clusterPods[readyPod.UID] = readyPod

	// Not ready pod
	notReadyPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:  types.UID("uid-not-ready"),
			Name: "not-ready-pod",
			Labels: map[string]string{
				LabelCluster:  "test-cluster",
				LabelPort:     "8081",
				LabelMemberID: "member-not-ready",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			PodIPs: []v1.PodIP{
				{IP: "10.0.0.2"},
			},
			PodIP: "10.0.0.2",
			ContainerStatuses: []v1.ContainerStatus{
				{Ready: false, Name: "main"},
			},
		},
	}
	clusterPods[notReadyPod.UID] = notReadyPod

	members := mapPodsToMembers(clusterPods, logger)
	require.Len(t, members, 1)
	assert.Equal(t, "member-ready", members[0].Id)
}

func TestMapPodsToMembers_PodWithNoContainerStatuses(t *testing.T) {
	logger := slog.Default()
	clusterPods := make(map[types.UID]*v1.Pod)

	// Pod with no container statuses should be considered alive
	// (the alive check only fails if a container is explicitly not ready)
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:  types.UID("uid-no-containers"),
			Name: "no-containers-pod",
			Labels: map[string]string{
				LabelCluster:  "test-cluster",
				LabelPort:     "8080",
				LabelMemberID: "member-no-containers",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			PodIPs: []v1.PodIP{
				{IP: "10.0.0.1"},
			},
			PodIP:             "10.0.0.1",
			ContainerStatuses: []v1.ContainerStatus{},
		},
	}
	clusterPods[pod.UID] = pod

	members := mapPodsToMembers(clusterPods, logger)
	require.Len(t, members, 1)
	assert.Equal(t, "member-no-containers", members[0].Id)
}

func TestMapPodsToMembers_ReturnsCorrectMemberFields(t *testing.T) {
	logger := slog.Default()
	clusterPods := make(map[types.UID]*v1.Pod)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:  types.UID("uid-full"),
			Name: "full-pod",
			Labels: map[string]string{
				LabelCluster:             "production-cluster",
				LabelPort:                "9090",
				LabelMemberID:            "member-abc-123",
				LabelKind + "-MyGrain":   "true",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			PodIPs: []v1.PodIP{
				{IP: "192.168.1.100"},
			},
			PodIP: "192.168.1.100",
			ContainerStatuses: []v1.ContainerStatus{
				{Ready: true, Name: "app"},
			},
		},
	}
	clusterPods[pod.UID] = pod

	members := mapPodsToMembers(clusterPods, logger)
	require.Len(t, members, 1)

	member := members[0]
	assert.Equal(t, "member-abc-123", member.Id)
	assert.Equal(t, "192.168.1.100", member.Host)
	assert.Equal(t, int32(9090), member.Port)
	assert.Equal(t, []string{"MyGrain"}, member.Kinds)
}

func TestLabels_Constants(t *testing.T) {
	// Verify label constants have the expected prefix
	assert.Equal(t, "cluster.proto.actor/", LabelPrefix)
	assert.Equal(t, LabelPrefix+"port", LabelPort)
	assert.Equal(t, LabelPrefix+"kind", LabelKind)
	assert.Equal(t, LabelPrefix+"cluster", LabelCluster)
	assert.Equal(t, LabelPrefix+"status-value", LabelStatusValue)
	assert.Equal(t, LabelPrefix+"member-id", LabelMemberID)
}

func TestProvider_ShutdownFlag(t *testing.T) {
	p := &Provider{
		clusterPods: make(map[types.UID]*v1.Pod),
	}

	assert.False(t, p.shutdown.Load())
	p.shutdown.Store(true)
	assert.True(t, p.shutdown.Load())
}

func TestProvider_ImplementsClusterProvider(t *testing.T) {
	// Verify at compile time that Provider implements ClusterProvider
	var _ cluster.ClusterProvider = (*Provider)(nil)
}

func TestErrProviderShuttingDown(t *testing.T) {
	assert.NotNil(t, ErrProviderShuttingDown)
	assert.Equal(t, "kubernetes cluster provider is being shut down", ErrProviderShuttingDown.Error())
	// Backward compatibility alias
	assert.Equal(t, ErrProviderShuttingDown, ProviderShuttingDownError)
}

func TestLabels_Type(t *testing.T) {
	labels := Labels{
		"key1": "value1",
		"key2": "value2",
	}
	assert.Equal(t, "value1", labels["key1"])
	assert.Equal(t, "value2", labels["key2"])
	assert.Len(t, labels, 2)
}

func TestLogCurrentPods_DoesNotPanic(t *testing.T) {
	logger := slog.Default()

	// Empty map
	clusterPods := make(map[types.UID]*v1.Pod)
	assert.NotPanics(t, func() {
		logCurrentPods(clusterPods, logger)
	})

	// With pods
	clusterPods[types.UID("uid-1")] = &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-1"},
	}
	clusterPods[types.UID("uid-2")] = &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-2"},
	}
	assert.NotPanics(t, func() {
		logCurrentPods(clusterPods, logger)
	})
}
