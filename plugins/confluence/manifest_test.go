package confluence

import (
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func TestManifestQuality(t *testing.T) {
	plugintest.AssertManifestQuality(t, Manifest())
}

func TestManifestDeclaresSharedAtlassianEnvFallbacks(t *testing.T) {
	manifest := Manifest()
	if manifest.Metadata[pluginbinding.ManifestProtocolKey] != protocol.Version {
		t.Fatalf("protocol metadata = %#v", manifest.Metadata)
	}
	if len(manifest.Auth) != 1 {
		t.Fatalf("auth = %#v", manifest.Auth)
	}
	fields := map[string]core.AuthField{}
	for _, field := range manifest.Auth[0].Fields {
		fields[field.Name] = field
	}
	if got := fields[AuthPurposeAPIToken].Env; len(got) != 2 || got[0] != EnvConfluenceAPIToken || got[1] != EnvAtlassianAPIToken {
		t.Fatalf("token env = %#v", got)
	}
	if len(fields) != 1 {
		t.Fatalf("auth fields = %#v", fields)
	}
	byEntity := map[string]core.DatasourceSpec{}
	for _, datasource := range manifest.Datasources {
		byEntity[datasource.Entity] = datasource
	}
	if byEntity[EntityPage].Fallback != core.DatasourceFallbackHostIndexFirst {
		t.Fatalf("page fallback = %q", byEntity[EntityPage].Fallback)
	}
}
