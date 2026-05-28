package kubernetes

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-dex/protocol"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEndpointDiscoverFindsPrometheusService(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Services: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Service, error) {
			return []corev1.Service{{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-prometheus-stack-prometheus", Namespace: "monitoring", Labels: map[string]string{"app.kubernetes.io/name": "prometheus"}},
				Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, Ports: []corev1.ServicePort{{Name: "http-web", Port: 9090}}},
			}}, nil
		},
		Secrets: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Secret, error) { return nil, nil },
	})

	out := plugintest.RunOK[EndpointDiscoverResult](t, plugin, OperationEndpointDiscover, map[string]any{"product": "prometheus"})
	if len(out.Candidates) != 1 {
		t.Fatalf("candidates = %#v", out.Candidates)
	}
	candidate := out.Candidates[0]
	if candidate.Product != "prometheus" || candidate.URL != "http://kube-prometheus-stack-prometheus.monitoring.svc:9090" || candidate.Source != "kubernetes" {
		t.Fatalf("candidate = %#v", candidate)
	}
}

func TestEndpointDiscoverFindsKubernetesClusterContext(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Contexts: func() (ClusterListResult, error) {
			return ClusterListResult{Contexts: []ClusterContext{{Name: "dev/context", Current: true, Cluster: "dev", User: "aws"}}}, nil
		},
	})

	out := plugintest.RunOK[EndpointDiscoverResult](t, plugin, OperationEndpointDiscover, map[string]any{"product": "kubernetes"})
	if len(out.Candidates) != 1 {
		t.Fatalf("candidates = %#v", out.Candidates)
	}
	candidate := out.Candidates[0]
	if candidate.Product != "kubernetes" || candidate.Protocol != "kubernetes" || candidate.Source != "kubeconfig" || candidate.URL != "kubernetes://context/dev%2Fcontext" {
		t.Fatalf("candidate = %#v", candidate)
	}
}

func TestClusterTestUsesContextFromEndpointURL(t *testing.T) {
	plugin := NewPluginWithService(Service{
		ClusterProbe: func(_ context.Context, input ClusterTestInput) (ClusterTestResult, error) {
			contextName := clusterContextFromTestInput(input)
			return ClusterTestResult{Context: contextName, OK: true, ServerVersion: "v1.30.0"}, nil
		},
	})

	out := plugintest.RunOK[ClusterTestResult](t, plugin, OperationClusterTest, map[string]any{"url": "kubernetes://context/dev%2Fcontext"})
	if !out.OK || out.Context != "dev/context" || out.ServerVersion != "v1.30.0" {
		t.Fatalf("result = %#v", out)
	}
}

func TestManifestIncludesInventoryCompletionMetadata(t *testing.T) {
	manifest := Manifest()
	if len(manifest.Datasources) != 1 {
		t.Fatalf("datasources = %#v", manifest.Datasources)
	}
	completion := manifest.Datasources[0].Completion
	if completion == nil || !containsString(completion.Fields, "context") || !containsString(completion.Fields, "namespace") || !containsString(completion.Fields, "pod") || !containsString(completion.Fields, "containers") {
		t.Fatalf("completion = %#v", completion)
	}
}

func TestManifestAdvertisesHostEndpointRefForOperations(t *testing.T) {
	manifest := Manifest()
	for _, operation := range manifest.Operations {
		var schema struct {
			Properties map[string]any `json:"properties"`
		}
		if err := json.Unmarshal(operation.Input, &schema); err != nil {
			t.Fatal(err)
		}
		if _, ok := schema.Properties["endpoint_ref"]; !ok {
			t.Fatalf("%s input schema missing endpoint_ref: %s", operation.Name, string(operation.Input))
		}
	}
}

func TestInventoryOperationsListResources(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Namespaces: func(_ context.Context, _ InventoryInput) ([]corev1.Namespace, error) {
			return []corev1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: "latest", Labels: map[string]string{"team": "platform"}}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}}}, nil
		},
		Services: func(_ context.Context, input EndpointDiscoverInput) ([]corev1.Service, error) {
			if input.Context != "dev" || input.Namespace != "latest" {
				t.Fatalf("input = %#v", input)
			}
			return []corev1.Service{{
				ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "latest", Labels: map[string]string{"app": "api"}},
				Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, ClusterIP: "10.0.0.1", Ports: []corev1.ServicePort{{Name: "http", Port: 8080}}},
			}}, nil
		},
		Pods: func(_ context.Context, _ InventoryInput) ([]corev1.Pod, error) {
			return []corev1.Pod{{
				ObjectMeta: metav1.ObjectMeta{Name: "api-123", Namespace: "latest", Labels: map[string]string{"app": "api"}},
				Spec:       corev1.PodSpec{NodeName: "ip-10-0-0-1", Containers: []corev1.Container{{Name: "api", Image: "registry.example.com/api:latest"}}},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{{
						Name:         "api",
						Image:        "registry.example.com/api:latest",
						ImageID:      "sha256:abc",
						ContainerID:  "containerd://123",
						Ready:        true,
						RestartCount: 1,
						State:        corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
					}},
				},
			}}, nil
		},
		Deployments: func(_ context.Context, _ InventoryInput) ([]appsv1.Deployment, error) {
			return []appsv1.Deployment{{
				ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "latest", Labels: map[string]string{"app": "api"}},
				Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(2), Strategy: appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType}},
				Status:     appsv1.DeploymentStatus{ReadyReplicas: 1, AvailableReplicas: 1, UpdatedReplicas: 2},
			}}, nil
		},
		Logs: func(_ context.Context, input PodLogsInput) (PodLogsResult, error) {
			if input.Namespace != "latest" || input.Name != "api-123" || input.Container != "api" || input.TailLines != 25 || !input.Timestamps {
				t.Fatalf("input = %#v", input)
			}
			return PodLogsResult{Namespace: input.Namespace, Name: input.Name, Container: input.Container, Lines: []string{"one", "two"}, Text: "one\ntwo", LineCount: 2, TailLines: input.TailLines, Timestamps: input.Timestamps}, nil
		},
	})

	namespaces := plugintest.RunOK[NamespaceListResult](t, plugin, OperationNamespaceList, map[string]any{"query": "platform"})
	if namespaces.Count != 1 || namespaces.Namespaces[0].Name != "latest" {
		t.Fatalf("namespaces = %#v", namespaces)
	}
	services := plugintest.RunOK[ServiceListResult](t, plugin, OperationServiceList, map[string]any{"context": "dev", "namespace": "latest"})
	if services.Count != 1 || services.Services[0].ID != "latest/api" || services.Services[0].Ports[0] != "http:8080" {
		t.Fatalf("services = %#v", services)
	}
	pods := plugintest.RunOK[PodListResult](t, plugin, OperationPodList, map[string]any{"query": "api"})
	if pods.Count != 1 || pods.Pods[0].Phase != "Running" || pods.Pods[0].Containers[0] != "api" {
		t.Fatalf("pods = %#v", pods)
	}
	if pods.Pods[0].Metadata != nil {
		t.Fatalf("typed pod records should not duplicate fields into metadata: %#v", pods.Pods[0].Metadata)
	}
	containers := plugintest.RunOK[ContainerListResult](t, plugin, OperationContainerList, map[string]any{"query": "api"})
	if containers.Count != 1 || containers.Containers[0].ID != "latest/api-123/api" || containers.Containers[0].State != "running" || !containers.Containers[0].Ready {
		t.Fatalf("containers = %#v", containers)
	}
	container := plugintest.RunOK[ContainerShowResult](t, plugin, OperationContainerShow, map[string]any{"namespace": "latest", "name": "api-123/api"})
	if container.Container.Image != "registry.example.com/api:latest" || container.Container.RestartCount != 1 {
		t.Fatalf("container = %#v", container)
	}
	deployments := plugintest.RunOK[DeploymentListResult](t, plugin, OperationDeploymentList, map[string]any{"query": "api"})
	if deployments.Count != 1 || deployments.Deployments[0].ReadyReplicas != 1 || deployments.Deployments[0].Replicas != 2 {
		t.Fatalf("deployments = %#v", deployments)
	}
	deployment := plugintest.RunOK[DeploymentShowResult](t, plugin, OperationDeploymentShow, map[string]any{"namespace": "latest", "name": "api"})
	if deployment.Deployment.ID != "latest/api" || deployment.Deployment.Strategy != "RollingUpdate" {
		t.Fatalf("deployment = %#v", deployment)
	}
	logs := plugintest.RunOK[PodLogsResult](t, plugin, OperationPodLogs, map[string]any{"namespace": "latest", "name": "api-123", "container": "api", "tail_lines": 25, "timestamps": true})
	if logs.LineCount != 2 || logs.Lines[1] != "two" {
		t.Fatalf("logs = %#v", logs)
	}
}

func TestPodLogBoundsDoNotDefaultTailWhenByteOrTimeBounded(t *testing.T) {
	bounds, err := podLogBounds(PodLogsInput{LimitBytes: 2048})
	if err != nil {
		t.Fatal(err)
	}
	if bounds.TailLines != nil || bounds.LimitBytes == nil || *bounds.LimitBytes != 2048 {
		t.Fatalf("limit-only bounds = %#v", bounds)
	}
	bounds, err = podLogBounds(PodLogsInput{Since: "2h"})
	if err != nil {
		t.Fatal(err)
	}
	if bounds.TailLines != nil || bounds.SinceSeconds == nil || *bounds.SinceSeconds != 7200 {
		t.Fatalf("since bounds = %#v", bounds)
	}
	bounds, err = podLogBounds(PodLogsInput{})
	if err != nil {
		t.Fatal(err)
	}
	if bounds.TailLines == nil || *bounds.TailLines != 100 {
		t.Fatalf("default bounds = %#v", bounds)
	}
}

func TestPodLogUntilFiltersTimestampedLines(t *testing.T) {
	bounds, err := podLogBounds(PodLogsInput{Until: "2026-05-28T10:01:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	text := filterPodLogText(strings.Join([]string{
		"2026-05-28T10:00:00Z first",
		"2026-05-28T10:02:00Z second",
	}, "\n"), bounds, false)
	if text != "first" {
		t.Fatalf("filtered text = %q", text)
	}
}

func TestPortForwardOperationsUseInjectedLifecycle(t *testing.T) {
	plugin := NewPluginWithService(Service{
		ForwardStart: func(_ context.Context, input PortForwardStartInput) (PortForwardResult, error) {
			if input.Namespace != "monitoring" || input.Resource != "service/loki" || input.RemotePort != 3100 {
				t.Fatalf("start input = %#v", input)
			}
			return PortForwardResult{ID: "kpf-test", Namespace: input.Namespace, Resource: input.Resource, LocalPort: 49152, RemotePort: input.RemotePort, LocalURL: "http://127.0.0.1:49152", PID: 123, ProcessGroup: 123}, nil
		},
		ForwardStop: func(_ context.Context, input PortForwardStopInput) (PortForwardStopResult, error) {
			if input.ID != "kpf-test" {
				t.Fatalf("stop input = %#v", input)
			}
			return PortForwardStopResult{ID: input.ID, Stopped: true, Signal: "SIGTERM"}, nil
		},
	})
	start := plugintest.RunOK[PortForwardResult](t, plugin, OperationPortForwardStart, map[string]any{"namespace": "monitoring", "resource": "service/loki", "remote_port": 3100})
	if start.ID != "kpf-test" || start.LocalURL != "http://127.0.0.1:49152" {
		t.Fatalf("start = %#v", start)
	}
	stop := plugintest.RunOK[PortForwardStopResult](t, plugin, OperationPortForwardStop, map[string]any{"id": "kpf-test"})
	if !stop.Stopped {
		t.Fatalf("stop = %#v", stop)
	}
}

func TestInventoryDatasourceSearchFindsServicesPodsAndDeployments(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Namespaces: func(_ context.Context, input InventoryInput) ([]corev1.Namespace, error) {
			if input.URL != "kubernetes://context/dev" || input.Namespace != "" {
				t.Fatalf("namespace input = %#v", input)
			}
			return []corev1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: "latest"}}}, nil
		},
		Services: func(_ context.Context, input EndpointDiscoverInput) ([]corev1.Service, error) {
			if input.Context != "dev" || input.Namespace != "" {
				t.Fatalf("service input = %#v", input)
			}
			return []corev1.Service{{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "latest"}}}, nil
		},
		Pods: func(_ context.Context, input InventoryInput) ([]corev1.Pod, error) {
			if input.URL != "kubernetes://context/dev" || input.Namespace != "" {
				t.Fatalf("pod input = %#v", input)
			}
			return []corev1.Pod{{
				ObjectMeta: metav1.ObjectMeta{Name: "api-123", Namespace: "latest"},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "api", Image: "registry.example.com/api:latest"}}},
				Status:     corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Name: "api", Ready: true}}},
			}}, nil
		},
		Deployments: func(_ context.Context, input InventoryInput) ([]appsv1.Deployment, error) {
			if input.URL != "kubernetes://context/dev" || input.Namespace != "" {
				t.Fatalf("deployment input = %#v", input)
			}
			return []appsv1.Deployment{{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "latest"}, Spec: appsv1.DeploymentSpec{Replicas: int32Ptr(1)}}}, nil
		},
	})

	out := plugintest.DatasourceSearchOK[InventorySearchResult](t, plugin, map[string]any{"query": "api", "limit": 10, "url": "kubernetes://context/dev"})
	if out.Count != 4 {
		t.Fatalf("search = %#v", out)
	}
	entities := map[string]bool{}
	for _, record := range out.Records {
		entities[record.Entity] = true
	}
	if !entities[EntityService] || !entities[EntityPod] || !entities[EntityDeployment] || !entities[EntityContainer] {
		t.Fatalf("records = %#v", out.Records)
	}
	for _, record := range out.Records {
		if len(record.Metadata) == 0 {
			t.Fatalf("inventory datasource record missing metadata: %#v", record)
		}
	}
}

func TestEndpointsDiscoverProtocolUsesKubernetes(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Services: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Service, error) {
			return []corev1.Service{{
				ObjectMeta: metav1.ObjectMeta{Name: "loki-gateway", Namespace: "logging", Labels: map[string]string{"app.kubernetes.io/name": "loki"}},
				Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, Ports: []corev1.ServicePort{{Name: "http", Port: 3100}}},
			}}, nil
		},
		Secrets: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Secret, error) { return nil, nil },
	})
	payload, _ := json.Marshal(map[string]any{"product": "loki"})
	resp := plugin.Handle(protocol.Request{Protocol: protocol.Version, Command: protocol.CommandEndpointsDiscover, Plugin: PluginName, Payload: payload})
	if !resp.OK {
		t.Fatalf("response = %#v", resp)
	}
	var out EndpointDiscoverResult
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Candidates) != 1 || out.Candidates[0].Product != "loki" {
		t.Fatalf("candidates = %#v", out.Candidates)
	}
}

func TestEndpointDiscoverFindsMySQLConnectionSecret(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Services: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Service, error) { return nil, nil },
		Secrets: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Secret, error) {
			return []corev1.Secret{{
				ObjectMeta: metav1.ObjectMeta{Name: "app-mysql", Namespace: "apps", Labels: map[string]string{"crossplane.io/claim-name": "app-mysql"}},
				Data: map[string][]byte{
					"host":     []byte("mysql.apps.svc"),
					"port":     []byte("3306"),
					"database": []byte("app"),
					"username": []byte("appuser"),
					"password": []byte("secret"),
				},
			}}, nil
		},
	})

	out := plugintest.RunOK[EndpointDiscoverResult](t, plugin, OperationEndpointDiscover, map[string]any{"product": "mysql", "context": "dev"})
	if len(out.Candidates) != 1 {
		t.Fatalf("candidates = %#v", out.Candidates)
	}
	candidate := out.Candidates[0]
	if candidate.Product != "mysql" || candidate.URL != "mysql://mysql.apps.svc:3306/app" || candidate.CredentialRef == "" {
		t.Fatalf("candidate = %#v", candidate)
	}
}

func TestEndpointDiscoverDefaultsExplicitMySQLSecretPort(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Services: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Service, error) { return nil, nil },
		Secrets: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Secret, error) {
			return []corev1.Secret{{
				ObjectMeta: metav1.ObjectMeta{Name: "connection-secret", Namespace: "apps"},
				Data: map[string][]byte{
					"host":     []byte("database.apps.svc"),
					"database": []byte("app"),
					"username": []byte("appuser"),
					"password": []byte("secret"),
				},
			}}, nil
		},
	})

	out := plugintest.RunOK[EndpointDiscoverResult](t, plugin, OperationEndpointDiscover, map[string]any{"product": "mysql", "context": "dev"})
	if len(out.Candidates) != 1 {
		t.Fatalf("candidates = %#v", out.Candidates)
	}
	candidate := out.Candidates[0]
	if candidate.Product != "mysql" || candidate.URL != "mysql://database.apps.svc:3306/app" {
		t.Fatalf("candidate = %#v", candidate)
	}
}

func TestEndpointDiscoverFindsPostgresConnectionSecret(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Services: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Service, error) { return nil, nil },
		Secrets: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Secret, error) {
			return []corev1.Secret{{
				ObjectMeta: metav1.ObjectMeta{Name: "crossplane-provider-sql-db-secret-user-latest-acd-providerconfig-latest-aurora-postgresql2", Namespace: "latest"},
				Data: map[string][]byte{
					"endpoint": []byte("postgres.example.com"),
					"port":     []byte("5432"),
					"username": []byte("latest-acd"),
					"password": []byte("secret"),
				},
			}}, nil
		},
	})

	out := plugintest.RunOK[EndpointDiscoverResult](t, plugin, OperationEndpointDiscover, map[string]any{"product": "postgres", "context": "dev", "namespace": "latest"})
	if len(out.Candidates) != 1 {
		t.Fatalf("candidates = %#v", out.Candidates)
	}
	candidate := out.Candidates[0]
	if candidate.Product != "postgres" || candidate.URL != "postgres://postgres.example.com:5432/latest-acd?sslmode=require" || candidate.CredentialRef == "" {
		t.Fatalf("candidate = %#v", candidate)
	}
}

func int32Ptr(value int32) *int32 {
	return &value
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
