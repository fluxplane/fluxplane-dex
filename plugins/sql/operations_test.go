package sql

import (
	"encoding/json"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
)

func TestQueryRejectsWrites(t *testing.T) {
	plugin := NewPluginWithService(Service{})
	err := plugintest.RunError(t, plugin, OperationQuery, QueryInput{EndpointRef: "db", Query: "delete from users"})
	if err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}

func TestQueryRequiresEndpointRef(t *testing.T) {
	plugin := NewPluginWithService(Service{})
	err := plugintest.RunError(t, plugin, OperationQuery, QueryInput{Query: "select 1"})
	if err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}

func TestDatasourceSearchRejectsFreeTextWithSearchSpecificMessage(t *testing.T) {
	plugin := NewPluginWithService(Service{})
	err := plugintest.DatasourceSearchError(t, plugin, map[string]any{
		"datasource": DatasourceQueryRows,
		"query":      "api",
	})
	if err.Code != "bad_input" || err.Message == readOnlySQLQueryMessage {
		t.Fatalf("err = %#v", err)
	}
}

func TestManifestQuality(t *testing.T) {
	plugintest.AssertManifestQuality(t, Manifest())
}

func TestQueryUsesHostSQLProvider(t *testing.T) {
	host := sqlTestHost{t: t}
	plugin := NewPluginWithService(Service{})
	out := plugintest.RunOK[QueryOutput](t, plugin, OperationQuery, QueryInput{
		EndpointRef: "warehouse",
		Driver:      "postgres",
		Database:    "app",
		Query:       "select id, name from users order by id",
		MaxRows:     10,
	}, plugintest.WithHost(host))
	if out.EndpointRef != "warehouse" || out.Driver != "postgres" || out.RowCount != 2 {
		t.Fatalf("out = %#v", out)
	}
	if out.Rows[0]["name"] != "Ada" || out.Rows[1]["name"] != "Linus" {
		t.Fatalf("rows = %#v", out.Rows)
	}
}

func TestQueryRowsBuildsDatasourceRecords(t *testing.T) {
	host := sqlTestHost{t: t}
	plugin := NewPluginWithService(Service{})
	records := plugintest.DatasourceSearchOK[QueryRowsResult](t, plugin, QueryInput{
		EndpointRef: "warehouse",
		Query:       "select id, name from users order by id",
		MaxRows:     10,
	}, plugintest.WithHost(host))
	if records.Count != 2 || records.Records[0].Row["name"] != "Ada" || records.Records[0].Driver != "postgres" {
		t.Fatalf("records = %#v", records)
	}
}

type sqlTestHost struct {
	t *testing.T
}

func (h sqlTestHost) Secret(string) (pluginbinding.SecretMaterial, error) {
	return pluginbinding.SecretMaterial{}, nil
}

func (h sqlTestHost) Lookup(pluginbinding.DatasourceLookupInput) (pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]], error) {
	return pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]{}, nil
}

func (h sqlTestHost) Search(pluginbinding.DatasourceSearchInput) (pluginbinding.DatasourceSearchResult[any], error) {
	return pluginbinding.DatasourceSearchResult[any]{}, nil
}

func (h sqlTestHost) Get(pluginbinding.DatasourceGetInput) (pluginbinding.DatasourceGetResult[any], error) {
	return pluginbinding.DatasourceGetResult[any]{}, nil
}

func (h sqlTestHost) ResolveEndpoint(string) (core.EndpointRef, error) {
	return core.EndpointRef{}, nil
}

func (h sqlTestHost) HTTP(pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	return pluginbinding.HTTPResponse{}, nil
}

func (h sqlTestHost) BlobRead(pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error) {
	return pluginbinding.BlobReadResponse{}, nil
}

func (h sqlTestHost) BlobWrite(pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h sqlTestHost) BlobInfo(pluginbinding.BlobInfoRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h sqlTestHost) EnvLookup(string) (pluginbinding.EnvLookupResponse, error) {
	return pluginbinding.EnvLookupResponse{}, nil
}

func (h sqlTestHost) CapabilityCall(input pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	if input.Provider != PluginName || input.Action != "query" {
		h.t.Fatalf("provider call = %#v", input)
	}
	var queryInput QueryInput
	if err := json.Unmarshal(input.Payload, &queryInput); err != nil {
		h.t.Fatal(err)
	}
	if queryInput.EndpointRef != "warehouse" || queryInput.Query == "" {
		h.t.Fatalf("query input = %#v", queryInput)
	}
	raw, err := json.Marshal(QueryOutput{
		EndpointRef: "warehouse",
		EndpointURL: "postgres://app:xxxxx@db.example.com/app",
		Driver:      "postgres",
		Database:    "app",
		Columns:     []string{"id", "name"},
		Rows: []map[string]any{
			{"id": float64(1), "name": "Ada"},
			{"id": float64(2), "name": "Linus"},
		},
		RowCount: 2,
	})
	if err != nil {
		return pluginbinding.ProviderCallResponse{}, err
	}
	return pluginbinding.ProviderCallResponse{Result: raw}, nil
}

var _ pluginbinding.HostClient = sqlTestHost{}
