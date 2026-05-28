package confluence

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/internal/atlassian"
)

func TestLiveClientCurrentUserHitsCurrentEndpoint(t *testing.T) {
	var seen *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"accountId":"acct-1","displayName":"Ada"}`))
	}))
	defer server.Close()

	client, err := NewLiveClient(atlassian.Credentials{BaseURL: server.URL, Token: pluginbinding.SecretMaterial{Value: "tok"}})
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
	if seen.URL.Path != "/wiki/rest/api/user/current" {
		t.Fatalf("path = %q", seen.URL.Path)
	}
	if got := seen.Header.Get("Authorization"); got != "Bearer tok" {
		t.Fatalf("auth = %q", got)
	}
}

func TestLiveClientGetPageDoesNotFetchAttachments(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"123","title":"Runbook"}`))
	}))
	defer server.Close()

	client, _ := NewLiveClient(atlassian.Credentials{BaseURL: server.URL, Token: pluginbinding.SecretMaterial{Value: "tok"}})
	page, err := client.GetPage(context.Background(), "123")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if page.ID != "123" {
		t.Fatalf("page = %#v", page)
	}
	if len(paths) != 1 {
		t.Fatalf("paths = %v (expected GetPage to not also list attachments)", paths)
	}
}

func TestLiveClientUploadPageAttachmentSendsMultipart(t *testing.T) {
	var (
		gotPath    string
		gotXAtlas  string
		gotForm    string
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
		part, _ := reader.NextPart()
		defer part.Close()
		if part.FileName() != "chart.png" {
			t.Errorf("filename = %q", part.FileName())
		}
		body, _ := io.ReadAll(part)
		gotContent = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[{"id":"A1","title":"chart.png"}]}`))
	}))
	defer server.Close()

	client, _ := NewLiveClient(atlassian.Credentials{BaseURL: server.URL, Token: pluginbinding.SecretMaterial{Value: "tok"}})
	out, err := client.UploadPageAttachment(context.Background(), "123", AttachmentUploadRequest{Filename: "chart.png", ContentType: "image/png", Data: []byte("png")})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !out.OK || out.PageID != "123" || len(out.Attachments) != 1 || out.Attachments[0].ID != "A1" {
		t.Fatalf("out = %#v", out)
	}
	if gotPath != "/wiki/rest/api/content/123/child/attachment" {
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
