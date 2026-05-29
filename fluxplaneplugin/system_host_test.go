package fluxplaneplugin

import (
	"testing"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

func TestSystemHTTPRequestURLCombinesPathAndQuery(t *testing.T) {
	got, err := systemHTTPRequestURL(pluginbinding.HTTPRequest{
		URL:   "https://api.atlassian.com/ex/jira/cloud-123",
		Path:  "/rest/api/3/issue/DEV-479",
		Query: map[string][]string{"fields": {"summary", "status"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "https://api.atlassian.com/ex/jira/cloud-123/rest/api/3/issue/DEV-479?fields=summary&fields=status"
	if got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}
}
