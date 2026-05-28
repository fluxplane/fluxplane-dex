package main

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	ollamaplugin "github.com/fluxplane/fluxplane-dex/plugins/ollama"
)

func main() {
	pluginbinding.Serve(ollamaplugin.NewPlugin())
}
