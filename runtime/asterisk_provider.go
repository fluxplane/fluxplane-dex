package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/internal/kuberneteshost"
)

type asteriskAMIPingInput struct {
	EndpointRef   string `json:"endpoint_ref,omitempty"`
	URL           string `json:"url,omitempty"`
	CredentialRef string `json:"credential_ref,omitempty"`
	Timeout       string `json:"timeout,omitempty"`
}

type asteriskAMIPingOutput struct {
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

type asteriskAMICredentials struct {
	Username string
	Secret   string
	Source   string
}

func (r Runner) hostAsteriskProviderCall(ctx context.Context, plugin, instance, grant string, input pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	if strings.TrimSpace(input.Action) != "ami.ping" {
		return pluginbinding.ProviderCallResponse{}, fmt.Errorf("unsupported Asterisk provider action %q", input.Action)
	}
	var pingInput asteriskAMIPingInput
	if len(input.Payload) > 0 {
		if err := json.Unmarshal(input.Payload, &pingInput); err != nil {
			return pluginbinding.ProviderCallResponse{}, err
		}
	}
	out, err := r.runAsteriskAMIPing(ctx, plugin, instance, grant, pingInput)
	if err != nil {
		return pluginbinding.ProviderCallResponse{}, err
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return pluginbinding.ProviderCallResponse{}, err
	}
	return pluginbinding.ProviderCallResponse{Result: raw}, nil
}

func (r Runner) runAsteriskAMIPing(ctx context.Context, plugin, instance, grant string, input asteriskAMIPingInput) (asteriskAMIPingOutput, error) {
	targetURL, credentialRef, err := r.asteriskAMITarget(input)
	if err != nil {
		return asteriskAMIPingOutput{}, err
	}
	timeout, err := asteriskProviderDuration(input.Timeout, 10*time.Second)
	if err != nil {
		return asteriskAMIPingOutput{}, err
	}
	creds, err := r.asteriskAMICredentials(ctx, plugin, instance, grant, credentialRef)
	if err != nil {
		return asteriskAMIPingOutput{}, err
	}
	start := time.Now()
	out := asteriskAMIPingOutput{EndpointRef: input.EndpointRef, URL: targetURL, CredentialSource: creds.Source}
	result, err := asteriskAMIPing(ctx, targetURL, creds, timeout)
	out.DurationMS = time.Since(start).Milliseconds()
	out.Greeting = result.Greeting
	out.Response = result.Response
	out.Message = result.Message
	out.Authenticated = result.Authenticated
	out.Pong = result.Pong
	out.OK = result.Authenticated && result.Pong && err == nil
	if err != nil {
		out.Error = err.Error()
	}
	return out, nil
}

func (r Runner) asteriskAMITarget(input asteriskAMIPingInput) (string, string, error) {
	endpointRef := strings.TrimSpace(input.EndpointRef)
	rawURL := strings.TrimSpace(input.URL)
	credentialRef := strings.TrimSpace(input.CredentialRef)
	if endpointRef != "" {
		endpoint, ok, err := r.State.GetEndpoint(endpointRef)
		if err != nil {
			return "", "", err
		}
		if !ok {
			return "", "", fmt.Errorf("unknown endpoint_ref %q", endpointRef)
		}
		if rawURL == "" {
			rawURL = endpoint.URL
		}
		if credentialRef == "" {
			credentialRef = endpoint.CredentialRef
		}
	}
	rawURL = normalizeAMIURL(rawURL)
	if rawURL == "" {
		return "", "", fmt.Errorf("endpoint_ref or url is required")
	}
	if _, _, err := amiDialAddress(rawURL); err != nil {
		return "", "", err
	}
	return rawURL, credentialRef, nil
}

func (r Runner) asteriskAMICredentials(ctx context.Context, plugin, instance, grant, credentialRef string) (asteriskAMICredentials, error) {
	username := r.optionalHostSecret(ctx, plugin, instance, "username", grant)
	secret := firstNonEmpty(
		r.optionalHostSecret(ctx, plugin, instance, "secret", grant),
		r.optionalHostSecret(ctx, plugin, instance, "password", grant),
	)
	if username != "" && secret != "" {
		return asteriskAMICredentials{Username: username, Secret: secret, Source: "store"}, nil
	}
	if strings.TrimSpace(credentialRef) != "" {
		creds, err := r.asteriskAMICredentialsFromCredentialRef(ctx, credentialRef)
		if err != nil {
			return asteriskAMICredentials{}, err
		}
		if creds.Username != "" && creds.Secret != "" {
			return creds, nil
		}
	}
	return asteriskAMICredentials{}, fmt.Errorf("AMI username and secret are required")
}

func (r Runner) asteriskAMICredentialsFromCredentialRef(ctx context.Context, ref string) (asteriskAMICredentials, error) {
	parsed, err := url.Parse(strings.TrimSpace(ref))
	if err != nil {
		return asteriskAMICredentials{}, err
	}
	if parsed.Scheme != "kubernetes" {
		return asteriskAMICredentials{}, fmt.Errorf("unsupported credential_ref scheme %q", parsed.Scheme)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 2 || strings.TrimSpace(parsed.Host) == "" {
		return asteriskAMICredentials{}, fmt.Errorf("invalid kubernetes credential_ref %q", ref)
	}
	namespace := strings.TrimSpace(parsed.Host)
	kind := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])
	contextName := strings.TrimSpace(parsed.Query().Get("context"))
	input := kuberneteshost.EndpointDiscoverInput{Context: contextName, Namespace: namespace, Name: name}
	switch kind {
	case "secrets", "secret":
		items, err := kuberneteshost.Secrets(ctx, input)
		if err != nil {
			return asteriskAMICredentials{}, err
		}
		if len(items) == 0 {
			return asteriskAMICredentials{}, fmt.Errorf("secret %s/%s not found", namespace, name)
		}
		creds := amiCredentialsFromData(bytesMapToStringMap(items[0].Data), "kubernetes_secret:"+namespace+"/"+name)
		if creds.Username == "" || creds.Secret == "" {
			return asteriskAMICredentials{}, fmt.Errorf("secret %s/%s does not contain AMI username and secret", namespace, name)
		}
		return creds, nil
	case "configmaps", "configmap":
		items, err := kuberneteshost.ConfigMaps(ctx, input)
		if err != nil {
			return asteriskAMICredentials{}, err
		}
		if len(items) == 0 {
			return asteriskAMICredentials{}, fmt.Errorf("configmap %s/%s not found", namespace, name)
		}
		creds := amiCredentialsFromData(items[0].Data, "kubernetes_configmap:"+namespace+"/"+name)
		if creds.Username == "" || creds.Secret == "" {
			return asteriskAMICredentials{}, fmt.Errorf("configmap %s/%s does not contain AMI username and secret", namespace, name)
		}
		return creds, nil
	default:
		return asteriskAMICredentials{}, fmt.Errorf("unsupported kubernetes credential_ref kind %q", kind)
	}
}

type amiPingResult struct {
	Greeting      string
	Response      string
	Message       string
	Authenticated bool
	Pong          bool
}

func asteriskAMIPing(ctx context.Context, rawURL string, creds asteriskAMICredentials, timeout time.Duration) (amiPingResult, error) {
	network, address, err := amiDialAddress(rawURL)
	if err != nil {
		return amiPingResult{}, err
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return amiPingResult{}, err
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	reader := bufio.NewReader(conn)
	greeting, err := reader.ReadString('\n')
	if err != nil {
		return amiPingResult{}, err
	}
	result := amiPingResult{Greeting: strings.TrimSpace(greeting)}
	if err := amiWriteAction(conn, map[string]string{
		"Action":   "Login",
		"Username": creds.Username,
		"Secret":   creds.Secret,
		"ActionID": "dex-login",
	}); err != nil {
		return result, err
	}
	login, err := amiReadMessage(reader)
	if err != nil {
		return result, err
	}
	result.Response = login["Response"]
	result.Message = login["Message"]
	if !strings.EqualFold(login["Response"], "Success") {
		return result, fmt.Errorf("AMI login failed: %s", firstNonEmpty(login["Message"], login["Response"]))
	}
	result.Authenticated = true
	if err := amiWriteAction(conn, map[string]string{"Action": "Ping", "ActionID": "dex-ping"}); err != nil {
		return result, err
	}
	pong, err := amiReadMessage(reader)
	if err != nil {
		return result, err
	}
	result.Response = pong["Response"]
	result.Message = firstNonEmpty(pong["Ping"], pong["Message"])
	result.Pong = strings.EqualFold(pong["Response"], "Success") && strings.EqualFold(pong["Ping"], "Pong")
	_ = amiWriteAction(conn, map[string]string{"Action": "Logoff", "ActionID": "dex-logoff"})
	if !result.Pong {
		return result, fmt.Errorf("AMI ping failed: %s", firstNonEmpty(result.Message, result.Response))
	}
	return result, nil
}

func amiDialAddress(rawURL string) (string, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", "", err
	}
	if parsed.Scheme != "ami" && parsed.Scheme != "tcp" {
		return "", "", fmt.Errorf("unsupported AMI endpoint scheme %q", parsed.Scheme)
	}
	host := parsed.Hostname()
	if host == "" {
		return "", "", fmt.Errorf("AMI endpoint host is required")
	}
	port := parsed.Port()
	if port == "" {
		port = "5038"
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", "", fmt.Errorf("invalid AMI endpoint port %q", port)
	}
	return "tcp", net.JoinHostPort(host, port), nil
}

func normalizeAMIURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	if !strings.Contains(rawURL, "://") {
		rawURL = "ami://" + rawURL
	}
	return rawURL
}

func amiWriteAction(conn net.Conn, fields map[string]string) error {
	for _, key := range []string{"Action", "Username", "Secret", "ActionID"} {
		if value := strings.TrimSpace(fields[key]); value != "" {
			if _, err := fmt.Fprintf(conn, "%s: %s\r\n", key, value); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprint(conn, "\r\n")
	return err
}

func amiReadMessage(reader *bufio.Reader) (map[string]string, error) {
	out := map[string]string{}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return out, err
		}
		line = strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(line) == "" {
			return out, nil
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		out[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
}

func asteriskProviderDuration(value string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	return time.ParseDuration(value)
}

func bytesMapToStringMap(input map[string][]byte) map[string]string {
	out := map[string]string{}
	for key, value := range input {
		out[key] = string(value)
	}
	return out
}

func amiCredentialsFromData(data map[string]string, source string) asteriskAMICredentials {
	if creds := amiCredentialsFromManagerConfData(data, source); creds.Username != "" && creds.Secret != "" {
		return creds
	}
	username := valueForKeys(data, "username", "user", "ami_username", "manager_username", "ASTERISK_AMI_USERNAME")
	secret := valueForKeys(data, "secret", "password", "pass", "ami_secret", "ami_password", "manager_secret", "ASTERISK_AMI_SECRET", "ASTERISK_AMI_PASSWORD")
	if username != "" && secret != "" && (isAsteriskCredentialHint(source) || hasAMIKey(data)) {
		return asteriskAMICredentials{Username: username, Secret: secret, Source: source}
	}
	return asteriskAMICredentials{}
}

func amiCredentialsFromManagerConfData(data map[string]string, source string) asteriskAMICredentials {
	for key, value := range data {
		if strings.Contains(strings.ToLower(key), "manager.conf") {
			if creds := amiCredentialsFromManagerConf(value, source+":"+key); creds.Username != "" && creds.Secret != "" {
				return creds
			}
		}
	}
	if value := valueForKeys(data, "manager.conf", "ami.conf", "asterisk.conf"); value != "" {
		return amiCredentialsFromManagerConf(value, source)
	}
	return asteriskAMICredentials{}
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

func amiCredentialsFromManagerConf(text, source string) asteriskAMICredentials {
	section := ""
	enabled := true
	var best asteriskAMICredentials
	var currentSecret string
	flush := func() {
		if section != "" && section != "general" && currentSecret != "" && enabled && best.Username == "" {
			best = asteriskAMICredentials{Username: section, Secret: currentSecret, Source: source}
		}
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.Contains(line, "]") {
			flush()
			section = strings.ToLower(strings.TrimSpace(line[1:strings.Index(line, "]")]))
			enabled = true
			currentSecret = ""
			continue
		}
		key, value, ok := strings.Cut(line, "=>")
		if !ok {
			key, value, ok = strings.Cut(line, "=")
		}
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		switch key {
		case "secret":
			currentSecret = value
		case "enabled":
			value = strings.ToLower(value)
			enabled = value == "" || value == "yes" || value == "true" || value == "1"
		}
	}
	flush()
	return best
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
