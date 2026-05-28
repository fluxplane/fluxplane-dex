package fluxplaneplugin

import (
	"context"
	"fmt"

	"github.com/fluxplane/fluxplane-core/core/policy"
	"github.com/fluxplane/fluxplane-core/core/resource"
	"github.com/fluxplane/fluxplane-core/orchestration/pluginhost"

	dex "github.com/fluxplane/fluxplane-dex"
)

// Bundles emits one resource.ContributionBundle per dex marketplace plugin so
// consumers can splice dex contributions into their product bundle path
// (e.g. coder's product.Bundle()) and have them flow through the standard
// surface/activation discovery alongside native plugins.
//
// Each bundle carries the plugin's operation specs, operation sets grouped
// by second dotted segment (e.g. "gitlab_mr" for every "gitlab.mr.*"),
// datasource specs, datasource entity declarations, the plugin's
// resource.PluginRef, and a stable resource.SourceRef tagged with the "dex"
// ecosystem so downstream catalogs can distinguish dex-sourced contributions.
//
// Plugins whose manifest can't be fetched — typically because the binary
// isn't installed in the active DEX_HOME — yield a stub bundle that still
// carries the PluginRef plus a warning Diagnostic. This keeps every
// marketplace entry discoverable for activation surfaces independent of
// install/auth state; the agent runs `dex plugin install <name>` (and any
// required `dex auth connect`) on demand before the ops will actually run.
//
// Bundles does NOT register executable bindings with a pluginhost. Pair it
// with Register so static contribution discovery and executable resolution
// stay on parallel rails.
func Bundles(ctx context.Context, engine *dex.Engine) ([]resource.ContributionBundle, error) {
	if engine == nil {
		return nil, fmt.Errorf("fluxplaneplugin: engine is nil")
	}
	entries := engine.Marketplace().Plugins()
	out := make([]resource.ContributionBundle, 0, len(entries))
	for _, entry := range entries {
		out = append(out, bundleFor(ctx, engine, entry.Name))
	}
	return out, nil
}

// Register adds one pluginhost.Plugin per dex marketplace plugin to host so
// the host can resolve executable bindings for dex operations and
// datasources. Pair with Bundles for the static contribution side.
//
// Returns an error if any plugin's name conflicts with an existing
// registration (pluginhost.Register is strict on duplicates) so wiring bugs
// fail loudly at startup rather than silently dropping a plugin.
func Register(engine *dex.Engine, host *pluginhost.Host) error {
	if engine == nil {
		return fmt.Errorf("fluxplaneplugin: engine is nil")
	}
	if host == nil {
		return fmt.Errorf("fluxplaneplugin: host is nil")
	}
	plugins, err := All(engine)
	if err != nil {
		return err
	}
	for _, p := range plugins {
		if err := host.Register(p); err != nil {
			return fmt.Errorf("fluxplaneplugin: register %q: %w", p.Manifest().Name, err)
		}
	}
	return nil
}

func bundleFor(ctx context.Context, engine *dex.Engine, name string) resource.ContributionBundle {
	plug, err := Wrap(engine, name)
	if err != nil {
		return stubBundle(name, err.Error())
	}
	bundle, err := plug.Contributions(ctx, pluginhost.Context{Ref: resource.PluginRef{Name: name}})
	if err != nil {
		return stubBundle(name, fmt.Sprintf("manifest unavailable: %v", err))
	}
	if !hasPluginRef(bundle.Plugins, name) {
		bundle.Plugins = append(bundle.Plugins, resource.PluginRef{Name: name})
	}
	if bundle.Source.ID == "" {
		bundle.Source = dexSource(name)
	}
	return bundle
}

func stubBundle(name, reason string) resource.ContributionBundle {
	source := dexSource(name)
	return resource.ContributionBundle{
		Source:  source,
		Plugins: []resource.PluginRef{{Name: name}},
		Diagnostics: []resource.Diagnostic{{
			Severity: resource.SeverityWarning,
			Source:   source,
			Message:  fmt.Sprintf("dex plugin %q: %s", name, reason),
		}},
	}
}

func dexSource(name string) resource.SourceRef {
	return resource.SourceRef{
		ID:        "dex-plugin:" + name,
		Ecosystem: "dex",
		Scope:     resource.ScopeEmbedded,
		Location:  "dex/" + name,
		Trust: policy.Trust{
			Kind:  policy.TrustSource,
			Level: policy.TrustVerified,
		},
	}
}

func hasPluginRef(refs []resource.PluginRef, name string) bool {
	for _, ref := range refs {
		if ref.Name == name {
			return true
		}
	}
	return false
}
