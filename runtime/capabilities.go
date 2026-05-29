package runtime

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type LocalCapabilityHost struct {
	Root      string
	Providers map[string]HostProvider
	Client    *http.Client
}

func NewLocalCapabilityHost(root string, providers map[string]HostProvider) LocalCapabilityHost {
	return LocalCapabilityHost{Root: root, Providers: providers}
}

func (h LocalCapabilityHost) HTTP(ctx context.Context, input pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	method := strings.ToUpper(strings.TrimSpace(input.Method))
	if method == "" {
		method = http.MethodGet
	}
	if !allowedHTTPMethod(method) {
		return pluginbinding.HTTPResponse{}, fmt.Errorf("unsupported HTTP method %q", method)
	}
	timeout := time.Duration(input.TimeoutMS) * time.Millisecond
	if timeout <= 0 || timeout > time.Minute {
		timeout = 30 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	endpoint, err := httpRequestURL(input)
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	req, err := http.NewRequestWithContext(reqCtx, method, endpoint, strings.NewReader(string(input.Body)))
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	if input.UserAgent != "" {
		req.Header.Set("User-Agent", input.UserAgent)
	}
	for key, value := range input.Headers {
		req.Header.Set(key, value)
	}
	client := h.Client
	if client == nil {
		client = http.DefaultClient
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	maxBytes := input.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 512 * 1024
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxBytes)+1))
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	truncated := len(body) > maxBytes
	if truncated {
		body = body[:maxBytes]
	}
	return pluginbinding.HTTPResponse{
		URL:         endpoint,
		FinalURL:    resp.Request.URL.String(),
		Method:      method,
		Status:      resp.Status,
		StatusCode:  resp.StatusCode,
		Headers:     resp.Header,
		ContentType: resp.Header.Get("Content-Type"),
		Body:        body,
		Truncated:   truncated,
		DurationMS:  int64(time.Since(start) / time.Millisecond),
	}, nil
}

func httpRequestURL(input pluginbinding.HTTPRequest) (string, error) {
	endpoint := strings.TrimSpace(input.URL)
	if endpoint == "" {
		return "", fmt.Errorf("HTTP url is required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(input.Path) != "" {
		basePath := strings.TrimRight(parsed.EscapedPath(), "/")
		relPath := strings.TrimLeft(strings.TrimSpace(input.Path), "/")
		parsed.Path = basePath + "/" + relPath
	}
	query := parsed.Query()
	for key, values := range input.Query {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (h LocalCapabilityHost) BlobRead(_ context.Context, input pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error) {
	path, rel, err := h.resolveExisting(blobPath(input.Ref, input.Path))
	if err != nil {
		return pluginbinding.BlobReadResponse{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return pluginbinding.BlobReadResponse{}, err
	}
	if info.IsDir() {
		return pluginbinding.BlobReadResponse{}, fmt.Errorf("blob path is a directory")
	}
	maxBytes := input.MaxBytes
	if maxBytes <= 0 {
		maxBytes = info.Size()
	}
	file, err := os.Open(path)
	if err != nil {
		return pluginbinding.BlobReadResponse{}, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return pluginbinding.BlobReadResponse{}, err
	}
	truncated := int64(len(data)) > maxBytes
	if truncated {
		data = data[:maxBytes]
	}
	return pluginbinding.BlobReadResponse{
		Blob:      pluginbinding.BlobRef{Ref: "workspace:" + rel, Path: rel, Filename: filepath.Base(rel), Size: info.Size()},
		Content:   data,
		Truncated: truncated,
	}, nil
}

func (h LocalCapabilityHost) BlobWrite(_ context.Context, input pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error) {
	path, rel, err := h.resolveCreate(blobPath(input.Ref, input.Path))
	if err != nil {
		return pluginbinding.BlobRef{}, err
	}
	if !input.Overwrite {
		if _, err := os.Lstat(path); err == nil {
			return pluginbinding.BlobRef{}, fmt.Errorf("blob path already exists")
		} else if !os.IsNotExist(err) {
			return pluginbinding.BlobRef{}, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return pluginbinding.BlobRef{}, err
	}
	if err := os.WriteFile(path, input.Content, 0o644); err != nil {
		return pluginbinding.BlobRef{}, err
	}
	return pluginbinding.BlobRef{
		Ref:       "workspace:" + rel,
		Path:      rel,
		Filename:  firstNonEmpty(strings.TrimSpace(input.Filename), filepath.Base(rel)),
		MediaType: strings.TrimSpace(input.MediaType),
		Size:      int64(len(input.Content)),
		Metadata:  cloneStringMap(input.Metadata),
	}, nil
}

func (h LocalCapabilityHost) BlobInfo(_ context.Context, input pluginbinding.BlobInfoRequest) (pluginbinding.BlobRef, error) {
	path, rel, err := h.resolveExisting(blobPath(input.Ref, input.Path))
	if err != nil {
		return pluginbinding.BlobRef{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return pluginbinding.BlobRef{}, err
	}
	return pluginbinding.BlobRef{Ref: "workspace:" + rel, Path: rel, Filename: filepath.Base(rel), Size: info.Size()}, nil
}

func (h LocalCapabilityHost) EnvLookup(_ context.Context, input pluginbinding.EnvLookupRequest) (pluginbinding.EnvLookupResponse, error) {
	key := strings.TrimSpace(input.Key)
	if key == "" {
		return pluginbinding.EnvLookupResponse{}, fmt.Errorf("environment key is empty")
	}
	value, ok := os.LookupEnv(key)
	return pluginbinding.EnvLookupResponse{Key: key, Value: value, Found: ok}, nil
}

func (h LocalCapabilityHost) ProviderCall(ctx context.Context, input pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	providerName := strings.TrimSpace(input.Provider)
	if providerName == "" {
		return pluginbinding.ProviderCallResponse{}, fmt.Errorf("provider is required")
	}
	provider := h.Providers[providerName]
	if provider == nil && providerName == systemProviderName {
		provider = localSystemProvider{}
	}
	if provider == nil {
		return pluginbinding.ProviderCallResponse{}, fmt.Errorf("host provider %q is unavailable", providerName)
	}
	result, err := provider.Call(ctx, strings.TrimSpace(input.Action), input.Payload)
	if err != nil {
		return pluginbinding.ProviderCallResponse{}, err
	}
	return pluginbinding.ProviderCallResponse{Result: result}, nil
}

func (h LocalCapabilityHost) resolveExisting(raw string) (string, string, error) {
	root, err := h.root()
	if err != nil {
		return "", "", err
	}
	candidate, err := h.resolveCandidate(root, raw)
	if err != nil {
		return "", "", err
	}
	real, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", "", err
	}
	rel, err := relPath(root, real)
	if err != nil {
		return "", "", err
	}
	return real, rel, nil
}

func (h LocalCapabilityHost) resolveCreate(raw string) (string, string, error) {
	root, err := h.root()
	if err != nil {
		return "", "", err
	}
	candidate, err := h.resolveCandidate(root, raw)
	if err != nil {
		return "", "", err
	}
	parent := filepath.Dir(candidate)
	realParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		if os.IsNotExist(err) {
			realParent = parent
		} else {
			return "", "", err
		}
	}
	if err := ensureInside(root, realParent); err != nil {
		return "", "", err
	}
	rel, err := relPath(root, candidate)
	if err != nil {
		return "", "", err
	}
	return candidate, rel, nil
}

func (h LocalCapabilityHost) resolveCandidate(root, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("blob path is empty")
	}
	if filepath.IsAbs(raw) {
		clean := filepath.Clean(raw)
		if err := ensureInside(root, clean); err != nil {
			return "", err
		}
		return clean, nil
	}
	candidate := filepath.Join(root, filepath.Clean(raw))
	if err := ensureInside(root, candidate); err != nil {
		return "", err
	}
	return candidate, nil
}

func (h LocalCapabilityHost) root() (string, error) {
	root := strings.TrimSpace(h.Root)
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		root = wd
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return real, nil
}

func blobPath(ref, path string) string {
	ref = strings.TrimSpace(ref)
	if strings.HasPrefix(ref, "workspace:") {
		return strings.TrimSpace(strings.TrimPrefix(ref, "workspace:"))
	}
	if ref != "" {
		return ref
	}
	return strings.TrimSpace(path)
}

func ensureInside(root, candidate string) error {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return err
	}
	if rel == "." || rel == "" {
		return nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("blob path escapes workspace root")
	}
	return nil
}

func relPath(root, path string) (string, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

func allowedHTTPMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

var _ CapabilityHost = LocalCapabilityHost{}
