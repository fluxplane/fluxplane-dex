package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	DefaultEndpoint = "http://localhost:11434"
	EnvOllamaHost   = "OLLAMA_HOST"
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Client struct {
	HTTPClient HTTPDoer
	Endpoint   string
}

func NewClient() Client {
	return Client{HTTPClient: http.DefaultClient, Endpoint: defaultEndpointFromEnv()}
}

func defaultEndpointFromEnv() string {
	if value := strings.TrimSpace(os.Getenv(EnvOllamaHost)); value != "" {
		return normalizeEndpoint(value)
	}
	return DefaultEndpoint
}

func normalizeEndpoint(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return DefaultEndpoint
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	return "http://" + value
}

func (c Client) baseURL() string {
	endpoint := strings.TrimSpace(c.Endpoint)
	if endpoint == "" {
		endpoint = defaultEndpointFromEnv()
	}
	return strings.TrimRight(endpoint, "/")
}

func (c Client) httpClient() HTTPDoer {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL()+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c Client) post(ctx context.Context, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL()+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.do(req, out)
}

func (c Client) do(req *http.Request, out any) error {
	req.Header.Set("User-Agent", "fluxplane-dex/0.1")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ollama %s %s: %s: %s", req.Method, req.URL.Path, resp.Status, ollamaErrorMessage(data))
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
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
