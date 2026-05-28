package main

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/plugins/grafana"
)

func main() {
	pluginbinding.Serve(grafana.NewPlugin())
}
