package main

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	jiraplugin "github.com/fluxplane/fluxplane-dex/plugins/jira"
)

func main() {
	pluginbinding.Serve(jiraplugin.NewPlugin())
}
