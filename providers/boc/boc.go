package boc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"time"

	"github.com/cpuguy83/strongerrors"
	"github.com/cpuguy83/strongerrors/status/ocstatus"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"

	"github.com/virtual-kubelet/virtual-kubelet/providers"
)

const (
	// Provider configuration defaults.
	defaultCPUCapacity    = "20"
	defaultMemoryCapacity = "100Gi"
	defaultPodCapacity    = "20"

	// Values used in tracing as attribute keys.
	namespaceKey     = "namespace"
	nameKey          = "name"
	containerNameKey = "containerName"
)

// BocProvider implements the virtual-kubelet provider interface and stores pods in memory.
type BocProvider struct {
	nodeName           string
	operatingSystem    string
	internalIP         string
	daemonEndpointPort int32
	pods               map[string]*v1.Pod
	config             BocConfig
	startTime          time.Time
}

// BocConfig contains a Boc virtual-kubelet's configurable parameters.
type BocConfig struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
	Pods   string `json:"pods,omitempty"`
}



func (p *BocProvider)CreatePod(ctx context.Context, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "CreatePod")
	defer span.End()

	// Add the pod's coordinates to the current span.
	ctx = addAttributes(ctx, span, namespaceKey, pod.Namespace, nameKey, pod.Name)

	log.G(ctx).Info("receive CreatePod %q", pod.Name)

	key, err := buildKey(pod)
	if err != nil {
		return err
	}

	p.pods[key] = pod

	return nil
}

// UpdatePod takes a Kubernetes Pod and updates it within the provider.
func (p *BocProvider)UpdatePod(ctx context.Context, pod *corev1.Pod) error{
	ctx, span := trace.StartSpan(ctx, "UpdatePod")
	defer span.End()

	// Add the pod's coordinates to the current span.
	ctx = addAttributes(ctx, span, namespaceKey, pod.Namespace, nameKey, pod.Name)

	log.G(ctx).Info("receive UpdatePod %q", pod.Name)

	key, err := buildKey(pod)
	if err != nil {
		return err
	}

	p.pods[key] = pod

	return nil
}

// DeletePod takes a Kubernetes Pod and deletes it from the provider. Once a pod is deleted, the provider is
// expected to call the NotifyPods callback with a terminal pod status where all the containers are in a terminal
// state, as well as the pod. DeletePod may be called multiple times for the same pod.
func (p *BocProvider)DeletePod(ctx context.Context, pod *corev1.Pod) error{
	ctx, span := trace.StartPan(ctx, "DeletePod")
	defer span.End()

	ctx = addAttributes(ctx, span, namespaceKey, pod.Namespace, nameKey, pod.Name)
	log.G(ctx).Info("receive DeltePod %q", pod.Name)

	key, err := buildKey(pod)
	if err != nil {
		return err
	}
	if _, exists := p.pods[key]; !exists {
		return strongerrors.NotFound(fmt.Errorf("pod not found"))
	}

	delete(p.pods, key)
	return nil
}

// GetPod retrieves a pod by name from the provider (can be cached).
// The Pod returned is expected to be immutable, and may be accessed
// concurrently outside of the calling goroutine. Therefore it is recommended
// to return a version after DeepCopy.
func (p *BocProvider)GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error){
	ctx, span := trace.StartPan(ctx, name)
	defer span.End()

	ctx = addAttributes(ctx, span, namespaceKey, pod.Namespace, nameKey, pod.Name)
	log.G(ctx).Info("receive GetPod %q", pod.name)

	key, err := buildKey(pod)
	if err != nil {
		return err
	}

	if pod, ok := p.pods[key]; ok {
		return pod, nil
	}

	return nil, strongerrors.NotFound(fmt.Errorf("pod \"%s/%s\" is not known to the provider", namespace, name))
}

// GetPodStatus retrieves the status of a pod by name from the provider.
// The PodStatus returned is expected to be immutable, and may be accessed
// concurrently outside of the calling goroutine. Therefore it is recommended
// to return a version after DeepCopy.
func (p *BocProvider)GetPodStatus(ctx context.Context, namespace, name string) (*corev1.PodStatus, error){
	ctx, span := trace.StartSpan(ctx, "GetPodStatus")
	defer span.End()

	// Add namespace and name as attributes to the current span.
	ctx = addAttributes(ctx, span, namespaceKey, namespace, nameKey, name)

	log.G(ctx).Info("receive GetPodStatus %q", name)

	now := metav1.NewTime(time.Now())

	status := &v1.PodStatus{
		Phase:     v1.PodRunning,
		HostIP:    "1.2.3.4",
		PodIP:     "5.6.7.8",
		StartTime: &now,
		Conditions: []v1.PodCondition{
			{
				Type:   v1.PodInitialized,
				Status: v1.ConditionTrue,
			},
			{
				Type:   v1.PodReady,
				Status: v1.ConditionTrue,
			},
			{
				Type:   v1.PodScheduled,
				Status: v1.ConditionTrue,
			},
		},
	}

	pod, err := p.GetPod(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	for _, container := range pod.Spec.Containers {
		status.ContainerStatuses = append(status.ContainerStatuses, v1.ContainerStatus{
			Name:         container.Name,
			Image:        container.Image,
			Ready:        true,
			RestartCount: 0,
			State: v1.ContainerState{
				Running: &v1.ContainerStateRunning{
					StartedAt: now,
				},
			},
		})
	}

	return status, nil
}

// GetPods retrieves a list of all pods running on the provider (can be cached).
// The Pods returned are expected to be immutable, and may be accessed
// concurrently outside of the calling goroutine. Therefore it is recommended
// to return a version after DeepCopy.
func (p *BocProvider)GetPods(context.Context) ([]*corev1.Pod, error){
	ctx, span := trace.StartPan(ctx, "GetPods")
	defer span.End()

	log.G(ctx).Info("receive GetPods")

	pods := make([]*corev1.pod, 0, len(p.pods))
	for _, pod := range p.pods {
		pods = append(pods, pod)
	}
	return pods, nil
}



func (p *BocProvider) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, opts api.ContainerLogOpts) (io.ReadCloser, error) {
	ctx, span := trace.StartSpan(ctx, "GetContainerLogs")
	defer span.End()

	// Add pod and container attributes to the current span.
	ctx = addAttributes(ctx, span, namespaceKey, namespace, nameKey, podName, containerNameKey, containerName)

	log.G(ctx).Info("receive GetContainerLogs %q", podName)
	return "", nil
}

func (p *BocProvider)RunInContainer(ctx context.Context, namespace, podName, containerName string, cmd []string, attach api.AttachIO) error {
	ctx, span := trace.StartSpan(ctx, "RunInContainer")
	defer span.End()

	// Add pod and container attributes to the current span.
	ctx = addAttributes(ctx, span, namespaceKey, namespace, nameKey, podName, containerNameKey, containerName)

	log.G(ctx).Info("receive RunInContainer %q", podName)
	return "", nil
}

func (p *BocProvider)ConfigureNode(context.Context, *v1.Node) {
	ctx, span := trace.StartSpan(ctx, "boc.ConfigureNode") //nolint:ineffassign
	defer span.End()

	p.Status.Capacity = p.capacity()
	p.Status.Allocatable = p.capacity()
	p.Status.Conditions = p.nodeConditions()
	p.Status.Addresses = p.nodeAddresses()
	p.Status.DaemonEndpoints = p.nodeDaemonEndpoints()
	os := p.operatingSystem
	if os == "" {
		os = "Linux"
	}
	p.Status.NodeInfo.OperatingSystem = os
	p.Status.NodeInfo.Architecture = "amd64"
	n.ObjectMeta.Labels["alpha.service-controller.kubernetes.io/exclude-balancer"] = "true"
}

// Capacity returns a resource list containing the capacity limits.
func (p *BocProvider) capacity() v1.ResourceList {
	return v1.ResourceList{
		"cpu":    resource.MustParse(p.config.CPU),
		"memory": resource.MustParse(p.config.Memory),
		"pods":   resource.MustParse(p.config.Pods),
	}
}


// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), for updates to the node status
// within Kubernetes.
func (p *BocProvider) nodeConditions() []v1.NodeCondition {
	// TODO: Make this configurable
	return []v1.NodeCondition{
		{
			Type:               "Ready",
			Status:             v1.ConditionTrue,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletReady",
			Message:            "kubelet is ready.",
		},
		{
			Type:               "OutOfDisk",
			Status:             v1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletHasSufficientDisk",
			Message:            "kubelet has sufficient disk space available",
		},
		{
			Type:               "MemoryPressure",
			Status:             v1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletHasSufficientMemory",
			Message:            "kubelet has sufficient memory available",
		},
		{
			Type:               "DiskPressure",
			Status:             v1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletHasNoDiskPressure",
			Message:            "kubelet has no disk pressure",
		},
		{
			Type:               "NetworkUnavailable",
			Status:             v1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "RouteCreated",
			Message:            "RouteController created a route",
		},
	}

}

// NodeAddresses returns a list of addresses for the node status
// within Kubernetes.
func (p *MockProvider) nodeAddresses() []v1.NodeAddress {
	return []v1.NodeAddress{
		{
			Type:    "InternalIP",
			Address: p.internalIP,
		},
	}
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
// within Kubernetes.
func (p *MockProvider) NodeDaemonEndpoints(ctx context.Context) *v1.NodeDaemonEndpoints {
	ctx, span := trace.StartSpan(ctx, "NodeDaemonEndpoints")
	defer span.End()

	return &v1.NodeDaemonEndpoints{
		KubeletEndpoint: v1.DaemonEndpoint{
			Port: p.daemonEndpointPort,
		},
	}
}