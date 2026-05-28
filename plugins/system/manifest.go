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
	ContextName   = "system.context"
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
		Context:     []core.ContextSpec{contextSpec()},
	}
}

func contextSpec() core.ContextSpec {
	return pluginbinding.ContextSpec(ContextName, "Local system context blocks.", pluginbinding.ContextKindText, pluginbinding.ContextKindData)
}

func infoSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InfoInput, InfoResult](
		OperationInfo,
		"Show local system information by category.",
		pluginbinding.ReadOnly(),
		pluginbinding.Compact(),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectLocalSystem),
		pluginbinding.Access(core.OperationAccessLocalSystem),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(core.OperationIdempotent),
	)
}
