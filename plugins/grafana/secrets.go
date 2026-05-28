package grafana

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type target struct {
	URL      string
	Token    string
	Username string
	Password string
}

func (s Service) target(ctx pluginbinding.Context, input GrafanaTargetInput) (target, error) {
	out := target{
		URL:      firstNonEmpty(input.URL, env(EnvGrafanaURL)),
		Token:    firstNonEmpty(input.Token, env(EnvGrafanaAPIToken)),
		Username: firstNonEmpty(input.Username, env(EnvGrafanaUsername)),
		Password: firstNonEmpty(input.Password, env(EnvGrafanaPassword)),
	}
	if out.URL == "" {
		if material, ok := ctx.OptionalSecret(AuthPurposeURL); ok {
			out.URL = strings.TrimSpace(material.Value)
		}
	}
	if out.Token == "" {
		if material, ok := ctx.OptionalSecret(AuthPurposeAPIToken); ok {
			out.Token = strings.TrimSpace(material.Value)
		}
	}
	if out.Username == "" {
		if material, ok := ctx.OptionalSecret(AuthPurposeUsername); ok {
			out.Username = strings.TrimSpace(material.Value)
		}
	}
	if out.Password == "" {
		if material, ok := ctx.OptionalSecret(AuthPurposePassword); ok {
			out.Password = strings.TrimSpace(material.Value)
		}
	}
	if strings.TrimSpace(input.CredentialRef) != "" && out.Token == "" && (out.Username == "" || out.Password == "") {
		credential, err := resolveCredentialRef(context.Background(), input.CredentialRef)
		if err != nil {
			return target{}, err
		}
		out.Token = firstNonEmpty(out.Token, credential.Token)
		out.Username = firstNonEmpty(out.Username, credential.Username)
		out.Password = firstNonEmpty(out.Password, credential.Password)
	}
	if out.URL == "" {
		return target{}, fmt.Errorf("url or endpoint_ref is required")
	}
	return out, nil
}

type credentialMaterial struct {
	Token    string
	Username string
	Password string
}

func resolveCredentialRef(ctx context.Context, ref string) (credentialMaterial, error) {
	parsed, err := url.Parse(strings.TrimSpace(ref))
	if err != nil {
		return credentialMaterial{}, err
	}
	if parsed.Scheme != "kubernetes" {
		return credentialMaterial{}, fmt.Errorf("unsupported credential_ref scheme %q", parsed.Scheme)
	}
	namespace := parsed.Host
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if namespace == "" || len(parts) != 2 || parts[0] != "secrets" || parts[1] == "" {
		return credentialMaterial{}, fmt.Errorf("invalid kubernetes credential_ref")
	}
	overrides := &clientcmd.ConfigOverrides{}
	if contextName := strings.TrimSpace(parsed.Query().Get("context")); contextName != "" {
		overrides.CurrentContext = contextName
	}
	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), overrides).ClientConfig()
	if err != nil {
		return credentialMaterial{}, err
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return credentialMaterial{}, err
	}
	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, parts[1], metav1.GetOptions{})
	if err != nil {
		return credentialMaterial{}, err
	}
	return credentialMaterial{
		Token:    secretString(secret.Data, "token", "api_token", "GRAFANA_API_TOKEN"),
		Username: secretString(secret.Data, "username", "user", "adminuser"),
		Password: secretString(secret.Data, "password", "pass", "adminpassword"),
	}, nil
}

func secretString(data map[string][]byte, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(string(data[key])); value != "" {
			return value
		}
	}
	return ""
}

func env(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}
