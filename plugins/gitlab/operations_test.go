package gitlab

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/fluxplane/fluxplane-dex/plugins/internal/pluginutil"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func TestOperationRunnerProjectListUsesClient(t *testing.T) {
	client := &fakeClient{
		projects: []Project{{ID: 1, Name: "dex", PathWithNamespace: "group/dex"}},
	}
	runner := testRunner(client, nil)

	result := runner.Run(protocol.Request{Instance: "work"}, callWithInput("gitlab.project.list", map[string]any{
		"limit":  5,
		"search": "dex",
	}), nil)
	if !result.OK {
		t.Fatalf("operation failed: %#v", result.Error)
	}
	if client.listOptions.Limit != 5 || client.listOptions.Search != "dex" {
		t.Fatalf("list options = %#v", client.listOptions)
	}
	var out struct {
		Count    int       `json:"count"`
		Projects []Project `json:"projects"`
	}
	decodeResult(t, result, &out)
	if out.Count != 1 || out.Projects[0].PathWithNamespace != "group/dex" {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func TestOperationRunnerProjectShowParsesNumericID(t *testing.T) {
	client := &fakeClient{project: Project{ID: 42, Name: "dex"}}
	runner := testRunner(client, nil)

	result := runner.Run(protocol.Request{Instance: "default"}, callWithInput("gitlab.project.show", map[string]any{"id": "42"}), nil)
	if !result.OK {
		t.Fatalf("operation failed: %#v", result.Error)
	}
	if client.projectID != int64(42) {
		t.Fatalf("project id = %#v", client.projectID)
	}
}

func TestOperationRunnerMRShowParsesReference(t *testing.T) {
	client := &fakeClient{mergeRequest: MergeRequest{IID: 7, ProjectID: 42, Title: "Update"}}
	runner := testRunner(client, nil)

	result := runner.Run(protocol.Request{Instance: "default"}, callWithInput("gitlab.mr.show", map[string]any{"ref": "group/dex!7"}), nil)
	if !result.OK {
		t.Fatalf("operation failed: %#v", result.Error)
	}
	if client.mrProject != "group/dex" || client.mrIID != 7 {
		t.Fatalf("mr lookup = %#v ! %d", client.mrProject, client.mrIID)
	}
}

func TestOperationRunnerMRListUsesProjectPathAndOptions(t *testing.T) {
	client := &fakeClient{
		mergeRequests: []MergeRequest{{IID: 11, ProjectID: 42, Title: "Ship"}},
	}
	runner := testRunner(client, nil)

	result := runner.Run(protocol.Request{Instance: "default"}, callWithInput("gitlab.mr.list", map[string]any{
		"project":  "group/dex",
		"state":    "merged",
		"search":   "ship",
		"limit":    7,
		"order_by": "created_at",
		"sort":     "asc",
	}), nil)
	if !result.OK {
		t.Fatalf("operation failed: %#v", result.Error)
	}
	if client.mrListOptions.Project != "group/dex" || client.mrListOptions.State != "merged" || client.mrListOptions.Search != "ship" {
		t.Fatalf("mr list options = %#v", client.mrListOptions)
	}
	if client.mrListOptions.Limit != 7 || client.mrListOptions.OrderBy != "created_at" || client.mrListOptions.Sort != "asc" {
		t.Fatalf("mr list options = %#v", client.mrListOptions)
	}
	var out struct {
		Count         int            `json:"count"`
		MergeRequests []MergeRequest `json:"merge_requests"`
	}
	decodeResult(t, result, &out)
	if out.Count != 1 || out.MergeRequests[0].IID != 11 {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func TestOperationRunnerMRListDefaultsAndNumericProject(t *testing.T) {
	client := &fakeClient{}
	runner := testRunner(client, nil)

	result := runner.Run(protocol.Request{Instance: "default"}, callWithInput("gitlab.mr.list", map[string]any{"project": "42"}), nil)
	if !result.OK {
		t.Fatalf("operation failed: %#v", result.Error)
	}
	if client.mrListOptions.Project != "42" || client.mrListOptions.State != "opened" || client.mrListOptions.Limit != 20 {
		t.Fatalf("mr list defaults = %#v", client.mrListOptions)
	}
	if client.mrListOptions.OrderBy != "updated_at" || client.mrListOptions.Sort != "desc" {
		t.Fatalf("mr list defaults = %#v", client.mrListOptions)
	}
}

func TestOperationRunnerIndexBuildReturnsNormalizedRecords(t *testing.T) {
	client := &fakeClient{
		projects: []Project{{ID: 1, Name: "dex", PathWithNamespace: "group/dex", WebURL: "https://gitlab.example.com/group/dex"}},
		users:    []User{{ID: 9, Username: "timo", Name: "Timo", WebURL: "https://gitlab.example.com/timo", State: "active"}},
		groups:   []Group{{ID: 2, Name: "group", FullPath: "group", WebURL: "https://gitlab.example.com/group"}},
		issues:   []Issue{{ID: 3, IID: 4, ProjectID: 1, Title: "Fix", WebURL: "https://gitlab.example.com/group/dex/-/issues/4", Reference: "group/dex#4"}},
		mergeRequests: []MergeRequest{{
			ID: 5, IID: 6, ProjectID: 1, Title: "Ship", WebURL: "https://gitlab.example.com/group/dex/-/merge_requests/6", Reference: "group/dex!6",
		}},
	}
	runner := testRunner(client, nil)

	result := runner.Run(protocol.Request{Instance: "default"}, callWithInput("gitlab.index.build", map[string]any{}), nil)
	if !result.OK {
		t.Fatalf("operation failed: %#v", result.Error)
	}
	var out struct {
		Index   string          `json:"index"`
		Records []ProjectRecord `json:"records"`
		Indexes []struct {
			Index   string            `json:"index"`
			Records []json.RawMessage `json:"records"`
		} `json:"indexes"`
	}
	decodeResult(t, result, &out)
	if out.Index != "gitlab.projects" || len(out.Records) != 1 || out.Records[0].Entity != "gitlab.project" {
		t.Fatalf("unexpected index output: %#v", out)
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

func TestOperationRunnerIndexBuildCanTargetOneIndex(t *testing.T) {
	client := &fakeClient{
		groups: []Group{{ID: 2, Name: "group", FullPath: "group", WebURL: "https://gitlab.example.com/group"}},
	}
	runner := testRunner(client, nil)

	result := runner.Run(protocol.Request{Instance: "default"}, callWithInput("gitlab.index.build", map[string]any{"entity": "gitlab.group"}), nil)
	if !result.OK {
		t.Fatalf("operation failed: %#v", result.Error)
	}
	var out struct {
		Indexes []struct {
			Index   string            `json:"index"`
			Records []json.RawMessage `json:"records"`
		} `json:"indexes"`
	}
	decodeResult(t, result, &out)
	if len(out.Indexes) != 1 || out.Indexes[0].Index != "gitlab.groups" || len(out.Indexes[0].Records) != 1 {
		t.Fatalf("unexpected targeted index output: %#v", out.Indexes)
	}
	if client.listOptions.All || client.userListOptions.All || client.issueListOptions.All || client.mrListOptions.All {
		t.Fatalf("targeted build fetched unrelated indexes: projects=%#v users=%#v issues=%#v mrs=%#v", client.listOptions, client.userListOptions, client.issueListOptions, client.mrListOptions)
	}
}

func TestOperationRunnerBuildsClientFromResolvedSecrets(t *testing.T) {
	client := &fakeClient{user: User{ID: 9, Username: "timo"}}
	var captured SecretSet
	runner := OperationRunner{
		SecretGetter: func(_ protocol.Request, purpose string, _ map[string]pluginutil.SecretMaterial) (pluginutil.SecretMaterial, error) {
			switch purpose {
			case "access_token":
				return pluginutil.SecretMaterial{Value: "token", Source: "store"}, nil
			case "gitlab_url":
				return pluginutil.SecretMaterial{Kind: "config", Value: "https://gitlab.example.com", Source: "store"}, nil
			default:
				return pluginutil.SecretMaterial{}, errors.New("unexpected purpose")
			}
		},
		ClientFactory: func(secrets SecretSet) (Client, error) {
			captured = secrets
			return client, nil
		},
	}

	result := runner.Run(protocol.Request{Instance: "work"}, protocol.OperationCall{Name: "gitlab.auth.test"}, nil)
	if !result.OK {
		t.Fatalf("operation failed: %#v", result.Error)
	}
	if captured.AccessToken.Value != "token" || captured.GitLabURL.Value != "https://gitlab.example.com" {
		t.Fatalf("captured secrets = %#v", captured)
	}
}

func testRunner(client Client, get SecretGetter) OperationRunner {
	if get == nil {
		get = func(_ protocol.Request, purpose string, _ map[string]pluginutil.SecretMaterial) (pluginutil.SecretMaterial, error) {
			return pluginutil.SecretMaterial{Value: purpose}, nil
		}
	}
	return OperationRunner{SecretGetter: get, ClientFactory: func(SecretSet) (Client, error) { return client, nil }}
}

func callWithInput(name string, input map[string]any) protocol.OperationCall {
	raw, _ := json.Marshal(input)
	return protocol.OperationCall{Name: name, Input: raw}
}

func decodeResult(t *testing.T, result protocol.OperationResult, target any) {
	t.Helper()
	if err := json.Unmarshal(result.Result, target); err != nil {
		t.Fatal(err)
	}
}

type fakeClient struct {
	user             User
	projects         []Project
	users            []User
	groups           []Group
	issues           []Issue
	project          Project
	mergeRequest     MergeRequest
	mergeRequests    []MergeRequest
	listOptions      ProjectListOptions
	userListOptions  UserListOptions
	groupListOptions GroupListOptions
	issueListOptions IssueListOptions
	mrListOptions    MergeRequestListOptions
	projectID        any
	mrProject        any
	mrIID            int64
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
