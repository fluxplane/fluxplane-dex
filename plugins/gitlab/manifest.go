package gitlab

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

const (
	PluginName        = "gitlab"
	PluginVersion     = "0.1.0"
	PluginDescription = "GitLab operations, datasources, indexes, and reverse lookups."

	AuthMethodPersonalAccessToken = "personal_access_token"
	AuthPurposeAccessToken        = "access_token"
	AuthPurposeGitLabURL          = "gitlab_url"

	EnvGitLabPersonalToken = "GITLAB_PERSONAL_TOKEN"
	EnvGitLabAccessToken   = "GITLAB_ACCESS_TOKEN"
	EnvGitLabToken         = "GITLAB_TOKEN"
	EnvGitLabURL           = "GITLAB_URL"

	OperationAuthTest    = "gitlab.auth.test"
	OperationIndexBuild  = "gitlab.index.build"
	OperationProjectList = "gitlab.project.list"
	OperationProjectShow = "gitlab.project.show"
	OperationMRList      = "gitlab.mr.list"
	OperationMRShow      = "gitlab.mr.show"

	DatasourceProjects      = "gitlab.projects"
	DatasourceUsers         = "gitlab.users"
	DatasourceGroups        = "gitlab.groups"
	DatasourceIssues        = "gitlab.issues"
	DatasourceMergeRequests = "gitlab.merge_requests"

	EntityProject      = "gitlab.project"
	EntityUser         = "gitlab.user"
	EntityGroup        = "gitlab.group"
	EntityIssue        = "gitlab.issue"
	EntityMergeRequest = "gitlab.merge_request"

	ContextName  = "gitlab.context"
	EndpointName = "gitlab.endpoint"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	auth := pluginbinding.BearerAuth(
		AuthMethodPersonalAccessToken,
		"GitLab personal access token resolved by dex secret broker.",
		pluginbinding.AuthField(AuthPurposeAccessToken, "GitLab personal access token", true, true, EnvGitLabPersonalToken, EnvGitLabAccessToken, EnvGitLabToken),
		pluginbinding.AuthField(AuthPurposeGitLabURL, "GitLab base URL", true, false, EnvGitLabURL),
	)
	auth.Env = []string{EnvGitLabPersonalToken, EnvGitLabAccessToken, EnvGitLabToken}
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{"gl", PluginName},
		Auth:        []core.AuthMethod{auth},
		Operations:  operationSpecs(),
		IndexedDatasources: []pluginbinding.IndexedDatasourceSpec{
			pluginbinding.IndexedDatasource(DatasourceProjects, EntityProject, "GitLab projects.", "Project metadata and reverse lookup index.", pluginbinding.SearchableIndexCapabilities()...),
			pluginbinding.IndexedDatasource(DatasourceUsers, EntityUser, "GitLab users.", "User metadata and reverse lookup index.", pluginbinding.SearchableIndexCapabilities()...),
			pluginbinding.IndexedDatasource(DatasourceGroups, EntityGroup, "GitLab groups and namespaces.", "Group and namespace reverse lookup index.", pluginbinding.SearchableIndexCapabilities()...),
			pluginbinding.IndexedDatasource(DatasourceIssues, EntityIssue, "GitLab issues.", "Issue metadata and reverse lookup index.", pluginbinding.SearchableIndexCapabilities()...),
			pluginbinding.IndexedDatasource(DatasourceMergeRequests, EntityMergeRequest, "GitLab merge requests.", "Merge request metadata and reverse lookup index.", pluginbinding.SearchableIndexCapabilities()...),
		},
		Context: []core.ContextSpec{
			pluginbinding.ContextSpec(ContextName, "GitLab context blocks.", pluginbinding.ContextKindText, pluginbinding.ContextKindReference),
		},
		Endpoints: []core.EndpointSpec{
			pluginbinding.Endpoint(EndpointName, "Configured GitLab API endpoint.", PluginName),
		},
	}
}

func operationSpecs() []core.OperationSpec {
	return []core.OperationSpec{
		authTestSpec(),
		indexBuildSpec(),
		projectListSpec(),
		projectShowSpec(),
		mergeRequestListSpec(),
		mergeRequestShowSpec(),
	}
}

func authTestSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[NoInput, AuthTestResult](OperationAuthTest, "Test GitLab authentication by fetching the current user.", pluginbinding.ReadOnly(), pluginbinding.SecretPurposes(AuthPurposeAccessToken, AuthPurposeGitLabURL))
}

func indexBuildSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[IndexBuildInput, pluginbinding.IndexBuildResult](OperationIndexBuild, "Build GitLab index records.", pluginbinding.ReadOnly(), pluginbinding.SecretPurposes(AuthPurposeAccessToken, AuthPurposeGitLabURL))
}

func projectListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ProjectListInput, pluginbinding.ListResult[Project]](OperationProjectList, "List accessible GitLab projects.", pluginbinding.ReadOnly(), pluginbinding.Compact(), pluginbinding.SecretPurposes(AuthPurposeAccessToken, AuthPurposeGitLabURL))
}

func projectShowSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ProjectShowInput, pluginbinding.ShowResult[Project]](OperationProjectShow, "Show one GitLab project.", pluginbinding.ReadOnly(), pluginbinding.SecretPurposes(AuthPurposeAccessToken, AuthPurposeGitLabURL))
}

func mergeRequestListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[MergeRequestListInput, pluginbinding.ListResult[MergeRequest]](OperationMRList, "List GitLab merge requests.", pluginbinding.ReadOnly(), pluginbinding.Compact(), pluginbinding.SecretPurposes(AuthPurposeAccessToken, AuthPurposeGitLabURL))
}

func mergeRequestShowSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[MergeRequestShowInput, pluginbinding.ShowResult[MergeRequest]](OperationMRShow, "Show one GitLab merge request.", pluginbinding.ReadOnly(), pluginbinding.SecretPurposes(AuthPurposeAccessToken, AuthPurposeGitLabURL))
}

func gitlabProjectsLookupSpec() core.DatasourceSpec {
	return gitlabLookupSpec(DatasourceProjects, EntityProject, "Lookup GitLab projects.")
}

func gitlabUsersLookupSpec() core.DatasourceSpec {
	return gitlabLookupSpec(DatasourceUsers, EntityUser, "Lookup GitLab users.")
}

func gitlabGroupsLookupSpec() core.DatasourceSpec {
	return gitlabLookupSpec(DatasourceGroups, EntityGroup, "Lookup GitLab groups and namespaces.")
}

func gitlabIssuesLookupSpec() core.DatasourceSpec {
	return gitlabLookupSpec(DatasourceIssues, EntityIssue, "Lookup GitLab issues.")
}

func gitlabMergeRequestsLookupSpec() core.DatasourceSpec {
	return gitlabLookupSpec(DatasourceMergeRequests, EntityMergeRequest, "Lookup GitLab merge requests.")
}

func gitlabLookupSpec(name, entity, description string) core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[LookupInput, LookupResult](name, entity, description, []string{pluginbinding.CapabilityLookup}, pluginbinding.DatasourceSecretPurposes(AuthPurposeAccessToken, AuthPurposeGitLabURL))
}
