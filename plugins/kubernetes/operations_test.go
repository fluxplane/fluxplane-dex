package kubernetes

import (
	"context"
	"encoding/json"
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
				Spec:       corev1.PodSpec{NodeName: "ip-10-0-0-1", Containers: []corev1.Container{{Name: "api"}}},
				Status:     corev1.PodStatus{Phase: corev1.PodRunning},
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

func TestInventoryDatasourceSearchFindsServicesPodsAndDeployments(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Namespaces: func(_ context.Context, input InventoryInput) ([]corev1.Namespace, error) {
			if input.URL != "kubernetes://context/dev" || input.Namespace != "latest" {
				t.Fatalf("namespace input = %#v", input)
			}
			return []corev1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: "latest"}}}, nil
		},
		Services: func(_ context.Context, input EndpointDiscoverInput) ([]corev1.Service, error) {
			if input.Context != "dev" || input.Namespace != "latest" {
				t.Fatalf("service input = %#v", input)
			}
			return []corev1.Service{{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "latest"}}}, nil
		},
		Pods: func(_ context.Context, input InventoryInput) ([]corev1.Pod, error) {
			if input.URL != "kubernetes://context/dev" || input.Namespace != "latest" {
				t.Fatalf("pod input = %#v", input)
			}
			return []corev1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: "api-123", Namespace: "latest"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}}, nil
		},
		Deployments: func(_ context.Context, input InventoryInput) ([]appsv1.Deployment, error) {
			if input.URL != "kubernetes://context/dev" || input.Namespace != "latest" {
				t.Fatalf("deployment input = %#v", input)
			}
			return []appsv1.Deployment{{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "latest"}, Spec: appsv1.DeploymentSpec{Replicas: int32Ptr(1)}}}, nil
		},
	})

	out := plugintest.DatasourceSearchOK[InventorySearchResult](t, plugin, map[string]any{"query": "api", "limit": 10, "url": "kubernetes://context/dev", "namespace": "latest"})
	if out.Count != 3 {
		t.Fatalf("search = %#v", out)
	}
	entities := map[string]bool{}
	for _, record := range out.Records {
		entities[record.Entity] = true
	}
	if !entities[EntityService] || !entities[EntityPod] || !entities[EntityDeployment] {
		t.Fatalf("records = %#v", out.Records)
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
