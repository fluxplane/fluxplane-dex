package openai

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
	DefaultBaseURL  = "https://api.openai.com/v1"
	EnvBaseURL      = "OPENAI_BASE_URL"
	EnvOrganization = "OPENAI_ORGANIZATION"
	EnvProject      = "OPENAI_PROJECT"
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Client struct {
	HTTPClient   HTTPDoer
	BaseURL      string
	APIKey       string
	Organization string
	Project      string
}

func NewClient() Client {
	return Client{
		HTTPClient:   http.DefaultClient,
		BaseURL:      baseURLFromEnv(),
		Organization: strings.TrimSpace(os.Getenv(EnvOrganization)),
		Project:      strings.TrimSpace(os.Getenv(EnvProject)),
	}
}

func baseURLFromEnv() string {
	if value := strings.TrimSpace(os.Getenv(EnvBaseURL)); value != "" {
		return normalizeBaseURL(value)
	}
	return DefaultBaseURL
}

func normalizeBaseURL(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return DefaultBaseURL
	}
	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		value = "https://" + value
	}
	return value
}

func (c Client) baseURL() string {
	if strings.TrimSpace(c.BaseURL) == "" {
		return baseURLFromEnv()
	}
	return strings.TrimRight(c.BaseURL, "/")
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
	key := strings.TrimSpace(c.APIKey)
	if key == "" {
		return fmt.Errorf("openai: api key is required")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("User-Agent", "fluxplane-dex/0.1")
	if c.Organization != "" {
		req.Header.Set("OpenAI-Organization", c.Organization)
	}
	if c.Project != "" {
		req.Header.Set("OpenAI-Project", c.Project)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024*1024))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("openai %s %s: %s: %s", req.Method, req.URL.Path, resp.Status, openaiErrorMessage(data))
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode openai response: %w", err)
	}
	return nil
}

func openaiErrorMessage(body []byte) string {
	var decoded struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &decoded); err == nil && strings.TrimSpace(decoded.Error.Message) != "" {
		return decoded.Error.Message
	}
	return strings.TrimSpace(string(body))
}
