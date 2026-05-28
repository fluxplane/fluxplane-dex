package atlassian

import (
	"errors"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
)

func TestResolveCredentialsUsesConfiguredPurposes(t *testing.T) {
	plugin := pluginbinding.New(core.PluginManifest{Name: "test"}).WithSecretGetter(func(_ pluginbinding.Context, purpose string) (pluginbinding.SecretMaterial, error) {
		values := map[string]string{
			"api_token": "token",
			"cloud_id":  "cloud-123",
		}
		if value := values[purpose]; value != "" {
			return pluginbinding.SecretMaterial{Value: value, Purpose: purpose}, nil
		}
		return pluginbinding.SecretMaterial{}, errors.New("missing")
	})
	pluginbinding.Operation(plugin, core.OperationSpec{Name: "test.resolve"}, func(ctx pluginbinding.Context, _ struct{}) (Credentials, error) {
		return ResolveCredentials(ctx, SecretConfig{Product: "confluence", TokenPurpose: "api_token", CloudIDPurpose: "cloud_id"})
	})

	creds := plugintest.RunOK[Credentials](t, plugin, "test.resolve", map[string]any{}, plugintest.WithInstance("work"))
	if creds.BaseURL != "https://api.atlassian.com/ex/confluence/cloud-123" || creds.Token.Value != "token" || creds.CloudID.Value != "cloud-123" {
		t.Fatalf("credentials = %#v", creds)
	}
}

func TestResolveCredentialsCanUseCloudID(t *testing.T) {
	plugin := pluginbinding.New(core.PluginManifest{Name: "test"}).WithSecretGetter(func(_ pluginbinding.Context, purpose string) (pluginbinding.SecretMaterial, error) {
		values := map[string]string{
			"api_token": "token",
			"cloud_id":  "cloud-123",
		}
		if value := values[purpose]; value != "" {
			return pluginbinding.SecretMaterial{Value: value, Purpose: purpose}, nil
		}
		return pluginbinding.SecretMaterial{}, errors.New("missing")
	})
	pluginbinding.Operation(plugin, core.OperationSpec{Name: "test.resolve"}, func(ctx pluginbinding.Context, _ struct{}) (Credentials, error) {
		return ResolveCredentials(ctx, SecretConfig{Product: "jira", TokenPurpose: "api_token", CloudIDPurpose: "cloud_id"})
	})

	creds := plugintest.RunOK[Credentials](t, plugin, "test.resolve", map[string]any{}, plugintest.WithInstance("work"))
	if creds.BaseURL != "https://api.atlassian.com/ex/jira/cloud-123" || creds.CloudID.Value != "cloud-123" {
		t.Fatalf("credentials = %#v", creds)
	}
}

func TestBearerAuthHeader(t *testing.T) {
	want := "Bearer token"
	if got := BearerAuthHeader(" token "); got != want {
		t.Fatalf("header = %q want %q", got, want)
	}
}
