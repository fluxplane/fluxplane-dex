package system

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

const (
	PluginName        = "system"
	PluginVersion     = "0.1.0"
	PluginDescription = "Local system information across OS, runtime, user, paths, CPU, time, environment, and network categories."

	OperationInfo = "system.info"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{"sys", PluginName},
		Operations:  []core.OperationSpec{infoSpec()},
	}
}

func infoSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InfoInput, InfoResult](OperationInfo, "Show local system information by category.", pluginbinding.ReadOnly(), pluginbinding.Compact())
}
