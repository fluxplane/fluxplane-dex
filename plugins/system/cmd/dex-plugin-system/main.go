package main

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	systemplugin "github.com/fluxplane/fluxplane-dex/plugins/system"
)

func main() {
	pluginbinding.Serve(systemplugin.NewPlugin())
}
