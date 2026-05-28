package main

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	slackplugin "github.com/fluxplane/fluxplane-dex/plugins/slack"
)

func main() {
	pluginbinding.Serve(slackplugin.NewPlugin())
}
