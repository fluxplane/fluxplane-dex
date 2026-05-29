package ollama

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type Client struct {
	EndpointRef string
	Host        pluginbinding.HostClient
}

func NewClient() Client {
	return Client{}
}

func (c Client) get(path string, out any) error {
	return c.do("GET", path, nil, out)
}

func (c Client) post(path string, body any, out any) error {
	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	return c.do("POST", path, raw, out)
}

func (c Client) do(method, path string, body []byte, out any) error {
	endpointRef := strings.TrimSpace(c.EndpointRef)
	if endpointRef == "" {
		return fmt.Errorf("ollama endpoint_ref is required")
	}
	if c.Host == nil {
		return fmt.Errorf("host client is unavailable")
	}
	headers := map[string]string{}
	if len(body) > 0 {
		headers["Content-Type"] = "application/json"
	}
	resp, err := c.Host.HTTP(pluginbinding.HTTPRequest{
		EndpointRef: endpointRef,
		Path:        path,
		Method:      method,
		Headers:     headers,
		Body:        body,
		TimeoutMS:   30000,
		MaxBytes:    16 * 1024 * 1024,
		UserAgent:   "fluxplane-dex/0.1",
	})
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ollama %s %s: %s: %s", method, path, resp.Status, ollamaErrorMessage(resp.Body))
	}
	if out == nil || len(resp.Body) == 0 {
		return nil
	}
	if err := json.Unmarshal(resp.Body, out); err != nil {
		return fmt.Errorf("decode ollama response: %w", err)
	}
	return nil
}

func ollamaErrorMessage(body []byte) string {
	var decoded struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &decoded); err == nil && strings.TrimSpace(decoded.Error) != "" {
		return decoded.Error
	}
	return strings.TrimSpace(string(body))
}
