package gitlab

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
)

func TestServiceProjectListUsesClient(t *testing.T) {
	client := &fakeClient{
		projects: []Project{{ID: 1, Name: "dex", PathWithNamespace: "group/dex"}},
	}
	plugin := testPlugin(client, nil)

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
	plugin := testPlugin(client, nil)

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
	plugin := testPlugin(client, nil)

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
	plugin := testPlugin(client, nil)

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
	plugin := testPlugin(client, nil)

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
	plugin := testPlugin(client, nil)

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
	plugin := testPlugin(client, nil)

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
	plugin := testPlugin(client, nil)

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

func TestServiceRepositoryTagCreateUsesClient(t *testing.T) {
	client := &fakeClient{repositoryTag: RepositoryTag{Name: "v1.2.3", Target: "abc"}}
	plugin := testPlugin(client, nil)

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
	plugin := testPlugin(client, nil)

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
	plugin := testPlugin(client, nil)

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
	plugin := testPlugin(client, nil)

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
	plugin := testPlugin(client, nil)

	out := plugintest.DatasourceLookupOK[LookupResult](t, plugin, map[string]any{"text": "timo", "entity": EntityUser})
	if out.Count != 1 || out.Matches[0].Entity != EntityUser || out.Matches[0].ID != "timo" {
		t.Fatalf("lookup output = %#v", out)
	}
	if client.listOptions.Search != "" {
		t.Fatalf("entity-filtered lookup should not fetch projects: %#v", client.listOptions)
	}
}

func TestServiceBuildsClientFromResolvedSecrets(t *testing.T) {
	client := &fakeClient{user: User{ID: 9, Username: "timo"}}
	var captured SecretSet
	plugin := NewPluginWithService(Service{
		SecretGetter: func(_ pluginbinding.Context, purpose string) (pluginbinding.SecretMaterial, error) {
			switch purpose {
			case "access_token":
				return pluginbinding.SecretMaterial{Value: "token", Source: "store"}, nil
			case "gitlab_url":
				return pluginbinding.SecretMaterial{Kind: "config", Value: "https://gitlab.example.com", Source: "store"}, nil
			default:
				return pluginbinding.SecretMaterial{}, errors.New("unexpected purpose")
			}
		},
		ClientFactory: func(secrets SecretSet) (Client, error) {
			captured = secrets
			return client, nil
		},
	})

	plugintest.RunOK[AuthTestResult](t, plugin, OperationAuthTest, map[string]any{}, plugintest.WithInstance("work"))
	if captured.AccessToken.Value != "token" || captured.GitLabURL.Value != "https://gitlab.example.com" {
		t.Fatalf("captured secrets = %#v", captured)
	}
}

func testPlugin(client Client, get pluginbinding.SecretGetter) *pluginbinding.Plugin {
	if get == nil {
		get = func(_ pluginbinding.Context, purpose string) (pluginbinding.SecretMaterial, error) {
			return pluginbinding.SecretMaterial{Value: purpose}, nil
		}
	}
	return NewPluginWithService(Service{SecretGetter: get, ClientFactory: func(SecretSet) (Client, error) { return client, nil }})
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
	listOptions          ProjectListOptions
	userListOptions      UserListOptions
	groupListOptions     GroupListOptions
	issueListOptions     IssueListOptions
	mrListOptions        MergeRequestListOptions
	mrCreateOptions      MergeRequestCreateOptions
	mrApproveOptions     MergeRequestApproveOptions
	mrMergeOptions       MergeRequestMergeOptions
	repositoryTagOptions RepositoryTagCreateOptions
	projectID            any
	mrProject            any
	mrIID                int64
	mrCreateProject      any
	mrApproveProject     any
	mrApproveIID         int64
	mrMergeProject       any
	mrMergeIID           int64
	repositoryTagProject any
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
