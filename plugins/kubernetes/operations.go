package kubernetes

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Service struct {
	Contexts func() (ClusterListResult, error)
	Services func(context.Context, EndpointDiscoverInput) ([]corev1.Service, error)
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

type EndpointDiscoverInput struct {
	Product   string `json:"product,omitempty" jsonschema:"description=Product to discover, for example prometheus or loki."`
	Context   string `json:"context,omitempty" jsonschema:"description=Kubeconfig context."`
	Namespace string `json:"namespace,omitempty" jsonschema:"description=Namespace to inspect. Empty means all namespaces."`
	Limit     int    `json:"limit,omitempty" jsonschema:"description=Maximum candidates."`
}

type EndpointDiscoverResult struct {
	Candidates []core.EndpointCandidate `json:"candidates"`
}

func (s Service) ClusterList(ctx pluginbinding.Context, input ClusterListInput) (ClusterListResult, error) {
	result, err := s.contexts()()
	if err != nil {
		return ClusterListResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	return result, nil
}

func (s Service) EndpointDiscover(ctx pluginbinding.Context, input EndpointDiscoverInput) (EndpointDiscoverResult, error) {
	services, err := s.services()(context.Background(), input)
	if err != nil {
		return EndpointDiscoverResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	return EndpointDiscoverResult{Candidates: serviceCandidates(services, input)}, nil
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

func (s Service) services() func(context.Context, EndpointDiscoverInput) ([]corev1.Service, error) {
	if s.Services != nil {
		return s.Services
	}
	return listKubernetesServices
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

func listKubernetesServices(ctx context.Context, input EndpointDiscoverInput) ([]corev1.Service, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	if strings.TrimSpace(input.Context) != "" {
		overrides.CurrentContext = strings.TrimSpace(input.Context)
	}
	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	namespace := strings.TrimSpace(input.Namespace)
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}
	list, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
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
				Protocol: "http",
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

func classifyService(item corev1.Service, productFilter string) (string, float64) {
	haystack := strings.ToLower(item.Name + " " + joinMap(item.Labels) + " " + joinMap(item.Annotations))
	products := []string{"prometheus", "loki", "homer", "mysql"}
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
		if product == "mysql" {
			scheme = "mysql"
		}
		urls = append(urls, scheme+"://"+clusterHost+":"+strconv.Itoa(int(port.Port)))
	}
	return uniqueStrings(urls)
}

func endpointCandidateID(product, endpoint, namespace, service string) string {
	sum := sha1.Sum([]byte(product + "\x00" + endpoint + "\x00" + namespace + "\x00" + service))
	return product + ":" + hex.EncodeToString(sum[:8])
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
