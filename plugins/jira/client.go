package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"strconv"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type Client interface {
	CurrentUser(context.Context) (User, error)
	SearchIssues(context.Context, IssueSearchOptions) ([]Issue, error)
	GetIssue(context.Context, string) (Issue, error)
	CreateIssue(context.Context, IssueCreateRequest) (IssueMutationResult, error)
	EditIssue(context.Context, string, IssueEditRequest) (IssueMutationResult, error)
	DeleteIssue(context.Context, string, bool) (IssueMutationResult, error)
	ListTransitions(context.Context, string) (IssueTransitionListResult, error)
	TransitionIssue(context.Context, string, IssueTransitionRequest) (IssueMutationResult, error)
	AddComment(context.Context, string, CommentRequest) (CommentResult, error)
	EditComment(context.Context, string, string, CommentRequest) (CommentResult, error)
	DeleteComment(context.Context, string, string) (CommentMutationResult, error)
	UploadIssueAttachment(context.Context, string, AttachmentUploadRequest) (AttachmentUploadResult, error)
	GetAttachment(context.Context, Attachment) (AttachmentGetResult, error)
	DeleteAttachment(context.Context, string) (AttachmentDeleteResult, error)
	CreateMeta(context.Context, IssueCreateMetaOptions) (IssueMetaResult, error)
	EditMeta(context.Context, string) (IssueMetaResult, error)
	SearchUsers(context.Context, UserSearchOptions) ([]User, error)
}

type ClientFactory func(pluginbinding.Context, string) (Client, error)

func NewLiveClient(ctx pluginbinding.Context, endpointRef string) (Client, error) {
	endpointRef = strings.TrimSpace(endpointRef)
	if endpointRef == "" {
		return nil, fmt.Errorf("jira endpoint_ref is required")
	}
	return liveClient{endpointRef: endpointRef, host: ctx.Host}, nil
}

type liveClient struct {
	endpointRef string
	host        pluginbinding.HostClient
}

func (c liveClient) CurrentUser(ctx context.Context) (User, error) {
	var out User
	err := c.getJSON(ctx, "/rest/api/3/myself", nil, &out)
	return out, err
}

func (c liveClient) SearchIssues(ctx context.Context, input IssueSearchOptions) ([]Issue, error) {
	limit := clamp(input.Limit, 20, 100)
	query := url.Values{}
	query.Set("jql", issueJQL(input))
	query.Set("maxResults", strconv.Itoa(limit))
	for _, field := range issueFields(input.Fields) {
		query.Add("fields", field)
	}
	var out []Issue
	for {
		if input.NextPageToken != "" {
			query.Set("nextPageToken", input.NextPageToken)
		}
		var page issueSearchResponse
		if err := c.getJSON(ctx, "/rest/api/3/search/jql", query, &page); err != nil {
			return nil, err
		}
		out = append(out, page.Issues...)
		if !input.All || page.IsLast || strings.TrimSpace(page.NextPageToken) == "" {
			return out, nil
		}
		input.NextPageToken = page.NextPageToken
	}
}

func (c liveClient) GetIssue(ctx context.Context, key string) (Issue, error) {
	query := url.Values{}
	for _, field := range issueFields(nil) {
		query.Add("fields", field)
	}
	var out Issue
	err := c.getJSON(ctx, "/rest/api/3/issue/"+url.PathEscape(strings.TrimSpace(key)), query, &out)
	return out, err
}

func (c liveClient) CreateIssue(ctx context.Context, request IssueCreateRequest) (IssueMutationResult, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return IssueMutationResult{}, err
	}
	var out IssueMutationResult
	err = c.doJSON(ctx, "POST", "/rest/api/3/issue", nil, bytes.NewReader(payload), &out)
	if err != nil {
		return IssueMutationResult{}, err
	}
	out.OK = true
	if strings.TrimSpace(out.Key) != "" {
		if issue, err := c.GetIssue(ctx, out.Key); err == nil {
			out.Issue = &issue
		}
	}
	return out, nil
}

func (c liveClient) EditIssue(ctx context.Context, key string, request IssueEditRequest) (IssueMutationResult, error) {
	key = strings.TrimSpace(key)
	payload, err := json.Marshal(request)
	if err != nil {
		return IssueMutationResult{}, err
	}
	if err := c.doJSON(ctx, "PUT", "/rest/api/3/issue/"+url.PathEscape(key), nil, bytes.NewReader(payload), nil); err != nil {
		return IssueMutationResult{}, err
	}
	out := IssueMutationResult{OK: true, Key: key}
	if issue, err := c.GetIssue(ctx, key); err == nil {
		out.ID = issue.ID
		out.Self = issue.Self
		out.Issue = &issue
	}
	return out, nil
}

func (c liveClient) DeleteIssue(ctx context.Context, key string, deleteSubtasks bool) (IssueMutationResult, error) {
	key = strings.TrimSpace(key)
	query := url.Values{}
	if deleteSubtasks {
		query.Set("deleteSubtasks", "true")
	}
	if err := c.doJSON(ctx, "DELETE", "/rest/api/3/issue/"+url.PathEscape(key), query, nil, nil); err != nil {
		return IssueMutationResult{}, err
	}
	return IssueMutationResult{OK: true, Key: key}, nil
}

func (c liveClient) ListTransitions(ctx context.Context, key string) (IssueTransitionListResult, error) {
	key = strings.TrimSpace(key)
	issue, err := c.GetIssue(ctx, key)
	if err != nil {
		return IssueTransitionListResult{}, err
	}
	var out issueTransitionsResponse
	if err := c.getJSON(ctx, "/rest/api/3/issue/"+url.PathEscape(key)+"/transitions", nil, &out); err != nil {
		return IssueTransitionListResult{}, err
	}
	return IssueTransitionListResult{IssueKey: key, CurrentStatus: issue.Fields.Status, Transitions: out.Transitions}, nil
}

func (c liveClient) TransitionIssue(ctx context.Context, key string, request IssueTransitionRequest) (IssueMutationResult, error) {
	key = strings.TrimSpace(key)
	payload, err := json.Marshal(map[string]any{"transition": map[string]string{"id": strings.TrimSpace(request.TransitionID)}})
	if err != nil {
		return IssueMutationResult{}, err
	}
	if err := c.doJSON(ctx, "POST", "/rest/api/3/issue/"+url.PathEscape(key)+"/transitions", nil, bytes.NewReader(payload), nil); err != nil {
		return IssueMutationResult{}, err
	}
	out := IssueMutationResult{OK: true, Key: key}
	if issue, err := c.GetIssue(ctx, key); err == nil {
		out.ID = issue.ID
		out.Self = issue.Self
		out.Issue = &issue
	}
	return out, nil
}

func (c liveClient) AddComment(ctx context.Context, key string, request CommentRequest) (CommentResult, error) {
	key = strings.TrimSpace(key)
	payload, err := json.Marshal(request)
	if err != nil {
		return CommentResult{}, err
	}
	var comment Comment
	if err := c.doJSON(ctx, "POST", "/rest/api/3/issue/"+url.PathEscape(key)+"/comment", nil, bytes.NewReader(payload), &comment); err != nil {
		return CommentResult{}, err
	}
	return CommentResult{OK: true, IssueKey: key, Comment: comment}, nil
}

func (c liveClient) EditComment(ctx context.Context, key, commentID string, request CommentRequest) (CommentResult, error) {
	key = strings.TrimSpace(key)
	commentID = strings.TrimSpace(commentID)
	payload, err := json.Marshal(request)
	if err != nil {
		return CommentResult{}, err
	}
	var comment Comment
	if err := c.doJSON(ctx, "PUT", "/rest/api/3/issue/"+url.PathEscape(key)+"/comment/"+url.PathEscape(commentID), nil, bytes.NewReader(payload), &comment); err != nil {
		return CommentResult{}, err
	}
	return CommentResult{OK: true, IssueKey: key, Comment: comment}, nil
}

func (c liveClient) DeleteComment(ctx context.Context, key, commentID string) (CommentMutationResult, error) {
	key = strings.TrimSpace(key)
	commentID = strings.TrimSpace(commentID)
	if err := c.doJSON(ctx, "DELETE", "/rest/api/3/issue/"+url.PathEscape(key)+"/comment/"+url.PathEscape(commentID), nil, nil, nil); err != nil {
		return CommentMutationResult{}, err
	}
	return CommentMutationResult{OK: true, IssueKey: key, CommentID: commentID}, nil
}

func (c liveClient) UploadIssueAttachment(ctx context.Context, key string, request AttachmentUploadRequest) (AttachmentUploadResult, error) {
	key = strings.TrimSpace(key)
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
	var out []Attachment
	err = c.do(ctx, "POST", "/rest/api/3/issue/"+url.PathEscape(key)+"/attachments", nil, &body, map[string]string{
		"Accept":            "application/json",
		"Content-Type":      writer.FormDataContentType(),
		"X-Atlassian-Token": "no-check",
	}, &out)
	if err != nil {
		return AttachmentUploadResult{}, err
	}
	return AttachmentUploadResult{OK: true, IssueKey: key, Attachments: out}, nil
}

func (c liveClient) GetAttachment(ctx context.Context, attachment Attachment) (AttachmentGetResult, error) {
	id := strings.TrimSpace(attachment.ID)
	path := "/rest/api/3/attachment/content/" + url.PathEscape(id)
	data, contentType, err := c.getBytes(ctx, path, nil)
	if err != nil {
		return AttachmentGetResult{}, err
	}
	return AttachmentGetResult{ID: id, Filename: attachment.Filename, MimeType: firstNonEmpty(attachment.MimeType, contentType), Size: len(data), ContentBytes: data}, nil
}

func (c liveClient) DeleteAttachment(ctx context.Context, id string) (AttachmentDeleteResult, error) {
	id = strings.TrimSpace(id)
	if err := c.doJSON(ctx, "DELETE", "/rest/api/3/attachment/"+url.PathEscape(id), nil, nil, nil); err != nil {
		return AttachmentDeleteResult{}, err
	}
	return AttachmentDeleteResult{OK: true, AttachmentID: id}, nil
}

func (c liveClient) CreateMeta(ctx context.Context, input IssueCreateMetaOptions) (IssueMetaResult, error) {
	query := url.Values{}
	query.Set("expand", "projects.issuetypes.fields")
	if project := strings.TrimSpace(input.ProjectKey); project != "" {
		query.Set("projectKeys", project)
	}
	if issueType := strings.TrimSpace(input.IssueType); issueType != "" {
		query.Set("issuetypeNames", issueType)
	}
	var out json.RawMessage
	err := c.getJSON(ctx, "/rest/api/3/issue/createmeta", query, &out)
	if err != nil {
		return IssueMetaResult{}, err
	}
	return IssueMetaResult{Metadata: out}, nil
}

func (c liveClient) EditMeta(ctx context.Context, key string) (IssueMetaResult, error) {
	var out json.RawMessage
	err := c.getJSON(ctx, "/rest/api/3/issue/"+url.PathEscape(strings.TrimSpace(key))+"/editmeta", nil, &out)
	if err != nil {
		return IssueMetaResult{}, err
	}
	return IssueMetaResult{Metadata: out}, nil
}

func (c liveClient) SearchUsers(ctx context.Context, input UserSearchOptions) ([]User, error) {
	limit := clamp(input.Limit, 20, 100)
	query := url.Values{}
	query.Set("query", strings.TrimSpace(input.Query))
	query.Set("maxResults", strconv.Itoa(limit))
	query.Set("startAt", strconv.Itoa(input.StartAt))
	var out []User
	for {
		var page []User
		if err := c.getJSON(ctx, "/rest/api/3/user/search", query, &page); err != nil {
			return nil, err
		}
		out = append(out, page...)
		if !input.All || len(page) < limit {
			return out, nil
		}
		input.StartAt += len(page)
		query.Set("startAt", strconv.Itoa(input.StartAt))
	}
}

func (c liveClient) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	return c.doJSON(ctx, "GET", path, query, nil, out)
}

func (c liveClient) doJSON(ctx context.Context, method, path string, query url.Values, body io.Reader, out any) error {
	headers := map[string]string{"Accept": "application/json"}
	if body != nil {
		headers["Content-Type"] = "application/json"
	}
	return c.do(ctx, method, path, query, body, headers, out)
}

func (c liveClient) do(ctx context.Context, method, path string, query url.Values, body io.Reader, headers map[string]string, out any) error {
	_ = ctx
	if c.host == nil {
		return fmt.Errorf("host client is unavailable")
	}
	var payload []byte
	if body != nil {
		data, err := io.ReadAll(io.LimitReader(body, 256*1024*1024))
		if err != nil {
			return err
		}
		payload = data
	}
	resp, err := c.host.HTTP(pluginbinding.HTTPRequest{
		EndpointRef: c.endpointRef,
		Path:        path,
		Query:       map[string][]string(query),
		Method:      method,
		Headers:     headers,
		Body:        payload,
		Auth: &pluginbinding.HTTPAuthRequest{
			BearerTokenPurpose: AuthPurposeAPIToken,
		},
		TimeoutMS: 30000,
		MaxBytes:  64 * 1024 * 1024,
		UserAgent: "fluxplane-dex/0.1",
	})
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return jiraHTTPError(resp.StatusCode, resp.Body)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(resp.Body, out)
}

func (c liveClient) getBytes(ctx context.Context, path string, query url.Values) ([]byte, string, error) {
	_ = ctx
	if c.host == nil {
		return nil, "", fmt.Errorf("host client is unavailable")
	}
	resp, err := c.host.HTTP(pluginbinding.HTTPRequest{
		EndpointRef: c.endpointRef,
		Path:        path,
		Query:       map[string][]string(query),
		Method:      "GET",
		Auth: &pluginbinding.HTTPAuthRequest{
			BearerTokenPurpose: AuthPurposeAPIToken,
		},
		TimeoutMS: 30000,
		MaxBytes:  256 * 1024 * 1024,
		UserAgent: "fluxplane-dex/0.1",
	})
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, "", jiraHTTPError(resp.StatusCode, resp.Body)
	}
	return resp.Body, resp.ContentType, nil
}

func jiraHTTPError(status int, data []byte) error {
	message := strings.TrimSpace(string(data))
	var payload struct {
		Message       string   `json:"message"`
		ErrorMessages []string `json:"errorMessages"`
	}
	if err := json.Unmarshal(data, &payload); err == nil {
		switch {
		case strings.TrimSpace(payload.Message) != "":
			message = strings.TrimSpace(payload.Message)
		case len(payload.ErrorMessages) > 0:
			message = strings.Join(payload.ErrorMessages, "; ")
		}
	}
	if message == "" {
		message = "request failed"
	}
	return fmt.Errorf("jira returned status %d: %s", status, message)
}

func issueFields(fields []string) []string {
	if len(fields) > 0 {
		return fields
	}
	return []string{"summary", "description", "attachment", "status", "assignee", "reporter", "creator", "updated", "created", "project", "issuetype", "priority", "labels"}
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
