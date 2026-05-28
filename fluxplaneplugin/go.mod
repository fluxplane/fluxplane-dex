module github.com/fluxplane/fluxplane-dex/fluxplaneplugin

go 1.26.1

require (
	github.com/fluxplane/fluxplane-core v0.0.0-00010101000000-000000000000
	github.com/fluxplane/fluxplane-dex v0.9.0
)

require (
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/buger/jsonparser v1.2.0 // indirect
	github.com/invopop/jsonschema v0.14.0 // indirect
	github.com/pb33f/ordered-map/v2 v2.3.1 // indirect
	go.yaml.in/yaml/v4 v4.0.0-rc.2 // indirect
)

replace (
	github.com/fluxplane/fluxplane-core => ../../fluxplane-core
	github.com/fluxplane/fluxplane-dex => ..
)
