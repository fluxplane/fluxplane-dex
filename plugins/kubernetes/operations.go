package kubernetes

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Service struct {
	Contexts     func() (ClusterListResult, error)
	ClusterProbe func(context.Context, ClusterTestInput) (ClusterTestResult, error)
	Namespaces   func(context.Context, InventoryInput) ([]corev1.Namespace, error)
	Services     func(context.Context, EndpointDiscoverInput) ([]corev1.Service, error)
	Pods         func(context.Context, InventoryInput) ([]corev1.Pod, error)
	Deployments  func(context.Context, InventoryInput) ([]appsv1.Deployment, error)
	Logs         func(context.Context, PodLogsInput) (PodLogsResult, error)
	ForwardStart func(context.Context, PortForwardStartInput) (PortForwardResult, error)
	ForwardStop  func(context.Context, PortForwardStopInput) (PortForwardStopResult, error)
	Secrets      func(context.Context, EndpointDiscoverInput) ([]corev1.Secret, error)
}

func NewService() Service {
	return Service{}
}

type ClusterListInput struct{}

type ClusterListResult struct {
	Contexts []ClusterContext `json:"contexts"`
}

type ClusterContext struct {
	Name    string `json:"name"`
	Current bool   `json:"current,omitempty"`
	Cluster string `json:"cluster,omitempty"`
	User    string `json:"user,omitempty"`
}

type ClusterTestInput struct {
	EndpointRef string `json:"endpoint_ref,omitempty" jsonschema:"description=Registered endpoint ref resolved by the host."`
	URL         string `json:"url,omitempty" jsonschema:"description=Kubernetes endpoint URL, usually kubernetes://context/<escaped-context>."`
	Context     string `json:"context,omitempty" jsonschema:"description=Kubeconfig context override."`
}

type ClusterTestResult struct {
	Context       string `json:"context,omitempty"`
	OK            bool   `json:"ok"`
	ServerVersion string `json:"server_version,omitempty"`
	Platform      string `json:"platform,omitempty"`
	DurationMS    int64  `json:"duration_ms,omitempty"`
}

type EndpointDiscoverInput struct {
	Product   string `json:"product,omitempty" jsonschema:"description=Product to discover, for example prometheus or loki."`
	Context   string `json:"context,omitempty" jsonschema:"description=Kubeconfig context."`
	Namespace string `json:"namespace,omitempty" jsonschema:"description=Namespace to inspect. Empty means all namespaces."`
	Limit     int    `json:"limit,omitempty" jsonschema:"description=Maximum candidates."`
}

type EndpointDiscoverResult struct {
	Candidates []core.EndpointCandidate `json:"candidates"`
}

type InventoryInput struct {
	EndpointRef string `json:"endpoint_ref,omitempty" jsonschema:"description=Registered Kubernetes cluster endpoint ref resolved by the host."`
	URL         string `json:"url,omitempty" jsonschema:"description=Kubernetes endpoint URL."`
	Context     string `json:"context,omitempty" jsonschema:"description=Kubeconfig context override."`
	Namespace   string `json:"namespace,omitempty" jsonschema:"description=Namespace filter. Empty means all namespaces where supported."`
	Name        string `json:"name,omitempty" jsonschema:"description=Resource name for show operations."`
	Query       string `json:"query,omitempty" jsonschema:"description=Search query."`
	Limit       int    `json:"limit,omitempty" jsonschema:"description=Maximum records."`
}

type NamespaceRecord struct {
	pluginbinding.DatasourceRecord
	Name      string            `json:"name"`
	Status    string            `json:"status,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	CreatedAt string            `json:"created_at,omitempty"`
}

type ServiceRecord struct {
	pluginbinding.DatasourceRecord
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Type      string            `json:"type,omitempty"`
	ClusterIP string            `json:"cluster_ip,omitempty"`
	Ports     []string          `json:"ports,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	CreatedAt string            `json:"created_at,omitempty"`
}

type PodRecord struct {
	pluginbinding.DatasourceRecord
	Name       string            `json:"name"`
	Namespace  string            `json:"namespace"`
	Phase      string            `json:"phase,omitempty"`
	Node       string            `json:"node,omitempty"`
	Containers []string          `json:"containers,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	CreatedAt  string            `json:"created_at,omitempty"`
}

type ContainerRecord struct {
	pluginbinding.DatasourceRecord
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	Pod          string            `json:"pod"`
	Type         string            `json:"type,omitempty"`
	Image        string            `json:"image,omitempty"`
	ImageID      string            `json:"image_id,omitempty"`
	ContainerID  string            `json:"container_id,omitempty"`
	State        string            `json:"state,omitempty"`
	Ready        bool              `json:"ready,omitempty"`
	RestartCount int32             `json:"restart_count,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	CreatedAt    string            `json:"created_at,omitempty"`
}

type DeploymentRecord struct {
	pluginbinding.DatasourceRecord
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Replicas          int32             `json:"replicas"`
	ReadyReplicas     int32             `json:"ready_replicas"`
	AvailableReplicas int32             `json:"available_replicas"`
	UpdatedReplicas   int32             `json:"updated_replicas"`
	Strategy          string            `json:"strategy,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	CreatedAt         string            `json:"created_at,omitempty"`
}

type PodLogsInput struct {
	EndpointRef string `json:"endpoint_ref,omitempty" jsonschema:"description=Registered Kubernetes cluster endpoint ref resolved by the host."`
	URL         string `json:"url,omitempty" jsonschema:"description=Kubernetes endpoint URL."`
	Context     string `json:"context,omitempty" jsonschema:"description=Kubeconfig context override."`
	Namespace   string `json:"namespace,omitempty" jsonschema:"description=Pod namespace."`
	Name        string `json:"name,omitempty" jsonschema:"description=Pod name."`
	Container   string `json:"container,omitempty" jsonschema:"description=Container name. Empty uses Kubernetes default selection."`
	TailLines   int64  `json:"tail_lines,omitempty" jsonschema:"description=Number of trailing lines to return. Defaults to 100 only when no time or byte bound is provided; capped at 1000."`
	LimitBytes  int64  `json:"limit_bytes,omitempty" jsonschema:"description=Maximum bytes to return. Values above 1048576 are capped. This can be used without tail_lines."`
	Since       string `json:"since,omitempty" jsonschema:"description=Relative duration such as 2h or absolute RFC3339 timestamp."`
	Until       string `json:"until,omitempty" jsonschema:"description=Absolute RFC3339 timestamp upper bound; filtered client-side."`
	Previous    bool   `json:"previous,omitempty" jsonschema:"description=Return previous terminated container logs."`
	Timestamps  bool   `json:"timestamps,omitempty" jsonschema:"description=Include Kubernetes log timestamps."`
}

type PodLogsResult struct {
	Namespace  string   `json:"namespace"`
	Name       string   `json:"name"`
	Container  string   `json:"container,omitempty"`
	Lines      []string `json:"lines"`
	Text       string   `json:"text,omitempty"`
	LineCount  int      `json:"line_count"`
	TailLines  int64    `json:"tail_lines,omitempty"`
	LimitBytes int64    `json:"limit_bytes,omitempty"`
	Since      string   `json:"since,omitempty"`
	Until      string   `json:"until,omitempty"`
	Previous   bool     `json:"previous,omitempty"`
	Timestamps bool     `json:"timestamps,omitempty"`
}

type PortForwardStartInput struct {
	EndpointRef     string `json:"endpoint_ref,omitempty" jsonschema:"description=Registered Kubernetes cluster endpoint ref resolved by the host."`
	URL             string `json:"url,omitempty" jsonschema:"description=Kubernetes endpoint URL."`
	Context         string `json:"context,omitempty" jsonschema:"description=Kubeconfig context override."`
	Namespace       string `json:"namespace,omitempty" jsonschema:"description=Namespace containing the target resource."`
	Resource        string `json:"resource,omitempty" jsonschema:"description=Resource reference such as service/loki, pod/api-123, or deployment/api."`
	ResourceType    string `json:"resource_type,omitempty" jsonschema:"description=Resource type when name is used.,enum=service,enum=pod,enum=deployment"`
	Name            string `json:"name,omitempty" jsonschema:"description=Resource name when resource is not used."`
	RemotePort      int    `json:"remote_port,omitempty" jsonschema:"description=Remote service or pod port to forward."`
	LocalPort       int    `json:"local_port,omitempty" jsonschema:"description=Local port. Empty or 0 allocates an available local port."`
	Address         string `json:"address,omitempty" jsonschema:"description=Local bind address. Defaults to 127.0.0.1."`
	DurationSeconds int    `json:"duration_seconds,omitempty" jsonschema:"description=Auto-cleanup timeout in seconds. Defaults to 3600 and is capped at 28800."`
}

type PortForwardResult struct {
	ID              string    `json:"id"`
	EndpointRef     string    `json:"endpoint_ref,omitempty"`
	Context         string    `json:"context,omitempty"`
	Namespace       string    `json:"namespace"`
	Resource        string    `json:"resource"`
	Address         string    `json:"address"`
	LocalPort       int       `json:"local_port"`
	RemotePort      int       `json:"remote_port"`
	LocalURL        string    `json:"local_url"`
	PID             int       `json:"pid"`
	ProcessGroup    int       `json:"process_group,omitempty"`
	DurationSeconds int       `json:"duration_seconds"`
	ExpiresAt       time.Time `json:"expires_at"`
	LogPath         string    `json:"log_path,omitempty"`
	Command         []string  `json:"command,omitempty"`
}

type PortForwardStopInput struct {
	ID           string `json:"id,omitempty" jsonschema:"description=Managed port-forward ID returned by kubernetes.portforward.start."`
	ProcessGroup int    `json:"process_group,omitempty" jsonschema:"description=Process group to terminate when ID is unavailable."`
	PID          int    `json:"pid,omitempty" jsonschema:"description=Process ID to terminate when process_group is unavailable."`
}

type PortForwardStopResult struct {
	ID      string `json:"id,omitempty"`
	Stopped bool   `json:"stopped"`
	Signal  string `json:"signal,omitempty"`
	Error   string `json:"error,omitempty"`
}

type NamespaceListResult struct {
	Count      int               `json:"count"`
	Namespaces []NamespaceRecord `json:"namespaces"`
}

type ServiceListResult struct {
	Count    int             `json:"count"`
	Services []ServiceRecord `json:"services"`
}

type ServiceShowResult struct {
	Service ServiceRecord `json:"service"`
}

type PodListResult struct {
	Count int         `json:"count"`
	Pods  []PodRecord `json:"pods"`
}

type PodShowResult struct {
	Pod PodRecord `json:"pod"`
}

type ContainerListResult struct {
	Count      int               `json:"count"`
	Containers []ContainerRecord `json:"containers"`
}

type ContainerShowResult struct {
	Container ContainerRecord `json:"container"`
}

type DeploymentListResult struct {
	Count       int                `json:"count"`
	Deployments []DeploymentRecord `json:"deployments"`
}

type DeploymentShowResult struct {
	Deployment DeploymentRecord `json:"deployment"`
}

type InventorySearchResult = pluginbinding.DatasourceSearchResult[pluginbinding.DatasourceRecord]

func (s Service) ClusterList(ctx pluginbinding.Context, input ClusterListInput) (ClusterListResult, error) {
	result, err := s.contexts()()
	if err != nil {
		return ClusterListResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	return result, nil
}

func (s Service) ClusterTest(ctx pluginbinding.Context, input ClusterTestInput) (ClusterTestResult, error) {
	result, err := s.clusterProbe()(context.Background(), input)
	if err != nil {
		return ClusterTestResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	return result, nil
}

func (s Service) NamespaceList(ctx pluginbinding.Context, input InventoryInput) (NamespaceListResult, error) {
	items, err := s.namespaces()(context.Background(), input)
	if err != nil {
		return NamespaceListResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	records := namespaceRecords(ctx.DatasourceSource(), items)
	records = filterNamespaceRecords(records, input.Query)
	records = limitSlice(records, input.Limit)
	return NamespaceListResult{Count: len(records), Namespaces: records}, nil
}

func (s Service) ServiceList(ctx pluginbinding.Context, input InventoryInput) (ServiceListResult, error) {
	items, err := s.services()(context.Background(), endpointInputFromInventory(input))
	if err != nil {
		return ServiceListResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	records := serviceRecords(ctx.DatasourceSource(), items)
	records = filterServiceRecords(records, input.Query)
	records = limitSlice(records, input.Limit)
	return ServiceListResult{Count: len(records), Services: records}, nil
}

func (s Service) ServiceShow(ctx pluginbinding.Context, input InventoryInput) (ServiceShowResult, error) {
	input = normalizeInventoryInput(input)
	items, err := s.services()(context.Background(), endpointInputFromInventory(input))
	if err != nil {
		return ServiceShowResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	records := serviceRecords(ctx.DatasourceSource(), items)
	for _, record := range records {
		if record.Name == strings.TrimSpace(input.Name) && (input.Namespace == "" || record.Namespace == strings.TrimSpace(input.Namespace)) {
			return ServiceShowResult{Service: record}, nil
		}
	}
	return ServiceShowResult{}, pluginbinding.Errorf("not_found", "service %q not found", input.Name)
}

func (s Service) PodList(ctx pluginbinding.Context, input InventoryInput) (PodListResult, error) {
	items, err := s.pods()(context.Background(), input)
	if err != nil {
		return PodListResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	records := podRecords(ctx.DatasourceSource(), items)
	records = filterPodRecords(records, input.Query)
	records = limitSlice(records, input.Limit)
	return PodListResult{Count: len(records), Pods: records}, nil
}

func (s Service) PodShow(ctx pluginbinding.Context, input InventoryInput) (PodShowResult, error) {
	input = normalizeInventoryInput(input)
	items, err := s.pods()(context.Background(), input)
	if err != nil {
		return PodShowResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	records := podRecords(ctx.DatasourceSource(), items)
	for _, record := range records {
		if record.Name == strings.TrimSpace(input.Name) && (input.Namespace == "" || record.Namespace == strings.TrimSpace(input.Namespace)) {
			return PodShowResult{Pod: record}, nil
		}
	}
	return PodShowResult{}, pluginbinding.Errorf("not_found", "pod %q not found", input.Name)
}

func (s Service) DeploymentList(ctx pluginbinding.Context, input InventoryInput) (DeploymentListResult, error) {
	items, err := s.deployments()(context.Background(), input)
	if err != nil {
		return DeploymentListResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	records := deploymentRecords(ctx.DatasourceSource(), items)
	records = filterDeploymentRecords(records, input.Query)
	records = limitSlice(records, input.Limit)
	return DeploymentListResult{Count: len(records), Deployments: records}, nil
}

func (s Service) DeploymentShow(ctx pluginbinding.Context, input InventoryInput) (DeploymentShowResult, error) {
	input = normalizeInventoryInput(input)
	items, err := s.deployments()(context.Background(), input)
	if err != nil {
		return DeploymentShowResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	records := deploymentRecords(ctx.DatasourceSource(), items)
	for _, record := range records {
		if record.Name == strings.TrimSpace(input.Name) && (input.Namespace == "" || record.Namespace == strings.TrimSpace(input.Namespace)) {
			return DeploymentShowResult{Deployment: record}, nil
		}
	}
	return DeploymentShowResult{}, pluginbinding.Errorf("not_found", "deployment %q not found", input.Name)
}

func (s Service) PodLogs(ctx pluginbinding.Context, input PodLogsInput) (PodLogsResult, error) {
	input = normalizePodLogsInput(input)
	result, err := s.logs()(context.Background(), input)
	if err != nil {
		return PodLogsResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	return result, nil
}

func (s Service) PortForwardStart(ctx pluginbinding.Context, input PortForwardStartInput) (PortForwardResult, error) {
	result, err := s.portForwardStart()(context.Background(), input)
	if err != nil {
		return PortForwardResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	return result, nil
}

func (s Service) PortForwardStop(ctx pluginbinding.Context, input PortForwardStopInput) (PortForwardStopResult, error) {
	result, err := s.portForwardStop()(context.Background(), input)
	if err != nil {
		return PortForwardStopResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	return result, nil
}

func (s Service) ContainerList(ctx pluginbinding.Context, input InventoryInput) (ContainerListResult, error) {
	items, err := s.pods()(context.Background(), input)
	if err != nil {
		return ContainerListResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	records := containerRecords(ctx.DatasourceSource(), items)
	records = filterContainerRecords(records, input.Query)
	records = limitSlice(records, input.Limit)
	return ContainerListResult{Count: len(records), Containers: records}, nil
}

func (s Service) ContainerShow(ctx pluginbinding.Context, input InventoryInput) (ContainerShowResult, error) {
	input = normalizeInventoryInput(input)
	items, err := s.pods()(context.Background(), input)
	if err != nil {
		return ContainerShowResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	records := containerRecords(ctx.DatasourceSource(), items)
	for _, record := range records {
		if containerRecordMatches(record, input) {
			return ContainerShowResult{Container: record}, nil
		}
	}
	return ContainerShowResult{}, pluginbinding.Errorf("not_found", "container %q not found", input.Name)
}

func (s Service) InventorySearch(ctx pluginbinding.Context, input pluginbinding.DatasourceSearchInput) (InventorySearchResult, error) {
	inventoryInput := InventoryInput{
		EndpointRef: input.EndpointRef,
		URL:         input.URL,
		Query:       input.Query,
		Limit:       input.Limit,
	}
	namespaces, nsErr := s.namespaces()(context.Background(), inventoryInput)
	services, svcErr := s.services()(context.Background(), endpointInputFromInventory(inventoryInput))
	pods, podErr := s.pods()(context.Background(), inventoryInput)
	deployments, deployErr := s.deployments()(context.Background(), inventoryInput)
	var firstErr error
	for _, err := range []error{nsErr, svcErr, podErr, deployErr} {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil && len(namespaces) == 0 && len(services) == 0 && len(pods) == 0 && len(deployments) == 0 {
		return InventorySearchResult{}, pluginbinding.Errorf("kubernetes", "%s", firstErr)
	}
	records := inventoryRecords(ctx.DatasourceSource(), namespaces, services, pods, deployments)
	records = filterDatasourceRecords(records, input.Query)
	records = limitSlice(records, input.Limit)
	return pluginbinding.NewDatasourceSearchResult(PluginName, input.Query, records), nil
}

func (s Service) EndpointDiscover(ctx pluginbinding.Context, input EndpointDiscoverInput) (EndpointDiscoverResult, error) {
	if shouldDiscoverKubernetesCluster(input.Product) {
		result, err := s.contexts()()
		if err != nil {
			return EndpointDiscoverResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
		}
		return EndpointDiscoverResult{Candidates: limitCandidates(clusterEndpointCandidates(result.Contexts, input), input.Limit)}, nil
	}
	services, err := s.services()(context.Background(), input)
	if err != nil {
		return EndpointDiscoverResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	candidates := serviceCandidates(services, input)
	if shouldDiscoverSQLSecret(input.Product) {
		secrets, err := s.secrets()(context.Background(), input)
		if err != nil {
			return EndpointDiscoverResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
		}
		candidates = append(candidates, secretCandidates(secrets, input)...)
	}
	return EndpointDiscoverResult{Candidates: limitCandidates(candidates, input.Limit)}, nil
}

func (s Service) DiscoverEndpointsCommand(ctx pluginbinding.Context) protocol.Response {
	input, err := pluginbinding.DecodePayload[EndpointDiscoverInput](ctx.Request.Payload)
	if err != nil {
		return protocol.Fail("bad_payload", err.Error())
	}
	result, err := s.EndpointDiscover(ctx, input)
	if err != nil {
		var pluginErr pluginbinding.Error
		if errors.As(err, &pluginErr) {
			return protocol.Fail(pluginErr.Code, pluginErr.Message)
		}
		return protocol.Fail("kubernetes", err.Error())
	}
	return protocol.OK(result)
}

func (s Service) contexts() func() (ClusterListResult, error) {
	if s.Contexts != nil {
		return s.Contexts
	}
	return loadKubeconfigContexts
}

func (s Service) clusterProbe() func(context.Context, ClusterTestInput) (ClusterTestResult, error) {
	if s.ClusterProbe != nil {
		return s.ClusterProbe
	}
	return probeKubernetesCluster
}

func (s Service) namespaces() func(context.Context, InventoryInput) ([]corev1.Namespace, error) {
	if s.Namespaces != nil {
		return s.Namespaces
	}
	return listKubernetesNamespaces
}

func (s Service) services() func(context.Context, EndpointDiscoverInput) ([]corev1.Service, error) {
	if s.Services != nil {
		return s.Services
	}
	return listKubernetesServices
}

func (s Service) pods() func(context.Context, InventoryInput) ([]corev1.Pod, error) {
	if s.Pods != nil {
		return s.Pods
	}
	return listKubernetesPods
}

func (s Service) deployments() func(context.Context, InventoryInput) ([]appsv1.Deployment, error) {
	if s.Deployments != nil {
		return s.Deployments
	}
	return listKubernetesDeployments
}

func (s Service) logs() func(context.Context, PodLogsInput) (PodLogsResult, error) {
	if s.Logs != nil {
		return s.Logs
	}
	return readKubernetesPodLogs
}

func (s Service) portForwardStart() func(context.Context, PortForwardStartInput) (PortForwardResult, error) {
	if s.ForwardStart != nil {
		return s.ForwardStart
	}
	return startKubernetesPortForward
}

func (s Service) portForwardStop() func(context.Context, PortForwardStopInput) (PortForwardStopResult, error) {
	if s.ForwardStop != nil {
		return s.ForwardStop
	}
	return stopKubernetesPortForward
}

func (s Service) secrets() func(context.Context, EndpointDiscoverInput) ([]corev1.Secret, error) {
	if s.Secrets != nil {
		return s.Secrets
	}
	return listKubernetesSecrets
}

func loadKubeconfigContexts() (ClusterListResult, error) {
	config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return ClusterListResult{}, err
	}
	current := strings.TrimSpace(config.CurrentContext)
	contexts := make([]ClusterContext, 0, len(config.Contexts))
	for name, ctx := range config.Contexts {
		contexts = append(contexts, ClusterContext{Name: name, Current: name == current, Cluster: ctx.Cluster, User: ctx.AuthInfo})
	}
	sort.Slice(contexts, func(i, j int) bool { return contexts[i].Name < contexts[j].Name })
	return ClusterListResult{Contexts: contexts}, nil
}

func probeKubernetesCluster(ctx context.Context, input ClusterTestInput) (ClusterTestResult, error) {
	contextName := clusterContextFromTestInput(input)
	start := time.Now()
	clientset, _, err := kubernetesClientWithTimeout(EndpointDiscoverInput{Context: contextName}, 10*time.Second)
	if err != nil {
		return ClusterTestResult{}, err
	}
	version, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return ClusterTestResult{}, err
	}
	out := ClusterTestResult{Context: contextName, OK: true, DurationMS: time.Since(start).Milliseconds()}
	if version != nil {
		out.ServerVersion = version.GitVersion
		out.Platform = version.Platform
	}
	return out, nil
}

func listKubernetesNamespaces(ctx context.Context, input InventoryInput) ([]corev1.Namespace, error) {
	clientset, _, err := kubernetesClient(endpointInputFromInventory(input))
	if err != nil {
		return nil, err
	}
	list, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func listKubernetesServices(ctx context.Context, input EndpointDiscoverInput) ([]corev1.Service, error) {
	clientset, namespace, err := kubernetesClient(input)
	if err != nil {
		return nil, err
	}
	list, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func listKubernetesPods(ctx context.Context, input InventoryInput) ([]corev1.Pod, error) {
	clientset, namespace, err := kubernetesClient(endpointInputFromInventory(input))
	if err != nil {
		return nil, err
	}
	list, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func listKubernetesDeployments(ctx context.Context, input InventoryInput) ([]appsv1.Deployment, error) {
	clientset, namespace, err := kubernetesClient(endpointInputFromInventory(input))
	if err != nil {
		return nil, err
	}
	list, err := clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func readKubernetesPodLogs(ctx context.Context, input PodLogsInput) (PodLogsResult, error) {
	namespace := strings.TrimSpace(input.Namespace)
	name := strings.TrimSpace(input.Name)
	if namespace == "" {
		return PodLogsResult{}, fmt.Errorf("namespace is required")
	}
	if name == "" {
		return PodLogsResult{}, fmt.Errorf("name is required")
	}
	bounds, err := podLogBounds(input)
	if err != nil {
		return PodLogsResult{}, err
	}
	clientset, _, err := kubernetesClient(EndpointDiscoverInput{
		Context:   firstNonEmpty(input.Context, clusterContextFromEndpointURL(input.URL)),
		Namespace: namespace,
	})
	if err != nil {
		return PodLogsResult{}, err
	}
	options := &corev1.PodLogOptions{
		Container:  strings.TrimSpace(input.Container),
		Previous:   input.Previous,
		Timestamps: input.Timestamps || bounds.Until != nil,
	}
	if bounds.TailLines != nil {
		options.TailLines = bounds.TailLines
	}
	if bounds.LimitBytes != nil {
		options.LimitBytes = bounds.LimitBytes
	}
	if bounds.SinceSeconds != nil {
		options.SinceSeconds = bounds.SinceSeconds
	}
	if bounds.SinceTime != nil {
		options.SinceTime = bounds.SinceTime
	}
	raw, err := clientset.CoreV1().Pods(namespace).GetLogs(name, options).DoRaw(ctx)
	if err != nil {
		return PodLogsResult{}, err
	}
	text := filterPodLogText(strings.TrimRight(string(raw), "\n"), bounds, input.Timestamps)
	var lines []string
	if text != "" {
		lines = strings.Split(text, "\n")
	}
	return PodLogsResult{
		Namespace:  namespace,
		Name:       name,
		Container:  strings.TrimSpace(input.Container),
		Lines:      lines,
		Text:       text,
		LineCount:  len(lines),
		TailLines:  valueOrZero(bounds.TailLines),
		LimitBytes: valueOrZero(bounds.LimitBytes),
		Since:      strings.TrimSpace(input.Since),
		Until:      strings.TrimSpace(input.Until),
		Previous:   input.Previous,
		Timestamps: input.Timestamps,
	}, nil
}

type podLogBoundOptions struct {
	TailLines    *int64
	LimitBytes   *int64
	SinceSeconds *int64
	SinceTime    *metav1.Time
	Until        *time.Time
}

func podLogBounds(input PodLogsInput) (podLogBoundOptions, error) {
	var out podLogBoundOptions
	if input.TailLines > 0 {
		tailLines := input.TailLines
		if tailLines > 1000 {
			tailLines = 1000
		}
		out.TailLines = &tailLines
	}
	if input.LimitBytes > 0 {
		limitBytes := input.LimitBytes
		if limitBytes > 1024*1024 {
			limitBytes = 1024 * 1024
		}
		out.LimitBytes = &limitBytes
	}
	if out.TailLines == nil && out.LimitBytes == nil && strings.TrimSpace(input.Since) == "" && strings.TrimSpace(input.Until) == "" {
		tailLines := int64(100)
		out.TailLines = &tailLines
	}
	if since := strings.TrimSpace(input.Since); since != "" {
		if duration, err := time.ParseDuration(since); err == nil {
			seconds := int64(duration.Seconds())
			if seconds < 1 {
				seconds = 1
			}
			out.SinceSeconds = &seconds
		} else {
			parsed, parseErr := time.Parse(time.RFC3339, since)
			if parseErr != nil {
				return out, fmt.Errorf("since must be a duration or RFC3339 timestamp")
			}
			out.SinceTime = &metav1.Time{Time: parsed}
		}
	}
	if until := strings.TrimSpace(input.Until); until != "" {
		parsed, err := time.Parse(time.RFC3339, until)
		if err != nil {
			return out, fmt.Errorf("until must be an RFC3339 timestamp")
		}
		out.Until = &parsed
	}
	return out, nil
}

func filterPodLogText(text string, bounds podLogBoundOptions, keepTimestamps bool) string {
	if text == "" || bounds.Until == nil {
		return text
	}
	var out []string
	for _, line := range strings.Split(text, "\n") {
		timestamp, rest, ok := splitKubernetesLogTimestamp(line)
		if !ok {
			out = append(out, line)
			continue
		}
		if timestamp.After(*bounds.Until) {
			continue
		}
		if keepTimestamps {
			out = append(out, line)
		} else {
			out = append(out, rest)
		}
	}
	return strings.Join(out, "\n")
}

func splitKubernetesLogTimestamp(line string) (time.Time, string, bool) {
	head, rest, ok := strings.Cut(line, " ")
	if !ok {
		return time.Time{}, "", false
	}
	timestamp, err := time.Parse(time.RFC3339Nano, head)
	if err != nil {
		return time.Time{}, "", false
	}
	return timestamp, rest, true
}

func valueOrZero(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func listKubernetesSecrets(ctx context.Context, input EndpointDiscoverInput) ([]corev1.Secret, error) {
	clientset, namespace, err := kubernetesClient(input)
	if err != nil {
		return nil, err
	}
	list, err := clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func startKubernetesPortForward(ctx context.Context, input PortForwardStartInput) (PortForwardResult, error) {
	namespace := strings.TrimSpace(input.Namespace)
	if namespace == "" {
		return PortForwardResult{}, fmt.Errorf("namespace is required")
	}
	resource := normalizedPortForwardResource(input)
	if resource == "" {
		return PortForwardResult{}, fmt.Errorf("resource or name is required")
	}
	remotePort := input.RemotePort
	if remotePort <= 0 {
		return PortForwardResult{}, fmt.Errorf("remote_port is required")
	}
	localPort := input.LocalPort
	if localPort <= 0 {
		port, err := availableLocalPort()
		if err != nil {
			return PortForwardResult{}, err
		}
		localPort = port
	}
	address := strings.TrimSpace(input.Address)
	if address == "" {
		address = "127.0.0.1"
	}
	duration := input.DurationSeconds
	if duration <= 0 {
		duration = 3600
	}
	if duration > 8*3600 {
		duration = 8 * 3600
	}
	contextName := firstNonEmpty(input.Context, clusterContextFromEndpointURL(input.URL))
	id := portForwardID(namespace, resource, localPort, remotePort)
	dir, err := portForwardStateDir()
	if err != nil {
		return PortForwardResult{}, err
	}
	logPath := filepath.Join(dir, id+".log")
	recordPath := filepath.Join(dir, id+".json")
	portArg := strconv.Itoa(localPort) + ":" + strconv.Itoa(remotePort)
	args := []string{}
	if contextName != "" {
		args = append(args, "--context", contextName)
	}
	args = append(args, "-n", namespace, "port-forward", resource, portArg, "--address", address)
	shell := "kubectl " + shellJoin(args) + " >>" + shellQuote(logPath) + " 2>&1 & child=$!; (sleep " + strconv.Itoa(duration) + "; kill $child >/dev/null 2>&1) & wait $child"
	cmd := exec.CommandContext(ctx, "sh", "-c", shell)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return PortForwardResult{}, err
	}
	if err := waitForPortForward(address, localPort, cmd.Process.Pid, logPath); err != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		return PortForwardResult{}, err
	}
	result := PortForwardResult{
		ID:              id,
		EndpointRef:     strings.TrimSpace(input.EndpointRef),
		Context:         contextName,
		Namespace:       namespace,
		Resource:        resource,
		Address:         address,
		LocalPort:       localPort,
		RemotePort:      remotePort,
		LocalURL:        "http://" + net.JoinHostPort(address, strconv.Itoa(localPort)),
		PID:             cmd.Process.Pid,
		ProcessGroup:    cmd.Process.Pid,
		DurationSeconds: duration,
		ExpiresAt:       time.Now().UTC().Add(time.Duration(duration) * time.Second),
		LogPath:         logPath,
		Command:         append([]string{"kubectl"}, args...),
	}
	if err := writePortForwardRecord(recordPath, result); err != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		return PortForwardResult{}, err
	}
	return result, nil
}

func waitForPortForward(address string, port, pid int, logPath string) error {
	host := strings.TrimSpace(address)
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	target := net.JoinHostPort(host, strconv.Itoa(port))
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", target, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if processExited(pid) {
			return fmt.Errorf("kubectl port-forward exited before %s became reachable: %s", target, tailFile(logPath, 2048))
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("kubectl port-forward did not become reachable at %s: %s", target, tailFile(logPath, 2048))
}

func processExited(pid int) bool {
	if pid <= 0 {
		return true
	}
	err := syscall.Kill(pid, 0)
	return errors.Is(err, syscall.ESRCH)
}

func tailFile(path string, max int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if max > 0 && len(data) > max {
		data = data[len(data)-max:]
	}
	return strings.TrimSpace(string(data))
}

func stopKubernetesPortForward(_ context.Context, input PortForwardStopInput) (PortForwardStopResult, error) {
	result := PortForwardStopResult{ID: strings.TrimSpace(input.ID)}
	processGroup := input.ProcessGroup
	pid := input.PID
	if result.ID != "" {
		record, err := readPortForwardRecord(result.ID)
		if err != nil {
			return result, err
		}
		if processGroup <= 0 {
			processGroup = record.ProcessGroup
		}
		if pid <= 0 {
			pid = record.PID
		}
	}
	if processGroup > 0 {
		if err := syscall.Kill(-processGroup, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
			result.Error = err.Error()
			return result, err
		}
		result.Stopped = true
		result.Signal = "SIGTERM"
		return result, nil
	}
	if pid > 0 {
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			result.Error = err.Error()
			return result, err
		}
		result.Stopped = true
		result.Signal = "SIGTERM"
		return result, nil
	}
	return result, fmt.Errorf("id, process_group, or pid is required")
}

func normalizedPortForwardResource(input PortForwardStartInput) string {
	resource := strings.TrimSpace(input.Resource)
	if resource != "" {
		return resource
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return ""
	}
	resourceType := strings.Trim(strings.ToLower(strings.TrimSpace(input.ResourceType)), "/")
	if resourceType == "" {
		resourceType = "service"
	}
	return resourceType + "/" + name
}

func availableLocalPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("could not allocate local TCP port")
	}
	return addr.Port, nil
}

func portForwardID(namespace, resource string, localPort, remotePort int) string {
	sum := sha1.Sum([]byte(namespace + "\x00" + resource + "\x00" + strconv.Itoa(localPort) + "\x00" + strconv.Itoa(remotePort) + "\x00" + strconv.FormatInt(time.Now().UnixNano(), 10)))
	return "kpf-" + hex.EncodeToString(sum[:6])
}

func portForwardStateDir() (string, error) {
	root := filepath.Join(os.TempDir(), "fluxplane-dex", "kubernetes", "portforwards")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	return root, nil
}

func writePortForwardRecord(path string, result PortForwardResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func readPortForwardRecord(id string) (PortForwardResult, error) {
	dir, err := portForwardStateDir()
	if err != nil {
		return PortForwardResult{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, strings.TrimSpace(id)+".json"))
	if err != nil {
		return PortForwardResult{}, err
	}
	var result PortForwardResult
	if err := json.Unmarshal(data, &result); err != nil {
		return PortForwardResult{}, err
	}
	return result, nil
}

func shellJoin(args []string) string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func kubernetesClient(input EndpointDiscoverInput) (*kubernetes.Clientset, string, error) {
	return kubernetesClientWithTimeout(input, 0)
}

func kubernetesClientWithTimeout(input EndpointDiscoverInput, timeout time.Duration) (*kubernetes.Clientset, string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	if strings.TrimSpace(input.Context) != "" {
		overrides.CurrentContext = strings.TrimSpace(input.Context)
	}
	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, "", err
	}
	if timeout > 0 {
		restConfig.Timeout = timeout
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, "", err
	}
	namespace := strings.TrimSpace(input.Namespace)
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}
	return clientset, namespace, nil
}

func shouldDiscoverKubernetesCluster(product string) bool {
	switch strings.ToLower(strings.TrimSpace(product)) {
	case "kubernetes", "k8s", "kube", "cluster":
		return true
	default:
		return false
	}
}

func kubernetesClusterEndpointURL(contextName string) string {
	return "kubernetes://context/" + url.PathEscape(strings.TrimSpace(contextName))
}

func clusterContextFromTestInput(input ClusterTestInput) string {
	if strings.TrimSpace(input.Context) != "" {
		return strings.TrimSpace(input.Context)
	}
	parsed, err := url.Parse(strings.TrimSpace(input.URL))
	if err != nil {
		return ""
	}
	if parsed.Scheme != "kubernetes" && parsed.Scheme != "k8s" {
		return ""
	}
	if parsed.Host == "context" && strings.Trim(parsed.Path, "/") != "" {
		path := strings.TrimPrefix(parsed.Path, "/")
		if value, err := url.PathUnescape(path); err == nil {
			return value
		}
		return path
	}
	return parsed.Host
}

func endpointInputFromInventory(input InventoryInput) EndpointDiscoverInput {
	return EndpointDiscoverInput{
		Context:   firstNonEmpty(input.Context, clusterContextFromEndpointURL(input.URL)),
		Namespace: input.Namespace,
		Limit:     input.Limit,
	}
}

func clusterContextFromEndpointURL(rawURL string) string {
	return clusterContextFromTestInput(ClusterTestInput{URL: rawURL})
}

func clusterEndpointCandidates(contexts []ClusterContext, input EndpointDiscoverInput) []core.EndpointCandidate {
	filter := strings.TrimSpace(input.Context)
	candidates := make([]core.EndpointCandidate, 0, len(contexts))
	for _, item := range contexts {
		if filter != "" && item.Name != filter {
			continue
		}
		endpoint := kubernetesClusterEndpointURL(item.Name)
		labels := map[string]string{"context": item.Name}
		if item.Cluster != "" {
			labels["cluster"] = item.Cluster
		}
		if item.User != "" {
			labels["user"] = item.User
		}
		if item.Current {
			labels["current"] = "true"
		}
		score := 0.8
		if item.Current {
			score = 1
		}
		candidates = append(candidates, core.EndpointCandidate{
			ID:          endpointCandidateID("kubernetes", endpoint, "", item.Name),
			URL:         endpoint,
			Product:     "kubernetes",
			Protocol:    "kubernetes",
			Source:      "kubeconfig",
			Score:       score,
			Labels:      labels,
			Annotations: map[string]string{"cluster": item.Cluster, "user": item.User},
		})
	}
	return candidates
}

func serviceCandidates(services []corev1.Service, input EndpointDiscoverInput) []core.EndpointCandidate {
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	productFilter := strings.ToLower(strings.TrimSpace(input.Product))
	var candidates []core.EndpointCandidate
	for _, item := range services {
		product, score := classifyService(item, productFilter)
		if product == "" {
			continue
		}
		for _, endpoint := range serviceURLs(item, product) {
			candidate := core.EndpointCandidate{
				ID:       endpointCandidateID(product, endpoint, item.Namespace, item.Name),
				URL:      endpoint,
				Product:  product,
				Protocol: endpointProtocol(endpoint),
				Source:   "kubernetes",
				Score:    score,
				Labels: map[string]string{
					"namespace": item.Namespace,
					"service":   item.Name,
					"type":      string(item.Spec.Type),
				},
				Annotations: cloneStringMap(item.Annotations),
			}
			if strings.TrimSpace(input.Context) != "" {
				candidate.Labels["context"] = strings.TrimSpace(input.Context)
			}
			candidates = append(candidates, candidate)
			if len(candidates) >= limit {
				return candidates
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].ID < candidates[j].ID
		}
		return candidates[i].Score > candidates[j].Score
	})
	return candidates
}

func secretCandidates(secrets []corev1.Secret, input EndpointDiscoverInput) []core.EndpointCandidate {
	var candidates []core.EndpointCandidate
	for _, secret := range secrets {
		endpoint, database, product, ok := sqlEndpointFromSecret(secret, input.Product)
		if !ok {
			continue
		}
		candidate := core.EndpointCandidate{
			ID:            endpointCandidateID(product, endpoint, secret.Namespace, secret.Name),
			URL:           endpoint,
			Product:       product,
			Protocol:      product,
			Source:        "kubernetes_secret",
			Score:         0.9,
			CredentialRef: kubernetesCredentialRef(input.Context, secret.Namespace, secret.Name),
			Labels: map[string]string{
				"namespace": secret.Namespace,
				"secret":    secret.Name,
			},
			Annotations: map[string]string{"database": database},
		}
		if strings.TrimSpace(input.Context) != "" {
			candidate.Labels["context"] = strings.TrimSpace(input.Context)
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func sqlEndpointFromSecret(secret corev1.Secret, productFilter string) (string, string, string, bool) {
	host := secretValue(secret, "host", "hostname", "endpoint", "address")
	port := secretValue(secret, "port")
	database := secretValue(secret, "database", "dbname", "db")
	username := secretValue(secret, "username", "user")
	password := secretValue(secret, "password", "pass")
	if host == "" || username == "" || password == "" {
		return "", "", "", false
	}
	haystack := strings.ToLower(secret.Name + " " + joinMap(secret.Labels) + " " + joinMap(secret.Annotations) + " " + host + " " + port)
	product := classifySQLSecretProduct(haystack, port, productFilter)
	if product == "" {
		return "", "", "", false
	}
	if port == "" {
		if product == "postgres" {
			port = "5432"
		} else {
			port = "3306"
		}
	}
	if database == "" && product == "postgres" {
		database = crossplaneSecretRole(secret.Name)
	}
	endpoint := product + "://" + host + ":" + port
	if database != "" {
		endpoint += "/" + database
	}
	if product == "postgres" {
		endpoint += "?sslmode=require"
	}
	return endpoint, database, product, true
}

func classifySQLSecretProduct(haystack, port, productFilter string) string {
	productFilter = strings.ToLower(strings.TrimSpace(productFilter))
	switch productFilter {
	case "postgres", "postgresql", "pg":
		return "postgres"
	case "mysql", "mariadb":
		return "mysql"
	case "", "database", "sql":
		if strings.Contains(haystack, "postgres") || port == "5432" {
			return "postgres"
		}
		if strings.Contains(haystack, "mysql") || port == "3306" {
			return "mysql"
		}
	}
	return ""
}

func crossplaneSecretRole(name string) string {
	const prefix = "crossplane-provider-sql-db-secret-user-"
	name = strings.TrimSpace(name)
	if !strings.HasPrefix(name, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(name, prefix)
	role, _, ok := strings.Cut(rest, "-providerconfig-")
	if !ok {
		return ""
	}
	return role
}

func secretValue(secret corev1.Secret, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(string(secret.Data[key])); value != "" {
			return value
		}
	}
	return ""
}

func kubernetesCredentialRef(contextName, namespace, secretName string) string {
	values := url.Values{}
	if strings.TrimSpace(contextName) != "" {
		values.Set("context", strings.TrimSpace(contextName))
	}
	return "kubernetes://" + namespace + "/secrets/" + secretName + "?" + values.Encode()
}

func shouldDiscoverSQLSecret(product string) bool {
	product = strings.ToLower(strings.TrimSpace(product))
	return product == "" || product == "mysql" || product == "mariadb" || product == "postgres" || product == "postgresql" || product == "pg" || product == "database" || product == "sql"
}

func limitCandidates(candidates []core.EndpointCandidate, limit int) []core.EndpointCandidate {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].ID < candidates[j].ID
		}
		return candidates[i].Score > candidates[j].Score
	})
	if limit <= 0 || len(candidates) <= limit {
		return candidates
	}
	return candidates[:limit]
}

func classifyService(item corev1.Service, productFilter string) (string, float64) {
	haystack := strings.ToLower(item.Name + " " + joinMap(item.Labels) + " " + joinMap(item.Annotations))
	products := []string{"prometheus", "loki", "homer", "mysql", "postgres"}
	for _, product := range products {
		if productFilter != "" && product != productFilter {
			continue
		}
		if product == "loki" && strings.Contains(haystack, "promtail") {
			continue
		}
		if strings.Contains(haystack, product) {
			score := 0.7
			if strings.Contains(strings.ToLower(item.Name), product) {
				score = 0.95
			}
			return product, score
		}
	}
	return "", 0
}

func serviceURLs(item corev1.Service, product string) []string {
	var urls []string
	for _, port := range item.Spec.Ports {
		if port.Port <= 0 {
			continue
		}
		scheme := "http"
		if strings.Contains(strings.ToLower(port.Name), "https") || port.Port == 443 {
			scheme = "https"
		}
		for _, ingress := range item.Status.LoadBalancer.Ingress {
			host := firstNonEmpty(ingress.Hostname, ingress.IP)
			if host != "" {
				urls = append(urls, scheme+"://"+host+":"+strconv.Itoa(int(port.Port)))
			}
		}
		clusterHost := item.Name + "." + item.Namespace + ".svc"
		switch product {
		case "mysql", "postgres":
			scheme = "mysql"
			if product == "postgres" {
				scheme = "postgres"
			}
		}
		urls = append(urls, scheme+"://"+clusterHost+":"+strconv.Itoa(int(port.Port)))
	}
	return uniqueStrings(urls)
}

func endpointCandidateID(product, endpoint, namespace, service string) string {
	sum := sha1.Sum([]byte(product + "\x00" + endpoint + "\x00" + namespace + "\x00" + service))
	return product + ":" + hex.EncodeToString(sum[:8])
}

func namespaceRecords(source pluginbinding.DatasourceSource, items []corev1.Namespace) []NamespaceRecord {
	records := make([]NamespaceRecord, 0, len(items))
	for _, item := range items {
		record := NamespaceRecord{
			DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityNamespace, item.Name, pluginbinding.RecordTitle(item.Name), pluginbinding.RecordLink("self", "kubernetes://namespace/"+url.PathEscape(item.Name))),
			Name:             item.Name,
			Status:           string(item.Status.Phase),
			Labels:           cloneStringMap(item.Labels),
			CreatedAt:        timestampText(item.CreationTimestamp),
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Name < records[j].Name })
	return records
}

func serviceRecords(source pluginbinding.DatasourceSource, items []corev1.Service) []ServiceRecord {
	records := make([]ServiceRecord, 0, len(items))
	for _, item := range items {
		id := item.Namespace + "/" + item.Name
		record := ServiceRecord{
			DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityService, id, pluginbinding.RecordTitle(id), pluginbinding.RecordLink("self", "kubernetes://service/"+url.PathEscape(id))),
			Name:             item.Name,
			Namespace:        item.Namespace,
			Type:             string(item.Spec.Type),
			ClusterIP:        item.Spec.ClusterIP,
			Ports:            servicePortTexts(item),
			Labels:           cloneStringMap(item.Labels),
			CreatedAt:        timestampText(item.CreationTimestamp),
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	return records
}

func podRecords(source pluginbinding.DatasourceSource, items []corev1.Pod) []PodRecord {
	records := make([]PodRecord, 0, len(items))
	for _, item := range items {
		id := item.Namespace + "/" + item.Name
		record := PodRecord{
			DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityPod, id, pluginbinding.RecordTitle(id), pluginbinding.RecordLink("self", "kubernetes://pod/"+url.PathEscape(id))),
			Name:             item.Name,
			Namespace:        item.Namespace,
			Phase:            string(item.Status.Phase),
			Node:             item.Spec.NodeName,
			Containers:       podContainerNames(item),
			Labels:           cloneStringMap(item.Labels),
			CreatedAt:        timestampText(item.CreationTimestamp),
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	return records
}

func containerRecords(source pluginbinding.DatasourceSource, items []corev1.Pod) []ContainerRecord {
	var records []ContainerRecord
	for _, pod := range items {
		statusByName := podContainerStatuses(pod)
		for _, container := range pod.Spec.InitContainers {
			records = append(records, containerRecord(source, pod, container, statusByName[container.Name], "init"))
		}
		for _, container := range pod.Spec.Containers {
			records = append(records, containerRecord(source, pod, container, statusByName[container.Name], "container"))
		}
		for _, container := range pod.Spec.EphemeralContainers {
			records = append(records, ephemeralContainerRecord(source, pod, container, statusByName[container.Name]))
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	return records
}

func containerRecord(source pluginbinding.DatasourceSource, pod corev1.Pod, container corev1.Container, status corev1.ContainerStatus, containerType string) ContainerRecord {
	id := pod.Namespace + "/" + pod.Name + "/" + container.Name
	record := ContainerRecord{
		DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityContainer, id, pluginbinding.RecordTitle(id), pluginbinding.RecordLink("self", "kubernetes://container/"+url.PathEscape(id))),
		Name:             container.Name,
		Namespace:        pod.Namespace,
		Pod:              pod.Name,
		Type:             containerType,
		Image:            container.Image,
		ImageID:          status.ImageID,
		ContainerID:      status.ContainerID,
		State:            containerStateText(status.State),
		Ready:            status.Ready,
		RestartCount:     status.RestartCount,
		Labels:           cloneStringMap(pod.Labels),
		CreatedAt:        timestampText(pod.CreationTimestamp),
	}
	return record
}

func ephemeralContainerRecord(source pluginbinding.DatasourceSource, pod corev1.Pod, container corev1.EphemeralContainer, status corev1.ContainerStatus) ContainerRecord {
	id := pod.Namespace + "/" + pod.Name + "/" + container.Name
	record := ContainerRecord{
		DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityContainer, id, pluginbinding.RecordTitle(id), pluginbinding.RecordLink("self", "kubernetes://container/"+url.PathEscape(id))),
		Name:             container.Name,
		Namespace:        pod.Namespace,
		Pod:              pod.Name,
		Type:             "ephemeral",
		Image:            container.Image,
		ImageID:          status.ImageID,
		ContainerID:      status.ContainerID,
		State:            containerStateText(status.State),
		Ready:            status.Ready,
		RestartCount:     status.RestartCount,
		Labels:           cloneStringMap(pod.Labels),
		CreatedAt:        timestampText(pod.CreationTimestamp),
	}
	return record
}

func containerRecordMetadata(record ContainerRecord) map[string]any {
	return map[string]any{
		"name":          record.Name,
		"namespace":     record.Namespace,
		"pod":           record.Pod,
		"type":          record.Type,
		"image":         record.Image,
		"image_id":      record.ImageID,
		"container_id":  record.ContainerID,
		"state":         record.State,
		"ready":         record.Ready,
		"restart_count": record.RestartCount,
		"labels":        record.Labels,
	}
}

func namespaceRecordMetadata(record NamespaceRecord) map[string]any {
	return map[string]any{"name": record.Name, "status": record.Status, "labels": record.Labels}
}

func serviceRecordMetadata(record ServiceRecord) map[string]any {
	return map[string]any{"name": record.Name, "namespace": record.Namespace, "type": record.Type, "cluster_ip": record.ClusterIP, "ports": record.Ports, "labels": record.Labels}
}

func podRecordMetadata(record PodRecord) map[string]any {
	return map[string]any{"name": record.Name, "namespace": record.Namespace, "phase": record.Phase, "node": record.Node, "containers": record.Containers, "labels": record.Labels}
}

func deploymentRecordMetadata(record DeploymentRecord) map[string]any {
	return map[string]any{"name": record.Name, "namespace": record.Namespace, "replicas": record.Replicas, "ready_replicas": record.ReadyReplicas, "available_replicas": record.AvailableReplicas, "updated_replicas": record.UpdatedReplicas, "strategy": record.Strategy, "labels": record.Labels}
}

func deploymentRecords(source pluginbinding.DatasourceSource, items []appsv1.Deployment) []DeploymentRecord {
	records := make([]DeploymentRecord, 0, len(items))
	for _, item := range items {
		id := item.Namespace + "/" + item.Name
		record := DeploymentRecord{
			DatasourceRecord:  pluginbinding.NewDatasourceRecord(source, EntityDeployment, id, pluginbinding.RecordTitle(id), pluginbinding.RecordLink("self", "kubernetes://deployment/"+url.PathEscape(id))),
			Name:              item.Name,
			Namespace:         item.Namespace,
			Replicas:          deploymentReplicas(item),
			ReadyReplicas:     item.Status.ReadyReplicas,
			AvailableReplicas: item.Status.AvailableReplicas,
			UpdatedReplicas:   item.Status.UpdatedReplicas,
			Strategy:          string(item.Spec.Strategy.Type),
			Labels:            cloneStringMap(item.Labels),
			CreatedAt:         timestampText(item.CreationTimestamp),
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	return records
}

func inventoryRecords(source pluginbinding.DatasourceSource, namespaces []corev1.Namespace, services []corev1.Service, pods []corev1.Pod, deployments []appsv1.Deployment) []pluginbinding.DatasourceRecord {
	var records []pluginbinding.DatasourceRecord
	for _, record := range namespaceRecords(source, namespaces) {
		base := record.DatasourceRecord
		base.Metadata = namespaceRecordMetadata(record)
		records = append(records, base)
	}
	for _, record := range serviceRecords(source, services) {
		base := record.DatasourceRecord
		base.Metadata = serviceRecordMetadata(record)
		records = append(records, base)
	}
	for _, record := range podRecords(source, pods) {
		base := record.DatasourceRecord
		base.Metadata = podRecordMetadata(record)
		records = append(records, base)
	}
	for _, record := range containerRecords(source, pods) {
		base := record.DatasourceRecord
		base.Metadata = containerRecordMetadata(record)
		records = append(records, base)
	}
	for _, record := range deploymentRecords(source, deployments) {
		base := record.DatasourceRecord
		base.Metadata = deploymentRecordMetadata(record)
		records = append(records, base)
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Entity != records[j].Entity {
			return records[i].Entity < records[j].Entity
		}
		return records[i].ID < records[j].ID
	})
	return records
}

func servicePortTexts(item corev1.Service) []string {
	var out []string
	for _, port := range item.Spec.Ports {
		text := firstNonEmpty(port.Name, string(port.Protocol)) + ":" + strconv.Itoa(int(port.Port))
		if port.TargetPort.String() != "" && port.TargetPort.String() != "0" {
			text += "->" + port.TargetPort.String()
		}
		out = append(out, text)
	}
	return out
}

func podContainerNames(item corev1.Pod) []string {
	var out []string
	for _, container := range item.Spec.Containers {
		if container.Name != "" {
			out = append(out, container.Name)
		}
	}
	return out
}

func podContainerStatuses(item corev1.Pod) map[string]corev1.ContainerStatus {
	out := map[string]corev1.ContainerStatus{}
	for _, status := range item.Status.InitContainerStatuses {
		out[status.Name] = status
	}
	for _, status := range item.Status.ContainerStatuses {
		out[status.Name] = status
	}
	for _, status := range item.Status.EphemeralContainerStatuses {
		out[status.Name] = status
	}
	return out
}

func containerStateText(state corev1.ContainerState) string {
	switch {
	case state.Running != nil:
		return "running"
	case state.Waiting != nil:
		if strings.TrimSpace(state.Waiting.Reason) != "" {
			return "waiting:" + strings.TrimSpace(state.Waiting.Reason)
		}
		return "waiting"
	case state.Terminated != nil:
		if strings.TrimSpace(state.Terminated.Reason) != "" {
			return "terminated:" + strings.TrimSpace(state.Terminated.Reason)
		}
		return "terminated"
	default:
		return ""
	}
}

func deploymentReplicas(item appsv1.Deployment) int32 {
	if item.Spec.Replicas == nil {
		return 1
	}
	return *item.Spec.Replicas
}

func timestampText(value metav1.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Time.Format(time.RFC3339)
}

func filterNamespaceRecords(records []NamespaceRecord, query string) []NamespaceRecord {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return records
	}
	var out []NamespaceRecord
	for _, record := range records {
		if strings.Contains(strings.ToLower(record.Name+" "+record.Status+" "+joinMap(record.Labels)), query) {
			out = append(out, record)
		}
	}
	return out
}

func filterServiceRecords(records []ServiceRecord, query string) []ServiceRecord {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return records
	}
	var out []ServiceRecord
	for _, record := range records {
		if strings.Contains(strings.ToLower(record.ID+" "+record.Name+" "+record.Namespace+" "+record.Type+" "+strings.Join(record.Ports, " ")+" "+joinMap(record.Labels)), query) {
			out = append(out, record)
		}
	}
	return out
}

func filterPodRecords(records []PodRecord, query string) []PodRecord {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return records
	}
	var out []PodRecord
	for _, record := range records {
		if strings.Contains(strings.ToLower(record.ID+" "+record.Name+" "+record.Namespace+" "+record.Phase+" "+record.Node+" "+strings.Join(record.Containers, " ")+" "+joinMap(record.Labels)), query) {
			out = append(out, record)
		}
	}
	return out
}

func filterContainerRecords(records []ContainerRecord, query string) []ContainerRecord {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return records
	}
	var out []ContainerRecord
	for _, record := range records {
		if strings.Contains(strings.ToLower(record.ID+" "+record.Name+" "+record.Namespace+" "+record.Pod+" "+record.Type+" "+record.Image+" "+record.State+" "+joinMap(record.Labels)), query) {
			out = append(out, record)
		}
	}
	return out
}

func containerRecordMatches(record ContainerRecord, input InventoryInput) bool {
	name := strings.TrimSpace(input.Name)
	namespace := strings.TrimSpace(input.Namespace)
	if namespace != "" && record.Namespace != namespace {
		return false
	}
	if name == "" {
		return false
	}
	return record.ID == name || record.Pod+"/"+record.Name == name || record.Name == name
}

func normalizeInventoryInput(input InventoryInput) InventoryInput {
	if strings.TrimSpace(input.Namespace) != "" {
		return input
	}
	namespace, name, ok := strings.Cut(strings.TrimSpace(input.Name), "/")
	if !ok {
		return input
	}
	input.Namespace = strings.TrimSpace(namespace)
	input.Name = strings.TrimSpace(name)
	return input
}

func normalizePodLogsInput(input PodLogsInput) PodLogsInput {
	if strings.TrimSpace(input.Namespace) != "" {
		return input
	}
	namespace, name, ok := strings.Cut(strings.TrimSpace(input.Name), "/")
	if !ok {
		return input
	}
	input.Namespace = strings.TrimSpace(namespace)
	input.Name = strings.TrimSpace(name)
	return input
}

func filterDeploymentRecords(records []DeploymentRecord, query string) []DeploymentRecord {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return records
	}
	var out []DeploymentRecord
	for _, record := range records {
		replicaText := fmt.Sprintf("%d/%d/%d/%d", record.ReadyReplicas, record.AvailableReplicas, record.UpdatedReplicas, record.Replicas)
		if strings.Contains(strings.ToLower(record.ID+" "+record.Name+" "+record.Namespace+" "+record.Strategy+" "+replicaText+" "+joinMap(record.Labels)), query) {
			out = append(out, record)
		}
	}
	return out
}

func filterDatasourceRecords(records []pluginbinding.DatasourceRecord, query string) []pluginbinding.DatasourceRecord {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return records
	}
	var out []pluginbinding.DatasourceRecord
	for _, record := range records {
		if strings.Contains(strings.ToLower(record.Entity+" "+record.ID+" "+record.Title+" "+metadataText(record.Metadata)), query) {
			out = append(out, record)
		}
	}
	return out
}

func metadataText(metadata map[string]any) string {
	var parts []string
	for key, value := range metadata {
		parts = append(parts, key+"="+strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(strings.Trim(fmt.Sprint(value), "[]"), "\n", " "), "  ", " ")))
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

func limitSlice[T any](items []T, limit int) []T {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

func joinMap(values map[string]string) string {
	var out []string
	for key, value := range values {
		out = append(out, key+"="+value)
	}
	sort.Strings(out)
	return strings.Join(out, " ")
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		if _, err := url.Parse(value); err != nil {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func endpointProtocol(value string) string {
	if parsed, err := url.Parse(value); err == nil {
		return parsed.Scheme
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
