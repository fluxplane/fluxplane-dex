package main

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	confluenceplugin "github.com/fluxplane/fluxplane-dex/plugins/confluence"
)

func main() {
	pluginbinding.Serve(confluenceplugin.NewPlugin())
}
