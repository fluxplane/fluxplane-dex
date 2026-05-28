package gitlab

import (
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
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
