package gitlab

import "github.com/fluxplane/fluxplane-dex/core/pluginbinding"

type SecretSet struct {
	GitLabURL   pluginbinding.SecretMaterial
	AccessToken pluginbinding.SecretMaterial
}

func resolveSecrets(ctx pluginbinding.Context) (SecretSet, error) {
	token, err := ctx.RequiredSecret(AuthPurposeAccessToken)
	if err != nil {
		return SecretSet{}, err
	}
	url, err := ctx.RequiredSecret(AuthPurposeGitLabURL)
	if err != nil {
		return SecretSet{}, err
	}
	return SecretSet{GitLabURL: url, AccessToken: token}, nil
}
