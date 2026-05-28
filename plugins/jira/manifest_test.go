package jira

import (
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func TestManifestQuality(t *testing.T) {
	plugintest.AssertManifestQuality(t, Manifest())
}

func TestManifestDeclaresSharedAtlassianEnvFallbacks(t *testing.T) {
	manifest := Manifest()
	if manifest.Metadata[pluginbinding.ManifestProtocolKey] != protocol.Version {
		t.Fatalf("protocol metadata = %#v", manifest.Metadata)
	}
	if len(manifest.Auth) != 1 {
		t.Fatalf("auth = %#v", manifest.Auth)
	}
	fields := map[string]core.AuthField{}
	for _, field := range manifest.Auth[0].Fields {
		fields[field.Name] = field
	}
	if got := fields[AuthPurposeAPIToken].Env; len(got) != 2 || got[0] != EnvJiraAPIToken || got[1] != EnvAtlassianAPIToken {
		t.Fatalf("token env = %#v", got)
	}
	if got := fields[AuthPurposeCloudID].Env; len(got) != 2 || got[0] != EnvJiraCloudID || got[1] != EnvAtlassianCloudID {
		t.Fatalf("cloud id env = %#v", got)
	}
	if len(fields) != 2 {
		t.Fatalf("auth fields = %#v", fields)
	}
	byEntity := map[string]core.DatasourceSpec{}
	for _, datasource := range manifest.Datasources {
		byEntity[datasource.Entity] = datasource
	}
	if byEntity[EntityIssue].Fallback != core.DatasourceFallbackHostIndexFirst {
		t.Fatalf("issue fallback = %q", byEntity[EntityIssue].Fallback)
	}
}

func TestManifestDeclaresJiraWriteOperations(t *testing.T) {
	manifest := Manifest()
	byName := map[string]core.OperationSpec{}
	for _, operation := range manifest.Operations {
		byName[operation.Name] = operation
	}
	for _, name := range []string{OperationCommentAdd, OperationCommentEdit, OperationCommentDelete, OperationIssueCreate, OperationIssueEdit} {
		operation, ok := byName[name]
		if !ok {
			t.Fatalf("missing operation %s", name)
		}
		if operation.ReadOnly {
			t.Fatalf("%s should not be read-only", name)
		}
		if operation.Risk != core.OperationRiskMedium || operation.Idempotency != core.OperationNonIdempotent {
			t.Fatalf("%s risk/idempotency = %s/%s", name, operation.Risk, operation.Idempotency)
		}
		if !hasOperationEffect(operation, core.OperationEffectWrite) || !hasOperationEffect(operation, core.OperationEffectNetwork) {
			t.Fatalf("%s effects = %#v", name, operation.Effects)
		}
		if len(operation.SecretPurposes) != 2 || operation.SecretPurposes[0] != AuthPurposeAPIToken || operation.SecretPurposes[1] != AuthPurposeCloudID {
			t.Fatalf("%s secret purposes = %#v", name, operation.SecretPurposes)
		}
	}
	deleteOperation, ok := byName[OperationIssueDelete]
	if !ok {
		t.Fatalf("missing operation %s", OperationIssueDelete)
	}
	if deleteOperation.ReadOnly || deleteOperation.Risk != core.OperationRiskDestructive {
		t.Fatalf("%s read-only/risk = %v/%s", OperationIssueDelete, deleteOperation.ReadOnly, deleteOperation.Risk)
	}
	for _, name := range []string{OperationCreateMeta, OperationEditMeta} {
		operation, ok := byName[name]
		if !ok {
			t.Fatalf("missing operation %s", name)
		}
		if !operation.ReadOnly {
			t.Fatalf("%s should be read-only", name)
		}
	}
}

func hasOperationEffect(operation core.OperationSpec, effect core.OperationEffect) bool {
	for _, candidate := range operation.Effects {
		if candidate == effect {
			return true
		}
	}
	return false
}
