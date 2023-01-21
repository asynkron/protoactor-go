package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/log"
	"github.com/google/uuid"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var ProviderShuttingDownError = fmt.Errorf("kubernetes cluster provider is being shut down")

// Convenience type to store cluster labels
type Labels map[string]string

// This data structure provides of k8s as cluster provider for Proto.Actor
type Provider struct {
	id             string
	cluster        *cluster.Cluster
	clusterName    string
	podName        string
	host           string
	address        string
	namespace      string
	knownKinds     []string
	clusterPods    map[types.UID]*v1.Pod
	port           int
	client         *kubernetes.Clientset
	clusterMonitor *actor.PID
	deregistered   bool
	shutdown       bool
	cancelWatch    context.CancelFunc
}

// make sure our Provider complies with the ClusterProvider interface
var _ cluster.ClusterProvider = (*Provider)(nil)

// New crates a new k8s Provider in the heap and return back a reference to its memory address
func New(opts ...Option) (*Provider, error) {
	// create new default k8s config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	return NewWithConfig(config, opts...)
}

// NewWithConfig creates a new k8s Provider in the heap using the given configuration
// and options, it returns a reference to its memory address or an error
func NewWithConfig(config *rest.Config, opts ...Option) (*Provider, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	p := Provider{
		client: clientset,
	}

	// process given options
	for _, opt := range opts {
		opt(&p)
	}
	return &p, nil
}

// initializes the cluster provider
func (p *Provider) init(c *cluster.Cluster) error {
	host, port, err := c.ActorSystem.GetHostPort()
	if err != nil {
		return err
	}

	p.cluster = c
	p.id = strings.Replace(uuid.New().String(), "-", "", -1)
	p.knownKinds = c.GetClusterKinds()
	p.clusterName = c.Config.Name
	p.clusterPods = make(map[types.UID]*v1.Pod)
	p.host = host
	p.port = port
	p.address = fmt.Sprintf("%s:%d", host, port)
	return nil
}

// StartMember registers the member in the cluster and start it
func (p *Provider) StartMember(c *cluster.Cluster) error {
	if err := p.init(c); err != nil {
		return err
	}

	if err := p.startClusterMonitor(c); err != nil {
		return err
	}

	p.registerMemberAsync(c)
	p.startWatchingClusterAsync(c)

	return nil
}

// StartClient starts the k8s client and monitor watch
func (p *Provider) StartClient(c *cluster.Cluster) error {
	if err := p.init(c); err != nil {
		return err
	}

	if err := p.startClusterMonitor(c); err != nil {
		return err
	}

	p.startWatchingClusterAsync(c)
	return nil
}

func (p *Provider) Shutdown(graceful bool) error {
	if p.shutdown {
		// we are already shut down or shutting down
		return nil
	}

	p.shutdown = true

	plog.Info("Shutting down k8s cluster provider")
	if p.clusterMonitor != nil {
		if err := p.cluster.ActorSystem.Root.RequestFuture(p.clusterMonitor, &DeregisterMember{}, 5*time.Second).Wait(); err != nil {
			plog.Error("Failed to deregister member - cluster monitor did not respond, proceeding with shutdown", log.Error(err))
		}

		if err := p.cluster.ActorSystem.Root.RequestFuture(p.clusterMonitor, &StopWatchingCluster{}, 5*time.Second).Wait(); err != nil {
			plog.Error("Failed to deregister member - cluster monitor did not respond, proceeding with shutdown", log.Error(err))
		}

		_ = p.cluster.ActorSystem.Root.StopFuture(p.clusterMonitor).Wait()
		p.clusterMonitor = nil
	}

	return nil
}

// starts the cluster monitor in its own goroutine
func (p *Provider) startClusterMonitor(c *cluster.Cluster) error {
	var err error
	p.clusterMonitor, err = c.ActorSystem.Root.SpawnNamed(actor.PropsFromProducer(func() actor.Actor {
		return newClusterMonitor(p)
	}), "k8s-cluster-monitor")

	if err != nil {
		plog.Error("Failed to start k8s-cluster-monitor actor", log.Error(err))
		return err
	}

	p.podName, _ = os.Hostname()
	return nil
}

// registers itself as a member asynchronously using an actor
func (p *Provider) registerMemberAsync(c *cluster.Cluster) {
	msg := RegisterMember{}
	c.ActorSystem.Root.Send(p.clusterMonitor, &msg)
}

// registers itself as a member in k8s cluster
func (p *Provider) registerMember(timeout time.Duration) error {
	plog.Info(fmt.Sprintf("Registering service %s on %s", p.podName, p.address))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pod, err := p.client.CoreV1().Pods(p.retrieveNamespace()).Get(ctx, p.podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("unable to get own pod information for %s: %w", p.podName, err)
	}

	plog.Info(fmt.Sprintf("Using Kubernetes namespace: %s\nUsing Kubernetes port: %d", pod.Namespace, p.port))

	labels := Labels{
		LabelCluster:  p.clusterName,
		LabelPort:     fmt.Sprintf("%d", p.port),
		LabelMemberID: p.id,
	}

	// add known kinds to labels
	for _, kind := range p.knownKinds {
		labelkey := fmt.Sprintf("%s-%s", LabelKind, kind)
		labels[labelkey] = "true"
	}

	// add existing labels back
	for key, value := range pod.ObjectMeta.Labels {
		labels[key] = value
	}
	pod.SetLabels(labels)

	return p.replacePodLabels(ctx, pod)
}

func (p *Provider) startWatchingClusterAsync(c *cluster.Cluster) {
	msg := StartWatchingCluster{p.clusterName}
	c.ActorSystem.Root.Send(p.clusterMonitor, &msg)
}

func (p *Provider) startWatchingCluster() error {
	selector := fmt.Sprintf("%s=%s", LabelCluster, p.clusterName)

	plog.Debug(fmt.Sprintf("Starting to watch pods with %s", selector), log.String("selector", selector))

	ctx, cancel := context.WithCancel(context.Background())
	p.cancelWatch = cancel

	// start a new goroutine to monitor the cluster events
	go func() {
		for {
			select {
			case <-ctx.Done():
				plog.Debug("Stopping watch on pods")
				return
			default:
				if err := p.watchPods(ctx, selector); err != nil {
					plog.Error("Error watching pods, will retry", log.Error(err))
					time.Sleep(5 * time.Second)
				}
			}
		}
	}()

	return nil
}

func (p *Provider) watchPods(ctx context.Context, selector string) error {
	watcher, err := p.client.CoreV1().Pods(p.retrieveNamespace()).Watch(context.Background(), metav1.ListOptions{LabelSelector: selector, Watch: true})
	if err != nil {
		err = fmt.Errorf("unable to watch pods: %w", err)
		plog.Error(err.Error(), log.Error(err))
		return err
	}

	plog.Info("Pod watcher started")

	for {
		select {
		case <-ctx.Done():
			watcher.Stop()
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("pod watcher channel closed abruptly")
			}
			pod, ok := event.Object.(*v1.Pod)
			if !ok {
				err := fmt.Errorf("could not cast %#v[%T] into v1.Pod", event.Object, event.Object)
				plog.Error(err.Error(), log.Error(err))
				continue
			}

			p.processPodEvent(event, pod)
		}
	}
}

func (p *Provider) processPodEvent(event watch.Event, pod *v1.Pod) {
	plog.Debug("Watcher reported event for pod", log.Object("eventType", event.Type), log.String("podName", pod.ObjectMeta.Name))

	podClusterName, hasClusterName := pod.ObjectMeta.Labels[LabelCluster]
	if !hasClusterName {
		plog.Info("The pod is not a cluster member", log.Object("podName", pod.ObjectMeta.Name))
		delete(p.clusterPods, pod.UID) // pod could have been in the cluster, but then it was deregistered
	} else if podClusterName != p.clusterName {
		plog.Info("The pod is a member of another cluster", log.Object("podName", pod.ObjectMeta.Name), log.String("otherCluster", podClusterName))
		return
	} else {
		switch event.Type {
		case watch.Deleted:
			delete(p.clusterPods, pod.UID)
		case watch.Error:
			err := apierrors.FromObject(event.Object)
			plog.Error(err.Error(), log.Error(err))
		default:
			p.clusterPods[pod.UID] = pod
		}
	}

	if plog.Level() == log.DebugLevel {
		logCurrentPods(p.clusterPods)
	}

	members := mapPodsToMembers(p.clusterPods)

	plog.Info("Topology received from Kubernetes", log.Object("members", members))
	p.cluster.MemberList.UpdateClusterTopology(members)
}

func logCurrentPods(clusterPods map[types.UID]*v1.Pod) {
	podNames := make([]string, 0, len(clusterPods))
	for _, pod := range clusterPods {
		podNames = append(podNames, pod.ObjectMeta.Name)
	}
	plog.Debug("Detected cluster pods are now", log.Int("numberOfPods", len(clusterPods)), log.Object("podNames", podNames))
}

func mapPodsToMembers(clusterPods map[types.UID]*v1.Pod) []*cluster.Member {
	members := make([]*cluster.Member, 0, len(clusterPods))
	for _, clusterPod := range clusterPods {
		if clusterPod.Status.Phase == "Running" && len(clusterPod.Status.PodIPs) > 0 {

			var kinds []string
			for key, value := range clusterPod.ObjectMeta.Labels {
				if strings.HasPrefix(key, LabelKind) && value == "true" {
					kinds = append(kinds, strings.Replace(key, fmt.Sprintf("%s-", LabelKind), "", 1))
				}
			}

			host := clusterPod.Status.PodIP
			port, err := strconv.Atoi(clusterPod.ObjectMeta.Labels[LabelPort])
			if err != nil {
				err = fmt.Errorf("can not convert pod meta %s into integer: %w", LabelPort, err)
				plog.Error(err.Error(), log.Error(err))
				continue
			}

			mid := clusterPod.ObjectMeta.Labels[LabelMemberID]
			alive := true
			for _, status := range clusterPod.Status.ContainerStatuses {
				if !status.Ready {
					plog.Debug("Pod container is not ready", log.String("podName", clusterPod.ObjectMeta.Name), log.String("containerName", status.Name))
					alive = false
					break
				}
			}

			if !alive {
				continue
			}

			plog.Debug("Pod is running and all containers are ready", log.String("podName", clusterPod.ObjectMeta.Name), log.Object("podIPs", clusterPod.Status.PodIPs), log.String("podPhase", string(clusterPod.Status.Phase)))

			members = append(members, &cluster.Member{
				Id:    mid,
				Host:  host,
				Port:  int32(port),
				Kinds: kinds,
			})
		} else {
			plog.Debug("Pod is not in Running state", log.String("podName", clusterPod.ObjectMeta.Name), log.Object("podIPs", clusterPod.Status.PodIPs), log.String("podPhase", string(clusterPod.Status.Phase)))
		}
	}

	return members
}

// deregister itself as a member from a k8s cluster
func (p *Provider) deregisterMember(timeout time.Duration) error {
	plog.Info(fmt.Sprintf("Deregistering service %s from %s", p.podName, p.address))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pod, err := p.client.CoreV1().Pods(p.retrieveNamespace()).Get(ctx, p.podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("unable to get own pod information for %s: %w", p.podName, err)
	}

	labels := pod.GetLabels()

	for labelKey := range labels {
		if strings.HasPrefix(labelKey, LabelPrefix) {
			delete(labels, labelKey)
		}
	}

	pod.SetLabels(labels)

	return p.replacePodLabels(ctx, pod)
}

// prepares a patching payload and sends it to kubernetes to replace labels
func (p *Provider) replacePodLabels(ctx context.Context, pod *v1.Pod) error {
	plog.Debug("Setting pod labels to ", log.Object("labels", pod.GetLabels()))

	payload := []struct {
		Op    string `json:"op"`
		Path  string `json:"path"`
		Value Labels `json:"value"`
	}{
		{
			Op:    "replace",
			Path:  "/metadata/labels",
			Value: pod.GetLabels(),
		},
	}

	payloadData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("unable to update pod labels, operation failed: %w", err)
	}

	_, patcherr := p.client.CoreV1().Pods(pod.GetNamespace()).Patch(ctx, pod.GetName(), types.JSONPatchType, payloadData, metav1.PatchOptions{})
	return patcherr
}

// get the namespace of the current pod
func (p *Provider) retrieveNamespace() string {
	if (p.namespace) == "" {
		filename := filepath.Join(string(filepath.Separator), "var", "run", "secrets", "kubernetes.io", "serviceaccount", "namespace")
		content, err := os.ReadFile(filename)
		if err != nil {
			plog.Warn(fmt.Sprintf("Could not read %s contents defaulting to empty namespace: %s", filename, err.Error()))
			return p.namespace
		}
		p.namespace = string(content)
	}

	return p.namespace
}
