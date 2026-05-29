package loki

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type Client struct {
	EndpointRef string
	Host        pluginbinding.HostClient
}

type lokiResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Stream map[string]string `json:"stream"`
			Values [][]string        `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

func (c Client) get(ctx context.Context, path string, values url.Values, out any) error {
	_ = ctx
	endpointRef := strings.TrimSpace(c.EndpointRef)
	if endpointRef == "" {
		return fmt.Errorf("loki endpoint_ref is required")
	}
	resp, err := c.Host.HTTP(pluginbinding.HTTPRequest{
		EndpointRef: endpointRef,
		Path:        path,
		Query:       map[string][]string(values),
		Method:      "GET",
		Auth: &pluginbinding.HTTPAuthRequest{
			HeaderPurposes: map[string]string{"X-Scope-OrgID": AuthPurposeTenantID},
		},
		TimeoutMS: 30000,
		MaxBytes:  32 * 1024 * 1024,
		UserAgent: "fluxplane-dex/0.1",
	})
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("loki returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(resp.Body)))
	}
	return json.Unmarshal(resp.Body, out)
}

func (c Client) ready(ctx context.Context) error {
	_ = ctx
	resp, err := c.Host.HTTP(pluginbinding.HTTPRequest{
		EndpointRef: strings.TrimSpace(c.EndpointRef),
		Path:        "/ready",
		Method:      "GET",
		Auth: &pluginbinding.HTTPAuthRequest{
			HeaderPurposes: map[string]string{"X-Scope-OrgID": AuthPurposeTenantID},
		},
		TimeoutMS: 5000,
		MaxBytes:  64 * 1024,
		UserAgent: "fluxplane-dex/0.1",
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("loki not ready, status %d", resp.StatusCode)
	}
	return nil
}

func parseLogTimestamp(value string) time.Time {
	ns, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(0, ns).UTC()
}
