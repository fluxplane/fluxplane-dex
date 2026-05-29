package gitlab

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func TestManifestInputSchemasDescribeAllFields(t *testing.T) {
	manifest := Manifest()
	for _, op := range manifest.Operations {
		assertSchemaPropertiesDescribed(t, "operation "+op.Name, op.Input)
	}
	for _, ds := range manifest.Datasources {
		assertSchemaPropertiesDescribed(t, "datasource "+ds.Name, ds.Input)
	}

	assertSchemaPropertyDescription(t, "repo file encoding", pluginbinding.MustSchemaFor[RepoFileCreateInput](), "encoding", "Content encoding, such as text or base64")
	assertSchemaPropertyDescription(t, "repository tag ref", pluginbinding.MustSchemaFor[RepositoryTagCreateInput](), "ref", "Commit SHA, branch name, or existing tag name")
	assertSchemaPropertyDescription(t, "branch ref", pluginbinding.MustSchemaFor[BranchCreateInput](), "ref", "Source ref (commit SHA, branch, or tag)")
}

func TestGitLabDatasourceCapabilitySchemas(t *testing.T) {
	assertSchemaHasProperty(t, "project search datasource", gitlabProjectsDatasourceSpec().Input, "query")
	assertSchemaHasProperty(t, "project get datasource", gitlabProjectsDatasourceGetSpec().Input, "id")
}

func assertSchemaHasProperty(t *testing.T, name string, raw json.RawMessage, field string) {
	t.Helper()
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("%s schema is invalid: %v", name, err)
	}
	if _, ok := schema.Properties[field]; !ok {
		t.Fatalf("%s schema is missing property %q: %s", name, field, string(raw))
	}
}

func assertSchemaPropertiesDescribed(t *testing.T, name string, raw json.RawMessage) {
	t.Helper()
	if len(raw) == 0 {
		return
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("%s input schema is invalid: %v", name, err)
	}
	for field, propRaw := range schema.Properties {
		var prop struct {
			Description string `json:"description"`
		}
		if err := json.Unmarshal(propRaw, &prop); err != nil {
			t.Fatalf("%s.%s schema is invalid: %v", name, field, err)
		}
		if strings.TrimSpace(prop.Description) == "" {
			t.Fatalf("%s.%s is missing a schema description: %s", name, field, string(propRaw))
		}
	}
}

func assertSchemaPropertyDescription(t *testing.T, name string, raw json.RawMessage, field, want string) {
	t.Helper()
	var schema struct {
		Properties map[string]struct {
			Description string `json:"description"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("%s schema is invalid: %v", name, err)
	}
	got := schema.Properties[field].Description
	if got != want {
		t.Fatalf("%s description = %q, want %q", name, got, want)
	}
}

func TestManifestQuality(t *testing.T) {
	plugintest.AssertManifestQuality(t, Manifest())
}

func TestManifestDeclaresDatasourceMetadata(t *testing.T) {
	manifest := Manifest()
	byEntity := map[string]core.DatasourceSpec{}
	for _, datasource := range manifest.Datasources {
		byEntity[datasource.Entity] = datasource
	}
	project := byEntity[EntityProject]
	if project.EntitySchema == nil || project.EntitySchema.IDField != "path_with_namespace" || project.EntitySchema.TitleField != "name_with_namespace" {
		t.Fatalf("project entity schema = %#v", project.EntitySchema)
	}
	if project.Fallback != core.DatasourceFallbackHostIndexFirst {
		t.Fatalf("project fallback = %q", project.Fallback)
	}
	mr := byEntity[EntityMergeRequest]
	if len(mr.Relations) == 0 || mr.Relations[0].Entity != EntityProject {
		t.Fatalf("merge request relations = %#v", mr.Relations)
	}
	if mr.Completion == nil || len(mr.Completion.Fields) == 0 {
		t.Fatalf("merge request completion = %#v", mr.Completion)
	}
}

func TestManifestDeclaresGitLabWriteOperations(t *testing.T) {
	manifest := Manifest()
	if manifest.Metadata["dex.protocol"] != protocol.Version {
		t.Fatalf("protocol metadata = %#v", manifest.Metadata)
	}
	operations := map[string]core.OperationSpec{}
	for _, operation := range manifest.Operations {
		operations[operation.Name] = operation
	}
	cases := []struct {
		name string
		risk core.OperationRisk
	}{
		{OperationMRCreate, core.OperationRiskMedium},
		{OperationMRApprove, core.OperationRiskMedium},
		{OperationMRMerge, core.OperationRiskMedium},
		{OperationTagCreate, core.OperationRiskMedium},
		{OperationBranchCreate, core.OperationRiskMedium},
		{OperationBranchDelete, core.OperationRiskDestructive},
		{OperationBranchDeleteMerged, core.OperationRiskDestructive},
		{OperationRepoFileCreate, core.OperationRiskMedium},
		{OperationRepoFileUpdate, core.OperationRiskMedium},
		{OperationRepoFileDelete, core.OperationRiskDestructive},
		{OperationCommitCreate, core.OperationRiskMedium},
		{OperationCIVariableCreate, core.OperationRiskHigh},
		{OperationCIVariableUpdate, core.OperationRiskHigh},
		{OperationCIVariableDelete, core.OperationRiskDestructive},
		{OperationPipelineCreate, core.OperationRiskHigh},
		{OperationPipelineRetry, core.OperationRiskHigh},
		{OperationPipelineCancel, core.OperationRiskHigh},
		{OperationSnippetCreate, core.OperationRiskMedium},
		{OperationSnippetDelete, core.OperationRiskDestructive},
	}
	for _, tc := range cases {
		operation, ok := operations[tc.name]
		if !ok {
			t.Fatalf("missing operation %s", tc.name)
		}
		if operation.ReadOnly {
			t.Fatalf("operation %s should not be read-only", tc.name)
		}
		if operation.Risk != tc.risk {
			t.Fatalf("operation %s risk = %q, want %q", tc.name, operation.Risk, tc.risk)
		}
		if operation.Idempotency == "" {
			t.Fatalf("operation %s missing idempotency", tc.name)
		}
	}
}
