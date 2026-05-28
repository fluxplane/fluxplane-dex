package confluence

import "testing"

func TestPageWebURLBuildsFromSelfAndWebUI(t *testing.T) {
	page := Page{Links: PageLinks{
		Self:  "https://example.atlassian.net/wiki/rest/api/content/123",
		WebUI: "/spaces/OPS/pages/123/Runbook",
	}}
	got := pageWebURL("", page)
	want := "https://example.atlassian.net/wiki/spaces/OPS/pages/123/Runbook"
	if got != want {
		t.Fatalf("url = %q want %q", got, want)
	}
}

func TestPageWebURLPrefersAbsoluteWebUI(t *testing.T) {
	page := Page{Links: PageLinks{WebUI: "https://wiki.example.com/page/123"}}
	if got := pageWebURL("https://base.example.com", page); got != "https://wiki.example.com/page/123" {
		t.Fatalf("url = %q", got)
	}
}

func TestPageWebURLFallsBackToBase(t *testing.T) {
	page := Page{Links: PageLinks{WebUI: "/spaces/OPS/pages/123/Runbook"}}
	got := pageWebURL("https://base.example.com", page)
	want := "https://base.example.com/spaces/OPS/pages/123/Runbook"
	if got != want {
		t.Fatalf("url = %q want %q", got, want)
	}
}

func TestPageWebURLEmptyWhenNoLinks(t *testing.T) {
	if pageWebURL("", Page{}) != "" {
		t.Fatal("expected empty")
	}
}

func TestPageWebURLPreservesBaseWikiPath(t *testing.T) {
	page := Page{Links: PageLinks{
		Self:  "https://example.atlassian.net/wiki/rest/api/content/123",
		WebUI: "/wiki/spaces/OPS/pages/123/Runbook",
	}}
	got := pageWebURL("", page)
	want := "https://example.atlassian.net/wiki/spaces/OPS/pages/123/Runbook"
	if got != want {
		t.Fatalf("url = %q want %q", got, want)
	}
}
