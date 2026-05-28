package main

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/plugins/docker"
)

func main() {
	pluginbinding.Serve(docker.NewPlugin())
}
