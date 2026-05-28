package pluginbinding

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

type definitionOKOutput struct {
	OK bool `json:"ok"`
}

func TestDefineRegistersOperationAndSchema(t *testing.T) {
	spec := TypedOperationSpec[helloInput, helloOutput]("test.hello", "Say hello.", ReadOnly(), Compact(), SecretPurposes("token"))
	plugin := Define(ManifestSpec{Name: "test"},
		RegisterOperation(spec, func(_ Context, input helloInput) (helloOutput, error) {
			return helloOutput{Message: "hello " + input.Name, Count: 1}, nil
		}),
	)
	manifest := plugin.Manifest()
	if len(manifest.Operations) != 1 {
		t.Fatalf("operations = %#v", manifest.Operations)
	}
	operation := manifest.Operations[0]
	if operation.Name != "test.hello" || !operation.ReadOnly || !operation.Compact || len(operation.SecretPurposes) != 1 || operation.SecretPurposes[0] != "token" {
		t.Fatalf("operation spec = %#v", operation)
	}
	if len(operation.Input) == 0 || len(operation.Output) == 0 {
		t.Fatalf("schemas were not generated: %#v", operation)
	}
}

func TestDefineRegistersDatasourceAndSchema(t *testing.T) {
	type searchInput struct {
		Query  string `json:"query,omitempty"`
		Entity string `json:"entity,omitempty"`
	}
	type searchOutput struct {
		Records []DatasourceRecord `json:"records"`
		Count   int                `json:"count"`
	}
	spec := TypedDatasourceSpec[searchInput, searchOutput]("test.items", "test.item", "Test items.", []string{CapabilitySearch})
	plugin := Define(ManifestSpec{Name: "test", Datasources: []core.DatasourceSpec{spec}},
		RegisterDatasourceSearch(spec, func(ctx Context, input searchInput) (searchOutput, error) {
			record := NewDatasourceRecord(ctx.DatasourceSource(), "test.item", input.Query)
			return searchOutput{Records: []DatasourceRecord{record}, Count: 1}, nil
		}),
	)
	resp := plugin.Handle(request(t, protocol.CommandDatasourcesSearch, map[string]any{"query": "a", "entity": "test.item"}))
	if !resp.OK {
		t.Fatalf("datasource search failed: %#v", resp.Error)
	}
	manifest := plugin.Manifest()
	if len(manifest.Datasources) != 1 || len(manifest.Datasources[0].Input) == 0 || len(manifest.Datasources[0].Output) == 0 {
		t.Fatalf("datasource schema missing: %#v", manifest.Datasources)
	}
}

func TestDefineGeneratesAuthConnectText(t *testing.T) {
	plugin := Define(ManifestSpec{
		Name: "test",
		Auth: []core.AuthMethod{BearerAuth(
			"token_set",
			"Tokens.",
			AuthField("access_token", "Access token", true, true, "TEST_TOKEN"),
			AuthField("optional_token", "Optional token", false, true, "TEST_OPTIONAL_TOKEN"),
			AuthField("base_url", "Base URL", true, false, "TEST_URL"),
		)},
	})
	resp := plugin.Handle(protocol.Request{Command: protocol.CommandAuthConnect, Plugin: "test", Instance: "default"})
	if !resp.OK {
		t.Fatalf("auth connect failed: %#v", resp.Error)
	}
	var out TextResult
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Text, "--field access_token=<value>") || !strings.Contains(out.Text, "--field base_url=<value>") {
		t.Fatalf("auth connect text = %q", out.Text)
	}
	if strings.Contains(out.Text, "optional_token") || strings.Contains(out.Text, "TEST_TOKEN") {
		t.Fatalf("auth connect text leaked unexpected values: %q", out.Text)
	}
}

func TestIndexedDatasourceAddsDatasourceAndIndex(t *testing.T) {
	manifest := Manifest(ManifestSpec{
		Name: "test",
		IndexedDatasources: []IndexedDatasourceSpec{
			IndexedDatasource("test.users", "test.user", "Users.", "User index.", CapabilitySearch, CapabilityIndex),
		},
	})
	if len(manifest.Datasources) != 1 || manifest.Datasources[0].Name != "test.users" || manifest.Datasources[0].Entity != "test.user" {
		t.Fatalf("datasources = %#v", manifest.Datasources)
	}
	if len(manifest.Indexes) != 1 || manifest.Indexes[0].Name != "test.users" || manifest.Indexes[0].Entities[0] != "test.user" {
		t.Fatalf("indexes = %#v", manifest.Indexes)
	}
}

func TestDefinitionCommandHelpers(t *testing.T) {
	spec := TypedOperationSpec[struct{}, definitionOKOutput]("test.index.build", "Build index.", ReadOnly())
	plugin := Define(ManifestSpec{Name: "test"},
		WithHostManagedAuthTest("Test"),
		WithIndexBuildOperation("test.index.build"),
		WithHostOwnedIndexStatus("Test"),
		RegisterOperation(spec, func(_ Context, _ struct{}) (definitionOKOutput, error) {
			return definitionOKOutput{OK: true}, nil
		}),
	)
	auth := plugin.Handle(protocol.Request{Command: protocol.CommandAuthTest, Plugin: "test", Instance: "default"})
	if !auth.OK {
		t.Fatalf("auth test failed: %#v", auth.Error)
	}
	status := plugin.Handle(protocol.Request{Command: protocol.CommandIndexStatus, Plugin: "test", Instance: "default"})
	if !status.OK {
		t.Fatalf("index status failed: %#v", status.Error)
	}
	build := plugin.Handle(protocol.Request{Command: protocol.CommandIndexBuild, Plugin: "test", Instance: "default", Grant: "grant"})
	if !build.OK {
		t.Fatalf("index build failed: %#v", build.Error)
	}
}

func TestNotImplementedOperationReturnsCallSpecificError(t *testing.T) {
	spec := TypedOperationSpec[struct{}, definitionOKOutput]("test.todo", "Todo.", ReadOnly())
	plugin := Define(ManifestSpec{Name: "test"},
		RegisterOperation(spec, NotImplementedOperation[struct{}, definitionOKOutput]("requires migration")),
	)
	resp := plugin.Handle(request(t, protocol.CommandOperationsCall, protocol.OperationCall{Name: "test.todo"}))
	if resp.OK || resp.Error == nil || resp.Error.Code != "not_implemented" {
		t.Fatalf("response = %#v", resp)
	}
	if !strings.Contains(resp.Error.Message, "test.todo") || !strings.Contains(resp.Error.Message, "requires migration") {
		t.Fatalf("message = %q", resp.Error.Message)
	}
}
