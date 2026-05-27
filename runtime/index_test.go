package runtime

import (
	"encoding/json"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func TestIndexStoreSearchGetAndStatus(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	records := []json.RawMessage{
		json.RawMessage(`{"entity":"gitlab.project","id":"sbf/services","name":"services","path_with_namespace":"sbf/services","web_url":"https://gitlab.example.com/sbf/services"}`),
		json.RawMessage(`{"entity":"gitlab.project","id":"sbf/manager-v2","name":"manager-v2","path_with_namespace":"sbf/manager-v2","web_url":"https://gitlab.example.com/sbf/manager-v2"}`),
	}
	snapshot, err := state.SaveIndexRecords("gitlab", "work", "gitlab.projects", records)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Records) != 2 {
		t.Fatalf("records = %d, want 2", len(snapshot.Records))
	}
	status, err := state.IndexStatus("gitlab", "work")
	if err != nil {
		t.Fatal(err)
	}
	if status.Records != 2 || len(status.Indexes) != 1 || status.Indexes[0] != "gitlab.projects" {
		t.Fatalf("status = %#v", status)
	}
	if len(status.Details) != 1 || status.Details[0].Index != "gitlab.projects" || status.Details[0].Records != 2 || len(status.Details[0].Metadata) == 0 {
		t.Fatalf("status details = %#v", status.Details)
	}
	var metadata map[string]any
	if err := json.Unmarshal(status.Details[0].Metadata, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["plugin"] != "gitlab" || metadata["instance"] != "work" || metadata["index"] != "gitlab.projects" || metadata["built_at"] == "" {
		t.Fatalf("metadata = %#v", metadata)
	}
	matches, err := state.SearchIndex("gitlab", "work", "manager", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].ID != "sbf/manager-v2" {
		t.Fatalf("matches = %#v", matches)
	}
	if matches[0].Title != "manager-v2" || matches[0].URL != "https://gitlab.example.com/sbf/manager-v2" {
		t.Fatalf("standard fields = %#v", matches[0])
	}
	if matches[0].Links["self"] != "https://gitlab.example.com/sbf/manager-v2" || matches[0].Links["namespace"] != "https://gitlab.example.com/sbf" || matches[0].Links["namespace_entity"] != "gitlab.group:sbf" {
		t.Fatalf("links = %#v", matches[0].Links)
	}
	if matches[0].Origin.Source != "host_index" || matches[0].Origin.Plugin != "gitlab" || matches[0].Origin.Instance != "work" || matches[0].Origin.Index != "gitlab.projects" {
		t.Fatalf("origin = %#v", matches[0].Origin)
	}
	var raw map[string]any
	if err := json.Unmarshal(matches[0].Record, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["entity"]; ok {
		t.Fatalf("raw record should not duplicate entity: %#v", raw)
	}
	if _, ok := raw["id"]; ok {
		t.Fatalf("raw record should not duplicate id: %#v", raw)
	}
	record, ok, err := state.GetIndexRecord("gitlab", "work", "sbf/services")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || record.Entity != "gitlab.project" || record.Links["namespace"] != "https://gitlab.example.com/sbf" {
		t.Fatalf("record = %#v, ok=%v", record, ok)
	}
}

func TestIndexStoreAddsRelationshipLinks(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = state.SaveIndexRecords("gitlab", "default", "gitlab.merge_requests", []json.RawMessage{
		json.RawMessage(`{"entity":"gitlab.merge_request","id":"sbf/services!12","title":"Ship","reference":"sbf/services!12","author_username":"timo","web_url":"https://gitlab.example.com/sbf/services/-/merge_requests/12"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	matches, err := state.SearchIndexWithOptions("gitlab", "default", SearchOptions{Query: "ship", Entity: "gitlab.merge_request", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches = %#v", matches)
	}
	links := matches[0].Links
	if links["self"] != "https://gitlab.example.com/sbf/services/-/merge_requests/12" || links["project"] != "https://gitlab.example.com/sbf/services" {
		t.Fatalf("url links = %#v", links)
	}
	if links["project_entity"] != "gitlab.project:sbf/services" || links["namespace_entity"] != "gitlab.group:sbf" || links["author_entity"] != "gitlab.user:timo" {
		t.Fatalf("relationship links = %#v", links)
	}
}

func TestIndexStoreLookupTermsAndTextURLs(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = state.SaveIndexRecords("gitlab", "default", "gitlab.users", []json.RawMessage{
		json.RawMessage(`{"entity":"gitlab.user","id":"timo","username":"timo","name":"Timo Friedl","web_url":"https://gitlab.example.com/timo","user_id":1234}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = state.SaveIndexRecords("gitlab", "default", "gitlab.merge_requests", []json.RawMessage{
		json.RawMessage(`{"entity":"gitlab.merge_request","id":"sbf/services!12","title":"Ship","reference":"sbf/services!12","author_username":"timo","web_url":"https://gitlab.example.com/sbf/services/-/merge_requests/12"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	users, err := state.LookupIndexWithOptions("gitlab", "default", LookupOptions{Terms: []string{"timo"}, Entity: "gitlab.user", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Source.Plugin != "gitlab" || users[0].Record.ID != "timo" {
		t.Fatalf("user lookup = %#v", users)
	}
	refs, err := state.LookupIndexWithOptions("gitlab", "default", LookupOptions{Text: "Look at https://gitlab.example.com/sbf/services/-/merge_requests/12", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) == 0 || refs[0].Entity != "gitlab.merge_request" || refs[0].ID != "sbf/services!12" {
		t.Fatalf("url lookup = %#v", refs)
	}
	if refs[0].Record.Links["project_entity"] != "gitlab.project:sbf/services" || refs[0].Record.Links["author_entity"] != "gitlab.user:timo" {
		t.Fatalf("canonical links = %#v", refs[0].Record.Links)
	}
}

func TestIndexSearchScoresAndFiltersEntities(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveIndexRecords("gitlab", "default", "gitlab.projects", []json.RawMessage{
		json.RawMessage(`{"entity":"gitlab.project","id":"timo/work-log","name":"work-log","name_with_namespace":"Timo Friedl / work-log","path_with_namespace":"timo/work-log","web_url":"https://gitlab.example.com/timo/work-log"}`),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveIndexRecords("gitlab", "default", "gitlab.users", []json.RawMessage{
		json.RawMessage(`{"entity":"gitlab.user","id":"timo","username":"timo","name":"Timo Friedl","web_url":"https://gitlab.example.com/timo"}`),
	}); err != nil {
		t.Fatal(err)
	}
	matches, err := state.SearchIndexWithOptions("gitlab", "default", SearchOptions{Query: "timo", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Fatalf("matches = %#v", matches)
	}
	if matches[0].Entity != "gitlab.user" || matches[0].ID != "timo" {
		t.Fatalf("first match = %#v", matches[0])
	}
	if matches[0].Score <= matches[1].Score || len(matches[0].MatchedFields) == 0 {
		t.Fatalf("scores = %#v", matches)
	}
	users, err := state.SearchIndexWithOptions("gitlab", "default", SearchOptions{Query: "timo", Entity: "gitlab.user", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Entity != "gitlab.user" {
		t.Fatalf("user matches = %#v", users)
	}
}

func TestRunnerServesDatasourceSearchAndGetFromIndex(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = state.SaveIndexRecords("gitlab", "default", "gitlab.projects", []json.RawMessage{
		json.RawMessage(`{"entity":"gitlab.project","id":"sbf/manager-v2","name":"manager-v2","path_with_namespace":"sbf/manager-v2","web_url":"https://gitlab.example.com/sbf/manager-v2"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{
		State: state,
		Marketplace: NewMarketplace(core.Marketplace{Version: "1", Plugins: []core.PluginEntry{{
			Name:   "gitlab",
			Binary: "missing-plugin-binary",
		}}}),
	}
	search, err := runner.InvokeInstance(nil, "gitlab", "default", protocol.CommandDatasourcesSearch, map[string]any{"query": "manager"})
	if err != nil {
		t.Fatal(err)
	}
	var searchResult struct {
		Source  string        `json:"source"`
		Count   int           `json:"count"`
		Records []IndexRecord `json:"records"`
	}
	if err := json.Unmarshal(search.Result, &searchResult); err != nil {
		t.Fatal(err)
	}
	if searchResult.Source != "host_index" || searchResult.Count != 1 || searchResult.Records[0].ID != "sbf/manager-v2" {
		t.Fatalf("search result = %#v", searchResult)
	}
	if searchResult.Records[0].Origin.Plugin != "gitlab" || searchResult.Records[0].Links["self"] == "" {
		t.Fatalf("standardized search record = %#v", searchResult.Records[0])
	}
	get, err := runner.InvokeInstance(nil, "gitlab", "default", protocol.CommandDatasourcesGet, map[string]any{"id": "sbf/manager-v2"})
	if err != nil {
		t.Fatal(err)
	}
	var getResult struct {
		Source string      `json:"source"`
		Record IndexRecord `json:"record"`
	}
	if err := json.Unmarshal(get.Result, &getResult); err != nil {
		t.Fatal(err)
	}
	if getResult.Source != "host_index" || getResult.Record.ID != "sbf/manager-v2" {
		t.Fatalf("get result = %#v", getResult)
	}
	lookup, err := runner.InvokeInstance(nil, "gitlab", "default", protocol.CommandDatasourcesLookup, map[string]any{"text": "open https://gitlab.example.com/sbf/manager-v2"})
	if err != nil {
		t.Fatal(err)
	}
	var lookupResult struct {
		Source  string        `json:"source"`
		Count   int           `json:"count"`
		Matches []LookupMatch `json:"matches"`
	}
	if err := json.Unmarshal(lookup.Result, &lookupResult); err != nil {
		t.Fatal(err)
	}
	if lookupResult.Source != "host_index" || lookupResult.Count != 1 || lookupResult.Matches[0].Record.ID != "sbf/manager-v2" {
		t.Fatalf("lookup result = %#v", lookupResult)
	}
}
