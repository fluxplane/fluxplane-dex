package main

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	gitlabplugin "github.com/fluxplane/fluxplane-dex/plugins/gitlab"
)

func main() {
	pluginbinding.Serve(gitlabplugin.NewPlugin())
}
