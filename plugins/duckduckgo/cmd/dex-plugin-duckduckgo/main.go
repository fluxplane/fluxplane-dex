package main

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	duckduckgoplugin "github.com/fluxplane/fluxplane-dex/plugins/duckduckgo"
)

func main() {
	pluginbinding.Serve(duckduckgoplugin.NewPlugin())
}
