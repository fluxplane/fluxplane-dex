package jira

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

func TestLiveClientCurrentUserHitsMyself(t *testing.T) {
	var seen *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"accountId":"acct-1","displayName":"Ada"}`))
	}))
	defer server.Close()

	client, err := NewLiveClient(pluginbinding.Context{Host: liveClientTestHost{t: t, baseURL: server.URL}}, "jira-dev")
	if err != nil {
		t.Fatalf("client err = %v", err)
	}
	user, err := client.CurrentUser(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if user.AccountID != "acct-1" {
		t.Fatalf("user = %#v", user)
	}
	if seen.URL.Path != "/rest/api/3/myself" {
		t.Fatalf("path = %q", seen.URL.Path)
	}
	if got := seen.Header.Get("Authorization"); got != "Bearer tok" {
		t.Fatalf("auth = %q", got)
	}
}

func TestLiveClientUploadIssueAttachmentSendsMultipart(t *testing.T) {
	var (
		gotPath    string
		gotForm    string
		gotXAtlas  string
		gotContent string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotXAtlas = r.Header.Get("X-Atlassian-Token")
		gotForm = r.Header.Get("Content-Type")
		mediaType, params, err := mime.ParseMediaType(gotForm)
		if err != nil || mediaType != "multipart/form-data" {
			t.Errorf("media type = %s", gotForm)
		}
		reader := multipart.NewReader(r.Body, params["boundary"])
		part, err := reader.NextPart()
		if err != nil {
			t.Errorf("read part = %v", err)
			return
		}
		defer part.Close()
		if part.FileName() != "chart.png" {
			t.Errorf("filename = %q", part.FileName())
		}
		body, _ := io.ReadAll(part)
		gotContent = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":"A1","filename":"chart.png"}]`))
	}))
	defer server.Close()

	client, _ := NewLiveClient(pluginbinding.Context{Host: liveClientTestHost{t: t, baseURL: server.URL}}, "jira-dev")
	out, err := client.UploadIssueAttachment(context.Background(), "DEX-9", AttachmentUploadRequest{Filename: "chart.png", ContentType: "image/png", Data: []byte("png")})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !out.OK || len(out.Attachments) != 1 || out.Attachments[0].ID != "A1" {
		t.Fatalf("out = %#v", out)
	}
	if gotPath != "/rest/api/3/issue/DEX-9/attachments" {
		t.Fatalf("path = %q", gotPath)
	}
	if !strings.HasPrefix(gotForm, "multipart/form-data") {
		t.Fatalf("content-type = %q", gotForm)
	}
	if gotXAtlas != "no-check" {
		t.Fatalf("x-atlassian-token = %q", gotXAtlas)
	}
	if gotContent != "png" {
		t.Fatalf("body = %q", gotContent)
	}
}

type liveClientTestHost struct {
	t       *testing.T
	baseURL string
}

func (h liveClientTestHost) Secret(string) (pluginbinding.SecretMaterial, error) {
	return pluginbinding.SecretMaterial{}, errors.New("unexpected secret call")
}

func (h liveClientTestHost) Lookup(pluginbinding.DatasourceLookupInput) (pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]], error) {
	return pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]{}, errors.New("unexpected lookup call")
}

func (h liveClientTestHost) Search(pluginbinding.DatasourceSearchInput) (pluginbinding.DatasourceSearchResult[any], error) {
	return pluginbinding.DatasourceSearchResult[any]{}, errors.New("unexpected search call")
}

func (h liveClientTestHost) Get(pluginbinding.DatasourceGetInput) (pluginbinding.DatasourceGetResult[any], error) {
	return pluginbinding.DatasourceGetResult[any]{}, errors.New("unexpected get call")
}

func (h liveClientTestHost) ResolveEndpoint(string) (core.EndpointRef, error) {
	return core.EndpointRef{}, errors.New("unexpected endpoint call")
}

func (h liveClientTestHost) HTTP(input pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	if input.EndpointRef != "jira-dev" {
		h.t.Fatalf("endpoint_ref = %q", input.EndpointRef)
	}
	if input.Auth == nil || input.Auth.BearerTokenPurpose != AuthPurposeAPIToken {
		h.t.Fatalf("auth = %#v", input.Auth)
	}
	base, err := url.Parse(h.baseURL)
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/" + strings.TrimLeft(input.Path, "/")
	base.RawQuery = url.Values(input.Query).Encode()
	method := strings.TrimSpace(input.Method)
	if method == "" {
		method = "GET"
	}
	req, err := http.NewRequest(method, base.String(), bytes.NewReader(input.Body))
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	for key, value := range input.Headers {
		req.Header.Set(key, value)
	}
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	return pluginbinding.HTTPResponse{
		URL:         base.String(),
		FinalURL:    resp.Request.URL.String(),
		Method:      method,
		Status:      resp.Status,
		StatusCode:  resp.StatusCode,
		Headers:     resp.Header,
		ContentType: resp.Header.Get("Content-Type"),
		Body:        body,
	}, nil
}

func (h liveClientTestHost) BlobRead(pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error) {
	return pluginbinding.BlobReadResponse{}, errors.New("unexpected blob read")
}

func (h liveClientTestHost) BlobWrite(pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, errors.New("unexpected blob write")
}

func (h liveClientTestHost) BlobInfo(pluginbinding.BlobInfoRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, errors.New("unexpected blob info")
}

func (h liveClientTestHost) EnvLookup(string) (pluginbinding.EnvLookupResponse, error) {
	return pluginbinding.EnvLookupResponse{}, errors.New("unexpected env lookup")
}

func (h liveClientTestHost) CapabilityCall(pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	return pluginbinding.ProviderCallResponse{}, errors.New("unexpected provider call")
}
