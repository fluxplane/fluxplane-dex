package kubernetes

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-dex/protocol"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEndpointDiscoverFindsPrometheusService(t *testing.T) {
	plugin := NewPluginWithService(Service{Services: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Service, error) {
		return []corev1.Service{{
			ObjectMeta: metav1.ObjectMeta{Name: "kube-prometheus-stack-prometheus", Namespace: "monitoring", Labels: map[string]string{"app.kubernetes.io/name": "prometheus"}},
			Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, Ports: []corev1.ServicePort{{Name: "http-web", Port: 9090}}},
		}}, nil
	}})

	out := plugintest.RunOK[EndpointDiscoverResult](t, plugin, OperationEndpointDiscover, map[string]any{"product": "prometheus"})
	if len(out.Candidates) != 1 {
		t.Fatalf("candidates = %#v", out.Candidates)
	}
	candidate := out.Candidates[0]
	if candidate.Product != "prometheus" || candidate.URL != "http://kube-prometheus-stack-prometheus.monitoring.svc:9090" || candidate.Source != "kubernetes" {
		t.Fatalf("candidate = %#v", candidate)
	}
}

func TestEndpointsDiscoverProtocolUsesKubernetes(t *testing.T) {
	plugin := NewPluginWithService(Service{Services: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Service, error) {
		return []corev1.Service{{
			ObjectMeta: metav1.ObjectMeta{Name: "loki-gateway", Namespace: "logging", Labels: map[string]string{"app.kubernetes.io/name": "loki"}},
			Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, Ports: []corev1.ServicePort{{Name: "http", Port: 3100}}},
		}}, nil
	}})
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
