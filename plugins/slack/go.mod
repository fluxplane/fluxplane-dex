module github.com/fluxplane/fluxplane-dex/plugins/slack

go 1.26

require (
	github.com/fluxplane/fluxplane-dex v0.0.0
	github.com/slack-go/slack v0.24.0
)

require github.com/gorilla/websocket v1.5.3 // indirect

replace github.com/fluxplane/fluxplane-dex => ../..
