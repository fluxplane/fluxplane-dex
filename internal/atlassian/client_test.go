package atlassian

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

func TestHTTPErrorDecodesAtlassianMessage(t *testing.T) {
	err := httpError(404, []byte(`{"message":"issue does not exist"}`))
	if err == nil || !strings.Contains(err.Error(), "issue does not exist") {
		t.Fatalf("err = %v", err)
	}
}

func TestHTTPErrorJoinsErrorMessages(t *testing.T) {
	err := httpError(400, []byte(`{"errorMessages":["bad project","missing summary"]}`))
	if err == nil || !strings.Contains(err.Error(), "bad project; missing summary") {
		t.Fatalf("err = %v", err)
	}
}

func TestHTTPErrorFallsBackToStatusText(t *testing.T) {
	err := httpError(503, nil)
	if err == nil || !strings.Contains(err.Error(), "Service Unavailable") {
		t.Fatalf("err = %v", err)
	}
}

func TestClientURLMergesQueryWithAbsolutePath(t *testing.T) {
	c := Client{Credentials: Credentials{BaseURL: "https://api.atlassian.com/ex/jira/cloud-1"}}
	q := url.Values{}
	q.Set("page", "2")
	got := c.url("https://example.com/rest?fixed=1", q)
	if !strings.Contains(got, "fixed=1") || !strings.Contains(got, "page=2") {
		t.Fatalf("url = %s", got)
	}
}

func TestClientURLJoinsBaseAndPath(t *testing.T) {
	c := Client{Credentials: Credentials{BaseURL: "https://api.atlassian.com/ex/jira/cloud-1/"}}
	got := c.url("/rest/api/3/myself", nil)
	if got != "https://api.atlassian.com/ex/jira/cloud-1/rest/api/3/myself" {
		t.Fatalf("url = %s", got)
	}
}

func TestDoJSONSendsBearerAndDecodesBody(t *testing.T) {
	var seen *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"hello":"world"}`))
	}))
	defer server.Close()
	c := Client{Credentials: Credentials{BaseURL: server.URL, Token: pluginbinding.SecretMaterial{Value: "tok"}}}
	var out map[string]string
	if err := c.GetJSON(context.Background(), "/probe", url.Values{"x": []string{"1"}}, &out); err != nil {
		t.Fatalf("err = %v", err)
	}
	if out["hello"] != "world" {
		t.Fatalf("out = %v", out)
	}
	if got := seen.Header.Get("Authorization"); got != "Bearer tok" {
		t.Fatalf("auth header = %q", got)
	}
	if seen.URL.Query().Get("x") != "1" {
		t.Fatalf("query = %s", seen.URL.RawQuery)
	}
}

func TestDoJSONReturnsServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"bad token"}`))
	}))
	defer server.Close()
	c := Client{Credentials: Credentials{BaseURL: server.URL, Token: pluginbinding.SecretMaterial{Value: "tok"}}}
	err := c.GetJSON(context.Background(), "/x", nil, &struct{}{})
	if err == nil || !strings.Contains(err.Error(), "bad token") || !strings.Contains(err.Error(), "401") {
		t.Fatalf("err = %v", err)
	}
}

func TestGetBytesReturnsContent(t *testing.T) {
	payload := bytes.Repeat([]byte("a"), 4096)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(payload)
	}))
	defer server.Close()
	c := Client{Credentials: Credentials{BaseURL: server.URL, Token: pluginbinding.SecretMaterial{Value: "tok"}}}
	data, contentType, err := c.GetBytes(context.Background(), "/file", nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !bytes.Equal(data, payload) {
		t.Fatalf("data len = %d", len(data))
	}
	if contentType != "image/png" {
		t.Fatalf("content type = %q", contentType)
	}
}
