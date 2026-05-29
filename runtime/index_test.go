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

func TestIndexLookupResolverMatrix(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveIndexRecords("gitlab", "default", "gitlab.projects", []json.RawMessage{
		json.RawMessage(`{"entity":"gitlab.project","id":"sbf/services","name":"services","path_with_namespace":"sbf/services","web_url":"https://gitlab.example.com/sbf/services"}`),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveIndexRecords("gitlab", "default", "gitlab.users", []json.RawMessage{
		json.RawMessage(`{"entity":"gitlab.user","id":"timo","username":"timo","name":"Timo Friedl","web_url":"https://gitlab.example.com/timo","user_id":1234}`),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveIndexRecords("gitlab", "default", "gitlab.issues", []json.RawMessage{
		json.RawMessage(`{"entity":"gitlab.issue","id":"sbf/services#34","title":"Bug","reference":"sbf/services#34","author_username":"timo","web_url":"https://gitlab.example.com/sbf/services/-/issues/34"}`),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveIndexRecords("gitlab", "default", "gitlab.merge_requests", []json.RawMessage{
		json.RawMessage(`{"entity":"gitlab.merge_request","id":"sbf/services!12","title":"Ship","reference":"sbf/services!12","author_username":"timo","web_url":"https://gitlab.example.com/sbf/services/-/merge_requests/12"}`),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveIndexRecords("slack", "default", "slack.users", []json.RawMessage{
		json.RawMessage(`{"entity":"slack.user","id":"U1234","user_id":"U1234","name":"timo","display_name":"Timo Friedl","web_url":"slack://user/U1234"}`),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveIndexRecords("slack", "default", "slack.channels", []json.RawMessage{
		json.RawMessage(`{"entity":"slack.channel","id":"C1234","channel_id":"C1234","name":"engineering","user":"U1234","web_url":"slack://channel/C1234"}`),
	}); err != nil {
		t.Fatal(err)
	}

	assertLookupMatch(t, state, "gitlab", LookupOptions{Text: "open https://gitlab.example.com/sbf/services", Limit: 10}, "gitlab.project", "sbf/services")
	assertLookupMatch(t, state, "gitlab", LookupOptions{Text: "open https://gitlab.example.com/timo", Limit: 10}, "gitlab.user", "timo")
	issue := assertLookupMatch(t, state, "gitlab", LookupOptions{Text: "open https://gitlab.example.com/sbf/services/-/issues/34", Limit: 10}, "gitlab.issue", "sbf/services#34")
	if issue.Record.Links["project_entity"] != "gitlab.project:sbf/services" || issue.Record.Links["namespace_entity"] != "gitlab.group:sbf" || issue.Record.Links["author_entity"] != "gitlab.user:timo" {
		t.Fatalf("issue links = %#v", issue.Record.Links)
	}
	mr := assertLookupMatch(t, state, "gitlab", LookupOptions{Text: "open https://gitlab.example.com/sbf/services/-/merge_requests/12", Limit: 10}, "gitlab.merge_request", "sbf/services!12")
	if mr.Record.Links["project_entity"] != "gitlab.project:sbf/services" || mr.Record.Links["project"] != "https://gitlab.example.com/sbf/services" {
		t.Fatalf("merge request links = %#v", mr.Record.Links)
	}
	assertLookupMatch(t, state, "slack", LookupOptions{Text: "ping slack://user/U1234", Limit: 10}, "slack.user", "U1234")
	channel := assertLookupMatch(t, state, "slack", LookupOptions{Text: "join slack://channel/C1234", Limit: 10}, "slack.channel", "C1234")
	if channel.Record.Links["self"] != "slack://channel/C1234" || channel.Record.Links["user_entity"] != "slack.user:U1234" {
		t.Fatalf("channel links = %#v", channel.Record.Links)
	}
	assertLookupMatch(t, state, "slack", LookupOptions{Text: "#engineering", Limit: 10}, "slack.channel", "C1234")
	assertLookupMatch(t, state, "slack", LookupOptions{Terms: []string{"timo"}, Limit: 10}, "slack.user", "U1234")
}

func assertLookupMatch(t *testing.T, state State, plugin string, options LookupOptions, entity, id string) LookupMatch {
	t.Helper()
	matches, err := state.LookupIndexWithOptions(plugin, "default", options)
	if err != nil {
		t.Fatal(err)
	}
	for _, match := range matches {
		if match.Entity == entity && match.ID == id {
			return match
		}
	}
	t.Fatalf("lookup %q %q did not include %s %s: %#v", options.Text, options.Terms, entity, id, matches)
	return LookupMatch{}
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

func TestIndexSearchMatchesMultiTermQueriesAcrossFields(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveIndexRecords("slack", "default", "slack.users", []json.RawMessage{
		json.RawMessage(`{"entity":"slack.user","id":"U03HY52RQLV","user_id":"U03HY52RQLV","name":"timo","display_name":"Timo Friedl","web_url":"slack://user/U03HY52RQLV"}`),
		json.RawMessage(`{"entity":"slack.user","id":"U0ACY99H9AN","user_id":"U0ACY99H9AN","name":"timo-ai","display_name":"Timo AI","web_url":"slack://user/U0ACY99H9AN"}`),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveIndexRecords("slack", "default", "slack.channels", []json.RawMessage{
		json.RawMessage(`{"entity":"slack.channel","id":"C02GXKL0B2B","channel_id":"C02GXKL0B2B","name":"general","web_url":"slack://channel/C02GXKL0B2B"}`),
		json.RawMessage(`{"entity":"slack.channel","id":"C05B6RU8KEV","channel_id":"C05B6RU8KEV","name":"general-ai","web_url":"slack://channel/C05B6RU8KEV"}`),
	}); err != nil {
		t.Fatal(err)
	}

	users, err := state.SearchIndexWithOptions("slack", "default", SearchOptions{Query: "timo U03HY52RQLV", Entity: "slack.user", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].ID != "U03HY52RQLV" {
		t.Fatalf("combined user query matches = %#v", users)
	}
	if len(users[0].MatchedFields) < 2 {
		t.Fatalf("combined user matched fields = %#v", users[0].MatchedFields)
	}

	channels, err := state.SearchIndexWithOptions("slack", "default", SearchOptions{Query: "general C02GXKL0B2B", Entity: "slack.channel", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 1 || channels[0].ID != "C02GXKL0B2B" {
		t.Fatalf("combined channel query matches = %#v", channels)
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

func TestDatasourceSearchResponseEnrichesRelationFieldsFromIndex(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = state.SaveIndexRecords("slack", "work", "slack.users", []json.RawMessage{
		json.RawMessage(`{"entity":"slack.user","id":"U123","title":"Timo Friedl","user_id":"U123","name":"timo","real_name":"Timo Friedl","display_name":"Timo","email":"timo@example.com","web_url":"slack://user/U123"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	resp := protocol.OK(map[string]any{
		"source": "slack.channel_members",
		"count":  2,
		"records": []map[string]any{
			{"entity": "slack.channel_member", "id": "C1:U123", "title": "U123", "channel": "C1", "user_id": "U123"},
			{"entity": "slack.channel_member", "id": "C1:U999", "title": "U999", "channel": "C1", "user_id": "U999"},
		},
	})

	enriched, err := enrichDatasourceSearchResponse(slackMemberManifestForTest(), state, "slack", "work", map[string]any{"datasource": "slack.channel_members", "entity": "slack.channel_member"}, resp)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Count   int `json:"count"`
		Records []struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			UserID      string `json:"user_id"`
			Name        string `json:"name,omitempty"`
			RealName    string `json:"real_name,omitempty"`
			DisplayName string `json:"display_name,omitempty"`
			Email       string `json:"email,omitempty"`
		} `json:"records"`
	}
	if err := json.Unmarshal(enriched.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Count != 2 || len(result.Records) != 2 {
		t.Fatalf("result = %#v", result)
	}
	if result.Records[0].Title != "Timo Friedl" || result.Records[0].Name != "timo" || result.Records[0].DisplayName != "Timo" || result.Records[0].Email != "timo@example.com" {
		t.Fatalf("enriched record = %#v", result.Records[0])
	}
	if result.Records[1].Title != "U999" || result.Records[1].Name != "" {
		t.Fatalf("missing index record should be unchanged: %#v", result.Records[1])
	}
}

func TestDatasourceSearchResponseLeavesRecordsWhenIndexMissing(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	resp := protocol.OK(map[string]any{
		"source": "slack.channel_members",
		"count":  1,
		"records": []map[string]any{
			{"entity": "slack.channel_member", "id": "C1:U123", "title": "U123", "channel": "C1", "user_id": "U123"},
		},
	})

	enriched, err := enrichDatasourceSearchResponse(slackMemberManifestForTest(), state, "slack", "work", map[string]any{"datasource": "slack.channel_members", "entity": "slack.channel_member"}, resp)
	if err != nil {
		t.Fatal(err)
	}
	if string(enriched.Result) != string(resp.Result) {
		t.Fatalf("response changed without an index:\n%s", string(enriched.Result))
	}
}

func TestHostIndexDatasourceSkipsUnindexedExactDatasource(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = state.SaveIndexRecords("slack", "default", "slack.users", []json.RawMessage{
		json.RawMessage(`{"entity":"slack.user","id":"U123","name":"timo"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	ds := hostIndexDatasource{state: state, plugin: "slack", instance: "default"}
	_, ok, err := ds.Response(protocol.CommandDatasourcesSearch, map[string]any{"datasource": "slack.channel_members", "entity": "slack.channel_member"})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatalf("host index should not handle an exact live datasource without a matching index")
	}
	resp, ok, err := ds.Response(protocol.CommandDatasourcesSearch, map[string]any{"datasource": "slack.users", "entity": "slack.user", "query": "timo"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok || !resp.OK {
		t.Fatalf("host index should handle exact indexed datasource: ok=%v resp=%#v", ok, resp)
	}
}

func slackMemberManifestForTest() core.PluginManifest {
	return core.PluginManifest{
		Name: "slack",
		Datasources: []core.DatasourceSpec{{
			Name:   "slack.channel_members",
			Entity: "slack.channel_member",
			EntitySchema: &core.DatasourceEntitySchema{Fields: []core.DatasourceFieldSpec{
				{Name: "title"},
				{Name: "channel"},
				{Name: "user_id"},
				{Name: "name"},
				{Name: "real_name"},
				{Name: "display_name"},
				{Name: "email"},
			}},
			Relations: []core.DatasourceRelationSpec{{Field: "user_id", Entity: "slack.user", Type: "reference"}},
		}},
	}
}
