package docker

import (
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
)

func TestManifestDeclaresDockerSurface(t *testing.T) {
	manifest := Manifest()
	plugintest.AssertManifestQuality(t, manifest)
	if manifest.Name != PluginName {
		t.Fatalf("name = %q", manifest.Name)
	}
	if len(manifest.Auth) != 0 {
		t.Fatalf("auth = %#v", manifest.Auth)
	}
	if len(manifest.Indexes) != 0 {
		t.Fatalf("indexes = %#v", manifest.Indexes)
	}
	if len(manifest.Operations) != 44 {
		t.Fatalf("operations = %#v", manifest.Operations)
	}
	for _, operation := range manifest.Operations {
		switch operation.Name {
		case OperationContainerStart, OperationContainerStop, OperationContainerRestart, OperationNetworkCreate, OperationVolumeCreate:
			if operation.ReadOnly {
				t.Fatalf("write operation %q is read-only", operation.Name)
			}
			if operation.Risk != core.OperationRiskMedium {
				t.Fatalf("write operation risk for %q = %#v", operation.Name, operation)
			}
		case OperationContainerRemove, OperationImageRemove, OperationNetworkRemove, OperationVolumeRemove,
			OperationContainerPrune, OperationImagePrune, OperationNetworkPrune, OperationVolumePrune, OperationBuildCachePrune, OperationSystemPrune:
			if operation.ReadOnly {
				t.Fatalf("destructive operation %q is read-only", operation.Name)
			}
			if operation.Risk != core.OperationRiskDestructive {
				t.Fatalf("destructive operation risk for %q = %#v", operation.Name, operation)
			}
		case OperationImagePull, OperationImageTag:
			if operation.ReadOnly || operation.Risk != core.OperationRiskMedium {
				t.Fatalf("medium write metadata = %#v", operation)
			}
		case OperationContainerExec, OperationContainerCopyFrom, OperationContainerCopyTo, OperationContainerCreate, OperationContainerRun, OperationImagePush, OperationImageBuild:
			if operation.ReadOnly || operation.Risk != core.OperationRiskHigh {
				t.Fatalf("high-risk write metadata = %#v", operation)
			}
		default:
			if !operation.ReadOnly {
				t.Fatalf("operation %q is not read-only", operation.Name)
			}
			if operation.Risk != core.OperationRiskLow || operation.Idempotency != core.OperationIdempotent {
				t.Fatalf("operation metadata for %q = %#v", operation.Name, operation)
			}
		}
	}
	if len(manifest.Datasources) != 4 {
		t.Fatalf("datasources = %#v", manifest.Datasources)
	}
	for _, datasource := range manifest.Datasources {
		for _, capability := range []string{pluginbinding.CapabilitySearch, pluginbinding.CapabilityLookup, pluginbinding.CapabilityGet} {
			if !hasCapability(datasource.Capabilities, capability) {
				t.Fatalf("datasource %q missing %q: %#v", datasource.Name, capability, datasource.Capabilities)
			}
		}
	}
}

func hasCapability(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
