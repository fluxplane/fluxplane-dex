package main

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	openaiplugin "github.com/fluxplane/fluxplane-dex/plugins/openai"
)

func main() {
	pluginbinding.Serve(openaiplugin.NewPlugin())
}
