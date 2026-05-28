package main

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/plugins/prometheus"
)

func main() {
	pluginbinding.Serve(prometheus.NewPlugin())
}
