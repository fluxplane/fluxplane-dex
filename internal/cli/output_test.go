package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestTextOutputRendersStructureInsteadOfJSON(t *testing.T) {
	var out bytes.Buffer
	err := renderValue(&out, "text", map[string]any{
		"plugin": "gitlab",
		"count":  25,
		"records": []map[string]any{{
			"id":    "1",
			"title": "issue",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"plugin: gitlab", "count: 25", "records:", "title: issue"} {
		if !strings.Contains(got, want) {
			t.Fatalf("text output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, `{"`) {
		t.Fatalf("text output should not fall back to JSON:\n%s", got)
	}
}

func TestCompactOutputSummarizesCollections(t *testing.T) {
	var out bytes.Buffer
	if err := renderValue(&out, "compact", map[string]any{"records": []any{"a", "b"}}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "2 records" {
		t.Fatalf("compact output = %q", out.String())
	}
}

func TestTextOutputDoesNotEchoEmptyLookupInput(t *testing.T) {
	var out bytes.Buffer
	if err := renderValue(&out, "text", map[string]any{"text": "test", "results": map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(out.String())
	if got == "test" || !strings.Contains(got, `no matches for "test"`) {
		t.Fatalf("empty lookup output = %q", got)
	}
}

func TestJSONOutputAddsGenericRecordsAlias(t *testing.T) {
	var out bytes.Buffer
	if err := renderValue(&out, "json", map[string]any{"pods": []map[string]any{{"name": "api"}}}); err != nil {
		t.Fatal(err)
	}
	var result struct {
		RecordsSource string           `json:"records_source"`
		Records       []map[string]any `json:"records"`
		Pods          []map[string]any `json:"pods"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.RecordsSource != "pods" || len(result.Records) != 1 || len(result.Pods) != 1 {
		t.Fatalf("records alias result = %#v", result)
	}
}

func TestJSONOutputDoesNotRewriteSchemas(t *testing.T) {
	var out bytes.Buffer
	if err := renderValue(&out, "json", map[string]any{
		"output_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"items": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if strings.Contains(got, "records_source") || strings.Contains(got, `"records"`) {
		t.Fatalf("schema output was rewritten:\n%s", got)
	}
}
