package atlassian

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

// SecretConfig describes how a plugin resolves Atlassian Cloud credentials.
// Product is both the URL path segment (api.atlassian.com/ex/{Product}/{CloudID})
// and the human-facing label.
type SecretConfig struct {
	Product        string
	TokenPurpose   string
	CloudIDPurpose string
}

type Credentials struct {
	Product string
	Token   pluginbinding.SecretMaterial
	CloudID pluginbinding.SecretMaterial
	BaseURL string
}

func ResolveCredentials(ctx pluginbinding.Context, cfg SecretConfig) (Credentials, error) {
	token, err := ctx.RequiredSecret(cfg.TokenPurpose)
	if err != nil {
		return Credentials{}, err
	}
	cloudID, err := ctx.RequiredSecret(cfg.CloudIDPurpose)
	if err != nil {
		return Credentials{}, err
	}
	baseURL, err := CloudGatewayURL(cfg.Product, cloudID.Value)
	if err != nil {
		return Credentials{}, err
	}
	return Credentials{
		Product: strings.TrimSpace(cfg.Product),
		Token:   token,
		CloudID: cloudID,
		BaseURL: baseURL,
	}, nil
}

func CloudGatewayURL(product, cloudID string) (string, error) {
	product = strings.Trim(strings.TrimSpace(product), "/")
	cloudID = strings.TrimSpace(cloudID)
	if product == "" {
		return "", fmt.Errorf("gateway product is empty")
	}
	if cloudID == "" {
		return "", fmt.Errorf("cloud_id is empty")
	}
	return "https://api.atlassian.com/ex/" + url.PathEscape(product) + "/" + url.PathEscape(cloudID), nil
}

func BearerAuthHeader(token string) string {
	return "Bearer " + strings.TrimSpace(token)
}

type Client struct {
	Credentials Credentials
	HTTPClient  *http.Client
}

func NewClient(credentials Credentials) Client {
	return Client{Credentials: credentials, HTTPClient: http.DefaultClient}
}

func (c Client) GetJSON(ctx context.Context, path string, query url.Values, out any) error {
	return c.DoJSON(ctx, http.MethodGet, path, query, nil, out)
}

func (c Client) DoJSON(ctx context.Context, method, path string, query url.Values, body io.Reader, out any) error {
	headers := map[string]string{"Accept": "application/json"}
	if body != nil {
		headers["Content-Type"] = "application/json"
	}
	return c.Do(ctx, method, path, query, body, headers, out)
}

func (c Client) Do(ctx context.Context, method, path string, query url.Values, body io.Reader, headers map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.url(path, query), body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", BearerAuthHeader(c.Credentials.Token.Value))
	for key, value := range headers {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			req.Header.Set(key, value)
		}
	}
	return c.DoRequest(req, out)
}

// MaxJSONResponseBytes caps the size of a JSON response body the client will
// decode. Atlassian REST endpoints return well under this; the cap is a
// defense against unbounded memory growth from a misbehaving server.
const MaxJSONResponseBytes = 64 * 1024 * 1024

// MaxAttachmentBytes caps the size of an attachment body the client will
// download. Matches Atlassian Cloud's largest published per-attachment limit.
const MaxAttachmentBytes = 256 * 1024 * 1024

func (c Client) DoRequest(req *http.Request, out any) error {
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
		return httpError(resp.StatusCode, data)
	}
	if out == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(io.LimitReader(resp.Body, MaxJSONResponseBytes)).Decode(out)
}

func (c Client) GetBytes(ctx context.Context, path string, query url.Values) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url(path, query), nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", BearerAuthHeader(c.Credentials.Token.Value))
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, "", httpError(resp.StatusCode, data)
	}
	return data, resp.Header.Get("Content-Type"), err
}

func (c Client) url(path string, query url.Values) string {
	if parsed, err := url.Parse(strings.TrimSpace(path)); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		if len(query) == 0 {
			return parsed.String()
		}
		values := parsed.Query()
		for key, entries := range query {
			for _, entry := range entries {
				values.Add(key, entry)
			}
		}
		parsed.RawQuery = values.Encode()
		return parsed.String()
	}
	base := strings.TrimRight(c.Credentials.BaseURL, "/")
	path = "/" + strings.TrimLeft(path, "/")
	if len(query) == 0 {
		return base + path
	}
	return base + path + "?" + query.Encode()
}

func httpError(status int, data []byte) error {
	message := strings.TrimSpace(string(data))
	var payload struct {
		Message       string   `json:"message"`
		ErrorMessages []string `json:"errorMessages"`
		Errors        any      `json:"errors"`
	}
	if err := json.Unmarshal(data, &payload); err == nil {
		switch {
		case strings.TrimSpace(payload.Message) != "":
			message = strings.TrimSpace(payload.Message)
		case len(payload.ErrorMessages) > 0:
			message = strings.Join(payload.ErrorMessages, "; ")
		}
	}
	if message == "" {
		message = http.StatusText(status)
	}
	return fmt.Errorf("atlassian API returned %d: %s", status, message)
}
