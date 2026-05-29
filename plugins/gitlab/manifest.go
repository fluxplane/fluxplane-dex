package gitlab

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

const (
	PluginName        = "gitlab"
	PluginVersion     = "0.11.0"
	PluginDescription = "GitLab operations, datasources, indexes, and reverse lookups."

	AuthMethodPersonalAccessToken = "personal_access_token"
	AuthPurposeAccessToken        = "access_token"

	EnvGitLabPersonalToken = "GITLAB_PERSONAL_TOKEN"
	EnvGitLabAccessToken   = "GITLAB_ACCESS_TOKEN"
	EnvGitLabToken         = "GITLAB_TOKEN"

	OperationAuthTest           = "gitlab.auth.test"
	OperationIndexBuild         = "gitlab.index.build"
	OperationProjectList        = "gitlab.project.list"
	OperationProjectShow        = "gitlab.project.show"
	OperationMRList             = "gitlab.mr.list"
	OperationMRShow             = "gitlab.mr.show"
	OperationMRCreate           = "gitlab.mr.create"
	OperationMRApprove          = "gitlab.mr.approve"
	OperationMRMerge            = "gitlab.mr.merge"
	OperationTagCreate          = "gitlab.repository.tag.create"
	OperationBranchCreate       = "gitlab.branch.create"
	OperationBranchDelete       = "gitlab.branch.delete"
	OperationBranchDeleteMerged = "gitlab.branch.delete_merged"
	OperationRepoFileCreate     = "gitlab.repository.file.create"
	OperationRepoFileUpdate     = "gitlab.repository.file.update"
	OperationRepoFileDelete     = "gitlab.repository.file.delete"
	OperationCommitCreate       = "gitlab.repository.commit.create"
	OperationCIVariableCreate   = "gitlab.ci.variable.create"
	OperationCIVariableUpdate   = "gitlab.ci.variable.update"
	OperationCIVariableDelete   = "gitlab.ci.variable.delete"
	OperationPipelineCreate     = "gitlab.pipeline.create"
	OperationPipelineRetry      = "gitlab.pipeline.retry"
	OperationPipelineCancel     = "gitlab.pipeline.cancel"
	OperationSnippetCreate      = "gitlab.snippet.create"
	OperationSnippetDelete      = "gitlab.snippet.delete"

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
		branchCreateSpec(),
		branchDeleteSpec(),
		branchDeleteMergedSpec(),
		repoFileCreateSpec(),
		repoFileUpdateSpec(),
		repoFileDeleteSpec(),
		commitCreateSpec(),
		ciVariableCreateSpec(),
		ciVariableUpdateSpec(),
		ciVariableDeleteSpec(),
		pipelineCreateSpec(),
		pipelineRetrySpec(),
		pipelineCancelSpec(),
		snippetCreateSpec(),
		snippetDeleteSpec(),
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

func branchCreateSpec() core.OperationSpec {
	return gitlabWriteOperation[BranchCreateInput, Branch](OperationBranchCreate, "Create a GitLab repository branch.", core.OperationNonIdempotent)
}

func branchDeleteSpec() core.OperationSpec {
	return gitlabDestructiveOperation[BranchDeleteInput, BranchActionResult](OperationBranchDelete, "Delete a GitLab repository branch.")
}

func branchDeleteMergedSpec() core.OperationSpec {
	return gitlabDestructiveOperation[BranchDeleteMergedInput, BranchActionResult](OperationBranchDeleteMerged, "Delete all merged branches in a GitLab project.")
}

func repoFileCreateSpec() core.OperationSpec {
	return gitlabWriteOperation[RepoFileCreateInput, RepoFile](OperationRepoFileCreate, "Create a file in a GitLab repository.", core.OperationNonIdempotent)
}

func repoFileUpdateSpec() core.OperationSpec {
	return gitlabWriteOperation[RepoFileUpdateInput, RepoFile](OperationRepoFileUpdate, "Update a file in a GitLab repository.", core.OperationNonIdempotent)
}

func repoFileDeleteSpec() core.OperationSpec {
	return gitlabDestructiveOperation[RepoFileDeleteInput, RepoFileActionResult](OperationRepoFileDelete, "Delete a file from a GitLab repository.")
}

func commitCreateSpec() core.OperationSpec {
	return gitlabWriteOperation[CommitCreateInput, Commit](OperationCommitCreate, "Create a GitLab commit with one or more file actions.", core.OperationNonIdempotent)
}

func ciVariableCreateSpec() core.OperationSpec {
	return gitlabHighRiskWriteOperation[CIVariableCreateInput, CIVariable](OperationCIVariableCreate, "Create a GitLab project CI/CD variable.", core.OperationNonIdempotent)
}

func ciVariableUpdateSpec() core.OperationSpec {
	return gitlabHighRiskWriteOperation[CIVariableUpdateInput, CIVariable](OperationCIVariableUpdate, "Update a GitLab project CI/CD variable.", core.OperationNonIdempotent)
}

func ciVariableDeleteSpec() core.OperationSpec {
	return gitlabDestructiveOperation[CIVariableDeleteInput, CIVariableActionResult](OperationCIVariableDelete, "Delete a GitLab project CI/CD variable.")
}

func pipelineCreateSpec() core.OperationSpec {
	return gitlabHighRiskWriteOperation[PipelineCreateInput, Pipeline](OperationPipelineCreate, "Create a GitLab CI pipeline.", core.OperationNonIdempotent)
}

func pipelineRetrySpec() core.OperationSpec {
	return gitlabHighRiskWriteOperation[PipelineRetryInput, Pipeline](OperationPipelineRetry, "Retry a GitLab CI pipeline.", core.OperationNonIdempotent)
}

func pipelineCancelSpec() core.OperationSpec {
	return gitlabHighRiskWriteOperation[PipelineCancelInput, Pipeline](OperationPipelineCancel, "Cancel a GitLab CI pipeline.", core.OperationNonIdempotent)
}

func snippetCreateSpec() core.OperationSpec {
	return gitlabWriteOperation[SnippetCreateInput, Snippet](OperationSnippetCreate, "Create a personal GitLab snippet.", core.OperationNonIdempotent)
}

func snippetDeleteSpec() core.OperationSpec {
	return gitlabDestructiveOperation[SnippetDeleteInput, SnippetActionResult](OperationSnippetDelete, "Delete a personal GitLab snippet.")
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
		pluginbinding.SecretPurposes(AuthPurposeAccessToken),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}

func gitlabWriteOperation[I any, O any](name, description string, idempotency core.OperationIdempotency) core.OperationSpec {
	return pluginbinding.TypedOperationSpec[I, O](name, description, gitlabWriteOptions(idempotency, core.OperationRiskMedium)...)
}

func gitlabHighRiskWriteOperation[I any, O any](name, description string, idempotency core.OperationIdempotency) core.OperationSpec {
	return pluginbinding.TypedOperationSpec[I, O](name, description, gitlabWriteOptions(idempotency, core.OperationRiskHigh)...)
}

func gitlabDestructiveOperation[I any, O any](name, description string) core.OperationSpec {
	return pluginbinding.TypedOperationSpec[I, O](name, description, gitlabWriteOptions(core.OperationNonIdempotent, core.OperationRiskDestructive)...)
}

func gitlabWriteOptions(idempotency core.OperationIdempotency, risk core.OperationRisk) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.SecretPurposes(AuthPurposeAccessToken),
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork),
		pluginbinding.Risk(risk),
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
	return pluginbinding.TypedDatasourceSpec[LookupInput, LookupResult](name, entity, description, []string{pluginbinding.CapabilityLookup}, pluginbinding.DatasourceSecretPurposes(AuthPurposeAccessToken))
}
