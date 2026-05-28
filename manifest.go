package dex

import (
	"encoding/json"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func decodeManifest(resp protocol.Response) (core.PluginManifest, error) {
	var manifest core.PluginManifest
	if len(resp.Result) == 0 {
		return manifest, nil
	}
	if err := json.Unmarshal(resp.Result, &manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}
