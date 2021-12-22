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

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/google/uuid"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	watchTimeoutSeconds       int64 = 30
	ProviderShuttingDownError       = fmt.Errorf("kubernetes cluster provider is being shut down")
)

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
	watching       bool
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
	if p.clusterMonitor != nil {
		if err := p.cluster.ActorSystem.Root.StopFuture(p.clusterMonitor).Wait(); err != nil {
			plog.Error("Failed to stop kubernetes-provider actor", log.Error(err))
		}
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
		return fmt.Errorf("Unable to get own pod information for %s: %w", p.podName, err)
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

func (p *Provider) startWatchingCluster(timeout time.Duration) error {

	selector := fmt.Sprintf("%s=%s", LabelCluster, p.clusterName)
	if p.watching {
		plog.Info(fmt.Sprintf("Pods for %s are being watched already", selector))
	}

	plog.Debug(fmt.Sprintf("Starting to watch pods with %s", selector), log.String("selector", selector))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	watcher, err := p.client.CoreV1().Pods(p.retrieveNamespace()).Watch(ctx, metav1.ListOptions{LabelSelector: selector, Watch: true, TimeoutSeconds: &watchTimeoutSeconds})
	if err != nil {
		return fmt.Errorf("Unable to watch the cluster status: %w", err)
	}

	// start a new goroutine to monitor the cluster events
	go func() {

		for !p.shutdown {

			event, ok := <-watcher.ResultChan()
			if !ok {
				plog.Error("watcher result channel closed abruptly")
				break
			}

			pod, ok := event.Object.(*v1.Pod)
			if !ok {
				err := fmt.Errorf("could not cast %#v[%T] into v1.Pod", event.Object, event.Object)
				plog.Error(err.Error(), log.Error(err))
				continue
			}

			podClusterName, ok := pod.ObjectMeta.Labels[LabelCluster]
			if !ok {
				plog.Info(fmt.Sprintf("The pod %s is not a Proto.Cluster node", pod.ObjectMeta.Name))
			}

			if podClusterName != p.clusterName {
				plog.Info(fmt.Sprintf("The pod %s is from another cluster %s", pod.ObjectMeta.Name, pod.ObjectMeta.Namespace))
			}

			switch event.Type {
			case watch.Deleted:
				delete(p.clusterPods, pod.UID)
			case watch.Error:
				err := apierrors.FromObject(event.Object)
				plog.Error(err.Error(), log.Error(err))
			default:
				p.clusterPods[pod.UID] = pod
			}

			members := make([]*cluster.Member, 0, len(p.clusterPods))
			for _, clusterPod := range p.clusterPods {
				if clusterPod.Status.Phase == "Running" && len(clusterPod.Status.PodIPs) > 0 {

					kinds := []string{}
					for key, value := range clusterPod.ObjectMeta.Labels {
						if strings.HasPrefix(key, LabelKind) && value == "true" {
							kinds = append(kinds, strings.Replace(key, fmt.Sprintf("%s-", LabelKind), "", 1))
						}
					}

					host := pod.Status.PodIP
					port, err := strconv.Atoi(pod.ObjectMeta.Labels[LabelPort])
					if err != nil {
						err = fmt.Errorf("can not convert pod meta %s into integer: %w", LabelPort, err)
						plog.Error(err.Error(), log.Error(err))
						continue
					}

					mid := pod.ObjectMeta.Labels[LabelMemberID]
					alive := true
					for _, status := range pod.Status.ContainerStatuses {
						if !status.Ready {
							alive = false
							break
						}
					}

					if !alive {
						continue
					}

					members = append(members, &cluster.Member{
						Id:    mid,
						Host:  host,
						Port:  int32(port),
						Kinds: kinds,
					})
				}
			}

			plog.Debug(fmt.Sprintf("Topology received from Kubernetes %#v", members))
			p.cluster.MemberList.UpdateClusterTopology(members)
		}
	}()

	return nil
}

// deregister itself as a member from a k8s cluster
func (p *Provider) deregisterMember(timeout time.Duration) error {

	plog.Info(fmt.Sprintf("Deregistering service %s from %s", p.podName, p.address))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pod, err := p.client.CoreV1().Pods(p.retrieveNamespace()).Get(ctx, p.podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Unable to get own pod information for %s: %w", p.podName, err)
	}

	labels := pod.GetLabels()
	for _, kind := range p.knownKinds {
		delete(labels, fmt.Sprintf("%s-%s", LabelKind, kind))
	}

	delete(labels, LabelCluster)
	pod.SetLabels(labels)

	return p.replacePodLabels(ctx, pod)
}

// prepares a patching payload and sends it to kubernetes to replace labels
func (p *Provider) replacePodLabels(ctx context.Context, pod *v1.Pod) error {

	payload := struct {
		Op    string `json:"op"`
		Path  string `json:"path"`
		Value Labels `json:"value"`
	}{
		Op:    "replace",
		Path:  "/metadata/labels",
		Value: pod.GetLabels(),
	}

	payloadData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("Unable to update pod labels, operation failed: %w", err)
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
