package jira

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

func TestLiveClientCurrentUserHitsMyself(t *testing.T) {
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

	client, _ := NewLiveClient(atlassian.Credentials{BaseURL: server.URL, Token: pluginbinding.SecretMaterial{Value: "tok"}})
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

