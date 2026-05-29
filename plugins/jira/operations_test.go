package jira

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
)

func requestJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

func TestServiceBuildsClientFromEndpointRef(t *testing.T) {
	client := &fakeClient{user: User{AccountID: "acct-1", DisplayName: "Ada"}}
	var capturedEndpointRef string
	plugin := NewPluginWithService(Service{
		ClientFactory: func(_ pluginbinding.Context, endpointRef string) (Client, error) {
			capturedEndpointRef = endpointRef
			return client, nil
		},
	})

	out := plugintest.RunOK[AuthTestResult](t, plugin, OperationAuthTest, map[string]any{"endpoint_ref": "jira-dev"}, plugintest.WithInstance("work"))
	if out.Status != "ok" || out.User.AccountID != "acct-1" {
		t.Fatalf("auth output = %#v", out)
	}
	if capturedEndpointRef != "jira-dev" {
		t.Fatalf("endpoint_ref = %q", capturedEndpointRef)
	}
}

func TestIssueSearchAndDatasourceNormalizeRecords(t *testing.T) {
	client := &fakeClient{issues: []Issue{{
		ID:  "10001",
		Key: "DEX-7",
		Fields: IssueFields{
			Summary: "Ship Jira plugin",
			Status:  NamedValue{Name: "In Progress"},
			Project: Project{Key: "DEX", Name: "Dex"},
			Assignee: &User{
				AccountID: "acct-1", DisplayName: "Ada",
			},
			Reporter: &User{AccountID: "acct-2", DisplayName: "Timo"},
			Updated:  "2026-05-28T10:00:00.000+0000",
		},
	}}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[IssueSearchResult](t, plugin, OperationIssueSearch, map[string]any{"query": "plugin", "project": "DEX", "limit": 5})
	if out.Count != 1 || out.Items[0].Key != "DEX-7" {
		t.Fatalf("issue search output = %#v", out)
	}
	if client.issueOptions.Query != "plugin" || client.issueOptions.Project != "DEX" || client.issueOptions.Limit != 5 {
		t.Fatalf("issue options = %#v", client.issueOptions)
	}

	ds := plugintest.DatasourceSearchOK[IssueDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceIssues, "query": "plugin"})
	if ds.Count != 1 || ds.Records[0].ID != "DEX-7" || ds.Records[0].WebURL != "" {
		t.Fatalf("datasource output = %#v", ds)
	}
}

func TestDatasourceSearchAcceptsEndpointRef(t *testing.T) {
	client := &fakeClient{}
	var capturedEndpointRef string
	plugin := NewPluginWithService(Service{ClientFactory: func(_ pluginbinding.Context, endpointRef string) (Client, error) {
		capturedEndpointRef = endpointRef
		return client, nil
	}})

	_ = plugintest.DatasourceSearchOK[IssueDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceIssues, "entity": EntityIssue, "query": "bug", "endpoint_ref": "jira-prod"})
	if capturedEndpointRef != "jira-prod" {
		t.Fatalf("endpoint_ref = %q", capturedEndpointRef)
	}
}

func TestDatasourceGetFetchesLiveIssue(t *testing.T) {
	client := &fakeClient{issue: Issue{
		ID:  "10001",
		Key: "DEX-7",
		Fields: IssueFields{
			Summary: "Ship Jira plugin",
			Status:  NamedValue{Name: "In Progress"},
			Project: Project{Key: "DEX", Name: "Dex"},
		},
	}}
	plugin := testPlugin(client)

	out := plugintest.DatasourceGetOK[pluginbinding.DatasourceGetResult[IssueRecord]](t, plugin, map[string]any{"datasource": DatasourceIssues, "entity": EntityIssue, "id": "DEX-7"})
	if out.Record.ID != "DEX-7" || out.Record.Summary != "Ship Jira plugin" || out.Record.ProjectKey != "DEX" {
		t.Fatalf("get output = %#v", out)
	}
	if client.issueKey != "DEX-7" {
		t.Fatalf("issue key = %q", client.issueKey)
	}
}

func TestDatasourceGetFetchesLiveUser(t *testing.T) {
	client := &fakeClient{user: User{AccountID: "acct-1", DisplayName: "Ada Lovelace", Active: true}}
	plugin := testPlugin(client)

	out := plugintest.DatasourceGetOK[pluginbinding.DatasourceGetResult[UserRecord]](t, plugin, map[string]any{"datasource": DatasourceUsers, "entity": EntityUser, "id": "acct-1"})
	if out.Record.ID != "acct-1" || out.Record.DisplayName != "Ada Lovelace" || !out.Record.Active {
		t.Fatalf("get output = %#v", out)
	}
	if client.userOptions.Query != "acct-1" {
		t.Fatalf("user id = %q", client.userOptions.Query)
	}
}

func TestIssueJQLCombinesFiltersWithAnd(t *testing.T) {
	got := issueJQL(IssueSearchOptions{Project: "DEX", Status: "In Progress", Query: "plugin", OrderBy: "updated DESC"})
	want := `project = "DEX" and status = "In Progress" and text ~ "plugin" order by updated DESC`
	if got != want {
		t.Fatalf("jql = %q want %q", got, want)
	}
}

func TestIndexBuildCanTargetUsers(t *testing.T) {
	client := &fakeClient{users: []User{{AccountID: "acct-1", DisplayName: "Ada"}}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[struct {
		Indexes []struct {
			Index   string            `json:"index"`
			Records []json.RawMessage `json:"records"`
		} `json:"indexes"`
	}](t, plugin, OperationIndexBuild, map[string]any{"entity": EntityUser, "user_query": "ada"})
	if len(out.Indexes) != 1 || out.Indexes[0].Index != DatasourceUsers || len(out.Indexes[0].Records) != 1 {
		t.Fatalf("index output = %#v", out.Indexes)
	}
	if client.issueOptions.All {
		t.Fatalf("targeted user build fetched issues: %#v", client.issueOptions)
	}
	if !client.userOptions.All || client.userOptions.Query != "ada" {
		t.Fatalf("user options = %#v", client.userOptions)
	}
}

func TestLookupPrefersExactIssueKey(t *testing.T) {
	client := &fakeClient{
		issue: Issue{ID: "10001", Key: "DEX-7", Fields: IssueFields{Summary: "Ship Jira plugin"}},
		users: []User{{AccountID: "acct-1", DisplayName: "Ada Lovelace"}},
	}
	plugin := testPlugin(client)

	out := plugintest.DatasourceLookupOK[LookupResult](t, plugin, map[string]any{"text": "See https://example.atlassian.net/browse/DEX-7 with Ada", "limit": 10})
	if out.Count < 2 || out.Matches[0].Entity != EntityIssue || out.Matches[0].ID != "DEX-7" {
		t.Fatalf("lookup output = %#v", out)
	}
	if client.issueKey != "DEX-7" {
		t.Fatalf("issue key = %q", client.issueKey)
	}
}

func TestIssueCreateConvertsMarkdownDescriptionToADF(t *testing.T) {
	client := &fakeClient{createResult: IssueMutationResult{OK: true, ID: "10001", Key: "DEX-9", Self: "https://api.example/issue/10001"}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[IssueMutationResult](t, plugin, OperationIssueCreate, map[string]any{
		"project_key":          "DEX",
		"issue_type":           "Task",
		"summary":              "Create from markdown",
		"description_markdown": "# Heading\n\nSome **bold** text.\n\n- one\n- two",
		"labels":               []string{"ai", "jira"},
		"assignee_account_id":  "acct-1",
		"reporter_account_id":  "acct-2",
		"priority":             "High",
		"parent_key":           "DEX-1",
		"fields":               map[string]any{"customfield_10042": "custom"},
	})
	if !out.OK || out.Key != "DEX-9" {
		t.Fatalf("create output = %#v", out)
	}
	body := requestJSON(client.createRequest)
	for _, want := range []string{
		`"project":{"key":"DEX"}`,
		`"issuetype":{"name":"Task"}`,
		`"summary":"Create from markdown"`,
		`"customfield_10042":"custom"`,
		`"description":{"content"`,
		`"type":"heading"`,
		`"type":"strong"`,
		`"type":"bulletList"`,
		`"assignee":{"accountId":"acct-1"}`,
		`"reporter":{"accountId":"acct-2"}`,
		`"priority":{"name":"High"}`,
		`"parent":{"key":"DEX-1"}`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("create request = %s, missing %s", body, want)
		}
	}
}

func TestIssueEditSendsOnlyProvidedFieldsAndRawUpdate(t *testing.T) {
	client := &fakeClient{editResult: IssueMutationResult{OK: true, Key: "DEX-9"}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[IssueMutationResult](t, plugin, OperationIssueEdit, map[string]any{
		"key":                  "DEX-9",
		"description_markdown": "See `worker.go`.",
		"labels":               []string{"edited"},
		"fields":               map[string]any{"customfield_10042": "new"},
		"update":               map[string]any{"comment": []any{map[string]any{"add": map[string]any{"body": "raw"}}}},
	})
	if !out.OK || out.Key != "DEX-9" {
		t.Fatalf("edit output = %#v", out)
	}
	if client.editKey != "DEX-9" {
		t.Fatalf("edit key = %q", client.editKey)
	}
	body := requestJSON(client.editRequest)
	for _, want := range []string{
		`"customfield_10042":"new"`,
		`"description":{"content"`,
		`"type":"code"`,
		`"labels":["edited"]`,
		`"update":{"comment"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("edit request = %s, missing %s", body, want)
		}
	}
	if strings.Contains(body, `"summary"`) {
		t.Fatalf("edit request should not include omitted summary: %s", body)
	}
}

func TestIssueEditRequiresAChange(t *testing.T) {
	plugin := testPlugin(&fakeClient{})
	err := plugintest.RunError(t, plugin, OperationIssueEdit, map[string]any{"key": "DEX-9"})
	if err == nil || err.Code != "bad_input" {
		t.Fatalf("error = %#v", err)
	}
}

func TestCommentsConvertMarkdownAndDeleteByID(t *testing.T) {
	client := &fakeClient{
		commentResult:       CommentResult{OK: true, IssueKey: "DEX-9", Comment: Comment{ID: "10042"}},
		commentDeleteResult: CommentMutationResult{OK: true, IssueKey: "DEX-9", CommentID: "10042"},
	}
	plugin := testPlugin(client)

	add := plugintest.RunOK[CommentResult](t, plugin, OperationCommentAdd, map[string]any{
		"key":           "DEX-9",
		"body_markdown": "## Added\n\n- **comment**",
	})
	if !add.OK || add.Comment.ID != "10042" || client.commentKey != "DEX-9" {
		t.Fatalf("add output = %#v key=%q", add, client.commentKey)
	}
	addBody := requestJSON(client.commentRequest)
	for _, want := range []string{`"body":{"content"`, `"type":"heading"`, `"type":"strong"`, `"type":"bulletList"`} {
		if !strings.Contains(addBody, want) {
			t.Fatalf("comment add request = %s, missing %s", addBody, want)
		}
	}

	edit := plugintest.RunOK[CommentResult](t, plugin, OperationCommentEdit, map[string]any{
		"key":           "DEX-9",
		"comment_id":    "10042",
		"body_markdown": "Edited with `code`.",
	})
	if !edit.OK || client.commentID != "10042" {
		t.Fatalf("edit output = %#v comment_id=%q", edit, client.commentID)
	}
	editBody := requestJSON(client.commentRequest)
	if !strings.Contains(editBody, `"type":"code"`) {
		t.Fatalf("comment edit request = %s", editBody)
	}

	deleted := plugintest.RunOK[CommentMutationResult](t, plugin, OperationCommentDelete, map[string]any{
		"key":        "DEX-9",
		"comment_id": "10042",
	})
	if !deleted.OK || deleted.CommentID != "10042" || client.deletedCommentID != "10042" {
		t.Fatalf("delete output = %#v deleted=%q", deleted, client.deletedCommentID)
	}
}

func TestIssueDeletePassesDeleteSubtasks(t *testing.T) {
	client := &fakeClient{deleteResult: IssueMutationResult{OK: true, Key: "DEX-9"}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[IssueMutationResult](t, plugin, OperationIssueDelete, map[string]any{"key": "DEX-9", "delete_subtasks": true})
	if !out.OK || out.Key != "DEX-9" {
		t.Fatalf("delete output = %#v", out)
	}
	if client.deleteKey != "DEX-9" || !client.deleteSubtasks {
		t.Fatalf("delete key/subtasks = %q/%v", client.deleteKey, client.deleteSubtasks)
	}
}

func TestAttachmentOperations(t *testing.T) {
	client := &fakeClient{
		issue:                  Issue{Key: "DEX-9", Fields: IssueFields{Attachments: []Attachment{{ID: "A1", Filename: "chart.png", MimeType: "image/png", Content: "https://api/attachment/content/A1"}}}},
		attachmentUploadResult: AttachmentUploadResult{OK: true, IssueKey: "DEX-9", Attachments: []Attachment{{ID: "A1", Filename: "chart.png"}}},
		attachmentGetResult:    AttachmentGetResult{ID: "A1", Filename: "chart.png", MimeType: "image/png", ContentBytes: []byte("png bytes"), Size: 9},
		attachmentDeleteResult: AttachmentDeleteResult{OK: true, AttachmentID: "A1"},
	}
	plugin := testPlugin(client)

	add := plugintest.RunOK[AttachmentUploadResult](t, plugin, OperationAttachmentAdd, map[string]any{"key": "DEX-9", "filename": "chart.png", "content_bytes": []byte("png bytes")})
	if !add.OK || client.attachmentKey != "DEX-9" || client.attachmentRequest.Filename != "chart.png" {
		t.Fatalf("add = %#v key=%q request=%#v", add, client.attachmentKey, client.attachmentRequest)
	}
	list := plugintest.RunOK[AttachmentListResult](t, plugin, OperationAttachmentList, map[string]any{"key": "DEX-9"})
	if list.Count != 1 || list.Attachments[0].ID != "A1" {
		t.Fatalf("list = %#v", list)
	}
	get := plugintest.RunOK[AttachmentGetResult](t, plugin, OperationAttachmentGet, map[string]any{"attachment_id": "A1", "filename": "chart.png"})
	if get.ID != "A1" || string(get.ContentBytes) != "png bytes" {
		t.Fatalf("get = %#v", get)
	}
	deleted := plugintest.RunOK[AttachmentDeleteResult](t, plugin, OperationAttachmentDelete, map[string]any{"attachment_id": "A1"})
	if !deleted.OK || client.deletedAttachmentID != "A1" {
		t.Fatalf("delete = %#v deleted=%q", deleted, client.deletedAttachmentID)
	}
}

func TestTransitionOperations(t *testing.T) {
	client := &fakeClient{
		transitions: []IssueTransitionListResult{{
			IssueKey:      "DEX-9",
			CurrentStatus: NamedValue{ID: "1", Name: "Backlog"},
			Transitions: []IssueTransition{
				{ID: "11", Name: "Start progress", To: NamedValue{ID: "3", Name: "In Progress"}},
				{ID: "21", Name: "Done", To: NamedValue{ID: "100", Name: "Done"}},
			},
		}},
		transitionResults: []IssueMutationResult{{OK: true, Key: "DEX-9", Issue: &Issue{Key: "DEX-9", Fields: IssueFields{Status: NamedValue{ID: "100", Name: "Done"}}}}},
	}
	plugin := testPlugin(client)

	list := plugintest.RunOK[IssueTransitionListResult](t, plugin, OperationTransitionList, map[string]any{"key": "DEX-9"})
	if list.CurrentStatus.Name != "Backlog" || len(list.Transitions) != 2 {
		t.Fatalf("list = %#v", list)
	}
	run := plugintest.RunOK[IssueTransitionRunResult](t, plugin, OperationTransitionRun, map[string]any{"key": "DEX-9", "target_status": "Done"})
	if !run.OK || run.Steps != 1 || client.transitionRequests[0].TransitionID != "21" {
		t.Fatalf("run = %#v requests=%#v", run, client.transitionRequests)
	}
}

func TestTransitionRunAutoTransition(t *testing.T) {
	client := &fakeClient{
		transitions: []IssueTransitionListResult{
			{
				IssueKey:      "DEX-9",
				CurrentStatus: NamedValue{ID: "1", Name: "Backlog"},
				Transitions:   []IssueTransition{{ID: "11", Name: "Start progress", To: NamedValue{ID: "3", Name: "In Progress"}}},
			},
			{
				IssueKey:      "DEX-9",
				CurrentStatus: NamedValue{ID: "3", Name: "In Progress"},
				Transitions:   []IssueTransition{{ID: "21", Name: "Done", To: NamedValue{ID: "100", Name: "Done"}}},
			},
			{
				IssueKey:      "DEX-9",
				CurrentStatus: NamedValue{ID: "100", Name: "Done"},
				Transitions:   nil,
			},
		},
		transitionResults: []IssueMutationResult{
			{OK: true, Key: "DEX-9", Issue: &Issue{Key: "DEX-9", Fields: IssueFields{Status: NamedValue{ID: "3", Name: "In Progress"}}}},
			{OK: true, Key: "DEX-9", Issue: &Issue{Key: "DEX-9", Fields: IssueFields{Status: NamedValue{ID: "100", Name: "Done"}}}},
		},
	}
	plugin := testPlugin(client)

	run := plugintest.RunOK[IssueTransitionRunResult](t, plugin, OperationTransitionRun, map[string]any{"key": "DEX-9", "target_status": "Done", "auto_transition": true})
	if !run.OK || run.Steps != 2 {
		t.Fatalf("run = %#v", run)
	}
	if got := []string{client.transitionRequests[0].TransitionID, client.transitionRequests[1].TransitionID}; got[0] != "11" || got[1] != "21" {
		t.Fatalf("transition requests = %#v", got)
	}
}

func TestUploadMarkdownBlobImagesRewritesBlobReferences(t *testing.T) {
	host := &jiraTestHost{blobs: map[string]pluginbinding.BlobReadResponse{
		"blob://diagram": {
			Blob:    pluginbinding.BlobRef{Ref: "blob://diagram", Filename: "diagram.png", MediaType: "image/png"},
			Content: []byte("png-bytes"),
		},
	}}
	ctx := pluginbinding.Context{Host: host}
	client := &fakeClient{
		attachmentUploadResult: AttachmentUploadResult{OK: true, IssueKey: "DEX-9", Attachments: []Attachment{{ID: "A1", Filename: "diagram.png", Content: "https://api.atlassian.com/ex/jira/cloud-1/rest/api/3/attachment/content/A1"}}},
	}

	markdown := `See ![architecture](blob://diagram) and ![remote](https://example.com/img.png) and titled ![alt](blob://diagram "title text")`
	rewritten, err := uploadMarkdownBlobImages(ctx, client, "DEX-9", markdown)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(rewritten, "blob://diagram") {
		t.Fatalf("rewritten still references blob: %s", rewritten)
	}
	if !strings.Contains(rewritten, "rest/api/3/attachment/content/A1") {
		t.Fatalf("rewritten missing uploaded URL: %s", rewritten)
	}
	if !strings.Contains(rewritten, "https://example.com/img.png") {
		t.Fatalf("remote URL was disturbed: %s", rewritten)
	}
	if client.attachmentRequest.Filename != "diagram.png" {
		t.Fatalf("upload request filename = %q", client.attachmentRequest.Filename)
	}
	if string(client.attachmentRequest.Data) != "png-bytes" {
		t.Fatalf("upload request data = %q", string(client.attachmentRequest.Data))
	}
}

func TestUploadMarkdownBlobImagesNoOpsWhenNoImages(t *testing.T) {
	client := &fakeClient{}
	out, err := uploadMarkdownBlobImages(pluginbinding.Context{Host: &jiraTestHost{}}, client, "DEX-9", "just text, no images")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if out != "just text, no images" {
		t.Fatalf("out = %q", out)
	}
}

func testPlugin(client Client) *pluginbinding.Plugin {
	return NewPluginWithService(Service{ClientFactory: func(pluginbinding.Context, string) (Client, error) { return client, nil }})
}

type jiraTestHost struct {
	blobs map[string]pluginbinding.BlobReadResponse
}

func (h *jiraTestHost) Secret(string) (pluginbinding.SecretMaterial, error) {
	return pluginbinding.SecretMaterial{}, nil
}

func (h *jiraTestHost) Lookup(pluginbinding.DatasourceLookupInput) (pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]], error) {
	return pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]{}, nil
}

func (h *jiraTestHost) Search(pluginbinding.DatasourceSearchInput) (pluginbinding.DatasourceSearchResult[any], error) {
	return pluginbinding.DatasourceSearchResult[any]{}, nil
}

func (h *jiraTestHost) Get(pluginbinding.DatasourceGetInput) (pluginbinding.DatasourceGetResult[any], error) {
	return pluginbinding.DatasourceGetResult[any]{}, nil
}

func (h *jiraTestHost) ResolveEndpoint(string) (core.EndpointRef, error) {
	return core.EndpointRef{}, nil
}

func (h *jiraTestHost) HTTP(pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	return pluginbinding.HTTPResponse{}, nil
}

func (h *jiraTestHost) BlobRead(input pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error) {
	return h.blobs[strings.TrimSpace(input.Ref)], nil
}

func (h *jiraTestHost) BlobWrite(input pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{Ref: firstNonEmpty(input.Ref, "blob://written"), Filename: input.Filename, MediaType: input.MediaType, Size: int64(len(input.Content))}, nil
}

func (h *jiraTestHost) BlobInfo(pluginbinding.BlobInfoRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h *jiraTestHost) EnvLookup(string) (pluginbinding.EnvLookupResponse, error) {
	return pluginbinding.EnvLookupResponse{}, nil
}

func (h *jiraTestHost) CapabilityCall(pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	return pluginbinding.ProviderCallResponse{}, nil
}

var _ pluginbinding.HostClient = (*jiraTestHost)(nil)

type fakeClient struct {
	user                   User
	issues                 []Issue
	issue                  Issue
	users                  []User
	createResult           IssueMutationResult
	editResult             IssueMutationResult
	deleteResult           IssueMutationResult
	commentResult          CommentResult
	commentDeleteResult    CommentMutationResult
	attachmentUploadResult AttachmentUploadResult
	attachmentGetResult    AttachmentGetResult
	attachmentDeleteResult AttachmentDeleteResult
	metaResult             IssueMetaResult
	transitions            []IssueTransitionListResult
	transitionResults      []IssueMutationResult
	issueOptions           IssueSearchOptions
	userOptions            UserSearchOptions
	issueKey               string
	editKey                string
	deleteKey              string
	deleteSubtasks         bool
	commentKey             string
	commentID              string
	deletedCommentID       string
	attachmentKey          string
	attachmentRequest      AttachmentUploadRequest
	deletedAttachmentID    string
	createMeta             IssueCreateMetaOptions
	editMetaKey            string
	createRequest          IssueCreateRequest
	editRequest            IssueEditRequest
	transitionRequests     []IssueTransitionRequest
	commentRequest         CommentRequest
}

func (c *fakeClient) CurrentUser(context.Context) (User, error) {
	return c.user, nil
}

func (c *fakeClient) SearchIssues(_ context.Context, options IssueSearchOptions) ([]Issue, error) {
	c.issueOptions = options
	return c.issues, nil
}

func (c *fakeClient) GetIssue(_ context.Context, key string) (Issue, error) {
	c.issueKey = key
	return c.issue, nil
}

func (c *fakeClient) CreateIssue(_ context.Context, request IssueCreateRequest) (IssueMutationResult, error) {
	c.createRequest = request
	return c.createResult, nil
}

func (c *fakeClient) EditIssue(_ context.Context, key string, request IssueEditRequest) (IssueMutationResult, error) {
	c.editKey = key
	c.editRequest = request
	return c.editResult, nil
}

func (c *fakeClient) DeleteIssue(_ context.Context, key string, deleteSubtasks bool) (IssueMutationResult, error) {
	c.deleteKey = key
	c.deleteSubtasks = deleteSubtasks
	return c.deleteResult, nil
}

func (c *fakeClient) ListTransitions(_ context.Context, key string) (IssueTransitionListResult, error) {
	c.issueKey = key
	if len(c.transitions) == 0 {
		return IssueTransitionListResult{IssueKey: key}, nil
	}
	out := c.transitions[0]
	if len(c.transitions) > 1 {
		c.transitions = c.transitions[1:]
	}
	return out, nil
}

func (c *fakeClient) TransitionIssue(_ context.Context, key string, request IssueTransitionRequest) (IssueMutationResult, error) {
	c.editKey = key
	c.transitionRequests = append(c.transitionRequests, request)
	if len(c.transitionResults) == 0 {
		return IssueMutationResult{OK: true, Key: key}, nil
	}
	out := c.transitionResults[0]
	if len(c.transitionResults) > 1 {
		c.transitionResults = c.transitionResults[1:]
	}
	return out, nil
}

func (c *fakeClient) AddComment(_ context.Context, key string, request CommentRequest) (CommentResult, error) {
	c.commentKey = key
	c.commentRequest = request
	return c.commentResult, nil
}

func (c *fakeClient) EditComment(_ context.Context, key, commentID string, request CommentRequest) (CommentResult, error) {
	c.commentKey = key
	c.commentID = commentID
	c.commentRequest = request
	return c.commentResult, nil
}

func (c *fakeClient) DeleteComment(_ context.Context, key, commentID string) (CommentMutationResult, error) {
	c.commentKey = key
	c.deletedCommentID = commentID
	return c.commentDeleteResult, nil
}

func (c *fakeClient) UploadIssueAttachment(_ context.Context, key string, request AttachmentUploadRequest) (AttachmentUploadResult, error) {
	c.attachmentKey = key
	c.attachmentRequest = request
	return c.attachmentUploadResult, nil
}

func (c *fakeClient) GetAttachment(_ context.Context, attachment Attachment) (AttachmentGetResult, error) {
	c.deletedAttachmentID = attachment.ID
	return c.attachmentGetResult, nil
}

func (c *fakeClient) DeleteAttachment(_ context.Context, id string) (AttachmentDeleteResult, error) {
	c.deletedAttachmentID = id
	return c.attachmentDeleteResult, nil
}

func (c *fakeClient) CreateMeta(_ context.Context, options IssueCreateMetaOptions) (IssueMetaResult, error) {
	c.createMeta = options
	return c.metaResult, nil
}

func (c *fakeClient) EditMeta(_ context.Context, key string) (IssueMetaResult, error) {
	c.editMetaKey = key
	return c.metaResult, nil
}

func (c *fakeClient) SearchUsers(_ context.Context, options UserSearchOptions) ([]User, error) {
	c.userOptions = options
	return c.users, nil
}

func (c *fakeClient) GetUser(_ context.Context, accountID string) (User, error) {
	c.userOptions.Query = accountID
	if c.user.AccountID != "" {
		return c.user, nil
	}
	if len(c.users) > 0 {
		return c.users[0], nil
	}
	return User{}, nil
}
