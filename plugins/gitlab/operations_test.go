package gitlab

import (
	"encoding/json"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
)

func TestServiceProjectListUsesClient(t *testing.T) {
	client := &fakeClient{
		projects: []Project{{ID: 1, Name: "dex", PathWithNamespace: "group/dex"}},
	}
	plugin := testPlugin(client)

	out := plugintest.RunOK[pluginbinding.ListResult[Project]](t, plugin, OperationProjectList, map[string]any{
		"limit":  5,
		"search": "dex",
	}, plugintest.WithInstance("work"))
	if client.listOptions.Limit != 5 || client.listOptions.Search != "dex" {
		t.Fatalf("list options = %#v", client.listOptions)
	}
	if out.Count != 1 || out.Items[0].PathWithNamespace != "group/dex" {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func TestServiceProjectShowParsesNumericID(t *testing.T) {
	client := &fakeClient{project: Project{ID: 42, Name: "dex", PathWithNamespace: "group/dex", WebURL: "https://gitlab.example.com/group/dex"}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[Project](t, plugin, OperationProjectShow, map[string]any{"id": 42})
	if client.projectID != int64(42) {
		t.Fatalf("project id = %#v", client.projectID)
	}
	if out.ID != 42 || out.Name != "dex" || out.PathWithNamespace != "group/dex" || out.WebURL == "" {
		t.Fatalf("project output = %#v", out)
	}
}

func TestServiceMRShowParsesReference(t *testing.T) {
	client := &fakeClient{mergeRequest: MergeRequest{IID: 7, ProjectID: 42, Title: "Update"}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[pluginbinding.ShowResult[MergeRequest]](t, plugin, OperationMRShow, map[string]any{"ref": "group/dex!7"})
	if client.mrProject != "group/dex" || client.mrIID != 7 {
		t.Fatalf("mr lookup = %#v ! %d", client.mrProject, client.mrIID)
	}
	if out.Record.IID != 7 || out.Metadata["ref"] != "group/dex!7" {
		t.Fatalf("mr output = %#v", out)
	}
}

func TestServiceMRListUsesProjectPathAndOptions(t *testing.T) {
	client := &fakeClient{
		mergeRequests: []MergeRequest{{IID: 11, ProjectID: 42, Title: "Ship"}},
	}
	plugin := testPlugin(client)

	out := plugintest.RunOK[pluginbinding.ListResult[MergeRequest]](t, plugin, OperationMRList, map[string]any{
		"project":  "group/dex",
		"state":    "merged",
		"search":   "ship",
		"limit":    7,
		"order_by": "created_at",
		"sort":     "asc",
	})
	if client.mrListOptions.Project != "group/dex" || client.mrListOptions.State != "merged" || client.mrListOptions.Search != "ship" {
		t.Fatalf("mr list options = %#v", client.mrListOptions)
	}
	if client.mrListOptions.Limit != 7 || client.mrListOptions.OrderBy != "created_at" || client.mrListOptions.Sort != "asc" {
		t.Fatalf("mr list options = %#v", client.mrListOptions)
	}
	if out.Count != 1 || out.Items[0].IID != 11 {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func TestServiceMRListDefaultsAndNumericProject(t *testing.T) {
	client := &fakeClient{}
	plugin := testPlugin(client)

	plugintest.RunOK[pluginbinding.ListResult[MergeRequest]](t, plugin, OperationMRList, map[string]any{"project": "42"})
	if client.mrListOptions.Project != "42" || client.mrListOptions.State != "opened" || client.mrListOptions.Limit != 20 {
		t.Fatalf("mr list defaults = %#v", client.mrListOptions)
	}
	if client.mrListOptions.OrderBy != "updated_at" || client.mrListOptions.Sort != "desc" {
		t.Fatalf("mr list defaults = %#v", client.mrListOptions)
	}
}

func TestServiceMRCreateUsesClient(t *testing.T) {
	client := &fakeClient{mergeRequest: MergeRequest{IID: 12, ProjectID: 42, Title: "Ship"}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[MergeRequest](t, plugin, OperationMRCreate, map[string]any{
		"project":              "group/dex",
		"title":                "Ship",
		"source_branch":        "feature",
		"target_branch":        "main",
		"description":          "Ready",
		"labels":               []any{"team", "release"},
		"reviewer_ids":         []any{float64(9), float64(10)},
		"remove_source_branch": true,
		"squash":               true,
	})
	if client.mrCreateProject != "group/dex" {
		t.Fatalf("mr create project = %#v", client.mrCreateProject)
	}
	if client.mrCreateOptions.Title != "Ship" || client.mrCreateOptions.SourceBranch != "feature" || client.mrCreateOptions.TargetBranch != "main" {
		t.Fatalf("mr create options = %#v", client.mrCreateOptions)
	}
	if len(client.mrCreateOptions.Labels) != 2 || client.mrCreateOptions.Labels[0] != "team" || len(client.mrCreateOptions.ReviewerIDs) != 2 {
		t.Fatalf("mr create options = %#v", client.mrCreateOptions)
	}
	if client.mrCreateOptions.RemoveSourceBranch == nil || !*client.mrCreateOptions.RemoveSourceBranch || client.mrCreateOptions.Squash == nil || !*client.mrCreateOptions.Squash {
		t.Fatalf("mr create bool options = %#v", client.mrCreateOptions)
	}
	if out.IID != 12 || out.Title != "Ship" {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func TestServiceMRApproveUsesRefAndSHA(t *testing.T) {
	client := &fakeClient{approval: MergeRequestApproval{IID: 12, ProjectID: 42, Approved: true}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[MergeRequestApproval](t, plugin, OperationMRApprove, map[string]any{"ref": "group/dex!12", "sha": "abc"})
	if client.mrApproveProject != "group/dex" || client.mrApproveIID != 12 || client.mrApproveOptions.SHA != "abc" {
		t.Fatalf("mr approve = %#v ! %d %#v", client.mrApproveProject, client.mrApproveIID, client.mrApproveOptions)
	}
	if !out.Approved || out.IID != 12 {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func TestServiceMRMergeUsesProjectIIDAndOptions(t *testing.T) {
	client := &fakeClient{mergeRequest: MergeRequest{IID: 12, ProjectID: 42, State: "merged"}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[MergeRequest](t, plugin, OperationMRMerge, map[string]any{
		"project":               "group/dex",
		"iid":                   12,
		"auto_merge":            true,
		"squash":                true,
		"remove_source_branch":  true,
		"merge_commit_message":  "Merge it",
		"squash_commit_message": "Squash it",
		"sha":                   "abc",
	})
	if client.mrMergeProject != "group/dex" || client.mrMergeIID != 12 {
		t.Fatalf("mr merge address = %#v ! %d", client.mrMergeProject, client.mrMergeIID)
	}
	if client.mrMergeOptions.AutoMerge == nil || !*client.mrMergeOptions.AutoMerge || client.mrMergeOptions.Squash == nil || !*client.mrMergeOptions.Squash {
		t.Fatalf("mr merge bool options = %#v", client.mrMergeOptions)
	}
	if client.mrMergeOptions.ShouldRemoveSourceBranch == nil || !*client.mrMergeOptions.ShouldRemoveSourceBranch || client.mrMergeOptions.SHA != "abc" {
		t.Fatalf("mr merge options = %#v", client.mrMergeOptions)
	}
	if out.State != "merged" {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func TestServiceBranchCreateUsesClient(t *testing.T) {
	client := &fakeClient{branch: Branch{Name: "feature/x", WebURL: "https://gitlab.example.com/group/dex/-/tree/feature/x"}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[Branch](t, plugin, OperationBranchCreate, map[string]any{
		"project": "group/dex",
		"branch":  "feature/x",
		"ref":     "main",
	})
	if client.branchProject != "group/dex" {
		t.Fatalf("branch project = %#v", client.branchProject)
	}
	if client.branchCreateOptions.Branch != "feature/x" || client.branchCreateOptions.Ref != "main" {
		t.Fatalf("branch options = %#v", client.branchCreateOptions)
	}
	if out.Name != "feature/x" {
		t.Fatalf("branch output = %#v", out)
	}
}

func TestServiceBranchCreateValidatesInput(t *testing.T) {
	client := &fakeClient{}
	plugin := testPlugin(client)
	plugintest.RunError(t, plugin, OperationBranchCreate, map[string]any{"branch": "x", "ref": "main"})
	plugintest.RunError(t, plugin, OperationBranchCreate, map[string]any{"project": "p", "ref": "main"})
	plugintest.RunError(t, plugin, OperationBranchCreate, map[string]any{"project": "p", "branch": "x"})
}

func TestServiceBranchDeleteUsesClient(t *testing.T) {
	client := &fakeClient{}
	plugin := testPlugin(client)

	out := plugintest.RunOK[BranchActionResult](t, plugin, OperationBranchDelete, map[string]any{
		"project": "group/dex",
		"branch":  "feature/x",
	})
	if client.branchProject != "group/dex" || client.branchDeleted != "feature/x" {
		t.Fatalf("branch delete = %#v %#v", client.branchProject, client.branchDeleted)
	}
	if out.Branch != "feature/x" || out.Message == "" {
		t.Fatalf("delete output = %#v", out)
	}
}

func TestServiceBranchDeleteMergedUsesClient(t *testing.T) {
	client := &fakeClient{}
	plugin := testPlugin(client)

	plugintest.RunOK[BranchActionResult](t, plugin, OperationBranchDeleteMerged, map[string]any{"project": "group/dex"})
	if client.mergedBranchProject != "group/dex" {
		t.Fatalf("merged branch project = %#v", client.mergedBranchProject)
	}
}

func TestServiceRepoFileCreateUsesClient(t *testing.T) {
	client := &fakeClient{repoFile: RepoFile{FilePath: "README.md", Branch: "main"}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[RepoFile](t, plugin, OperationRepoFileCreate, map[string]any{
		"project":        "group/dex",
		"file_path":      "README.md",
		"branch":         "main",
		"content":        "# hi",
		"commit_message": "init",
	})
	if client.repoFileProject != "group/dex" {
		t.Fatalf("repo file project = %#v", client.repoFileProject)
	}
	if client.repoFileCreate.FilePath != "README.md" || client.repoFileCreate.Branch != "main" || client.repoFileCreate.Content != "# hi" || client.repoFileCreate.CommitMessage != "init" {
		t.Fatalf("repo file create options = %#v", client.repoFileCreate)
	}
	if out.FilePath != "README.md" {
		t.Fatalf("repo file output = %#v", out)
	}
}

func TestServiceRepoFileCreateRequiresContent(t *testing.T) {
	client := &fakeClient{}
	plugin := testPlugin(client)
	plugintest.RunError(t, plugin, OperationRepoFileCreate, map[string]any{
		"project":        "group/dex",
		"file_path":      "README.md",
		"branch":         "main",
		"commit_message": "init",
	})
}

func TestServiceRepoFileUpdateUsesClient(t *testing.T) {
	client := &fakeClient{repoFile: RepoFile{FilePath: "README.md", Branch: "main"}}
	plugin := testPlugin(client)

	plugintest.RunOK[RepoFile](t, plugin, OperationRepoFileUpdate, map[string]any{
		"project":        "group/dex",
		"file_path":      "README.md",
		"branch":         "main",
		"content":        "# v2",
		"commit_message": "update",
		"last_commit_id": "abc",
	})
	if client.repoFileUpdate.LastCommitID != "abc" || client.repoFileUpdate.Content != "# v2" {
		t.Fatalf("repo file update options = %#v", client.repoFileUpdate)
	}
}

func TestServiceRepoFileDeleteUsesClient(t *testing.T) {
	client := &fakeClient{}
	plugin := testPlugin(client)

	out := plugintest.RunOK[RepoFileActionResult](t, plugin, OperationRepoFileDelete, map[string]any{
		"project":        "group/dex",
		"file_path":      "README.md",
		"branch":         "main",
		"commit_message": "remove",
	})
	if client.repoFileDelete.FilePath != "README.md" || client.repoFileDelete.Branch != "main" || client.repoFileDelete.CommitMessage != "remove" {
		t.Fatalf("repo file delete options = %#v", client.repoFileDelete)
	}
	if out.Message == "" {
		t.Fatalf("delete output = %#v", out)
	}
}

func TestServiceCommitCreateUsesClient(t *testing.T) {
	client := &fakeClient{commit: Commit{ID: "deadbeef", ShortID: "dead", Title: "init"}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[Commit](t, plugin, OperationCommitCreate, map[string]any{
		"project":        "group/dex",
		"branch":         "main",
		"commit_message": "bundle",
		"actions": []any{
			map[string]any{"action": "create", "file_path": "a.txt", "content": "a"},
			map[string]any{"action": "update", "file_path": "b.txt", "content": "b"},
		},
	})
	if client.commitProject != "group/dex" {
		t.Fatalf("commit project = %#v", client.commitProject)
	}
	if len(client.commitOptions.Actions) != 2 || client.commitOptions.Actions[0].Action != "create" || client.commitOptions.Actions[1].Action != "update" {
		t.Fatalf("commit actions = %#v", client.commitOptions.Actions)
	}
	if out.ID != "deadbeef" {
		t.Fatalf("commit output = %#v", out)
	}
}

func TestServiceCommitCreateValidatesActions(t *testing.T) {
	client := &fakeClient{}
	plugin := testPlugin(client)
	plugintest.RunError(t, plugin, OperationCommitCreate, map[string]any{
		"project":        "group/dex",
		"branch":         "main",
		"commit_message": "bundle",
	})
	plugintest.RunError(t, plugin, OperationCommitCreate, map[string]any{
		"project":        "group/dex",
		"branch":         "main",
		"commit_message": "bundle",
		"actions": []any{
			map[string]any{"action": "create"},
		},
	})
	plugintest.RunError(t, plugin, OperationCommitCreate, map[string]any{
		"project":        "group/dex",
		"branch":         "main",
		"commit_message": "bundle",
		"actions": []any{
			map[string]any{"action": "bogus", "file_path": "a.txt"},
		},
	})
}

func TestServiceCIVariableCreateUsesClient(t *testing.T) {
	client := &fakeClient{ciVariable: CIVariable{Key: "K", Value: "V", EnvironmentScope: "*"}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[CIVariable](t, plugin, OperationCIVariableCreate, map[string]any{
		"project":           "group/dex",
		"key":               "K",
		"value":             "V",
		"environment_scope": "*",
		"variable_type":     "env_var",
	})
	if client.ciVariableProject != "group/dex" || client.ciVariableCreate.Key != "K" || client.ciVariableCreate.Value != "V" || client.ciVariableCreate.VariableType != "env_var" {
		t.Fatalf("ci variable create options = %#v", client.ciVariableCreate)
	}
	if out.Key != "K" {
		t.Fatalf("ci variable output = %#v", out)
	}
}

func TestServiceCIVariableUpdateUsesClient(t *testing.T) {
	client := &fakeClient{ciVariable: CIVariable{Key: "K", Value: "V2"}}
	plugin := testPlugin(client)

	plugintest.RunOK[CIVariable](t, plugin, OperationCIVariableUpdate, map[string]any{
		"project": "group/dex",
		"key":     "K",
		"value":   "V2",
	})
	if client.ciVariableUpdateKey != "K" || client.ciVariableUpdate.Value != "V2" {
		t.Fatalf("ci variable update = %#v %#v", client.ciVariableUpdateKey, client.ciVariableUpdate)
	}
}

func TestServiceCIVariableDeleteUsesClient(t *testing.T) {
	client := &fakeClient{}
	plugin := testPlugin(client)

	plugintest.RunOK[CIVariableActionResult](t, plugin, OperationCIVariableDelete, map[string]any{
		"project":           "group/dex",
		"key":               "K",
		"environment_scope": "prod",
	})
	if client.ciVariableDeleteKey != "K" || client.ciVariableDelete.EnvironmentScope != "prod" {
		t.Fatalf("ci variable delete = %#v %#v", client.ciVariableDeleteKey, client.ciVariableDelete)
	}
}

func TestServicePipelineCreateUsesClient(t *testing.T) {
	client := &fakeClient{pipeline: Pipeline{ID: 5, Ref: "main", Status: "pending"}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[Pipeline](t, plugin, OperationPipelineCreate, map[string]any{
		"project": "group/dex",
		"ref":     "main",
		"variables": []any{
			map[string]any{"key": "TOKEN", "value": "v", "variable_type": "env_var"},
		},
	})
	if client.pipelineProject != "group/dex" || client.pipelineCreate.Ref != "main" {
		t.Fatalf("pipeline create = %#v %#v", client.pipelineProject, client.pipelineCreate)
	}
	if len(client.pipelineCreate.Variables) != 1 || client.pipelineCreate.Variables[0].Key != "TOKEN" {
		t.Fatalf("pipeline variables = %#v", client.pipelineCreate.Variables)
	}
	if out.ID != 5 {
		t.Fatalf("pipeline output = %#v", out)
	}
}

func TestServicePipelineRetryAndCancelUseClient(t *testing.T) {
	client := &fakeClient{pipeline: Pipeline{ID: 7, Status: "running"}}
	plugin := testPlugin(client)

	plugintest.RunOK[Pipeline](t, plugin, OperationPipelineRetry, map[string]any{"project": "group/dex", "pipeline_id": 7})
	if client.pipelineRetryProj != "group/dex" || client.pipelineRetryID != 7 {
		t.Fatalf("pipeline retry = %#v %d", client.pipelineRetryProj, client.pipelineRetryID)
	}

	plugintest.RunOK[Pipeline](t, plugin, OperationPipelineCancel, map[string]any{"project": "group/dex", "pipeline_id": 7})
	if client.pipelineCancelProj != "group/dex" || client.pipelineCancelID != 7 {
		t.Fatalf("pipeline cancel = %#v %d", client.pipelineCancelProj, client.pipelineCancelID)
	}
}

func TestServicePipelineRetryRequiresID(t *testing.T) {
	client := &fakeClient{}
	plugin := testPlugin(client)
	plugintest.RunError(t, plugin, OperationPipelineRetry, map[string]any{"project": "group/dex"})
}

func TestServiceSnippetCreateUsesClient(t *testing.T) {
	client := &fakeClient{snippet: Snippet{ID: 99, Title: "Note"}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[Snippet](t, plugin, OperationSnippetCreate, map[string]any{
		"title":      "Note",
		"visibility": "private",
		"files": []any{
			map[string]any{"file_path": "note.txt", "content": "hi"},
		},
	})
	if client.snippetCreate.Title != "Note" || client.snippetCreate.Visibility != "private" {
		t.Fatalf("snippet create options = %#v", client.snippetCreate)
	}
	if len(client.snippetCreate.Files) != 1 || client.snippetCreate.Files[0].FilePath != "note.txt" {
		t.Fatalf("snippet files = %#v", client.snippetCreate.Files)
	}
	if out.ID != 99 {
		t.Fatalf("snippet output = %#v", out)
	}
}

func TestServiceSnippetCreateRejectsInvalidVisibility(t *testing.T) {
	client := &fakeClient{}
	plugin := testPlugin(client)
	plugintest.RunError(t, plugin, OperationSnippetCreate, map[string]any{
		"title":      "Note",
		"visibility": "secret",
		"files": []any{
			map[string]any{"file_path": "note.txt", "content": "hi"},
		},
	})
}

func TestServiceSnippetDeleteUsesClient(t *testing.T) {
	client := &fakeClient{}
	plugin := testPlugin(client)

	plugintest.RunOK[SnippetActionResult](t, plugin, OperationSnippetDelete, map[string]any{"snippet_id": 99})
	if client.snippetDeleted != 99 {
		t.Fatalf("snippet deleted = %d", client.snippetDeleted)
	}
}

func TestServiceRepositoryTagCreateUsesClient(t *testing.T) {
	client := &fakeClient{repositoryTag: RepositoryTag{Name: "v1.2.3", Target: "abc"}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[RepositoryTag](t, plugin, OperationTagCreate, map[string]any{
		"project":  "group/dex",
		"tag_name": "v1.2.3",
		"ref":      "main",
		"message":  "release",
	})
	if client.repositoryTagProject != "group/dex" {
		t.Fatalf("tag create project = %#v", client.repositoryTagProject)
	}
	if client.repositoryTagOptions.TagName != "v1.2.3" || client.repositoryTagOptions.Ref != "main" || client.repositoryTagOptions.Message != "release" {
		t.Fatalf("tag create options = %#v", client.repositoryTagOptions)
	}
	if out.Name != "v1.2.3" || out.Target != "abc" {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func TestServiceIndexBuildReturnsNormalizedRecords(t *testing.T) {
	client := &fakeClient{
		projects: []Project{{ID: 1, Name: "dex", PathWithNamespace: "group/dex", WebURL: "https://gitlab.example.com/group/dex"}},
		users:    []User{{ID: 9, Username: "timo", Name: "Timo", WebURL: "https://gitlab.example.com/timo", State: "active"}},
		groups:   []Group{{ID: 2, Name: "group", FullPath: "group", WebURL: "https://gitlab.example.com/group"}},
		issues:   []Issue{{ID: 3, IID: 4, ProjectID: 1, Title: "Fix", WebURL: "https://gitlab.example.com/group/dex/-/issues/4", Reference: "group/dex#4"}},
		mergeRequests: []MergeRequest{{
			ID: 5, IID: 6, ProjectID: 1, Title: "Ship", WebURL: "https://gitlab.example.com/group/dex/-/merge_requests/6", Reference: "group/dex!6",
		}},
	}
	plugin := testPlugin(client)

	out := plugintest.RunOK[struct {
		Index   string          `json:"index"`
		Records []ProjectRecord `json:"records"`
		Indexes []struct {
			Index   string            `json:"index"`
			Records []json.RawMessage `json:"records"`
		} `json:"indexes"`
	}](t, plugin, OperationIndexBuild, map[string]any{})
	if out.Index != "gitlab.projects" || len(out.Records) != 1 || out.Records[0].Entity != "gitlab.project" {
		t.Fatalf("unexpected index output: %#v", out)
	}
	if out.Records[0].Source.Plugin != PluginName || out.Records[0].Source.Instance != "default" || out.Records[0].Links["self"] != "https://gitlab.example.com/group/dex" {
		t.Fatalf("unexpected record source/links: %#v", out.Records[0])
	}
	if len(out.Indexes) != 5 || out.Indexes[0].Index != "gitlab.projects" || out.Indexes[1].Index != "gitlab.users" || out.Indexes[2].Index != "gitlab.groups" || out.Indexes[3].Index != "gitlab.issues" || out.Indexes[4].Index != "gitlab.merge_requests" {
		t.Fatalf("unexpected multi-index output: %#v", out.Indexes)
	}
	if len(out.Indexes[1].Records) != 1 || len(out.Indexes[2].Records) != 1 || len(out.Indexes[3].Records) != 1 || len(out.Indexes[4].Records) != 1 {
		t.Fatalf("unexpected multi-index records: %#v", out.Indexes)
	}
	if !client.listOptions.All || client.listOptions.Limit != 100 {
		t.Fatalf("index list options = %#v", client.listOptions)
	}
	if !client.userListOptions.All || client.userListOptions.Limit != 100 {
		t.Fatalf("user list options = %#v", client.userListOptions)
	}
	if !client.groupListOptions.All || client.groupListOptions.Limit != 100 {
		t.Fatalf("group list options = %#v", client.groupListOptions)
	}
	if !client.issueListOptions.All || client.issueListOptions.Limit != 100 || client.issueListOptions.State != "all" {
		t.Fatalf("issue list options = %#v", client.issueListOptions)
	}
	if !client.mrListOptions.All || client.mrListOptions.Limit != 100 || client.mrListOptions.State != "all" {
		t.Fatalf("mr list options = %#v", client.mrListOptions)
	}
}

func TestServiceIndexBuildCanTargetOneIndex(t *testing.T) {
	client := &fakeClient{
		groups: []Group{{ID: 2, Name: "group", FullPath: "group", WebURL: "https://gitlab.example.com/group"}},
	}
	plugin := testPlugin(client)

	out := plugintest.RunOK[struct {
		Indexes []struct {
			Index   string            `json:"index"`
			Records []json.RawMessage `json:"records"`
		} `json:"indexes"`
	}](t, plugin, OperationIndexBuild, map[string]any{"entity": "gitlab.group"})
	if len(out.Indexes) != 1 || out.Indexes[0].Index != "gitlab.groups" || len(out.Indexes[0].Records) != 1 {
		t.Fatalf("unexpected targeted index output: %#v", out.Indexes)
	}
	if client.listOptions.All || client.userListOptions.All || client.issueListOptions.All || client.mrListOptions.All {
		t.Fatalf("targeted build fetched unrelated indexes: projects=%#v users=%#v issues=%#v mrs=%#v", client.listOptions, client.userListOptions, client.issueListOptions, client.mrListOptions)
	}
}

func TestServiceLookupUsesSharedDatasourceShape(t *testing.T) {
	client := &fakeClient{
		project:       Project{ID: 1, Name: "dex", NameWithNamespace: "group / dex", PathWithNamespace: "group/dex", WebURL: "https://gitlab.example.com/group/dex"},
		users:         []User{{ID: 9, Username: "timo", Name: "Timo Friedl", WebURL: "https://gitlab.example.com/timo"}},
		mergeRequest:  MergeRequest{ID: 5, IID: 6, ProjectID: 1, Title: "Ship", WebURL: "https://gitlab.example.com/group/dex/-/merge_requests/6", Reference: "group/dex!6"},
		mergeRequests: []MergeRequest{{ID: 7, IID: 8, ProjectID: 1, Title: "Timo change", WebURL: "https://gitlab.example.com/group/dex/-/merge_requests/8", Reference: "group/dex!8"}},
	}
	plugin := testPlugin(client)

	out := plugintest.DatasourceLookupOK[LookupResult](t, plugin, map[string]any{"text": "look at https://gitlab.example.com/group/dex/-/merge_requests/6 with timo", "limit": 10}, plugintest.WithInstance("work"))
	if out.Source != PluginName || out.Count < 3 {
		t.Fatalf("lookup output = %#v", out)
	}
	if out.Matches[0].Entity != EntityMergeRequest || out.Matches[0].ID != "group/dex!6" {
		t.Fatalf("first match = %#v", out.Matches[0])
	}
	if out.Matches[0].Source.Plugin != PluginName || out.Matches[0].Source.Instance != "work" || out.Matches[0].Source.Index != DatasourceMergeRequests {
		t.Fatalf("lookup source = %#v", out.Matches[0].Source)
	}
	if client.mrProject != "group/dex" || client.mrIID != 6 {
		t.Fatalf("mr lookup = %#v ! %d", client.mrProject, client.mrIID)
	}
}

func TestServiceLookupCanFilterEntity(t *testing.T) {
	client := &fakeClient{
		projects: []Project{{ID: 1, Name: "timo", PathWithNamespace: "group/timo", WebURL: "https://gitlab.example.com/group/timo"}},
		users:    []User{{ID: 9, Username: "timo", Name: "Timo Friedl", WebURL: "https://gitlab.example.com/timo"}},
	}
	plugin := testPlugin(client)

	out := plugintest.DatasourceLookupOK[LookupResult](t, plugin, map[string]any{"text": "timo", "entity": EntityUser})
	if out.Count != 1 || out.Matches[0].Entity != EntityUser || out.Matches[0].ID != "timo" {
		t.Fatalf("lookup output = %#v", out)
	}
	if client.listOptions.Search != "" {
		t.Fatalf("entity-filtered lookup should not fetch projects: %#v", client.listOptions)
	}
}

func TestServiceDatasourceSearchUsesProvider(t *testing.T) {
	client := &fakeClient{
		projects: []Project{{ID: 1, Name: "slack-bot", NameWithNamespace: "ai / agents / slack-bot", PathWithNamespace: "ai/agents/slack-bot", WebURL: "https://gitlab.example.com/ai/agents/slack-bot"}},
	}
	plugin := testPlugin(client)

	out := plugintest.DatasourceSearchOK[ProjectSearchResult](t, plugin, map[string]any{"entity": EntityProject, "query": "slack-bot", "limit": 5}, plugintest.WithInstance("work"))
	if out.Source != PluginName || out.Count != 1 || out.Records[0].ID != "ai/agents/slack-bot" {
		t.Fatalf("search output = %#v", out)
	}
	if client.listOptions.Search != "slack-bot" || client.listOptions.Limit != 5 || client.listOptions.Membership == nil || !*client.listOptions.Membership {
		t.Fatalf("project search options = %#v", client.listOptions)
	}
	if out.Records[0].Source.Instance != "work" {
		t.Fatalf("record source = %#v", out.Records[0].Source)
	}
}

func TestServiceDatasourceGetUsesProvider(t *testing.T) {
	client := &fakeClient{project: Project{ID: 1, Name: "slack-bot", NameWithNamespace: "ai / agents / slack-bot", PathWithNamespace: "ai/agents/slack-bot"}}
	plugin := testPlugin(client)

	out := plugintest.DatasourceGetOK[ProjectGetResult](t, plugin, map[string]any{"entity": EntityProject, "id": "ai/agents/slack-bot"})
	if out.Source != PluginName || out.Record.ID != "ai/agents/slack-bot" {
		t.Fatalf("get output = %#v", out)
	}
	if client.projectID != "ai/agents/slack-bot" {
		t.Fatalf("project id = %#v", client.projectID)
	}
}

func TestServiceBuildsClientFromHostContext(t *testing.T) {
	client := &fakeClient{user: User{ID: 9, Username: "timo"}}
	var captured pluginbinding.Context
	plugin := NewPluginWithService(Service{
		ClientFactory: func(ctx pluginbinding.Context) (Client, error) {
			captured = ctx
			return client, nil
		},
	})

	plugintest.RunOK[AuthTestResult](t, plugin, OperationAuthTest, map[string]any{}, plugintest.WithInstance("work"))
	if captured.Request.Instance != "work" || captured.Host == nil {
		t.Fatalf("captured context = %#v", captured)
	}
}

func testPlugin(client Client) *pluginbinding.Plugin {
	return NewPluginWithService(Service{ClientFactory: func(pluginbinding.Context) (Client, error) { return client, nil }})
}

type fakeClient struct {
	user                 User
	projects             []Project
	users                []User
	groups               []Group
	issues               []Issue
	project              Project
	mergeRequest         MergeRequest
	mergeRequests        []MergeRequest
	approval             MergeRequestApproval
	repositoryTag        RepositoryTag
	branch               Branch
	repoFile             RepoFile
	commit               Commit
	ciVariable           CIVariable
	pipeline             Pipeline
	snippet              Snippet
	listOptions          ProjectListOptions
	userListOptions      UserListOptions
	groupListOptions     GroupListOptions
	issueListOptions     IssueListOptions
	mrListOptions        MergeRequestListOptions
	mrCreateOptions      MergeRequestCreateOptions
	mrApproveOptions     MergeRequestApproveOptions
	mrMergeOptions       MergeRequestMergeOptions
	repositoryTagOptions RepositoryTagCreateOptions
	branchCreateOptions  BranchCreateOptions
	repoFileCreate       RepoFileCreateOptions
	repoFileUpdate       RepoFileUpdateOptions
	repoFileDelete       RepoFileDeleteOptions
	commitOptions        CommitCreateOptions
	ciVariableCreate     CIVariableCreateOptions
	ciVariableUpdate     CIVariableUpdateOptions
	ciVariableDelete     CIVariableDeleteOptions
	pipelineCreate       PipelineCreateOptions
	snippetCreate        SnippetCreateOptions
	projectID            any
	mrProject            any
	mrIID                int64
	mrCreateProject      any
	mrApproveProject     any
	mrApproveIID         int64
	mrMergeProject       any
	mrMergeIID           int64
	repositoryTagProject any
	branchProject        any
	branchDeleted        string
	mergedBranchProject  any
	repoFileProject      any
	commitProject        any
	ciVariableProject    any
	ciVariableUpdateKey  string
	ciVariableDeleteKey  string
	pipelineProject      any
	pipelineRetryID      int64
	pipelineCancelID     int64
	pipelineRetryProj    any
	pipelineCancelProj   any
	snippetDeleted       int64
}

func (c *fakeClient) CurrentUser() (User, error) {
	return c.user, nil
}

func (c *fakeClient) ListProjects(options ProjectListOptions) ([]Project, error) {
	c.listOptions = options
	return c.projects, nil
}

func (c *fakeClient) GetProject(id any) (Project, error) {
	c.projectID = id
	return c.project, nil
}

func (c *fakeClient) ListUsers(options UserListOptions) ([]User, error) {
	c.userListOptions = options
	return c.users, nil
}

func (c *fakeClient) ListGroups(options GroupListOptions) ([]Group, error) {
	c.groupListOptions = options
	return c.groups, nil
}

func (c *fakeClient) ListIssues(options IssueListOptions) ([]Issue, error) {
	c.issueListOptions = options
	return c.issues, nil
}

func (c *fakeClient) ListMergeRequests(options MergeRequestListOptions) ([]MergeRequest, error) {
	c.mrListOptions = options
	return c.mergeRequests, nil
}

func (c *fakeClient) GetMergeRequest(project any, iid int64) (MergeRequest, error) {
	c.mrProject = project
	c.mrIID = iid
	return c.mergeRequest, nil
}

func (c *fakeClient) CreateMergeRequest(project any, options MergeRequestCreateOptions) (MergeRequest, error) {
	c.mrCreateProject = project
	c.mrCreateOptions = options
	return c.mergeRequest, nil
}

func (c *fakeClient) ApproveMergeRequest(project any, iid int64, options MergeRequestApproveOptions) (MergeRequestApproval, error) {
	c.mrApproveProject = project
	c.mrApproveIID = iid
	c.mrApproveOptions = options
	return c.approval, nil
}

func (c *fakeClient) MergeMergeRequest(project any, iid int64, options MergeRequestMergeOptions) (MergeRequest, error) {
	c.mrMergeProject = project
	c.mrMergeIID = iid
	c.mrMergeOptions = options
	return c.mergeRequest, nil
}

func (c *fakeClient) CreateRepositoryTag(project any, options RepositoryTagCreateOptions) (RepositoryTag, error) {
	c.repositoryTagProject = project
	c.repositoryTagOptions = options
	return c.repositoryTag, nil
}

func (c *fakeClient) CreateBranch(project any, options BranchCreateOptions) (Branch, error) {
	c.branchProject = project
	c.branchCreateOptions = options
	return c.branch, nil
}

func (c *fakeClient) DeleteBranch(project any, branch string) error {
	c.branchProject = project
	c.branchDeleted = branch
	return nil
}

func (c *fakeClient) DeleteMergedBranches(project any) error {
	c.mergedBranchProject = project
	return nil
}

func (c *fakeClient) CreateRepositoryFile(project any, options RepoFileCreateOptions) (RepoFile, error) {
	c.repoFileProject = project
	c.repoFileCreate = options
	return c.repoFile, nil
}

func (c *fakeClient) UpdateRepositoryFile(project any, options RepoFileUpdateOptions) (RepoFile, error) {
	c.repoFileProject = project
	c.repoFileUpdate = options
	return c.repoFile, nil
}

func (c *fakeClient) DeleteRepositoryFile(project any, options RepoFileDeleteOptions) error {
	c.repoFileProject = project
	c.repoFileDelete = options
	return nil
}

func (c *fakeClient) CreateCommit(project any, options CommitCreateOptions) (Commit, error) {
	c.commitProject = project
	c.commitOptions = options
	return c.commit, nil
}

func (c *fakeClient) CreateCIVariable(project any, options CIVariableCreateOptions) (CIVariable, error) {
	c.ciVariableProject = project
	c.ciVariableCreate = options
	return c.ciVariable, nil
}

func (c *fakeClient) UpdateCIVariable(project any, key string, options CIVariableUpdateOptions) (CIVariable, error) {
	c.ciVariableProject = project
	c.ciVariableUpdateKey = key
	c.ciVariableUpdate = options
	return c.ciVariable, nil
}

func (c *fakeClient) DeleteCIVariable(project any, key string, options CIVariableDeleteOptions) error {
	c.ciVariableProject = project
	c.ciVariableDeleteKey = key
	c.ciVariableDelete = options
	return nil
}

func (c *fakeClient) CreatePipeline(project any, options PipelineCreateOptions) (Pipeline, error) {
	c.pipelineProject = project
	c.pipelineCreate = options
	return c.pipeline, nil
}

func (c *fakeClient) RetryPipeline(project any, id int64) (Pipeline, error) {
	c.pipelineRetryProj = project
	c.pipelineRetryID = id
	return c.pipeline, nil
}

func (c *fakeClient) CancelPipeline(project any, id int64) (Pipeline, error) {
	c.pipelineCancelProj = project
	c.pipelineCancelID = id
	return c.pipeline, nil
}

func (c *fakeClient) CreateSnippet(options SnippetCreateOptions) (Snippet, error) {
	c.snippetCreate = options
	return c.snippet, nil
}

func (c *fakeClient) DeleteSnippet(id int64) error {
	c.snippetDeleted = id
	return nil
}
