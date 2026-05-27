package main

import (
	slackplugin "github.com/fluxplane/fluxplane-dex/plugins/slack"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func main() {
	protocol.Serve(slackplugin.Handle)
}
