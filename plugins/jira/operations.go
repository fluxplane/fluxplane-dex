package jira

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/codewandler/md2adf"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/internal/atlassian"
)

type Service struct {
	ClientFactory ClientFactory
}

func NewService() Service {
	return Service{ClientFactory: NewLiveClient}
}

func (s Service) client(ctx pluginbinding.Context, input any) (Client, string, error) {
	endpointRef := strings.TrimSpace(pluginbinding.StringFromInput(pluginbinding.InputMap(input), "endpoint_ref"))
	factory := s.ClientFactory
	if factory == nil {
		factory = NewLiveClient
	}
	client, err := factory(ctx, endpointRef)
	return client, "", err
}

type JiraTargetInput struct {
	EndpointRef string `json:"endpoint_ref,omitempty" jsonschema:"description=Registered Jira endpoint ref resolved by the host."`
}

type AuthTestInput struct {
	JiraTargetInput
}

type LookupInput = pluginbinding.DatasourceLookupInput
type LookupResult = pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]
type IssueDatasourceResult = pluginbinding.DatasourceSearchResult[IssueRecord]
type UserDatasourceResult = pluginbinding.DatasourceSearchResult[UserRecord]
type IssueSearchResult = pluginbinding.ListResult[Issue]
type UserSearchResult = pluginbinding.ListResult[User]

type AuthTestResult struct {
	Text   string `json:"text"`
	Status string `json:"status"`
	User   User   `json:"user"`
}

type IssueSearchInput struct {
	pluginbinding.DatasourceSearchInput
	JQL     string   `json:"jql,omitempty" jsonschema:"description=Jira JQL query"`
	Project string   `json:"project,omitempty" jsonschema:"description=Project key filter"`
	Status  string   `json:"status,omitempty" jsonschema:"description=Status filter"`
	Fields  []string `json:"fields,omitempty" jsonschema:"description=Jira fields to request"`
	OrderBy string   `json:"order_by,omitempty" jsonschema:"description=JQL order by expression"`
}

type IssueShowInput struct {
	JiraTargetInput
	Key string `json:"key,omitempty" jsonschema:"description=Issue key"`
	ID  string `json:"id,omitempty" jsonschema:"description=Alias for key"`
}

type IssueCreateMetaInput struct {
	JiraTargetInput
	ProjectKey string `json:"project_key,omitempty" jsonschema:"description=Project key filter."`
	IssueType  string `json:"issue_type,omitempty" jsonschema:"description=Issue type name filter."`
}

type IssueEditMetaInput struct {
	JiraTargetInput
	Key string `json:"key,omitempty" jsonschema:"required,description=Issue key."`
	ID  string `json:"id,omitempty" jsonschema:"description=Alias for key."`
}

type IssueCreateInput struct {
	JiraTargetInput
	ProjectKey          string         `json:"project_key,omitempty" jsonschema:"required,description=Project key such as DEV."`
	IssueType           string         `json:"issue_type,omitempty" jsonschema:"required,description=Issue type name such as Task or Bug."`
	Summary             string         `json:"summary,omitempty" jsonschema:"required,description=Issue summary."`
	DescriptionMarkdown string         `json:"description_markdown,omitempty" jsonschema:"description=Issue description as Markdown converted to Jira ADF."`
	Labels              []string       `json:"labels,omitempty" jsonschema:"description=Labels to set."`
	AssigneeAccountID   string         `json:"assignee_account_id,omitempty" jsonschema:"description=Assignee Atlassian account ID."`
	ReporterAccountID   string         `json:"reporter_account_id,omitempty" jsonschema:"description=Reporter Atlassian account ID."`
	Priority            string         `json:"priority,omitempty" jsonschema:"description=Priority name."`
	ParentKey           string         `json:"parent_key,omitempty" jsonschema:"description=Parent issue key for subtasks."`
	Fields              map[string]any `json:"fields,omitempty" jsonschema:"description=Raw Jira fields. Explicit typed inputs override matching fields."`
	Update              map[string]any `json:"update,omitempty" jsonschema:"description=Raw Jira update instructions."`
}

type IssueEditInput struct {
	JiraTargetInput
	Key                 string         `json:"key,omitempty" jsonschema:"required,description=Issue key."`
	ID                  string         `json:"id,omitempty" jsonschema:"description=Alias for key."`
	Summary             string         `json:"summary,omitempty" jsonschema:"description=Issue summary."`
	DescriptionMarkdown string         `json:"description_markdown,omitempty" jsonschema:"description=Issue description as Markdown converted to Jira ADF."`
	Labels              []string       `json:"labels,omitempty" jsonschema:"description=Labels to set."`
	AssigneeAccountID   string         `json:"assignee_account_id,omitempty" jsonschema:"description=Assignee Atlassian account ID."`
	Priority            string         `json:"priority,omitempty" jsonschema:"description=Priority name."`
	Fields              map[string]any `json:"fields,omitempty" jsonschema:"description=Raw Jira fields. Explicit typed inputs override matching fields."`
	Update              map[string]any `json:"update,omitempty" jsonschema:"description=Raw Jira update instructions."`
}

type IssueDeleteInput struct {
	JiraTargetInput
	Key            string `json:"key,omitempty" jsonschema:"required,description=Issue key."`
	ID             string `json:"id,omitempty" jsonschema:"description=Alias for key."`
	DeleteSubtasks bool   `json:"delete_subtasks,omitempty" jsonschema:"description=Delete subtasks when deleting a parent issue."`
}

type IssueTransitionListInput struct {
	JiraTargetInput
	Key string `json:"key,omitempty" jsonschema:"required,description=Issue key."`
	ID  string `json:"id,omitempty" jsonschema:"description=Alias for key."`
}

type IssueTransitionRunInput struct {
	JiraTargetInput
	Key            string `json:"key,omitempty" jsonschema:"required,description=Issue key."`
	ID             string `json:"id,omitempty" jsonschema:"description=Alias for key."`
	TransitionID   string `json:"transition_id,omitempty" jsonschema:"description=Jira transition ID to apply."`
	TransitionName string `json:"transition_name,omitempty" jsonschema:"description=Jira transition name to apply."`
	TargetStatus   string `json:"target_status,omitempty" jsonschema:"description=Desired status name or ID. Without auto_transition this must be a currently available next status."`
	AutoTransition bool   `json:"auto_transition,omitempty" jsonschema:"description=When true repeatedly fetch available transitions and take intermediate transitions until target_status is reached or max_steps is hit."`
	MaxSteps       int    `json:"max_steps,omitempty" jsonschema:"description=Maximum transitions for auto_transition. Defaults to 5 and max 20."`
}

type CommentAddInput struct {
	JiraTargetInput
	Key          string `json:"key,omitempty" jsonschema:"required,description=Issue key."`
	ID           string `json:"id,omitempty" jsonschema:"description=Alias for key."`
	BodyMarkdown string `json:"body_markdown,omitempty" jsonschema:"required,description=Comment body as Markdown converted to Jira ADF."`
}

type CommentEditInput struct {
	JiraTargetInput
	Key          string `json:"key,omitempty" jsonschema:"required,description=Issue key."`
	ID           string `json:"id,omitempty" jsonschema:"description=Alias for key."`
	CommentID    string `json:"comment_id,omitempty" jsonschema:"required,description=Jira comment ID."`
	BodyMarkdown string `json:"body_markdown,omitempty" jsonschema:"required,description=Comment body as Markdown converted to Jira ADF."`
}

type CommentDeleteInput struct {
	JiraTargetInput
	Key       string `json:"key,omitempty" jsonschema:"required,description=Issue key."`
	ID        string `json:"id,omitempty" jsonschema:"description=Alias for key."`
	CommentID string `json:"comment_id,omitempty" jsonschema:"required,description=Jira comment ID."`
}

type AttachmentAddInput struct {
	JiraTargetInput
	Key          string `json:"key,omitempty" jsonschema:"required,description=Issue key."`
	ID           string `json:"id,omitempty" jsonschema:"description=Alias for key."`
	BlobRef      string `json:"blob_ref,omitempty" jsonschema:"description=Host blob ref to upload. Mutually exclusive with content_bytes."`
	ContentBytes []byte `json:"content_bytes,omitempty" jsonschema:"description=Base64-encoded inline bytes. Mutually exclusive with blob_ref."`
	Filename     string `json:"filename,omitempty" jsonschema:"description=Filename shown in Jira. Defaults to host blob filename when using blob_ref."`
	ContentType  string `json:"content_type,omitempty" jsonschema:"description=Attachment MIME type."`
}

type AttachmentListInput struct {
	JiraTargetInput
	Key string `json:"key,omitempty" jsonschema:"required,description=Issue key."`
	ID  string `json:"id,omitempty" jsonschema:"description=Alias for key."`
}

type AttachmentGetInput struct {
	JiraTargetInput
	AttachmentID string `json:"attachment_id,omitempty" jsonschema:"required,description=Jira attachment ID."`
	Filename     string `json:"filename,omitempty" jsonschema:"description=Optional filename metadata."`
	MimeType     string `json:"mime_type,omitempty" jsonschema:"description=Optional MIME type metadata."`
	BlobRef      string `json:"blob_ref,omitempty" jsonschema:"description=Optional host blob ref for downloaded attachment bytes."`
}

type AttachmentDeleteInput struct {
	JiraTargetInput
	AttachmentID string `json:"attachment_id,omitempty" jsonschema:"required,description=Jira attachment ID."`
}

type UserSearchInput struct {
	pluginbinding.DatasourceSearchInput
}

type IndexBuildInput struct {
	JiraTargetInput
	pluginbinding.IndexBuildInput
	IssueLimit int    `json:"issue_limit,omitempty" jsonschema:"description=Issue page size"`
	IssueQuery string `json:"issue_query,omitempty" jsonschema:"description=Issue text query"`
	IssueJQL   string `json:"issue_jql,omitempty" jsonschema:"description=Issue JQL query"`
	Project    string `json:"project,omitempty" jsonschema:"description=Issue project key filter"`
	Status     string `json:"status,omitempty" jsonschema:"description=Issue status filter"`
	UserLimit  int    `json:"user_limit,omitempty" jsonschema:"description=User page size"`
	UserQuery  string `json:"user_query,omitempty" jsonschema:"description=User search query"`
}

func (s Service) AuthTest(ctx pluginbinding.Context, input AuthTestInput) (AuthTestResult, error) {
	client, _, err := s.client(ctx, input)
	if err != nil {
		return AuthTestResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	user, err := client.CurrentUser(context.Background())
	if err != nil {
		return AuthTestResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	return AuthTestResult{Text: "Jira auth OK", Status: "ok", User: user}, nil
}

func (s Service) IssueSearch(ctx pluginbinding.Context, input IssueSearchInput) (IssueSearchResult, error) {
	client, _, err := s.client(ctx, input)
	if err != nil {
		return IssueSearchResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	issues, err := client.SearchIssues(context.Background(), issueSearchOptions(pluginbinding.InputMap(input), 20))
	if err != nil {
		return IssueSearchResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	return pluginbinding.NewListResult(issues), nil
}

func (s Service) IssueShow(ctx pluginbinding.Context, input IssueShowInput) (pluginbinding.ShowResult[Issue], error) {
	client, _, err := s.client(ctx, input)
	if err != nil {
		return pluginbinding.ShowResult[Issue]{}, pluginbinding.Errorf("secret", "%s", err)
	}
	key := strings.TrimSpace(pluginbinding.FirstString(pluginbinding.InputMap(input), "key", "id"))
	if key == "" {
		return pluginbinding.ShowResult[Issue]{}, pluginbinding.Fail("bad_input", "issue key is required")
	}
	issue, err := client.GetIssue(context.Background(), key)
	if err != nil {
		return pluginbinding.ShowResult[Issue]{}, pluginbinding.Errorf("jira", "%s", err)
	}
	return pluginbinding.NewShowResult(issue, map[string]any{"key": key}), nil
}

func (s Service) CreateMeta(ctx pluginbinding.Context, input IssueCreateMetaInput) (IssueMetaResult, error) {
	client, _, err := s.client(ctx, input)
	if err != nil {
		return IssueMetaResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	result, err := client.CreateMeta(context.Background(), IssueCreateMetaOptions{ProjectKey: input.ProjectKey, IssueType: input.IssueType})
	if err != nil {
		return IssueMetaResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	return result, nil
}

func (s Service) EditMeta(ctx pluginbinding.Context, input IssueEditMetaInput) (IssueMetaResult, error) {
	client, _, err := s.client(ctx, input)
	if err != nil {
		return IssueMetaResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	key := strings.TrimSpace(pluginbinding.FirstString(pluginbinding.InputMap(input), "key", "id"))
	if key == "" {
		return IssueMetaResult{}, pluginbinding.Fail("bad_input", "issue key is required")
	}
	result, err := client.EditMeta(context.Background(), key)
	if err != nil {
		return IssueMetaResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	return result, nil
}

func (s Service) TransitionList(ctx pluginbinding.Context, input IssueTransitionListInput) (IssueTransitionListResult, error) {
	client, _, err := s.client(ctx, input)
	if err != nil {
		return IssueTransitionListResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	key := strings.TrimSpace(pluginbinding.FirstString(pluginbinding.InputMap(input), "key", "id"))
	if key == "" {
		return IssueTransitionListResult{}, pluginbinding.Fail("bad_input", "issue key is required")
	}
	result, err := client.ListTransitions(context.Background(), key)
	if err != nil {
		return IssueTransitionListResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	return result, nil
}

func (s Service) TransitionRun(ctx pluginbinding.Context, input IssueTransitionRunInput) (IssueTransitionRunResult, error) {
	client, _, err := s.client(ctx, input)
	if err != nil {
		return IssueTransitionRunResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	key := strings.TrimSpace(pluginbinding.FirstString(pluginbinding.InputMap(input), "key", "id"))
	if key == "" {
		return IssueTransitionRunResult{}, pluginbinding.Fail("bad_input", "issue key is required")
	}
	targetStatus := strings.TrimSpace(input.TargetStatus)
	if strings.TrimSpace(input.TransitionID) == "" && strings.TrimSpace(input.TransitionName) == "" && targetStatus == "" {
		return IssueTransitionRunResult{}, pluginbinding.Fail("bad_input", "transition_id, transition_name, or target_status is required")
	}

	state, err := client.ListTransitions(context.Background(), key)
	if err != nil {
		return IssueTransitionRunResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	result := IssueTransitionRunResult{
		OK:                   true,
		IssueKey:             key,
		InitialStatus:        state.CurrentStatus,
		CurrentStatus:        state.CurrentStatus,
		TargetStatus:         targetStatus,
		AvailableTransitions: state.Transitions,
	}
	if targetStatus != "" && statusMatches(state.CurrentStatus, targetStatus) {
		issue, err := client.GetIssue(context.Background(), key)
		if err != nil {
			return IssueTransitionRunResult{}, pluginbinding.Errorf("jira", "%s", err)
		}
		if strings.TrimSpace(issue.Key) != "" {
			result.Issue = &issue
		}
		return result, nil
	}

	maxSteps := boundedTransitionSteps(input.MaxSteps)
	tried := map[string]bool{}
	for result.Steps < maxSteps {
		transition, ok := selectTransition(state, input, result.Steps > 0 || input.AutoTransition, tried)
		if !ok {
			if input.AutoTransition && targetStatus != "" && result.Steps > 0 {
				return IssueTransitionRunResult{}, pluginbinding.Errorf("jira", "target status %q was not reachable after %d transition(s); current status is %q", targetStatus, result.Steps, result.CurrentStatus.Name)
			}
			return IssueTransitionRunResult{}, pluginbinding.Errorf("jira", "no currently available transition matches the request; available transitions: %s", transitionSummary(state.Transitions))
		}
		if tried[transitionKey(transition)] {
			return IssueTransitionRunResult{}, pluginbinding.Errorf("jira", "transition walk repeated %q before reaching target status %q", transition.Name, targetStatus)
		}
		tried[transitionKey(transition)] = true
		mutation, err := client.TransitionIssue(context.Background(), key, IssueTransitionRequest{TransitionID: transition.ID})
		if err != nil {
			return IssueTransitionRunResult{}, pluginbinding.Errorf("jira", "%s", err)
		}
		result.AppliedTransitions = append(result.AppliedTransitions, transition)
		result.Steps++
		if mutation.Issue != nil {
			result.Issue = mutation.Issue
			result.CurrentStatus = mutation.Issue.Fields.Status
		}
		if targetStatus == "" {
			return result, nil
		}
		state, err = client.ListTransitions(context.Background(), key)
		if err != nil {
			return IssueTransitionRunResult{}, pluginbinding.Errorf("jira", "%s", err)
		}
		result.CurrentStatus = state.CurrentStatus
		result.AvailableTransitions = state.Transitions
		if statusMatches(state.CurrentStatus, targetStatus) {
			return result, nil
		}
		if !input.AutoTransition {
			return result, nil
		}
	}
	return IssueTransitionRunResult{}, pluginbinding.Errorf("jira", "target status %q was not reached within max_steps=%d; current status is %q", targetStatus, maxSteps, result.CurrentStatus.Name)
}

func (s Service) IssueCreate(ctx pluginbinding.Context, input IssueCreateInput) (IssueMutationResult, error) {
	client, _, err := s.client(ctx, input)
	if err != nil {
		return IssueMutationResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	request, err := issueCreateRequest(input)
	if err != nil {
		return IssueMutationResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	result, err := client.CreateIssue(context.Background(), request)
	if err != nil {
		return IssueMutationResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	if strings.TrimSpace(input.DescriptionMarkdown) != "" && strings.TrimSpace(result.Key) != "" {
		rewritten, uploadErr := uploadMarkdownBlobImages(ctx, client, result.Key, input.DescriptionMarkdown)
		if uploadErr == nil && rewritten != input.DescriptionMarkdown {
			editRequest, buildErr := issueEditRequest(IssueEditInput{DescriptionMarkdown: rewritten})
			if buildErr != nil {
				result.Warning = fmt.Sprintf("issue %s created, but rewriting description with uploaded images failed: %s", result.Key, buildErr)
				return result, nil
			}
			updated, editErr := client.EditIssue(context.Background(), result.Key, editRequest)
			if editErr != nil {
				result.Warning = fmt.Sprintf("issue %s created, but updating description with uploaded images failed: %s", result.Key, editErr)
				return result, nil
			}
			result.Issue = updated.Issue
		} else if uploadErr != nil {
			result.Warning = fmt.Sprintf("issue %s created, but uploading inline images failed: %s", result.Key, uploadErr)
		}
	}
	return result, nil
}

func (s Service) CommentAdd(ctx pluginbinding.Context, input CommentAddInput) (CommentResult, error) {
	client, _, err := s.client(ctx, input)
	if err != nil {
		return CommentResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	key := strings.TrimSpace(pluginbinding.FirstString(pluginbinding.InputMap(input), "key", "id"))
	if key == "" {
		return CommentResult{}, pluginbinding.Fail("bad_input", "issue key is required")
	}
	bodyMarkdown, err := uploadMarkdownBlobImages(ctx, client, key, input.BodyMarkdown)
	if err != nil {
		return CommentResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	input.BodyMarkdown = bodyMarkdown
	request, err := commentRequest(input.BodyMarkdown)
	if err != nil {
		return CommentResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	result, err := client.AddComment(context.Background(), key, request)
	if err != nil {
		return CommentResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	return result, nil
}

func (s Service) CommentEdit(ctx pluginbinding.Context, input CommentEditInput) (CommentResult, error) {
	client, _, err := s.client(ctx, input)
	if err != nil {
		return CommentResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	key := strings.TrimSpace(pluginbinding.FirstString(pluginbinding.InputMap(input), "key", "id"))
	commentID := strings.TrimSpace(input.CommentID)
	if key == "" {
		return CommentResult{}, pluginbinding.Fail("bad_input", "issue key is required")
	}
	if commentID == "" {
		return CommentResult{}, pluginbinding.Fail("bad_input", "comment_id is required")
	}
	bodyMarkdown, err := uploadMarkdownBlobImages(ctx, client, key, input.BodyMarkdown)
	if err != nil {
		return CommentResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	input.BodyMarkdown = bodyMarkdown
	request, err := commentRequest(input.BodyMarkdown)
	if err != nil {
		return CommentResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	result, err := client.EditComment(context.Background(), key, commentID, request)
	if err != nil {
		return CommentResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	return result, nil
}

func (s Service) CommentDelete(ctx pluginbinding.Context, input CommentDeleteInput) (CommentMutationResult, error) {
	client, _, err := s.client(ctx, input)
	if err != nil {
		return CommentMutationResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	key := strings.TrimSpace(pluginbinding.FirstString(pluginbinding.InputMap(input), "key", "id"))
	commentID := strings.TrimSpace(input.CommentID)
	if key == "" {
		return CommentMutationResult{}, pluginbinding.Fail("bad_input", "issue key is required")
	}
	if commentID == "" {
		return CommentMutationResult{}, pluginbinding.Fail("bad_input", "comment_id is required")
	}
	result, err := client.DeleteComment(context.Background(), key, commentID)
	if err != nil {
		return CommentMutationResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	return result, nil
}

func (s Service) AttachmentAdd(ctx pluginbinding.Context, input AttachmentAddInput) (AttachmentUploadResult, error) {
	client, _, err := s.client(ctx, input)
	if err != nil {
		return AttachmentUploadResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	key := strings.TrimSpace(pluginbinding.FirstString(pluginbinding.InputMap(input), "key", "id"))
	if key == "" {
		return AttachmentUploadResult{}, pluginbinding.Fail("bad_input", "issue key is required")
	}
	request, err := attachmentUploadRequest(ctx, input.BlobRef, input.ContentBytes, input.Filename, input.ContentType)
	if err != nil {
		return AttachmentUploadResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	result, err := client.UploadIssueAttachment(context.Background(), key, request)
	if err != nil {
		return AttachmentUploadResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	return result, nil
}

func (s Service) AttachmentList(ctx pluginbinding.Context, input AttachmentListInput) (AttachmentListResult, error) {
	client, _, err := s.client(ctx, input)
	if err != nil {
		return AttachmentListResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	key := strings.TrimSpace(pluginbinding.FirstString(pluginbinding.InputMap(input), "key", "id"))
	if key == "" {
		return AttachmentListResult{}, pluginbinding.Fail("bad_input", "issue key is required")
	}
	issue, err := client.GetIssue(context.Background(), key)
	if err != nil {
		return AttachmentListResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	return AttachmentListResult{IssueKey: key, Count: len(issue.Fields.Attachments), Attachments: issue.Fields.Attachments}, nil
}

func (s Service) AttachmentGet(ctx pluginbinding.Context, input AttachmentGetInput) (AttachmentGetResult, error) {
	client, _, err := s.client(ctx, input)
	if err != nil {
		return AttachmentGetResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	attachmentID := strings.TrimSpace(input.AttachmentID)
	if attachmentID == "" {
		return AttachmentGetResult{}, pluginbinding.Fail("bad_input", "attachment_id is required")
	}
	result, err := client.GetAttachment(context.Background(), Attachment{ID: attachmentID, Filename: input.Filename, MimeType: input.MimeType})
	if err != nil {
		return AttachmentGetResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	if strings.TrimSpace(input.BlobRef) != "" && len(result.ContentBytes) > 0 {
		blob, err := ctx.Host.BlobWrite(pluginbinding.BlobWriteRequest{
			Ref:       strings.TrimSpace(input.BlobRef),
			Content:   result.ContentBytes,
			Filename:  firstNonEmpty(input.Filename, result.Filename, attachmentID),
			MediaType: result.MimeType,
			Metadata: map[string]string{
				"source":        "jira",
				"attachment_id": attachmentID,
			},
		})
		if err != nil {
			return AttachmentGetResult{}, pluginbinding.Errorf("blob", "%s", err)
		}
		result.Blob = blob
		result.ContentBytes = nil
	}
	return result, nil
}

func (s Service) AttachmentDelete(ctx pluginbinding.Context, input AttachmentDeleteInput) (AttachmentDeleteResult, error) {
	client, _, err := s.client(ctx, input)
	if err != nil {
		return AttachmentDeleteResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	attachmentID := strings.TrimSpace(input.AttachmentID)
	if attachmentID == "" {
		return AttachmentDeleteResult{}, pluginbinding.Fail("bad_input", "attachment_id is required")
	}
	result, err := client.DeleteAttachment(context.Background(), attachmentID)
	if err != nil {
		return AttachmentDeleteResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	return result, nil
}

func (s Service) IssueEdit(ctx pluginbinding.Context, input IssueEditInput) (IssueMutationResult, error) {
	client, _, err := s.client(ctx, input)
	if err != nil {
		return IssueMutationResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	key := strings.TrimSpace(pluginbinding.FirstString(pluginbinding.InputMap(input), "key", "id"))
	if key == "" {
		return IssueMutationResult{}, pluginbinding.Fail("bad_input", "issue key is required")
	}
	descriptionMarkdown, err := uploadMarkdownBlobImages(ctx, client, key, input.DescriptionMarkdown)
	if err != nil {
		return IssueMutationResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	input.DescriptionMarkdown = descriptionMarkdown
	request, err := issueEditRequest(input)
	if err != nil {
		return IssueMutationResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	result, err := client.EditIssue(context.Background(), key, request)
	if err != nil {
		return IssueMutationResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	return result, nil
}

func (s Service) IssueDelete(ctx pluginbinding.Context, input IssueDeleteInput) (IssueMutationResult, error) {
	client, _, err := s.client(ctx, input)
	if err != nil {
		return IssueMutationResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	key := strings.TrimSpace(pluginbinding.FirstString(pluginbinding.InputMap(input), "key", "id"))
	if key == "" {
		return IssueMutationResult{}, pluginbinding.Fail("bad_input", "issue key is required")
	}
	result, err := client.DeleteIssue(context.Background(), key, input.DeleteSubtasks)
	if err != nil {
		return IssueMutationResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	return result, nil
}

func (s Service) UserSearch(ctx pluginbinding.Context, input UserSearchInput) (UserSearchResult, error) {
	client, _, err := s.client(ctx, input)
	if err != nil {
		return UserSearchResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	users, err := client.SearchUsers(context.Background(), userSearchOptions(pluginbinding.InputMap(input), 20))
	if err != nil {
		return UserSearchResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	return pluginbinding.NewListResult(users), nil
}

func (s Service) IssueDatasource(ctx pluginbinding.Context, input IssueSearchInput) (IssueDatasourceResult, error) {
	client, baseURL, err := s.client(ctx, input)
	if err != nil {
		return IssueDatasourceResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	issues, err := client.SearchIssues(context.Background(), issueSearchOptions(pluginbinding.InputMap(input), 20))
	if err != nil {
		return IssueDatasourceResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	records := issueRecords(ctx.DatasourceSource(), baseURL, issues)
	return pluginbinding.NewDatasourceSearchResult(DatasourceIssues, issueSearchDisplayQuery(pluginbinding.InputMap(input)), records), nil
}

func (s Service) UserDatasource(ctx pluginbinding.Context, input UserSearchInput) (UserDatasourceResult, error) {
	client, _, err := s.client(ctx, input)
	if err != nil {
		return UserDatasourceResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	users, err := client.SearchUsers(context.Background(), userSearchOptions(pluginbinding.InputMap(input), 20))
	if err != nil {
		return UserDatasourceResult{}, pluginbinding.Errorf("jira", "%s", err)
	}
	records := userRecords(ctx.DatasourceSource(), users)
	return pluginbinding.NewDatasourceSearchResult(DatasourceUsers, strings.TrimSpace(input.Query), records), nil
}

func (s Service) IssueDatasourceGet(ctx pluginbinding.Context, input pluginbinding.DatasourceGetInput) (pluginbinding.DatasourceGetResult[IssueRecord], error) {
	client, baseURL, err := s.client(ctx, input)
	if err != nil {
		return pluginbinding.DatasourceGetResult[IssueRecord]{}, pluginbinding.Errorf("secret", "%s", err)
	}
	key := strings.TrimSpace(pluginbinding.FirstString(pluginbinding.InputMap(input), "id", "key"))
	if key == "" {
		return pluginbinding.DatasourceGetResult[IssueRecord]{}, pluginbinding.Fail("bad_input", "issue key is required")
	}
	issue, err := client.GetIssue(context.Background(), key)
	if err != nil {
		return pluginbinding.DatasourceGetResult[IssueRecord]{}, pluginbinding.Errorf("jira", "%s", err)
	}
	record, ok := normalizeIssueRecord(ctx.DatasourceSource(), baseURL, issue)
	if !ok {
		return pluginbinding.DatasourceGetResult[IssueRecord]{}, pluginbinding.Fail("not_found", "jira issue not found")
	}
	return pluginbinding.NewDatasourceGetResult(DatasourceIssues, record), nil
}

func (s Service) UserDatasourceGet(ctx pluginbinding.Context, input pluginbinding.DatasourceGetInput) (pluginbinding.DatasourceGetResult[UserRecord], error) {
	client, _, err := s.client(ctx, input)
	if err != nil {
		return pluginbinding.DatasourceGetResult[UserRecord]{}, pluginbinding.Errorf("secret", "%s", err)
	}
	accountID := strings.TrimSpace(pluginbinding.FirstString(pluginbinding.InputMap(input), "id", "account_id", "accountId"))
	if accountID == "" {
		return pluginbinding.DatasourceGetResult[UserRecord]{}, pluginbinding.Fail("bad_input", "user account_id is required")
	}
	user, err := client.GetUser(context.Background(), accountID)
	if err != nil {
		return pluginbinding.DatasourceGetResult[UserRecord]{}, pluginbinding.Errorf("jira", "%s", err)
	}
	record, ok := normalizeUserRecord(ctx.DatasourceSource(), user)
	if !ok {
		return pluginbinding.DatasourceGetResult[UserRecord]{}, pluginbinding.Fail("not_found", "jira user not found")
	}
	return pluginbinding.NewDatasourceGetResult(DatasourceUsers, record), nil
}

func (s Service) IndexBuild(ctx pluginbinding.Context, input IndexBuildInput) (pluginbinding.IndexBuildResult, error) {
	client, baseURL, err := s.client(ctx, input)
	if err != nil {
		return pluginbinding.IndexBuildResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	selector, err := indexBuildSelector(pluginbinding.InputMap(input))
	if err != nil {
		return pluginbinding.IndexBuildResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	values := pluginbinding.InputMap(input)
	issueOptions := issueIndexOptions(values, 100)
	userOptions := userIndexOptions(values, 100)
	return pluginbinding.RunIndexJobs(ctx, selector, "jira",
		pluginbinding.NewIndexJob(DatasourceIssues, EntityIssue, OperationIndexBuild, func() ([]Issue, error) {
			return client.SearchIssues(context.Background(), issueOptions)
		}, func(source pluginbinding.DatasourceSource, issue Issue) (IssueRecord, bool) {
			return normalizeIssueRecord(source, baseURL, issue)
		}, indexMetadata(EntityIssue, map[string]any{"jql": issueOptions.JQL, "query": issueOptions.Query, "project": issueOptions.Project, "status": issueOptions.Status, "limit": issueOptions.Limit})),
		pluginbinding.NewIndexJob(DatasourceUsers, EntityUser, OperationIndexBuild, func() ([]User, error) {
			return client.SearchUsers(context.Background(), userOptions)
		}, normalizeUserRecord, indexMetadata(EntityUser, map[string]any{"query": userOptions.Query, "limit": userOptions.Limit})),
	)
}

func (s Service) Lookup(ctx pluginbinding.Context, input LookupInput) (LookupResult, error) {
	client, baseURL, err := s.client(ctx, input)
	if err != nil {
		return LookupResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	var candidates []pluginbinding.LookupCandidate
	if input.Entity == "" || input.Entity == EntityIssue {
		for _, key := range lookupIssueKeys(input) {
			if issue, err := client.GetIssue(context.Background(), key); err == nil {
				record, ok := normalizeIssueRecord(ctx.DatasourceSource(), baseURL, issue)
				if ok {
					candidates = append(candidates, pluginbinding.NewExactLookupCandidate(ctx.LookupSource(PluginName, DatasourceIssues), record.Entity, record.ID, 1200, []string{"key"}, record, issueLookupValues(record)))
				}
			}
		}
		for _, term := range lookupSearchTerms(input) {
			issues, err := client.SearchIssues(context.Background(), IssueSearchOptions{Query: term, Limit: pluginbinding.LookupLimit(input, 20, 100)})
			if err != nil {
				return LookupResult{}, pluginbinding.Errorf("jira", "%s", err)
			}
			for _, issue := range issues {
				record, ok := normalizeIssueRecord(ctx.DatasourceSource(), baseURL, issue)
				if ok {
					candidates = append(candidates, pluginbinding.NewLookupCandidate(ctx.LookupSource(PluginName, DatasourceIssues), record.Entity, record.ID, record, issueLookupValues(record)))
				}
			}
		}
	}
	if input.Entity == "" || input.Entity == EntityUser {
		for _, term := range lookupSearchTerms(input) {
			users, err := client.SearchUsers(context.Background(), UserSearchOptions{Query: term, Limit: pluginbinding.LookupLimit(input, 20, 100)})
			if err != nil {
				return LookupResult{}, pluginbinding.Errorf("jira", "%s", err)
			}
			for _, user := range users {
				record, ok := normalizeUserRecord(ctx.DatasourceSource(), user)
				if ok {
					candidates = append(candidates, pluginbinding.NewLookupCandidate(ctx.LookupSource(PluginName, DatasourceUsers), record.Entity, record.ID, record, userLookupValues(record)))
				}
			}
		}
	}
	return pluginbinding.NewDatasourceLookupResultFromCandidates(PluginName, input, candidates), nil
}

func issueSearchOptions(input map[string]any, defaultLimit int) IssueSearchOptions {
	return IssueSearchOptions{
		Query:   pluginbinding.StringFromInput(input, "query", "search"),
		JQL:     pluginbinding.StringFromInput(input, "jql"),
		Project: pluginbinding.StringFromInput(input, "project"),
		Status:  pluginbinding.StringFromInput(input, "status"),
		Limit:   pluginbinding.BoundedIntFromInput(input, "limit", defaultLimit, 100),
		Fields:  stringSliceFromInput(input, "fields"),
		OrderBy: pluginbinding.DefaultStringFromInput(input, "updated DESC", "order_by"),
	}
}

func issueIndexOptions(input map[string]any, defaultLimit int) IssueSearchOptions {
	options := issueSearchOptions(map[string]any{
		"query":    pluginbinding.StringFromInput(input, "issue_query", "query"),
		"jql":      pluginbinding.StringFromInput(input, "issue_jql", "jql"),
		"project":  pluginbinding.StringFromInput(input, "project"),
		"status":   pluginbinding.StringFromInput(input, "status"),
		"limit":    pluginbinding.BoundedIntFromInput(input, "issue_limit", defaultLimit, 100),
		"order_by": pluginbinding.DefaultStringFromInput(input, "updated DESC", "order_by"),
	}, defaultLimit)
	options.All = true
	return options
}

func userSearchOptions(input map[string]any, defaultLimit int) UserSearchOptions {
	return UserSearchOptions{
		Query: pluginbinding.StringFromInput(input, "query", "search", "user_query"),
		Limit: pluginbinding.BoundedIntFromInput(input, "limit", defaultLimit, 100),
	}
}

func userIndexOptions(input map[string]any, defaultLimit int) UserSearchOptions {
	options := UserSearchOptions{
		Query: pluginbinding.StringFromInput(input, "user_query", "query"),
		Limit: pluginbinding.BoundedIntFromInput(input, "user_limit", defaultLimit, 100),
		All:   true,
	}
	return options
}

func selectTransition(state IssueTransitionListResult, input IssueTransitionRunInput, allowIntermediate bool, tried map[string]bool) (IssueTransition, bool) {
	if id := strings.TrimSpace(input.TransitionID); id != "" {
		for _, transition := range state.Transitions {
			if !tried[transitionKey(transition)] && strings.EqualFold(strings.TrimSpace(transition.ID), id) {
				return transition, true
			}
		}
		return IssueTransition{}, false
	}
	if name := strings.TrimSpace(input.TransitionName); name != "" {
		for _, transition := range state.Transitions {
			if !tried[transitionKey(transition)] && strings.EqualFold(strings.TrimSpace(transition.Name), name) {
				return transition, true
			}
		}
		return IssueTransition{}, false
	}
	if target := strings.TrimSpace(input.TargetStatus); target != "" {
		for _, transition := range state.Transitions {
			if !tried[transitionKey(transition)] && statusMatches(transition.To, target) {
				return transition, true
			}
		}
		if !allowIntermediate {
			return IssueTransition{}, false
		}
		if transition, ok := bestIntermediateTransition(state, tried); ok {
			return transition, true
		}
	}
	if allowIntermediate && len(state.Transitions) > 0 {
		return bestIntermediateTransition(state, tried)
	}
	return IssueTransition{}, false
}

func statusMatches(status NamedValue, target string) bool {
	target = strings.TrimSpace(target)
	return target != "" && (strings.EqualFold(strings.TrimSpace(status.ID), target) || strings.EqualFold(strings.TrimSpace(status.Name), target))
}

func boundedTransitionSteps(value int) int {
	if value <= 0 {
		return 5
	}
	if value > 20 {
		return 20
	}
	return value
}

func transitionKey(transition IssueTransition) string {
	return strings.TrimSpace(transition.ID) + "\x00" + strings.TrimSpace(transition.To.ID) + "\x00" + strings.TrimSpace(transition.To.Name)
}

func bestIntermediateTransition(state IssueTransitionListResult, tried map[string]bool) (IssueTransition, bool) {
	var best IssueTransition
	bestScore := 1000
	for _, transition := range state.Transitions {
		if tried[transitionKey(transition)] || statusMatches(transition.To, state.CurrentStatus.Name) || statusMatches(transition.To, state.CurrentStatus.ID) {
			continue
		}
		score := intermediateTransitionScore(transition)
		if score < bestScore {
			best = transition
			bestScore = score
		}
	}
	return best, bestScore < 1000
}

func intermediateTransitionScore(transition IssueTransition) int {
	text := strings.ToLower(strings.TrimSpace(transition.Name) + " " + strings.TrimSpace(transition.To.Name))
	for _, term := range []string{"blocked", "block", "hold", "abandoned", "closed", "cancel", "rejected"} {
		if strings.Contains(text, term) {
			return 100
		}
	}
	for _, term := range []string{"progress", "prepare", "preparation", "selected", "todo", "to do", "review", "test", "qa"} {
		if strings.Contains(text, term) {
			return 0
		}
	}
	if strings.Contains(text, "done") || strings.Contains(text, "resolved") {
		return 50
	}
	return 10
}

func transitionSummary(transitions []IssueTransition) string {
	if len(transitions) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(transitions))
	for _, transition := range transitions {
		name := strings.TrimSpace(transition.Name)
		if name == "" {
			name = strings.TrimSpace(transition.ID)
		}
		to := strings.TrimSpace(transition.To.Name)
		if to != "" {
			parts = append(parts, fmt.Sprintf("%s -> %s", name, to))
			continue
		}
		parts = append(parts, name)
	}
	return strings.Join(parts, ", ")
}

func issueCreateRequest(input IssueCreateInput) (IssueCreateRequest, error) {
	projectKey := strings.TrimSpace(input.ProjectKey)
	issueType := strings.TrimSpace(input.IssueType)
	summary := strings.TrimSpace(input.Summary)
	if projectKey == "" || issueType == "" || summary == "" {
		return IssueCreateRequest{}, fmt.Errorf("project_key, issue_type, and summary are required")
	}
	fields := cloneAnyMap(input.Fields)
	fields["project"] = map[string]string{"key": projectKey}
	fields["issuetype"] = map[string]string{"name": issueType}
	fields["summary"] = summary
	applyIssueCommonFields(fields, input.DescriptionMarkdown, input.Labels, input.AssigneeAccountID, input.Priority)
	if reporter := strings.TrimSpace(input.ReporterAccountID); reporter != "" {
		fields["reporter"] = map[string]string{"accountId": reporter}
	}
	if parent := strings.TrimSpace(input.ParentKey); parent != "" {
		fields["parent"] = map[string]string{"key": parent}
	}
	return IssueCreateRequest{Fields: fields, Update: cloneAnyMap(input.Update)}, nil
}

func issueEditRequest(input IssueEditInput) (IssueEditRequest, error) {
	fields := cloneAnyMap(input.Fields)
	if summary := strings.TrimSpace(input.Summary); summary != "" {
		fields["summary"] = summary
	}
	applyIssueCommonFields(fields, input.DescriptionMarkdown, input.Labels, input.AssigneeAccountID, input.Priority)
	update := cloneAnyMap(input.Update)
	if len(fields) == 0 && len(update) == 0 {
		return IssueEditRequest{}, fmt.Errorf("at least one field or update instruction is required")
	}
	return IssueEditRequest{Fields: fields, Update: update}, nil
}

func applyIssueCommonFields(fields map[string]any, descriptionMarkdown string, labels []string, assigneeAccountID, priority string) {
	if strings.TrimSpace(descriptionMarkdown) != "" {
		fields["description"] = md2adf.Convert(descriptionMarkdown)
	}
	if labels := cleanedStrings(labels); len(labels) > 0 {
		fields["labels"] = labels
	}
	if assignee := strings.TrimSpace(assigneeAccountID); assignee != "" {
		fields["assignee"] = map[string]string{"accountId": assignee}
	}
	if priority := strings.TrimSpace(priority); priority != "" {
		fields["priority"] = map[string]string{"name": priority}
	}
}

func commentRequest(bodyMarkdown string) (CommentRequest, error) {
	if strings.TrimSpace(bodyMarkdown) == "" {
		return CommentRequest{}, fmt.Errorf("body_markdown is required")
	}
	return CommentRequest{Body: md2adf.Convert(bodyMarkdown)}, nil
}

func attachmentUploadRequest(ctx pluginbinding.Context, blobRef string, contentBytes []byte, filename, contentType string) (AttachmentUploadRequest, error) {
	blobRef = strings.TrimSpace(blobRef)
	hasBlob := blobRef != ""
	hasBytes := len(contentBytes) > 0
	if hasBlob == hasBytes {
		return AttachmentUploadRequest{}, fmt.Errorf("provide exactly one of blob_ref or content_bytes")
	}
	if hasBlob {
		blob, err := ctx.Host.BlobRead(pluginbinding.BlobReadRequest{Ref: blobRef, MaxBytes: atlassian.MaxAttachmentUploadBytes})
		if err != nil {
			return AttachmentUploadRequest{}, err
		}
		if blob.Truncated {
			return AttachmentUploadRequest{}, fmt.Errorf("blob %s exceeds %d byte cap", blobRef, atlassian.MaxAttachmentUploadBytes)
		}
		contentBytes = append([]byte(nil), blob.Content...)
		filename = firstNonEmpty(filename, blob.Blob.Filename, blobPathFilename(blob.Blob.Path), blob.Blob.Ref)
		contentType = firstNonEmpty(contentType, blob.Blob.MediaType)
	}
	out, err := atlassian.BuildAttachmentUploadRequest(contentBytes, filename, contentType)
	if err != nil {
		return AttachmentUploadRequest{}, err
	}
	return AttachmentUploadRequest{Filename: out.Filename, ContentType: out.ContentType, Data: out.Data}, nil
}

func blobPathFilename(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = strings.TrimRight(path, "/")
	if index := strings.LastIndex(path, "/"); index >= 0 {
		return strings.TrimSpace(path[index+1:])
	}
	return path
}

// markdownImagePattern matches ![alt](url) and ![alt](url "title"). The URL
// stops at the first whitespace so a trailing title block is captured but
// dropped during rewriting; URLs that themselves contain a literal `)` are
// rare and would need a full markdown parser to handle correctly.
var markdownImagePattern = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)

func uploadMarkdownBlobImages(ctx pluginbinding.Context, client Client, key, markdown string) (string, error) {
	if strings.TrimSpace(markdown) == "" || !strings.Contains(markdown, "![") {
		return markdown, nil
	}
	var firstErr error
	out := markdownImagePattern.ReplaceAllStringFunc(markdown, func(match string) string {
		if firstErr != nil {
			return match
		}
		parts := markdownImagePattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		alt := parts[1]
		target := strings.TrimSpace(parts[2])
		if !strings.HasPrefix(target, "blob:") {
			return match
		}
		request, err := attachmentUploadRequest(ctx, target, nil, "", "")
		if err != nil {
			firstErr = err
			return match
		}
		result, err := client.UploadIssueAttachment(context.Background(), key, request)
		if err != nil {
			firstErr = err
			return match
		}
		if len(result.Attachments) == 0 || strings.TrimSpace(result.Attachments[0].Content) == "" {
			return match
		}
		return "![" + alt + "](" + result.Attachments[0].Content + ")"
	})
	return out, firstErr
}

func isRemoteURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && parsed.Scheme != "" && parsed.Host != ""
}

func cloneAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cleanedStrings(input []string) []string {
	out := make([]string, 0, len(input))
	seen := map[string]bool{}
	for _, value := range input {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func issueJQL(input IssueSearchOptions) string {
	if strings.TrimSpace(input.JQL) != "" {
		return strings.TrimSpace(input.JQL)
	}
	var conditions []string
	if input.Project != "" {
		conditions = append(conditions, "project = "+jqlString(input.Project))
	}
	if input.Status != "" {
		conditions = append(conditions, "status = "+jqlString(input.Status))
	}
	if input.Query != "" {
		conditions = append(conditions, "text ~ "+jqlString(input.Query))
	}
	if len(conditions) == 0 {
		return "order by " + defaultOrderBy(input.OrderBy)
	}
	return strings.Join(conditions, " and ") + " order by " + defaultOrderBy(input.OrderBy)
}

func defaultOrderBy(value string) string {
	if strings.TrimSpace(value) == "" {
		return "updated DESC"
	}
	return strings.TrimSpace(value)
}

func jqlString(value string) string {
	escaped := strings.ReplaceAll(strings.TrimSpace(value), `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

func issueSearchDisplayQuery(input map[string]any) string {
	if jql := pluginbinding.StringFromInput(input, "jql"); jql != "" {
		return jql
	}
	return pluginbinding.StringFromInput(input, "query", "search")
}

func issueRecords(source pluginbinding.DatasourceSource, baseURL string, issues []Issue) []IssueRecord {
	records := make([]IssueRecord, 0, len(issues))
	for _, issue := range issues {
		record, ok := normalizeIssueRecord(source, baseURL, issue)
		if ok {
			records = append(records, record)
		}
	}
	return records
}

func userRecords(source pluginbinding.DatasourceSource, users []User) []UserRecord {
	records := make([]UserRecord, 0, len(users))
	for _, user := range users {
		record, ok := normalizeUserRecord(source, user)
		if ok {
			records = append(records, record)
		}
	}
	return records
}

func indexBuildSelector(input map[string]any) (pluginbinding.IndexSelector, error) {
	known := map[string]string{
		DatasourceIssues: DatasourceIssues,
		EntityIssue:      DatasourceIssues,
		"issue":          DatasourceIssues,
		"issues":         DatasourceIssues,
		DatasourceUsers:  DatasourceUsers,
		EntityUser:       DatasourceUsers,
		"user":           DatasourceUsers,
		"users":          DatasourceUsers,
	}
	return pluginbinding.NewIndexSelector(input, known, "Jira")
}

func indexMetadata(entity string, values map[string]any) map[string]any {
	return pluginbinding.IndexBuildMetadata(entity, OperationIndexBuild, values)
}

func lookupSearchTerms(input LookupInput) []string {
	return pluginbinding.FilterLookupTerms(input, 3, func(term string) bool {
		return !strings.Contains(term, "://") && !strings.Contains(term, "/browse/")
	})
}

var issueKeyPattern = regexp.MustCompile(`(?i)\b[A-Z][A-Z0-9]+-\d+\b`)

func lookupIssueKeys(input LookupInput) []string {
	seen := map[string]bool{}
	var out []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		out = append(out, value)
	}
	for _, term := range pluginbinding.LookupTerms(input) {
		if parsed, err := url.Parse(strings.TrimSpace(term)); err == nil && parsed.Host != "" {
			path := strings.Trim(parsed.Path, "/")
			if _, rest, ok := strings.Cut(path, "browse/"); ok {
				if key := strings.Trim(strings.Split(rest, "/")[0], "/"); key != "" {
					if decoded, err := url.PathUnescape(key); err == nil {
						key = decoded
					}
					add(strings.ToUpper(key))
				}
			}
		}
		for _, match := range issueKeyPattern.FindAllString(term, -1) {
			add(strings.ToUpper(match))
		}
	}
	return out
}

func issueLookupValues(record IssueRecord) map[string]string {
	return map[string]string{
		"id":                 record.ID,
		"key":                record.Key,
		"title":              record.Title,
		"record.summary":     record.Summary,
		"record.project_key": record.ProjectKey,
		"record.status":      record.Status,
		"record.web_url":     record.WebURL,
		"record.assignee":    record.AssigneeDisplayName,
		"record.reporter":    record.ReporterDisplayName,
		"record.issue_id":    record.IssueID,
		"record.issue_type":  record.IssueType,
		"record.updated":     record.Updated,
	}
}

func userLookupValues(record UserRecord) map[string]string {
	return map[string]string{
		"id":                  record.ID,
		"record.account_id":   record.AccountID,
		"record.display_name": record.DisplayName,
		"record.email":        record.EmailAddress,
		"record.self":         record.Self,
	}
}

func stringSliceFromInput(input map[string]any, key string) []string {
	value, ok := input[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		var out []string
		for _, item := range typed {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" {
				out = append(out, text)
			}
		}
		return out
	case string:
		var out []string
		for _, part := range strings.Split(typed, ",") {
			if text := strings.TrimSpace(part); text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}
