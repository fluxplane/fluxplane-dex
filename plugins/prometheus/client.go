package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type Client struct {
	EndpointRef string
	Host        pluginbinding.HostClient
}

type promResponse struct {
	Status    string          `json:"status"`
	Data      json.RawMessage `json:"data"`
	ErrorType string          `json:"errorType,omitempty"`
	Error     string          `json:"error,omitempty"`
}

func (c Client) get(ctx context.Context, path string, values url.Values) (json.RawMessage, error) {
	_ = ctx
	endpointRef := strings.TrimSpace(c.EndpointRef)
	if endpointRef == "" {
		return nil, fmt.Errorf("prometheus endpoint_ref is required")
	}
	resp, err := c.Host.HTTP(pluginbinding.HTTPRequest{
		EndpointRef: endpointRef,
		Path:        path,
		Query:       map[string][]string(values),
		Method:      "GET",
		TimeoutMS:   30000,
		MaxBytes:    32 * 1024 * 1024,
		UserAgent:   "fluxplane-dex/0.1",
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("prometheus returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(resp.Body)))
	}
	var envelope promResponse
	if err := json.Unmarshal(resp.Body, &envelope); err != nil {
		return nil, err
	}
	if envelope.Status != "success" {
		return nil, fmt.Errorf("prometheus error %s: %s", envelope.ErrorType, envelope.Error)
	}
	return envelope.Data, nil
}

func (c Client) ready(ctx context.Context) error {
	_ = ctx
	resp, err := c.Host.HTTP(pluginbinding.HTTPRequest{
		EndpointRef: c.EndpointRef,
		Path:        "/-/ready",
		Method:      "GET",
		TimeoutMS:   5000,
		MaxBytes:    64 * 1024,
		UserAgent:   "fluxplane-dex/0.1",
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("prometheus not ready, status %d", resp.StatusCode)
	}
	return nil
}
