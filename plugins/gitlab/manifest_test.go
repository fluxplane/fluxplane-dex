package gitlab

import (
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

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
