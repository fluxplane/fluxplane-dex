package asterisk

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

type Service struct {
	ProviderCall func(pluginbinding.Context, string, any) (json.RawMessage, error)
}

func NewService() Service {
	return Service{}
}

type AMITargetInput struct {
	EndpointRef   string `json:"endpoint_ref,omitempty" jsonschema:"description=Registered Asterisk AMI endpoint ref resolved by the host."`
	URL           string `json:"url,omitempty" jsonschema:"description=Asterisk AMI URL. Defaults to ami://host:5038 when no scheme is supplied."`
	CredentialRef string `json:"credential_ref,omitempty" jsonschema:"description=Host-resolved credential reference from endpoint discovery."`
}

type AMIPingInput struct {
	AMITargetInput
	Timeout string `json:"timeout,omitempty" jsonschema:"description=AMI connection timeout duration. Defaults to 10s."`
}

type AMIPingResult struct {
	EndpointRef      string `json:"endpoint_ref,omitempty"`
	URL              string `json:"url,omitempty"`
	OK               bool   `json:"ok"`
	Authenticated    bool   `json:"authenticated,omitempty"`
	Pong             bool   `json:"pong,omitempty"`
	Greeting         string `json:"greeting,omitempty"`
	Response         string `json:"response,omitempty"`
	Message          string `json:"message,omitempty"`
	CredentialSource string `json:"credential_source,omitempty"`
	DurationMS       int64  `json:"duration_ms,omitempty"`
	Error            string `json:"error,omitempty"`
}

type EndpointDiscoverInput struct {
	Product   string `json:"product,omitempty" jsonschema:"description=Product to discover. Empty, asterisk, ami, and asterisk-ami discover AMI endpoints."`
	Context   string `json:"context,omitempty" jsonschema:"description=Kubeconfig context."`
	Namespace string `json:"namespace,omitempty" jsonschema:"description=Namespace to inspect. Empty means all namespaces."`
	Limit     int    `json:"limit,omitempty" jsonschema:"description=Maximum candidates."`
}

type EndpointDiscoverResult struct {
	Candidates []core.EndpointCandidate `json:"candidates"`
}

type k8sObjectMeta struct {
	Name        string            `json:"name,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type k8sService struct {
	Metadata k8sObjectMeta `json:"metadata"`
	Spec     struct {
		Type  string `json:"type,omitempty"`
		Ports []struct {
			Name       string `json:"name,omitempty"`
			Port       int    `json:"port,omitempty"`
			TargetPort any    `json:"targetPort,omitempty"`
			Protocol   string `json:"protocol,omitempty"`
		} `json:"ports,omitempty"`
	} `json:"spec"`
}

type k8sSecret struct {
	Metadata k8sObjectMeta     `json:"metadata"`
	Data     map[string][]byte `json:"data,omitempty"`
}

type k8sConfigMap struct {
	Metadata k8sObjectMeta     `json:"metadata"`
	Data     map[string]string `json:"data,omitempty"`
}

type amiCredentialCandidate struct {
	Namespace      string
	Name           string
	Kind           string
	Ref            string
	Score          float64
	CredentialKeys string
}

func (s Service) AMIPing(ctx pluginbinding.Context, input AMIPingInput) (AMIPingResult, error) {
	if strings.TrimSpace(input.EndpointRef) == "" && strings.TrimSpace(input.URL) == "" {
		return AMIPingResult{}, pluginbinding.Fail("bad_input", "endpoint_ref or url is required")
	}
	raw, err := s.providerCall(ctx, PluginName, "ami.ping", input)
	if err != nil {
		return AMIPingResult{}, pluginbinding.Errorf("asterisk", "%s", err)
	}
	var out AMIPingResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return AMIPingResult{}, pluginbinding.Errorf("asterisk", "decode AMI ping result: %s", err)
	}
	return out, nil
}

func (s Service) EndpointDiscover(ctx pluginbinding.Context, input EndpointDiscoverInput) (EndpointDiscoverResult, error) {
	if !shouldDiscoverAMI(input.Product) {
		return EndpointDiscoverResult{Candidates: nil}, nil
	}
	services, err := providerCallAs[[]k8sService](s, ctx, "services", input)
	if err != nil {
		return EndpointDiscoverResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	secrets, err := providerCallAs[[]k8sSecret](s, ctx, "secrets", input)
	if err != nil {
		return EndpointDiscoverResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	configMaps, err := providerCallAs[[]k8sConfigMap](s, ctx, "configmaps", input)
	if err != nil {
		return EndpointDiscoverResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	credentials := append(secretAMICredentials(secrets, input), configMapAMICredentials(configMaps, input)...)
	candidates := serviceAMICandidates(services, credentials, input)
	candidates = append(candidates, credentialEndpointCandidates(credentials, secrets, configMaps, input)...)
	return EndpointDiscoverResult{Candidates: limitCandidates(dedupeCandidates(candidates), input.Limit)}, nil
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
		return protocol.Fail("asterisk", err.Error())
	}
	return protocol.OK(result)
}

func (s Service) providerCall(ctx pluginbinding.Context, provider, action string, input any) (json.RawMessage, error) {
	if s.ProviderCall != nil {
		return s.ProviderCall(ctx, action, input)
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	response, err := ctx.Host.CapabilityCall(pluginbinding.ProviderCallRequest{
		Provider: provider,
		Action:   action,
		Payload:  payload,
	})
	if err != nil {
		return nil, err
	}
	return response.Result, nil
}

func providerCallAs[T any](s Service, ctx pluginbinding.Context, action string, input any) (T, error) {
	var out T
	raw, err := s.providerCall(ctx, "kubernetes", action, input)
	if err != nil {
		return out, err
	}
	if len(raw) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, err
	}
	return out, nil
}

func serviceAMICandidates(services []k8sService, credentials []amiCredentialCandidate, input EndpointDiscoverInput) []core.EndpointCandidate {
	var candidates []core.EndpointCandidate
	for _, service := range services {
		namespace := service.Metadata.Namespace
		if namespace == "" {
			namespace = strings.TrimSpace(input.Namespace)
		}
		for _, port := range service.Spec.Ports {
			if !isAMIPort(service, port.Name, port.Port) {
				continue
			}
			endpoint := "ami://" + joinHostPort(service.Metadata.Name+"."+namespace+".svc", strconv.Itoa(port.Port))
			credential := bestCredentialForNamespace(credentials, namespace, service.Metadata.Name)
			labels := map[string]string{
				"namespace": namespace,
				"service":   service.Metadata.Name,
				"protocol":  "ami",
			}
			if strings.TrimSpace(input.Context) != "" {
				labels["context"] = strings.TrimSpace(input.Context)
			}
			if service.Spec.Type != "" {
				labels["type"] = service.Spec.Type
			}
			annotations := cloneStringMap(service.Metadata.Annotations)
			if credential.CredentialKeys != "" {
				if annotations == nil {
					annotations = map[string]string{}
				}
				annotations["credential_keys"] = credential.CredentialKeys
			}
			candidates = append(candidates, core.EndpointCandidate{
				ID:            endpointCandidateID("asterisk", endpoint, namespace, service.Metadata.Name),
				URL:           endpoint,
				Product:       "asterisk",
				Protocol:      "ami",
				Source:        "kubernetes_service",
				Score:         serviceAMIScore(service, port.Name, port.Port, credential.Score),
				CredentialRef: credential.Ref,
				Labels:        labels,
				Annotations:   annotations,
			})
		}
	}
	return candidates
}

func secretAMICredentials(secrets []k8sSecret, input EndpointDiscoverInput) []amiCredentialCandidate {
	var out []amiCredentialCandidate
	for _, secret := range secrets {
		data := map[string]string{}
		for key, value := range secret.Data {
			data[key] = string(value)
		}
		hint := secret.Metadata.Name + " " + joinMap(secret.Metadata.Labels) + " " + joinMap(secret.Metadata.Annotations)
		if !hasAMICredentials(data, hint) {
			continue
		}
		namespace := firstNonEmpty(secret.Metadata.Namespace, input.Namespace)
		out = append(out, amiCredentialCandidate{
			Namespace:      namespace,
			Name:           secret.Metadata.Name,
			Kind:           "secret",
			Ref:            kubernetesCredentialRef(input.Context, namespace, "secrets", secret.Metadata.Name),
			Score:          credentialScore(secret.Metadata.Name, secret.Metadata.Labels, secret.Metadata.Annotations),
			CredentialKeys: credentialKeys(data),
		})
	}
	return out
}

func configMapAMICredentials(configMaps []k8sConfigMap, input EndpointDiscoverInput) []amiCredentialCandidate {
	var out []amiCredentialCandidate
	for _, configMap := range configMaps {
		hint := configMap.Metadata.Name + " " + joinMap(configMap.Metadata.Labels) + " " + joinMap(configMap.Metadata.Annotations)
		if !hasAMICredentials(configMap.Data, hint) {
			continue
		}
		namespace := firstNonEmpty(configMap.Metadata.Namespace, input.Namespace)
		out = append(out, amiCredentialCandidate{
			Namespace:      namespace,
			Name:           configMap.Metadata.Name,
			Kind:           "configmap",
			Ref:            kubernetesCredentialRef(input.Context, namespace, "configmaps", configMap.Metadata.Name),
			Score:          credentialScore(configMap.Metadata.Name, configMap.Metadata.Labels, configMap.Metadata.Annotations),
			CredentialKeys: credentialKeys(configMap.Data),
		})
	}
	return out
}

func credentialEndpointCandidates(credentials []amiCredentialCandidate, secrets []k8sSecret, configMaps []k8sConfigMap, input EndpointDiscoverInput) []core.EndpointCandidate {
	var candidates []core.EndpointCandidate
	for _, secret := range secrets {
		data := map[string]string{}
		for key, value := range secret.Data {
			data[key] = string(value)
		}
		candidate, ok := credentialEndpointCandidate(data, "secret", firstNonEmpty(secret.Metadata.Namespace, input.Namespace), secret.Metadata.Name, input)
		if ok {
			candidates = append(candidates, candidate)
		}
	}
	for _, configMap := range configMaps {
		candidate, ok := credentialEndpointCandidate(configMap.Data, "configmap", firstNonEmpty(configMap.Metadata.Namespace, input.Namespace), configMap.Metadata.Name, input)
		if ok {
			candidates = append(candidates, candidate)
		}
	}
	_ = credentials
	return candidates
}

func credentialEndpointCandidate(data map[string]string, kind, namespace, name string, input EndpointDiscoverInput) (core.EndpointCandidate, bool) {
	if !hasAMICredentials(data, name) {
		return core.EndpointCandidate{}, false
	}
	host := valueForKeys(data, "host", "hostname", "endpoint", "address", "ami_host", "ASTERISK_AMI_HOST")
	if host == "" {
		return core.EndpointCandidate{}, false
	}
	port := valueForKeys(data, "port", "ami_port", "ASTERISK_AMI_PORT")
	if port == "" {
		port = "5038"
	}
	endpoint := "ami://" + joinHostPort(host, port)
	labels := map[string]string{
		"namespace": namespace,
		kind:        name,
		"protocol":  "ami",
	}
	if strings.TrimSpace(input.Context) != "" {
		labels["context"] = strings.TrimSpace(input.Context)
	}
	refKind := "secrets"
	source := "kubernetes_secret"
	if kind == "configmap" {
		refKind = "configmaps"
		source = "kubernetes_configmap"
	}
	return core.EndpointCandidate{
		ID:            endpointCandidateID("asterisk", endpoint, namespace, name),
		URL:           endpoint,
		Product:       "asterisk",
		Protocol:      "ami",
		Source:        source,
		Score:         0.92,
		CredentialRef: kubernetesCredentialRef(input.Context, namespace, refKind, name),
		Labels:        labels,
		Annotations:   map[string]string{"credential_keys": credentialKeys(data)},
	}, true
}

func hasAMICredentials(data map[string]string, hint string) bool {
	if managerConfDataHasAMIUser(data) {
		return true
	}
	if valueForKeys(data, "username", "user", "ami_username", "manager_username", "ASTERISK_AMI_USERNAME") != "" &&
		valueForKeys(data, "secret", "password", "pass", "ami_secret", "ami_password", "manager_secret", "ASTERISK_AMI_SECRET", "ASTERISK_AMI_PASSWORD") != "" {
		return isAsteriskCredentialHint(hint) || hasAMIKey(data)
	}
	return false
}

func managerConfDataHasAMIUser(data map[string]string) bool {
	for key, value := range data {
		if strings.Contains(strings.ToLower(key), "manager.conf") && managerConfHasAMIUser(value) {
			return true
		}
	}
	return managerConfHasAMIUser(valueForKeys(data, "manager.conf", "ami.conf", "asterisk.conf"))
}

func isAsteriskCredentialHint(hint string) bool {
	return hasHintToken(hint, "asterisk", "ami", "manager")
}

func hasAMIKey(data map[string]string) bool {
	for key := range data {
		if hasHintToken(key, "asterisk", "ami", "manager") {
			return true
		}
	}
	return false
}

func managerConfHasAMIUser(text string) bool {
	section := ""
	enabled := true
	hasSecret := false
	flush := func() bool {
		return section != "" && section != "general" && enabled && hasSecret
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.Contains(line, "]") {
			if flush() {
				return true
			}
			section = strings.ToLower(strings.TrimSpace(line[1:strings.Index(line, "]")]))
			enabled = true
			hasSecret = false
			continue
		}
		key, value, ok := strings.Cut(line, "=>")
		if !ok {
			key, value, ok = strings.Cut(line, "=")
		}
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "secret":
			hasSecret = strings.TrimSpace(value) != ""
		case "enabled":
			value = strings.ToLower(strings.TrimSpace(value))
			enabled = value == "" || value == "yes" || value == "true" || value == "1"
		}
	}
	return flush()
}

func isAMIPort(service k8sService, portName string, port int) bool {
	if port == 5038 {
		return true
	}
	haystack := strings.ToLower(service.Metadata.Name + " " + portName + " " + joinMap(service.Metadata.Labels) + " " + joinMap(service.Metadata.Annotations))
	return strings.Contains(haystack, "asterisk") && (strings.Contains(haystack, "ami") || strings.Contains(haystack, "manager"))
}

func serviceAMIScore(service k8sService, portName string, port int, credentialScore float64) float64 {
	score := 0.7
	haystack := strings.ToLower(service.Metadata.Name + " " + portName)
	if strings.Contains(haystack, "asterisk") || strings.Contains(haystack, "freepbx") {
		score = 0.9
	}
	if port == 5038 {
		score += 0.04
	}
	if credentialScore > 0 {
		score += 0.04
	}
	if score > 0.99 {
		score = 0.99
	}
	return score
}

func bestCredentialForNamespace(credentials []amiCredentialCandidate, namespace, serviceName string) amiCredentialCandidate {
	var best amiCredentialCandidate
	serviceName = strings.ToLower(serviceName)
	for _, credential := range credentials {
		if credential.Namespace != namespace {
			continue
		}
		score := credential.Score
		if strings.Contains(strings.ToLower(credential.Name), serviceName) || strings.Contains(serviceName, strings.ToLower(credential.Name)) {
			score += 0.05
		}
		if score > best.Score {
			best = credential
			best.Score = score
		}
	}
	return best
}

func credentialScore(name string, labels, annotations map[string]string) float64 {
	haystack := strings.ToLower(name + " " + joinMap(labels) + " " + joinMap(annotations))
	score := 0.75
	if strings.Contains(haystack, "asterisk") || strings.Contains(haystack, "freepbx") {
		score = 0.9
	}
	if strings.Contains(haystack, "ami") || strings.Contains(haystack, "manager") {
		score += 0.04
	}
	return score
}

func credentialKeys(data map[string]string) string {
	if valueForKeys(data, "username", "user", "ami_username", "manager_username", "ASTERISK_AMI_USERNAME") != "" {
		if valueForKeys(data, "secret", "password", "pass", "ami_secret", "ami_password", "manager_secret", "ASTERISK_AMI_SECRET", "ASTERISK_AMI_PASSWORD") != "" {
			return "username,secret"
		}
	}
	for key, value := range data {
		if strings.Contains(strings.ToLower(key), "manager.conf") && managerConfHasAMIUser(value) {
			return key
		}
	}
	return "manager.conf"
}

func shouldDiscoverAMI(product string) bool {
	switch strings.ToLower(strings.TrimSpace(product)) {
	case "", "asterisk", "ami", "asterisk-ami", "asterisk_ami":
		return true
	default:
		return false
	}
}

func kubernetesCredentialRef(contextName, namespace, kind, name string) string {
	values := url.Values{}
	if strings.TrimSpace(contextName) != "" {
		values.Set("context", strings.TrimSpace(contextName))
	}
	return "kubernetes://" + namespace + "/" + kind + "/" + name + "?" + values.Encode()
}

func endpointCandidateID(product, endpoint, namespace, name string) string {
	sum := sha1.Sum([]byte(product + "\x00" + endpoint + "\x00" + namespace + "\x00" + name))
	return product + "-" + hex.EncodeToString(sum[:6])
}

func endpointProtocol(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Scheme
}

func dedupeCandidates(candidates []core.EndpointCandidate) []core.EndpointCandidate {
	byKey := map[string]core.EndpointCandidate{}
	for _, candidate := range candidates {
		key := candidate.Product + "\x00" + candidate.Protocol + "\x00" + candidate.URL
		if existing, ok := byKey[key]; ok && existing.Score >= candidate.Score {
			continue
		}
		byKey[key] = candidate
	}
	out := make([]core.EndpointCandidate, 0, len(byKey))
	for _, candidate := range byKey {
		out = append(out, candidate)
	}
	return out
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

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := map[string]string{}
	for key, value := range input {
		out[key] = value
	}
	return out
}

func joinMap(input map[string]string) string {
	var values []string
	for key, value := range input {
		values = append(values, key, value)
	}
	sort.Strings(values)
	return strings.Join(values, " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func valueForKeys(data map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(data[key]); value != "" {
			return value
		}
	}
	lowerKeys := map[string]bool{}
	for _, key := range keys {
		lowerKeys[strings.ToLower(key)] = true
	}
	for key, value := range data {
		if lowerKeys[strings.ToLower(key)] {
			if value = strings.TrimSpace(value); value != "" {
				return value
			}
		}
	}
	return ""
}

func joinHostPort(host, port string) string {
	host = strings.TrimSpace(host)
	port = strings.TrimSpace(port)
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	if port == "" {
		return host
	}
	return host + ":" + port
}

func hasHintToken(value string, tokens ...string) bool {
	allowed := map[string]bool{}
	for _, token := range tokens {
		allowed[strings.ToLower(strings.TrimSpace(token))] = true
	}
	for _, field := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}) {
		if allowed[field] {
			return true
		}
	}
	return false
}

var _ = endpointProtocol
