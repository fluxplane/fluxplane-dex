package jira

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type User struct {
	AccountID    string `json:"accountId,omitempty"`
	AccountType  string `json:"accountType,omitempty"`
	DisplayName  string `json:"displayName,omitempty"`
	EmailAddress string `json:"emailAddress,omitempty"`
	Active       bool   `json:"active,omitempty"`
	Self         string `json:"self,omitempty"`
}

type UserRecord struct {
	pluginbinding.DatasourceRecord
	AccountID    string `json:"account_id" datasource:"id,completion,view=compact|lookup|table"`
	DisplayName  string `json:"display_name,omitempty" datasource:"title,completion,view=compact|lookup|table"`
	EmailAddress string `json:"email,omitempty" datasource:"completion,view=lookup|table"`
	AccountType  string `json:"account_type,omitempty"`
	Active       bool   `json:"active,omitempty"`
	Self         string `json:"self,omitempty"`
}

type Issue struct {
	ID     string      `json:"id,omitempty"`
	Key    string      `json:"key,omitempty"`
	Self   string      `json:"self,omitempty"`
	Fields IssueFields `json:"fields,omitempty"`
}

type IssueFields struct {
	Summary     string          `json:"summary,omitempty"`
	Description json.RawMessage `json:"description,omitempty"`
	Attachments []Attachment    `json:"attachment,omitempty"`
	Status      NamedValue      `json:"status,omitempty"`
	Assignee    *User           `json:"assignee,omitempty"`
	Reporter    *User           `json:"reporter,omitempty"`
	Creator     *User           `json:"creator,omitempty"`
	Project     Project         `json:"project,omitempty"`
	IssueType   NamedValue      `json:"issuetype,omitempty"`
	Priority    NamedValue      `json:"priority,omitempty"`
	Labels      []string        `json:"labels,omitempty"`
	Updated     string          `json:"updated,omitempty"`
	Created     string          `json:"created,omitempty"`
	Raw         interface{}     `json:"-"`
}

type Project struct {
	ID   string `json:"id,omitempty"`
	Key  string `json:"key,omitempty"`
	Name string `json:"name,omitempty"`
	Self string `json:"self,omitempty"`
}

type NamedValue struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Self string `json:"self,omitempty"`
}

type IssueRecord struct {
	pluginbinding.DatasourceRecord
	IssueID             string   `json:"issue_id,omitempty"`
	Key                 string   `json:"key" datasource:"id,completion,view=compact|lookup|table"`
	Summary             string   `json:"summary,omitempty" datasource:"title,completion,view=compact|lookup|table"`
	ProjectKey          string   `json:"project_key,omitempty" datasource:"completion,view=compact|lookup|table"`
	ProjectName         string   `json:"project_name,omitempty" datasource:"completion,view=lookup|table"`
	IssueType           string   `json:"issue_type,omitempty" datasource:"completion,view=compact|lookup|table"`
	Status              string   `json:"status,omitempty" datasource:"completion,view=compact|lookup|table"`
	Priority            string   `json:"priority,omitempty" datasource:"completion,view=lookup|table"`
	AssigneeAccountID   string   `json:"assignee_account_id,omitempty" datasource:"relation=jira.user:assignee"`
	AssigneeDisplayName string   `json:"assignee_display_name,omitempty" datasource:"completion,view=compact|lookup|table"`
	ReporterAccountID   string   `json:"reporter_account_id,omitempty" datasource:"relation=jira.user:reporter"`
	ReporterDisplayName string   `json:"reporter_display_name,omitempty" datasource:"completion,view=lookup|table"`
	CreatorAccountID    string   `json:"creator_account_id,omitempty" datasource:"relation=jira.user:creator"`
	CreatorDisplayName  string   `json:"creator_display_name,omitempty"`
	Labels              []string `json:"labels,omitempty"`
	Created             string   `json:"created,omitempty"`
	Updated             string   `json:"updated,omitempty" datasource:"view=compact|lookup|table"`
	WebURL              string   `json:"web_url,omitempty" datasource:"completion,view=lookup|table"`
	Self                string   `json:"self,omitempty"`
}

type IssueSearchOptions struct {
	Query         string
	JQL           string
	Project       string
	Status        string
	Limit         int
	All           bool
	NextPageToken string
	Fields        []string
	OrderBy       string
}

type IssueCreateMetaOptions struct {
	ProjectKey string
	IssueType  string
}

type IssueCreateRequest struct {
	Fields map[string]any `json:"fields"`
	Update map[string]any `json:"update,omitempty"`
}

type IssueEditRequest struct {
	Fields map[string]any `json:"fields,omitempty"`
	Update map[string]any `json:"update,omitempty"`
}

type IssueMutationResult struct {
	OK      bool   `json:"ok"`
	ID      string `json:"id,omitempty"`
	Key     string `json:"key,omitempty"`
	Self    string `json:"self,omitempty"`
	Issue   *Issue `json:"issue,omitempty"`
	Warning string `json:"warning,omitempty"`
}

type IssueTransition struct {
	ID   string     `json:"id,omitempty"`
	Name string     `json:"name,omitempty"`
	To   NamedValue `json:"to,omitempty"`
}

type IssueTransitionListResult struct {
	IssueKey      string            `json:"issue_key,omitempty"`
	CurrentStatus NamedValue        `json:"current_status,omitempty"`
	Transitions   []IssueTransition `json:"transitions"`
}

type IssueTransitionRequest struct {
	TransitionID string `json:"-"`
}

type IssueTransitionRunResult struct {
	OK                   bool              `json:"ok"`
	IssueKey             string            `json:"issue_key,omitempty"`
	InitialStatus        NamedValue        `json:"initial_status,omitempty"`
	CurrentStatus        NamedValue        `json:"current_status,omitempty"`
	TargetStatus         string            `json:"target_status,omitempty"`
	AppliedTransitions   []IssueTransition `json:"applied_transitions,omitempty"`
	AvailableTransitions []IssueTransition `json:"available_transitions,omitempty"`
	Steps                int               `json:"steps"`
	Issue                *Issue            `json:"issue,omitempty"`
}

type CommentRequest struct {
	Body any `json:"body"`
}

type Comment struct {
	ID           string          `json:"id,omitempty"`
	Self         string          `json:"self,omitempty"`
	Body         json.RawMessage `json:"body,omitempty"`
	Author       *User           `json:"author,omitempty"`
	UpdateAuthor *User           `json:"updateAuthor,omitempty"`
	Created      string          `json:"created,omitempty"`
	Updated      string          `json:"updated,omitempty"`
}

type CommentResult struct {
	OK       bool    `json:"ok"`
	IssueKey string  `json:"issue_key,omitempty"`
	Comment  Comment `json:"comment"`
}

type CommentMutationResult struct {
	OK        bool   `json:"ok"`
	IssueKey  string `json:"issue_key,omitempty"`
	CommentID string `json:"comment_id,omitempty"`
}

type Attachment struct {
	ID        string `json:"id,omitempty"`
	Filename  string `json:"filename,omitempty"`
	MimeType  string `json:"mimeType,omitempty"`
	Size      int64  `json:"size,omitempty"`
	Content   string `json:"content,omitempty"`
	Thumbnail string `json:"thumbnail,omitempty"`
	Self      string `json:"self,omitempty"`
	Created   string `json:"created,omitempty"`
	Author    *User  `json:"author,omitempty"`
}

type AttachmentUploadRequest struct {
	Filename    string
	ContentType string
	Data        []byte
}

type AttachmentUploadResult struct {
	OK          bool         `json:"ok"`
	IssueKey    string       `json:"issue_key,omitempty"`
	Attachments []Attachment `json:"attachments"`
}

type AttachmentListResult struct {
	IssueKey    string       `json:"issue_key,omitempty"`
	Count       int          `json:"count"`
	Attachments []Attachment `json:"attachments"`
}

type AttachmentGetResult struct {
	ID           string `json:"id"`
	Filename     string `json:"filename,omitempty"`
	MimeType     string `json:"mime_type,omitempty"`
	Size         int    `json:"size"`
	ContentBytes []byte `json:"content_bytes,omitempty"`
	OutputPath   string `json:"output_path,omitempty"`
}

type AttachmentDeleteResult struct {
	OK           bool   `json:"ok"`
	AttachmentID string `json:"attachment_id"`
}

type IssueMetaResult struct {
	Metadata json.RawMessage `json:"metadata"`
}

type UserSearchOptions struct {
	Query   string
	Limit   int
	All     bool
	StartAt int
}

type issueSearchResponse struct {
	Issues        []Issue `json:"issues"`
	NextPageToken string  `json:"nextPageToken,omitempty"`
	IsLast        bool    `json:"isLast,omitempty"`
}

type issueTransitionsResponse struct {
	Transitions []IssueTransition `json:"transitions"`
}

func normalizeIssueRecord(source pluginbinding.DatasourceSource, baseURL string, issue Issue) (IssueRecord, bool) {
	key := strings.TrimSpace(issue.Key)
	if key == "" {
		return IssueRecord{}, false
	}
	fields := issue.Fields
	record := IssueRecord{
		DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityIssue, key, pluginbinding.RecordTitle(fields.Summary), pluginbinding.RecordLink("self", issueWebURL(baseURL, key))),
		IssueID:          issue.ID,
		Key:              key,
		Summary:          fields.Summary,
		ProjectKey:       fields.Project.Key,
		ProjectName:      fields.Project.Name,
		IssueType:        fields.IssueType.Name,
		Status:           fields.Status.Name,
		Priority:         fields.Priority.Name,
		Labels:           fields.Labels,
		Created:          fields.Created,
		Updated:          fields.Updated,
		WebURL:           issueWebURL(baseURL, key),
		Self:             issue.Self,
	}
	if fields.Assignee != nil {
		record.AssigneeAccountID = fields.Assignee.AccountID
		record.AssigneeDisplayName = fields.Assignee.DisplayName
	}
	if fields.Reporter != nil {
		record.ReporterAccountID = fields.Reporter.AccountID
		record.ReporterDisplayName = fields.Reporter.DisplayName
	}
	if fields.Creator != nil {
		record.CreatorAccountID = fields.Creator.AccountID
		record.CreatorDisplayName = fields.Creator.DisplayName
	}
	return record, true
}

func normalizeUserRecord(source pluginbinding.DatasourceSource, user User) (UserRecord, bool) {
	id := strings.TrimSpace(user.AccountID)
	if id == "" {
		return UserRecord{}, false
	}
	return UserRecord{
		DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityUser, id, pluginbinding.RecordTitle(user.DisplayName), pluginbinding.RecordLink("self", user.Self)),
		AccountID:        id,
		DisplayName:      user.DisplayName,
		EmailAddress:     user.EmailAddress,
		AccountType:      user.AccountType,
		Active:           user.Active,
		Self:             user.Self,
	}, true
}

func issueWebURL(baseURL, key string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	key = strings.TrimSpace(key)
	if baseURL == "" || key == "" {
		return ""
	}
	if parsed, err := url.Parse(baseURL); err == nil && parsed.Host == "api.atlassian.com" {
		return ""
	}
	return baseURL + "/browse/" + url.PathEscape(key)
}
