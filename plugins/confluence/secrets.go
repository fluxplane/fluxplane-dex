package confluence

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/internal/atlassian"
)

func resolveCredentials(ctx pluginbinding.Context) (atlassian.Credentials, error) {
	return atlassian.ResolveCredentials(ctx, atlassian.SecretConfig{
		Product:        PluginName,
		TokenPurpose:   AuthPurposeAPIToken,
		CloudIDPurpose: AuthPurposeCloudID,
	})
}
