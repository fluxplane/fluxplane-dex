package grafana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	Token      string
	Username   string
	Password   string
	HTTPClient *http.Client
}

func (c Client) get(ctx context.Context, path string, values url.Values) (json.RawMessage, error) {
	return c.request(ctx, http.MethodGet, path, values, nil)
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
	return c.request(ctx, http.MethodPost, path, values, payload)
}

func (c Client) delete(ctx context.Context, path string, values url.Values) (json.RawMessage, error) {
	return c.request(ctx, http.MethodDelete, path, values, nil)
}

func (c Client) request(ctx context.Context, method, path string, values url.Values, payload []byte) (json.RawMessage, error) {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("grafana url is required")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	endpoint := base + path
	if len(values) > 0 {
		endpoint += "?" + values.Encode()
	}
	var requestBody io.Reader
	if len(payload) > 0 {
		requestBody = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, requestBody)
	if err != nil {
		return nil, err
	}
	if len(payload) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	c.authorize(req)
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("grafana returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.RawMessage(body), nil
}

func (c Client) authorize(req *http.Request) {
	if strings.TrimSpace(c.Token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Token))
		return
	}
	if strings.TrimSpace(c.Username) != "" || strings.TrimSpace(c.Password) != "" {
		req.SetBasicAuth(c.Username, c.Password)
	}
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
