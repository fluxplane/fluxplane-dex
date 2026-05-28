package main

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/plugins/sql"
)

func main() {
	pluginbinding.Serve(sql.NewPlugin())
}
