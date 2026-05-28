package main

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/plugins/kubernetes"
)

func main() {
	pluginbinding.Serve(kubernetes.NewPlugin())
}
