package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/internal/kuberneteshost"
	kube "github.com/fluxplane/fluxplane-dex/plugins/kubernetes"
)

func (r Runner) hostKubernetesProviderCall(ctx context.Context, plugin, instance, grant string, input pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	action := strings.TrimSpace(input.Action)
	if action == "" {
		return pluginbinding.ProviderCallResponse{}, fmt.Errorf("unsupported kubernetes provider action %q", input.Action)
	}
	var result any
	var err error
	switch action {
	case "contexts":
		result, err = kuberneteshost.Contexts()
	case "cluster.probe":
		var request kube.ClusterTestInput
		request, err = decodeProviderPayload[kube.ClusterTestInput](input.Payload)
		if err == nil {
			request, err = r.resolveKubernetesClusterTestInput(request)
		}
		if err == nil {
			result, err = kuberneteshost.ClusterProbe(ctx, request)
		}
	case "namespaces":
		var request kube.InventoryInput
		request, err = decodeProviderPayload[kube.InventoryInput](input.Payload)
		if err == nil {
			request, err = r.resolveKubernetesInventoryInput(request)
		}
		if err == nil {
			result, err = kuberneteshost.Namespaces(ctx, request)
		}
	case "services":
		var request kube.EndpointDiscoverInput
		request, err = decodeProviderPayload[kube.EndpointDiscoverInput](input.Payload)
		if err == nil {
			result, err = kuberneteshost.Services(ctx, request)
		}
	case "ingresses":
		var request kube.EndpointDiscoverInput
		request, err = decodeProviderPayload[kube.EndpointDiscoverInput](input.Payload)
		if err == nil {
			result, err = kuberneteshost.Ingresses(ctx, request)
		}
	case "pods":
		var request kube.InventoryInput
		request, err = decodeProviderPayload[kube.InventoryInput](input.Payload)
		if err == nil {
			request, err = r.resolveKubernetesInventoryInput(request)
		}
		if err == nil {
			result, err = kuberneteshost.Pods(ctx, request)
		}
	case "deployments":
		var request kube.InventoryInput
		request, err = decodeProviderPayload[kube.InventoryInput](input.Payload)
		if err == nil {
			request, err = r.resolveKubernetesInventoryInput(request)
		}
		if err == nil {
			result, err = kuberneteshost.Deployments(ctx, request)
		}
	case "pod.logs":
		var request kube.PodLogsInput
		request, err = decodeProviderPayload[kube.PodLogsInput](input.Payload)
		if err == nil {
			request, err = r.resolveKubernetesPodLogsInput(request)
		}
		if err == nil {
			result, err = kuberneteshost.PodLogs(ctx, request)
		}
	case "portforward.start":
		var request kube.PortForwardStartInput
		request, err = decodeProviderPayload[kube.PortForwardStartInput](input.Payload)
		if err == nil {
			request, err = r.resolveKubernetesPortForwardStartInput(request)
		}
		if err == nil {
			result, err = kuberneteshost.PortForwardStart(ctx, request)
		}
	case "portforward.stop":
		var request kube.PortForwardStopInput
		request, err = decodeProviderPayload[kube.PortForwardStopInput](input.Payload)
		if err == nil {
			result, err = kuberneteshost.PortForwardStop(ctx, request)
		}
	case "secrets":
		var request kube.EndpointDiscoverInput
		request, err = decodeProviderPayload[kube.EndpointDiscoverInput](input.Payload)
		if err == nil {
			result, err = kuberneteshost.Secrets(ctx, request)
		}
	default:
		err = fmt.Errorf("unsupported kubernetes provider action %q", action)
	}
	if err != nil {
		return pluginbinding.ProviderCallResponse{}, err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return pluginbinding.ProviderCallResponse{}, err
	}
	return pluginbinding.ProviderCallResponse{Result: raw}, nil
}

func decodeProviderPayload[T any](payload json.RawMessage) (T, error) {
	var out T
	if len(payload) == 0 || string(payload) == "null" {
		return out, nil
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return out, err
	}
	return out, nil
}

func (r Runner) resolveKubernetesClusterTestInput(input kube.ClusterTestInput) (kube.ClusterTestInput, error) {
	if strings.TrimSpace(input.EndpointRef) == "" {
		return input, nil
	}
	urlValue, err := r.kubernetesEndpointURL(input.EndpointRef)
	if err != nil {
		return input, err
	}
	if strings.TrimSpace(input.URL) == "" {
		input.URL = urlValue
	}
	if strings.TrimSpace(input.Context) == "" {
		input.Context = kubernetesContextFromEndpointURL(urlValue)
	}
	return input, nil
}

func (r Runner) resolveKubernetesInventoryInput(input kube.InventoryInput) (kube.InventoryInput, error) {
	if strings.TrimSpace(input.EndpointRef) == "" {
		return input, nil
	}
	urlValue, err := r.kubernetesEndpointURL(input.EndpointRef)
	if err != nil {
		return input, err
	}
	if strings.TrimSpace(input.URL) == "" {
		input.URL = urlValue
	}
	if strings.TrimSpace(input.Context) == "" {
		input.Context = kubernetesContextFromEndpointURL(urlValue)
	}
	return input, nil
}

func (r Runner) resolveKubernetesPodLogsInput(input kube.PodLogsInput) (kube.PodLogsInput, error) {
	if strings.TrimSpace(input.EndpointRef) == "" {
		return input, nil
	}
	urlValue, err := r.kubernetesEndpointURL(input.EndpointRef)
	if err != nil {
		return input, err
	}
	if strings.TrimSpace(input.URL) == "" {
		input.URL = urlValue
	}
	if strings.TrimSpace(input.Context) == "" {
		input.Context = kubernetesContextFromEndpointURL(urlValue)
	}
	return input, nil
}

func (r Runner) resolveKubernetesPortForwardStartInput(input kube.PortForwardStartInput) (kube.PortForwardStartInput, error) {
	if strings.TrimSpace(input.EndpointRef) == "" {
		return input, nil
	}
	urlValue, err := r.kubernetesEndpointURL(input.EndpointRef)
	if err != nil {
		return input, err
	}
	if strings.TrimSpace(input.URL) == "" {
		input.URL = urlValue
	}
	if strings.TrimSpace(input.Context) == "" {
		input.Context = kubernetesContextFromEndpointURL(urlValue)
	}
	return input, nil
}

func (r Runner) kubernetesEndpointURL(ref string) (string, error) {
	endpoint, ok, err := r.State.GetEndpoint(ref)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("unknown endpoint_ref %q", strings.TrimSpace(ref))
	}
	return endpoint.URL, nil
}

func kubernetesContextFromEndpointURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	if parsed.Scheme != "kubernetes" && parsed.Scheme != "k8s" {
		return ""
	}
	if parsed.Host != "context" {
		return ""
	}
	return strings.Trim(strings.TrimSpace(parsed.Path), "/")
}
