// Package confluence implements a dex plugin against the Atlassian Cloud
// Confluence REST API.
//
// All HTTP paths in this file target the v1 API surface
// (`/wiki/rest/api/...`). Atlassian has announced sunset plans for several
// v1 content endpoints in favor of v2 (`/wiki/api/v2/...`). Migrating is
// tracked as a follow-up; the v1 endpoints used here are still supported on
// Confluence Cloud at the time of writing.
package confluence

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/fluxplane/fluxplane-dex/internal/atlassian"
)

type Client interface {
	CurrentUser(context.Context) (User, error)
	SearchPages(context.Context, PageSearchOptions) ([]Page, error)
	GetPage(context.Context, string) (Page, error)
	CreatePage(context.Context, PageCreateRequest) (PageMutationResult, error)
	DeletePage(context.Context, string) (PageMutationResult, error)
	UploadPageAttachment(context.Context, string, AttachmentUploadRequest) (AttachmentUploadResult, error)
	ListPageAttachments(context.Context, string) (AttachmentListResult, error)
	GetAttachment(context.Context, string, string, bool) (AttachmentGetResult, error)
	DeleteAttachment(context.Context, string) (AttachmentDeleteResult, error)
	SearchUsers(context.Context, UserSearchOptions) ([]User, error)
}

type ClientFactory func(atlassian.Credentials) (Client, error)

func NewLiveClient(credentials atlassian.Credentials) (Client, error) {
	return liveClient{client: atlassian.NewClient(credentials)}, nil
}

type liveClient struct {
	client atlassian.Client
}

func (c liveClient) CurrentUser(ctx context.Context) (User, error) {
	var out User
	err := c.client.GetJSON(ctx, "/wiki/rest/api/user/current", nil, &out)
	return out, err
}

func (c liveClient) SearchPages(ctx context.Context, input PageSearchOptions) ([]Page, error) {
	if shouldUsePageList(input) {
		return c.listPages(ctx, input)
	}
	return c.searchPages(ctx, input)
}

func (c liveClient) listPages(ctx context.Context, input PageSearchOptions) ([]Page, error) {
	limit := clamp(input.Limit, 20, 100)
	query := url.Values{}
	query.Set("limit", strconv.Itoa(limit))
	query.Set("status", defaultString(input.Status, "current"))
	query.Set("type", "page")
	if input.Title != "" {
		query.Set("title", input.Title)
	}
	if input.SpaceKey != "" {
		query.Set("spaceKey", input.SpaceKey)
	}
	if input.Cursor != "" {
		query.Set("start", input.Cursor)
	}
	var out []Page
	for {
		var page pageListResponse
		if err := c.client.GetJSON(ctx, "/wiki/rest/api/content", query, &page); err != nil {
			return nil, err
		}
		out = append(out, page.Results...)
		next := startFromNext(page.Links.Next)
		if !input.All || next == "" {
			return out, nil
		}
		query.Set("start", next)
	}
}

func (c liveClient) searchPages(ctx context.Context, input PageSearchOptions) ([]Page, error) {
	limit := clamp(input.Limit, 20, 100)
	query := url.Values{}
	query.Set("cql", pageCQL(input))
	query.Set("limit", strconv.Itoa(limit))
	if input.Cursor != "" {
		query.Set("cursor", input.Cursor)
	}
	var out []Page
	for {
		var page searchResponse
		if err := c.client.GetJSON(ctx, "/wiki/rest/api/search", query, &page); err != nil {
			return nil, err
		}
		out = append(out, pagesFromSearch(page.Results)...)
		next := cursorFromNext(page.Links.Next)
		if !input.All || next == "" {
			return out, nil
		}
		query.Set("cursor", next)
	}
}

func (c liveClient) GetPage(ctx context.Context, id string) (Page, error) {
	query := url.Values{}
	query.Set("expand", "body.storage,version,space,ancestors")
	var out Page
	err := c.client.GetJSON(ctx, "/wiki/rest/api/content/"+url.PathEscape(strings.TrimSpace(id)), query, &out)
	return out, err
}

func (c liveClient) CreatePage(ctx context.Context, request PageCreateRequest) (PageMutationResult, error) {
	payload := map[string]any{
		"type":  "page",
		"title": strings.TrimSpace(request.Title),
		"space": map[string]string{"key": strings.TrimSpace(request.SpaceKey)},
		"body": map[string]any{
			"storage": map[string]string{
				"value":          request.BodyStorage,
				"representation": "storage",
			},
		},
	}
	if parentID := strings.TrimSpace(request.ParentID); parentID != "" {
		payload["ancestors"] = []map[string]string{{"id": parentID}}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return PageMutationResult{}, err
	}
	var created Page
	if err := c.client.DoJSON(ctx, http.MethodPost, "/wiki/rest/api/content", nil, bytes.NewReader(data), &created); err != nil {
		return PageMutationResult{}, err
	}
	out := PageMutationResult{OK: true, ID: created.ID, Page: &created}
	if page, err := c.GetPage(ctx, created.ID); err == nil {
		out.Page = &page
	}
	return out, nil
}

func (c liveClient) DeletePage(ctx context.Context, id string) (PageMutationResult, error) {
	id = strings.TrimSpace(id)
	if err := c.client.DoJSON(ctx, http.MethodDelete, "/wiki/rest/api/content/"+url.PathEscape(id), nil, nil, nil); err != nil {
		return PageMutationResult{}, err
	}
	return PageMutationResult{OK: true, ID: id}, nil
}

func (c liveClient) UploadPageAttachment(ctx context.Context, pageID string, request AttachmentUploadRequest) (AttachmentUploadResult, error) {
	pageID = strings.TrimSpace(pageID)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", firstNonEmpty(request.Filename, "attachment"))
	if err != nil {
		return AttachmentUploadResult{}, err
	}
	if _, err := part.Write(request.Data); err != nil {
		return AttachmentUploadResult{}, err
	}
	if err := writer.Close(); err != nil {
		return AttachmentUploadResult{}, err
	}
	var out attachmentUploadResponse
	err = c.client.Do(ctx, http.MethodPost, "/wiki/rest/api/content/"+url.PathEscape(pageID)+"/child/attachment", nil, &body, map[string]string{
		"Accept":            "application/json",
		"Content-Type":      writer.FormDataContentType(),
		"X-Atlassian-Token": "no-check",
	}, &out)
	if err != nil {
		return AttachmentUploadResult{}, err
	}
	return AttachmentUploadResult{OK: true, PageID: pageID, Attachments: out.Results}, nil
}

func (c liveClient) ListPageAttachments(ctx context.Context, pageID string) (AttachmentListResult, error) {
	pageID = strings.TrimSpace(pageID)
	query := url.Values{}
	query.Set("limit", "100")
	var out attachmentListResponse
	err := c.client.GetJSON(ctx, "/wiki/rest/api/content/"+url.PathEscape(pageID)+"/child/attachment", query, &out)
	if err != nil {
		return AttachmentListResult{}, err
	}
	return AttachmentListResult{PageID: pageID, Count: len(out.Results), Attachments: out.Results}, nil
}

func (c liveClient) GetAttachment(ctx context.Context, id, pageID string, downloadContent bool) (AttachmentGetResult, error) {
	id = strings.TrimSpace(id)
	var attachment Attachment
	if err := c.client.GetJSON(ctx, "/wiki/rest/api/content/"+url.PathEscape(id), nil, &attachment); err != nil {
		return AttachmentGetResult{}, err
	}
	download := confluenceDownloadURL(attachment)
	if download == "" || !downloadContent {
		return AttachmentGetResult{ID: id, Filename: firstNonEmpty(attachment.Filename, attachment.Title), MimeType: attachment.MediaType, Attachment: attachment}, nil
	}
	if parentID := firstNonEmpty(pageID, confluenceAttachmentPageID(attachment)); parentID != "" {
		download = "/wiki/rest/api/content/" + url.PathEscape(parentID) + "/child/attachment/" + url.PathEscape(id) + "/download"
	}
	data, contentType, err := c.client.GetBytes(ctx, download, nil)
	if err != nil {
		return AttachmentGetResult{}, err
	}
	return AttachmentGetResult{ID: id, Filename: firstNonEmpty(attachment.Filename, attachment.Title), MimeType: firstNonEmpty(attachment.MediaType, contentType), Size: len(data), ContentBytes: data, Attachment: attachment}, nil
}

func (c liveClient) DeleteAttachment(ctx context.Context, id string) (AttachmentDeleteResult, error) {
	id = strings.TrimSpace(id)
	if err := c.client.DoJSON(ctx, http.MethodDelete, "/wiki/rest/api/content/"+url.PathEscape(id), nil, nil, nil); err != nil {
		return AttachmentDeleteResult{}, err
	}
	return AttachmentDeleteResult{OK: true, AttachmentID: id}, nil
}

func (c liveClient) SearchUsers(ctx context.Context, input UserSearchOptions) ([]User, error) {
	if strings.TrimSpace(input.Query) == "" && strings.TrimSpace(input.CQL) == "" {
		user, err := c.CurrentUser(ctx)
		if err != nil {
			return nil, err
		}
		return []User{user}, nil
	}
	limit := clamp(input.Limit, 20, 100)
	query := url.Values{}
	query.Set("cql", userCQL(input))
	query.Set("limit", strconv.Itoa(limit))
	if input.Cursor != "" {
		query.Set("cursor", input.Cursor)
	}
	var out []User
	for {
		var page searchResponse
		if err := c.client.GetJSON(ctx, "/wiki/rest/api/search", query, &page); err != nil {
			return nil, err
		}
		out = append(out, usersFromSearch(page.Results)...)
		next := cursorFromNext(page.Links.Next)
		if !input.All || next == "" {
			return out, nil
		}
		query.Set("cursor", next)
	}
}

func shouldUsePageList(input PageSearchOptions) bool {
	return strings.TrimSpace(input.Query) == "" && strings.TrimSpace(input.CQL) == ""
}

func clamp(value, fallback, max int) int {
	if value <= 0 {
		value = fallback
	}
	if value > max {
		return max
	}
	return value
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
