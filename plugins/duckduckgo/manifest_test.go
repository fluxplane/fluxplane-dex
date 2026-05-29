package duckduckgo

import (
	"testing"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func TestManifestQuality(t *testing.T) {
	manifest := Manifest()
	if manifest.Metadata[pluginbinding.ManifestProtocolKey] != protocol.Version {
		t.Fatalf("protocol metadata = %#v", manifest.Metadata)
	}
	plugintest.AssertManifestQuality(t, manifest)
}
