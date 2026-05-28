package confluence

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-dex/internal/atlassian"
)

func TestServiceBuildsClientFromResolvedSecrets(t *testing.T) {
	client := &fakeClient{user: User{AccountID: "acct-1", DisplayName: "Ada"}}
	var captured atlassian.Credentials
	plugin := NewPluginWithService(Service{
		SecretGetter: testSecretGetter,
		ClientFactory: func(credentials atlassian.Credentials) (Client, error) {
			captured = credentials
			return client, nil
		},
	})

	out := plugintest.RunOK[AuthTestResult](t, plugin, OperationAuthTest, map[string]any{})
	if out.Status != "ok" || out.User.AccountID != "acct-1" {
		t.Fatalf("auth output = %#v", out)
	}
	if captured.BaseURL != "https://api.atlassian.com/ex/confluence/cloud-123" || captured.CloudID.Value != "cloud-123" || captured.Token.Value != "token" {
		t.Fatalf("credentials = %#v", captured)
	}
}

func TestPageSearchAndDatasourceNormalizeRecords(t *testing.T) {
	client := &fakeClient{pages: []Page{{
		ID:       "123",
		Title:    "Runbook",
		SpaceID:  "999",
		SpaceKey: "OPS",
		Status:   "current",
		AuthorID: "acct-1",
		Version:  PageVersion{Number: 4, CreatedAt: "2026-05-28T10:00:00.000Z"},
		Links:    PageLinks{Self: "https://example.atlassian.net/wiki/rest/api/content/123", WebUI: "/wiki/spaces/OPS/pages/123/Runbook"},
	}}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[PageSearchResult](t, plugin, OperationPageSearch, map[string]any{"query": "runbook", "limit": 5})
	if out.Count != 1 || out.Items[0].ID != "123" {
		t.Fatalf("page search output = %#v", out)
	}
	if client.pageOptions.Query != "runbook" || client.pageOptions.Limit != 5 {
		t.Fatalf("page options = %#v", client.pageOptions)
	}

	ds := plugintest.DatasourceSearchOK[PageDatasourceResult](t, plugin, map[string]any{"datasource": DatasourcePages, "query": "runbook"})
	if ds.Count != 1 || ds.Records[0].PageID != "123" || ds.Records[0].WebURL != "https://example.atlassian.net/wiki/spaces/OPS/pages/123/Runbook" {
		t.Fatalf("datasource output = %#v", ds)
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
	if client.pageOptions.All {
		t.Fatalf("targeted user build fetched pages: %#v", client.pageOptions)
	}
	if !client.userOptions.All || client.userOptions.Query != "ada" {
		t.Fatalf("user options = %#v", client.userOptions)
	}
}

func TestLookupPrefersExactPageURL(t *testing.T) {
	client := &fakeClient{
		page:  Page{ID: "123", Title: "Runbook", Links: PageLinks{WebUI: "/wiki/spaces/OPS/pages/123/Runbook"}},
		users: []User{{AccountID: "acct-1", DisplayName: "Ada Lovelace"}},
	}
	plugin := testPlugin(client)

	out := plugintest.DatasourceLookupOK[LookupResult](t, plugin, map[string]any{"text": "Read https://example.atlassian.net/wiki/spaces/OPS/pages/123/Runbook with Ada", "limit": 10})
	if out.Count < 2 || out.Matches[0].Entity != EntityPage || out.Matches[0].ID != "123" {
		t.Fatalf("lookup output = %#v", out)
	}
	if client.pageID != "123" {
		t.Fatalf("page id = %q", client.pageID)
	}
}

func TestAttachmentOperations(t *testing.T) {
	client := &fakeClient{
		attachmentUploadResult: AttachmentUploadResult{OK: true, PageID: "123", Attachments: []Attachment{{ID: "A1", Title: "chart.png"}}},
		attachmentListResult:   AttachmentListResult{PageID: "123", Count: 1, Attachments: []Attachment{{ID: "A1", Title: "chart.png"}}},
		attachmentGetResult:    AttachmentGetResult{ID: "A1", Filename: "chart.png", MimeType: "image/png", ContentBytes: []byte("png bytes"), Size: 9},
		attachmentDeleteResult: AttachmentDeleteResult{OK: true, AttachmentID: "A1"},
	}
	plugin := testPlugin(client)

	add := plugintest.RunOK[AttachmentUploadResult](t, plugin, OperationAttachmentAdd, map[string]any{"page_id": "123", "filename": "chart.png", "content_bytes": []byte("png bytes")})
	if !add.OK || client.attachmentPageID != "123" || client.attachmentRequest.Filename != "chart.png" {
		t.Fatalf("add = %#v page=%q request=%#v", add, client.attachmentPageID, client.attachmentRequest)
	}
	list := plugintest.RunOK[AttachmentListResult](t, plugin, OperationAttachmentList, map[string]any{"page_id": "123"})
	if list.Count != 1 || list.Attachments[0].ID != "A1" {
		t.Fatalf("list = %#v", list)
	}
	get := plugintest.RunOK[AttachmentGetResult](t, plugin, OperationAttachmentGet, map[string]any{"attachment_id": "A1", "page_id": "123", "download": true})
	if get.ID != "A1" || string(get.ContentBytes) != "png bytes" {
		t.Fatalf("get = %#v", get)
	}
	if !client.attachmentDownload {
		t.Fatalf("download flag was not forwarded")
	}
	if client.attachmentGetPageID != "123" {
		t.Fatalf("attachment get page id = %q", client.attachmentGetPageID)
	}
	deleted := plugintest.RunOK[AttachmentDeleteResult](t, plugin, OperationAttachmentDelete, map[string]any{"attachment_id": "A1"})
	if !deleted.OK || client.deletedAttachmentID != "A1" {
		t.Fatalf("delete = %#v deleted=%q", deleted, client.deletedAttachmentID)
	}
}

func TestPageMutationOperations(t *testing.T) {
	client := &fakeClient{
		pageCreateResult: PageMutationResult{OK: true, ID: "123", Page: &Page{ID: "123", Title: "Dex test"}},
		pageDeleteResult: PageMutationResult{OK: true, ID: "123"},
	}
	plugin := testPlugin(client)

	created := plugintest.RunOK[PageMutationResult](t, plugin, OperationPageCreate, map[string]any{"space_key": "DEV", "title": "Dex test", "body_storage": "<p>hello</p>"})
	if !created.OK || created.ID != "123" {
		t.Fatalf("create = %#v", created)
	}
	if client.pageCreateRequest.SpaceKey != "DEV" || client.pageCreateRequest.Title != "Dex test" || client.pageCreateRequest.BodyStorage != "<p>hello</p>" {
		t.Fatalf("create request = %#v", client.pageCreateRequest)
	}
	deleted := plugintest.RunOK[PageMutationResult](t, plugin, OperationPageDelete, map[string]any{"page_id": "123"})
	if !deleted.OK || client.deletedPageID != "123" {
		t.Fatalf("delete = %#v deleted=%q", deleted, client.deletedPageID)
	}
}

func testPlugin(client Client) *pluginbinding.Plugin {
	return NewPluginWithService(Service{SecretGetter: testSecretGetter, ClientFactory: func(atlassian.Credentials) (Client, error) { return client, nil }})
}

func testSecretGetter(_ pluginbinding.Context, purpose string) (pluginbinding.SecretMaterial, error) {
	switch purpose {
	case AuthPurposeAPIToken:
		return pluginbinding.SecretMaterial{Value: "token"}, nil
	case AuthPurposeCloudID:
		return pluginbinding.SecretMaterial{Value: "cloud-123"}, nil
	default:
		return pluginbinding.SecretMaterial{}, errors.New("unexpected purpose")
	}
}

type fakeClient struct {
	user                   User
	pages                  []Page
	page                   Page
	users                  []User
	pageOptions            PageSearchOptions
	userOptions            UserSearchOptions
	pageID                 string
	pageCreateResult       PageMutationResult
	pageDeleteResult       PageMutationResult
	pageCreateRequest      PageCreateRequest
	deletedPageID          string
	attachmentUploadResult AttachmentUploadResult
	attachmentListResult   AttachmentListResult
	attachmentGetResult    AttachmentGetResult
	attachmentDeleteResult AttachmentDeleteResult
	attachmentPageID       string
	attachmentRequest      AttachmentUploadRequest
	attachmentDownload     bool
	attachmentGetPageID    string
	deletedAttachmentID    string
}

func (c *fakeClient) CurrentUser(context.Context) (User, error) {
	return c.user, nil
}

func (c *fakeClient) SearchPages(_ context.Context, options PageSearchOptions) ([]Page, error) {
	c.pageOptions = options
	return c.pages, nil
}

func (c *fakeClient) GetPage(_ context.Context, id string) (Page, error) {
	c.pageID = id
	return c.page, nil
}

func (c *fakeClient) CreatePage(_ context.Context, request PageCreateRequest) (PageMutationResult, error) {
	c.pageCreateRequest = request
	return c.pageCreateResult, nil
}

func (c *fakeClient) DeletePage(_ context.Context, id string) (PageMutationResult, error) {
	c.deletedPageID = id
	return c.pageDeleteResult, nil
}

func (c *fakeClient) UploadPageAttachment(_ context.Context, pageID string, request AttachmentUploadRequest) (AttachmentUploadResult, error) {
	c.attachmentPageID = pageID
	c.attachmentRequest = request
	return c.attachmentUploadResult, nil
}

func (c *fakeClient) ListPageAttachments(_ context.Context, pageID string) (AttachmentListResult, error) {
	c.attachmentPageID = pageID
	return c.attachmentListResult, nil
}

func (c *fakeClient) GetAttachment(_ context.Context, id, pageID string, download bool) (AttachmentGetResult, error) {
	c.deletedAttachmentID = id
	c.attachmentGetPageID = pageID
	c.attachmentDownload = download
	return c.attachmentGetResult, nil
}

func (c *fakeClient) DeleteAttachment(_ context.Context, id string) (AttachmentDeleteResult, error) {
	c.deletedAttachmentID = id
	return c.attachmentDeleteResult, nil
}

func (c *fakeClient) SearchUsers(_ context.Context, options UserSearchOptions) ([]User, error) {
	c.userOptions = options
	return c.users, nil
}
