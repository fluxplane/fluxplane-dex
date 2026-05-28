package gitlab

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

const (
	PluginName        = "gitlab"
	PluginVersion     = "0.7.0"
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
	OperationMRCreate    = "gitlab.mr.create"
	OperationMRApprove   = "gitlab.mr.approve"
	OperationMRMerge     = "gitlab.mr.merge"
	OperationTagCreate   = "gitlab.repository.tag.create"

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
			pluginbinding.IndexedDatasourceWithOptions(DatasourceProjects, EntityProject, "GitLab projects.", "Project metadata and reverse lookup index.", pluginbinding.SearchableIndexCapabilities(),
				pluginbinding.EntitySchemaFor[ProjectRecord](),
				pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "path_with_namespace", TitleField: "name_with_namespace"}),
				pluginbinding.Fallback(core.DatasourceFallbackHostIndexFirst),
			),
			pluginbinding.IndexedDatasourceWithOptions(DatasourceUsers, EntityUser, "GitLab users.", "User metadata and reverse lookup index.", pluginbinding.SearchableIndexCapabilities(),
				pluginbinding.EntitySchemaFor[UserRecord](),
				pluginbinding.Fallback(core.DatasourceFallbackHostIndexFirst),
			),
			pluginbinding.IndexedDatasourceWithOptions(DatasourceGroups, EntityGroup, "GitLab groups and namespaces.", "Group and namespace reverse lookup index.", pluginbinding.SearchableIndexCapabilities(),
				pluginbinding.EntitySchemaFor[GroupRecord](),
				pluginbinding.Fallback(core.DatasourceFallbackHostIndexFirst),
			),
			pluginbinding.IndexedDatasourceWithOptions(DatasourceIssues, EntityIssue, "GitLab issues.", "Issue metadata and reverse lookup index.", pluginbinding.SearchableIndexCapabilities(),
				pluginbinding.EntitySchemaFor[IssueRecord](),
				pluginbinding.Fallback(core.DatasourceFallbackHostIndexFirst),
			),
			pluginbinding.IndexedDatasourceWithOptions(DatasourceMergeRequests, EntityMergeRequest, "GitLab merge requests.", "Merge request metadata and reverse lookup index.", pluginbinding.SearchableIndexCapabilities(),
				pluginbinding.EntitySchemaFor[MergeRequestRecord](),
				pluginbinding.Fallback(core.DatasourceFallbackHostIndexFirst),
			),
		},
		Context: []core.ContextSpec{
			pluginbinding.ContextSpec(ContextName, "GitLab context blocks.", pluginbinding.ContextKindText, pluginbinding.ContextKindReference),
		},
		Endpoints: []core.EndpointSpec{
			pluginbinding.Endpoint(EndpointName, "Configured GitLab API endpoint.", PluginName),
		},
		Metadata: map[string]string{pluginbinding.ManifestProtocolKey: protocol.Version},
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
		mergeRequestCreateSpec(),
		mergeRequestApproveSpec(),
		mergeRequestMergeSpec(),
		repositoryTagCreateSpec(),
	}
}

func authTestSpec() core.OperationSpec {
	return gitlabReadOperation[NoInput, AuthTestResult](OperationAuthTest, "Test GitLab authentication by fetching the current user.")
}

func indexBuildSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[IndexBuildInput, pluginbinding.IndexBuildResult](
		OperationIndexBuild,
		"Build GitLab index records.",
		gitlabReadOptions(core.OperationConditional)...,
	)
}

func projectListSpec() core.OperationSpec {
	return gitlabCompactReadOperation[ProjectListInput, pluginbinding.ListResult[Project]](OperationProjectList, "List accessible GitLab projects.")
}

func projectShowSpec() core.OperationSpec {
	return gitlabReadOperation[ProjectShowInput, Project](OperationProjectShow, "Show one GitLab project.")
}

func mergeRequestListSpec() core.OperationSpec {
	return gitlabCompactReadOperation[MergeRequestListInput, pluginbinding.ListResult[MergeRequest]](OperationMRList, "List GitLab merge requests.")
}

func mergeRequestShowSpec() core.OperationSpec {
	return gitlabReadOperation[MergeRequestShowInput, pluginbinding.ShowResult[MergeRequest]](OperationMRShow, "Show one GitLab merge request.")
}

func mergeRequestCreateSpec() core.OperationSpec {
	return gitlabWriteOperation[MergeRequestCreateInput, MergeRequest](OperationMRCreate, "Create a GitLab merge request.", core.OperationNonIdempotent)
}

func mergeRequestApproveSpec() core.OperationSpec {
	return gitlabWriteOperation[MergeRequestApproveInput, MergeRequestApproval](OperationMRApprove, "Approve a GitLab merge request.", core.OperationConditional)
}

func mergeRequestMergeSpec() core.OperationSpec {
	return gitlabWriteOperation[MergeRequestMergeInput, MergeRequest](OperationMRMerge, "Merge a GitLab merge request.", core.OperationNonIdempotent)
}

func repositoryTagCreateSpec() core.OperationSpec {
	return gitlabWriteOperation[RepositoryTagCreateInput, RepositoryTag](OperationTagCreate, "Create a GitLab repository tag.", core.OperationNonIdempotent)
}

func gitlabReadOperation[I any, O any](name, description string) core.OperationSpec {
	return pluginbinding.TypedOperationSpec[I, O](name, description, gitlabReadOptions(core.OperationIdempotent)...)
}

func gitlabCompactReadOperation[I any, O any](name, description string) core.OperationSpec {
	options := append(gitlabReadOptions(core.OperationIdempotent), pluginbinding.Compact())
	return pluginbinding.TypedOperationSpec[I, O](name, description, options...)
}

func gitlabReadOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.SecretPurposes(AuthPurposeAccessToken, AuthPurposeGitLabURL),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}

func gitlabWriteOperation[I any, O any](name, description string, idempotency core.OperationIdempotency) core.OperationSpec {
	return pluginbinding.TypedOperationSpec[I, O](name, description, gitlabWriteOptions(idempotency)...)
}

func gitlabWriteOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.SecretPurposes(AuthPurposeAccessToken, AuthPurposeGitLabURL),
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(idempotency),
	}
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
