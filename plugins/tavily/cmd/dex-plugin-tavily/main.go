package main

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	tavilyplugin "github.com/fluxplane/fluxplane-dex/plugins/tavily"
)

func main() {
	pluginbinding.Serve(tavilyplugin.NewPlugin())
}
