package slack

import (
	"strings"

	"github.com/fluxplane/fluxplane-dex/plugins/internal/pluginutil"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

type SecretGetter func(req protocol.Request, purpose string, cache map[string]pluginutil.SecretMaterial) (pluginutil.SecretMaterial, error)

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

func optionalSecret(req protocol.Request, purpose string, cache map[string]pluginutil.SecretMaterial, get SecretGetter) (pluginutil.SecretMaterial, bool) {
	if get == nil {
		get = defaultSecretGetter
	}
	material, err := get(req, purpose, cache)
	if err != nil || strings.TrimSpace(material.Value) == "" {
		return pluginutil.SecretMaterial{}, false
	}
	return material, true
}
