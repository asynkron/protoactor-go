package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
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

	p.cluster.Logger().Info("Shutting down k8s cluster provider")
	if p.clusterMonitor != nil {
		if err := p.cluster.ActorSystem.Root.RequestFuture(p.clusterMonitor, &DeregisterMember{}, 5*time.Second).Wait(); err != nil {
			p.cluster.Logger().Error("Failed to deregister member - cluster monitor did not respond, proceeding with shutdown", slog.Any("error", err))
		}

		if err := p.cluster.ActorSystem.Root.RequestFuture(p.clusterMonitor, &StopWatchingCluster{}, 5*time.Second).Wait(); err != nil {
			p.cluster.Logger().Error("Failed to deregister member - cluster monitor did not respond, proceeding with shutdown", slog.Any("error", err))
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
		p.cluster.Logger().Error("Failed to start k8s-cluster-monitor actor", slog.Any("error", err))
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
	p.cluster.Logger().Info("Registering service in Kubernetes", slog.String("podName", p.podName), slog.String("address", p.address))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pod, err := p.client.CoreV1().Pods(p.retrieveNamespace()).Get(ctx, p.podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("unable to get own pod information for %s: %w", p.podName, err)
	}

	p.cluster.Logger().Info("Using Kubernetes namespace", slog.String("namespace", pod.Namespace), slog.Int("port", p.port))

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

	p.cluster.Logger().Debug("Starting to watch pods", slog.String("selector", selector))

	ctx, cancel := context.WithCancel(context.Background())
	p.cancelWatch = cancel

	// start a new goroutine to monitor the cluster events
	go func() {
		for {
			select {
			case <-ctx.Done():
				p.cluster.Logger().Debug("Stopping watch on pods")
				return
			default:
				if err := p.watchPods(ctx, selector); err != nil {
					p.cluster.Logger().Error("Error watching pods, will retry", slog.Any("error", err))
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
		p.cluster.Logger().Error(err.Error(), slog.Any("error", err))
		return err
	}

	p.cluster.Logger().Info("Pod watcher started")

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
				p.cluster.Logger().Error(err.Error(), slog.Any("error", err))
				continue
			}

			p.processPodEvent(event, pod)
		}
	}
}

func (p *Provider) processPodEvent(event watch.Event, pod *v1.Pod) {
	p.cluster.Logger().Debug("Watcher reported event for pod", slog.Any("eventType", event.Type), slog.String("podName", pod.ObjectMeta.Name))

	podClusterName, hasClusterName := pod.ObjectMeta.Labels[LabelCluster]
	if !hasClusterName {
		p.cluster.Logger().Info("The pod is not a cluster member", slog.Any("podName", pod.ObjectMeta.Name))
		delete(p.clusterPods, pod.UID) // pod could have been in the cluster, but then it was deregistered
	} else if podClusterName != p.clusterName {
		p.cluster.Logger().Info("The pod is a member of another cluster", slog.Any("podName", pod.ObjectMeta.Name), slog.String("otherCluster", podClusterName))
		return
	} else {
		switch event.Type {
		case watch.Deleted:
			delete(p.clusterPods, pod.UID)
		case watch.Error:
			err := apierrors.FromObject(event.Object)
			p.cluster.Logger().Error(err.Error(), slog.Any("error", err))
		default:
			p.clusterPods[pod.UID] = pod
		}
	}

	if p.cluster.Logger().Enabled(nil, slog.LevelDebug) {
		logCurrentPods(p.clusterPods, p.cluster.Logger())
	}

	members := mapPodsToMembers(p.clusterPods, p.cluster.Logger())

	p.cluster.Logger().Info("Topology received from Kubernetes", slog.Any("members", members))
	p.cluster.MemberList.UpdateClusterTopology(members)
}

func logCurrentPods(clusterPods map[types.UID]*v1.Pod, logger *slog.Logger) {
	podNames := make([]string, 0, len(clusterPods))
	for _, pod := range clusterPods {
		podNames = append(podNames, pod.ObjectMeta.Name)
	}
	logger.Debug("Detected cluster pods are now", slog.Int("numberOfPods", len(clusterPods)), slog.Any("podNames", podNames))
}

func mapPodsToMembers(clusterPods map[types.UID]*v1.Pod, logger *slog.Logger) []*cluster.Member {
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
				logger.Error(err.Error(), slog.Any("error", err))
				continue
			}

			mid := clusterPod.ObjectMeta.Labels[LabelMemberID]
			alive := true
			for _, status := range clusterPod.Status.ContainerStatuses {
				if !status.Ready {
					logger.Debug("Pod container is not ready", slog.String("podName", clusterPod.ObjectMeta.Name), slog.String("containerName", status.Name))
					alive = false
					break
				}
			}

			if !alive {
				continue
			}

			logger.Debug("Pod is running and all containers are ready", slog.String("podName", clusterPod.ObjectMeta.Name), slog.Any("podIPs", clusterPod.Status.PodIPs), slog.String("podPhase", string(clusterPod.Status.Phase)))

			members = append(members, &cluster.Member{
				Id:    mid,
				Host:  host,
				Port:  int32(port),
				Kinds: kinds,
			})
		} else {
			logger.Debug("Pod is not in Running state", slog.String("podName", clusterPod.ObjectMeta.Name), slog.Any("podIPs", clusterPod.Status.PodIPs), slog.String("podPhase", string(clusterPod.Status.Phase)))
		}
	}

	return members
}

// deregister itself as a member from a k8s cluster
func (p *Provider) deregisterMember(timeout time.Duration) error {
	p.cluster.Logger().Info("Deregistering service from Kubernetes", slog.String("podName", p.podName), slog.String("address", p.address))

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
	p.cluster.Logger().Debug("Setting pod labels to ", slog.Any("labels", pod.GetLabels()))

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
			p.cluster.Logger().Warn("Could not read contents, defaulting to empty namespace", slog.String("filename", filename), slog.Any("error", err))
			return p.namespace
		}
		p.namespace = string(content)
	}

	return p.namespace
}
