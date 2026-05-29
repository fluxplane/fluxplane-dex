package main

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/plugins/asterisk"
)

func main() {
	pluginbinding.Serve(asterisk.NewPlugin())
}
