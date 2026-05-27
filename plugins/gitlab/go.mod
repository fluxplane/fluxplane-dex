module github.com/fluxplane/fluxplane-dex/plugins/gitlab

go 1.26

require (
	github.com/fluxplane/fluxplane-dex v0.0.0
	gitlab.com/gitlab-org/api/client-go/v2 v2.34.0
)

require (
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.8 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/time v0.14.0 // indirect
)

replace github.com/fluxplane/fluxplane-dex => ../..
