package grafana

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

func (c Client) get(ctx context.Context, path string, values url.Values) (json.RawMessage, error) {
	return c.request(ctx, "GET", path, values, nil)
}

func (c Client) postJSON(ctx context.Context, path string, values url.Values, body any) (json.RawMessage, error) {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	return c.request(ctx, "POST", path, values, payload)
}

func (c Client) delete(ctx context.Context, path string, values url.Values) (json.RawMessage, error) {
	return c.request(ctx, "DELETE", path, values, nil)
}

func (c Client) request(ctx context.Context, method, path string, values url.Values, payload []byte) (json.RawMessage, error) {
	_ = ctx
	endpointRef := strings.TrimSpace(c.EndpointRef)
	if endpointRef == "" {
		return nil, fmt.Errorf("grafana endpoint_ref is required")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	headers := map[string]string{}
	if len(payload) > 0 {
		headers["Content-Type"] = "application/json"
	}
	resp, err := c.Host.HTTP(pluginbinding.HTTPRequest{
		EndpointRef: endpointRef,
		Path:        path,
		Query:       map[string][]string(values),
		Method:      method,
		Headers:     headers,
		Body:        payload,
		Auth: &pluginbinding.HTTPAuthRequest{
			BearerTokenPurpose: AuthPurposeAPIToken,
			UsernamePurpose:    AuthPurposeUsername,
			PasswordPurpose:    AuthPurposePassword,
		},
		TimeoutMS: 30000,
		MaxBytes:  32 * 1024 * 1024,
		UserAgent: "fluxplane-dex/0.1",
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("grafana returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(resp.Body)))
	}
	return json.RawMessage(resp.Body), nil
}

func grafanaProxyPath(uid, nativePath string) string {
	uid = strings.TrimSpace(uid)
	nativePath = strings.TrimSpace(nativePath)
	if nativePath == "" {
		nativePath = "/"
	}
	if !strings.HasPrefix(nativePath, "/") {
		nativePath = "/" + nativePath
	}
	return "/api/datasources/proxy/uid/" + url.PathEscape(uid) + nativePath
}
