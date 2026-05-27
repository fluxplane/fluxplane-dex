package gitlab

import (
	"fmt"
	"strings"

	"github.com/fluxplane/fluxplane-dex/plugins/internal/pluginutil"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

type SecretGetter func(req protocol.Request, purpose string, cache map[string]pluginutil.SecretMaterial) (pluginutil.SecretMaterial, error)

type SecretSet struct {
	GitLabURL   pluginutil.SecretMaterial
	AccessToken pluginutil.SecretMaterial
}

func defaultSecretGetter(req protocol.Request, purpose string, cache map[string]pluginutil.SecretMaterial) (pluginutil.SecretMaterial, error) {
	if cache != nil {
		if material, ok := cache[purpose]; ok {
			return material, nil
		}
	}
	material, err := pluginutil.SecretGet(PluginName, req.Instance, req.Grant, purpose)
	if err != nil {
		return pluginutil.SecretMaterial{}, err
	}
	if cache != nil {
		cache[purpose] = material
	}
	return material, nil
}

func resolveSecrets(req protocol.Request, cache map[string]pluginutil.SecretMaterial, get SecretGetter) (SecretSet, error) {
	if get == nil {
		get = defaultSecretGetter
	}
	token, err := get(req, "access_token", cache)
	if err != nil {
		return SecretSet{}, err
	}
	url, err := get(req, "gitlab_url", cache)
	if err != nil {
		return SecretSet{}, err
	}
	if strings.TrimSpace(token.Value) == "" {
		return SecretSet{}, fmt.Errorf("access_token is empty")
	}
	if strings.TrimSpace(url.Value) == "" {
		return SecretSet{}, fmt.Errorf("gitlab_url is empty")
	}
	return SecretSet{GitLabURL: url, AccessToken: token}, nil
}
