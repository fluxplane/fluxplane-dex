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
	"time"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Service struct {
	Contexts     func() (ClusterListResult, error)
	ClusterProbe func(context.Context, ClusterTestInput) (ClusterTestResult, error)
	Services     func(context.Context, EndpointDiscoverInput) ([]corev1.Service, error)
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

func (s Service) services() func(context.Context, EndpointDiscoverInput) ([]corev1.Service, error) {
	if s.Services != nil {
		return s.Services
	}
	return listKubernetesServices
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
