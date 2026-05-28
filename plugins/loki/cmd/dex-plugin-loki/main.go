package main

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/plugins/loki"
)

func main() {
	pluginbinding.Serve(loki.NewPlugin())
}
