package grafana

import (
	"fmt"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type target struct {
	EndpointRef string
	URL         string
}

func (s Service) target(ctx pluginbinding.Context, input GrafanaTargetInput) (target, error) {
	_ = s
	_ = ctx
	out := target{
		EndpointRef: strings.TrimSpace(input.EndpointRef),
	}
	if out.EndpointRef == "" {
		return target{}, fmt.Errorf("endpoint_ref is required")
	}
	out.URL = out.EndpointRef
	return out, nil
}
