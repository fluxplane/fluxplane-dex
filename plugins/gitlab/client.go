package gitlab

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	gitlabapi "gitlab.com/gitlab-org/api/client-go/v2"
)

type Client interface {
	CurrentUser() (User, error)
	ListProjects(ProjectListOptions) ([]Project, error)
	GetProject(any) (Project, error)
	ListUsers(UserListOptions) ([]User, error)
	ListGroups(GroupListOptions) ([]Group, error)
	ListIssues(IssueListOptions) ([]Issue, error)
	ListMergeRequests(MergeRequestListOptions) ([]MergeRequest, error)
	GetMergeRequest(any, int64) (MergeRequest, error)
	CreateMergeRequest(any, MergeRequestCreateOptions) (MergeRequest, error)
	ApproveMergeRequest(any, int64, MergeRequestApproveOptions) (MergeRequestApproval, error)
	MergeMergeRequest(any, int64, MergeRequestMergeOptions) (MergeRequest, error)
	CreateRepositoryTag(any, RepositoryTagCreateOptions) (RepositoryTag, error)
	CreateBranch(any, BranchCreateOptions) (Branch, error)
	DeleteBranch(any, string) error
	DeleteMergedBranches(any) error
	CreateRepositoryFile(any, RepoFileCreateOptions) (RepoFile, error)
	UpdateRepositoryFile(any, RepoFileUpdateOptions) (RepoFile, error)
	DeleteRepositoryFile(any, RepoFileDeleteOptions) error
	CreateCommit(any, CommitCreateOptions) (Commit, error)
	CreateCIVariable(any, CIVariableCreateOptions) (CIVariable, error)
	UpdateCIVariable(any, string, CIVariableUpdateOptions) (CIVariable, error)
	DeleteCIVariable(any, string, CIVariableDeleteOptions) error
	CreatePipeline(any, PipelineCreateOptions) (Pipeline, error)
	RetryPipeline(any, int64) (Pipeline, error)
	CancelPipeline(any, int64) (Pipeline, error)
	CreateSnippet(SnippetCreateOptions) (Snippet, error)
	DeleteSnippet(int64) error
}

type ClientFactory func(pluginbinding.Context) (Client, error)

const gitLabHostHTTPBaseURL = "https://gitlab.endpoint.local"

func NewLiveClient(ctx pluginbinding.Context) (Client, error) {
	endpointRef, err := gitLabEndpointRef(ctx)
	if err != nil {
		return nil, err
	}
	client, err := gitlabapi.NewClient("",
		gitlabapi.WithBaseURL(gitLabHostHTTPBaseURL),
		gitlabapi.WithHTTPClient(pluginbinding.HostHTTPClient(ctx.Host,
			pluginbinding.HostHTTPClientEndpointRef(endpointRef),
			pluginbinding.HostHTTPClientAuth(pluginbinding.HTTPAuthRequest{BearerTokenPurpose: AuthPurposeAccessToken}),
			pluginbinding.HostHTTPClientTimeout(30000),
			pluginbinding.HostHTTPClientMaxBytes(32*1024*1024),
		)),
	)
	if err != nil {
		return nil, err
	}
	return liveClient{client: client}, nil
}

func gitLabEndpointRef(ctx pluginbinding.Context) (string, error) {
	for _, raw := range []json.RawMessage{ctx.Call.Input, ctx.Request.Payload} {
		endpointRef := endpointRefFromRaw(raw)
		if endpointRef != "" {
			return endpointRef, nil
		}
	}
	return "", fmt.Errorf("endpoint_ref is required")
}

func endpointRefFromRaw(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var input struct {
		EndpointRef string `json:"endpoint_ref"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return ""
	}
	return strings.TrimSpace(input.EndpointRef)
}

type liveClient struct {
	client *gitlabapi.Client
}

func (c liveClient) CurrentUser() (User, error) {
	user, _, err := c.client.Users.CurrentUser()
	if err != nil {
		return User{}, err
	}
	return userFromAPI(user), nil
}

func (c liveClient) ListProjects(input ProjectListOptions) ([]Project, error) {
	opt := projectListAPIOptions(input)
	if input.All {
		opt.PerPage = 100
	}
	var out []Project
	for {
		projects, resp, err := c.client.Projects.ListProjects(opt)
		if err != nil {
			return nil, err
		}
		for _, project := range projects {
			out = append(out, projectFromAPI(project))
		}
		if !input.All || resp == nil || resp.NextPage == 0 {
			return out, nil
		}
		opt.Page = resp.NextPage
	}
}

func projectListAPIOptions(input ProjectListOptions) *gitlabapi.ListProjectsOptions {
	opt := &gitlabapi.ListProjectsOptions{}
	opt.PerPage = int64(clampProjectPageSize(input.Limit, 20))
	opt.Page = 1
	if strings.TrimSpace(input.Search) != "" {
		search := strings.TrimSpace(input.Search)
		opt.Search = &search
	}
	if strings.TrimSpace(input.OrderBy) != "" {
		orderBy := strings.TrimSpace(input.OrderBy)
		opt.OrderBy = &orderBy
	}
	if strings.TrimSpace(input.Sort) != "" {
		sort := strings.TrimSpace(input.Sort)
		opt.Sort = &sort
	}
	membership := true
	if input.Membership != nil {
		membership = *input.Membership
	}
	opt.Membership = &membership
	return opt
}

func clampProjectPageSize(limit, fallback int) int {
	if limit <= 0 {
		limit = fallback
	}
	if limit > 100 {
		limit = 100
	}
	return limit
}

func (c liveClient) GetProject(id any) (Project, error) {
	project, _, err := c.client.Projects.GetProject(id, nil)
	if err != nil {
		return Project{}, err
	}
	return projectFromAPI(project), nil
}

func (c liveClient) ListUsers(input UserListOptions) ([]User, error) {
	opt := userListAPIOptions(input)
	if input.All {
		opt.PerPage = 100
	}
	var out []User
	for {
		users, resp, err := c.client.Users.ListUsers(opt)
		if err != nil {
			return nil, err
		}
		for _, user := range users {
			out = append(out, userFromAPI(user))
		}
		if !input.All || resp == nil || resp.NextPage == 0 {
			return out, nil
		}
		opt.Page = resp.NextPage
	}
}

func userListAPIOptions(input UserListOptions) *gitlabapi.ListUsersOptions {
	opt := &gitlabapi.ListUsersOptions{}
	opt.PerPage = int64(clampProjectPageSize(input.Limit, 20))
	opt.Page = 1
	if strings.TrimSpace(input.Search) != "" {
		search := strings.TrimSpace(input.Search)
		opt.Search = &search
	}
	if input.Active != nil {
		opt.Active = input.Active
	}
	return opt
}

func (c liveClient) ListGroups(input GroupListOptions) ([]Group, error) {
	opt := groupListAPIOptions(input)
	if input.All {
		opt.PerPage = 100
	}
	var out []Group
	for {
		groups, resp, err := c.client.Groups.ListGroups(opt)
		if err != nil {
			return nil, err
		}
		for _, group := range groups {
			out = append(out, groupFromAPI(group))
		}
		if !input.All || resp == nil || resp.NextPage == 0 {
			return out, nil
		}
		opt.Page = resp.NextPage
	}
}

func groupListAPIOptions(input GroupListOptions) *gitlabapi.ListGroupsOptions {
	opt := &gitlabapi.ListGroupsOptions{}
	opt.PerPage = int64(clampProjectPageSize(input.Limit, 20))
	opt.Page = 1
	if strings.TrimSpace(input.Search) != "" {
		search := strings.TrimSpace(input.Search)
		opt.Search = &search
	}
	if strings.TrimSpace(input.OrderBy) != "" {
		orderBy := strings.TrimSpace(input.OrderBy)
		opt.OrderBy = &orderBy
	}
	if strings.TrimSpace(input.Sort) != "" {
		sort := strings.TrimSpace(input.Sort)
		opt.Sort = &sort
	}
	if input.Active != nil {
		opt.Active = input.Active
	}
	if input.TopLevel != nil {
		opt.TopLevelOnly = input.TopLevel
	}
	if input.AllVisible != nil {
		opt.AllAvailable = input.AllVisible
	}
	return opt
}

func (c liveClient) ListIssues(input IssueListOptions) ([]Issue, error) {
	opt := issueListAPIOptions(input)
	if input.All {
		opt.PerPage = 100
	}
	var out []Issue
	for {
		issues, resp, err := c.client.Issues.ListIssues(opt)
		if err != nil {
			return nil, err
		}
		for _, issue := range issues {
			out = append(out, issueFromAPI(issue))
		}
		if !input.All || resp == nil || resp.NextPage == 0 {
			return out, nil
		}
		opt.Page = resp.NextPage
	}
}

func issueListAPIOptions(input IssueListOptions) *gitlabapi.ListIssuesOptions {
	opt := &gitlabapi.ListIssuesOptions{}
	opt.PerPage = int64(clampProjectPageSize(input.Limit, 20))
	opt.Page = 1
	if strings.TrimSpace(input.Search) != "" {
		search := strings.TrimSpace(input.Search)
		opt.Search = &search
	}
	if strings.TrimSpace(input.State) != "" {
		state := strings.TrimSpace(input.State)
		opt.State = &state
	}
	if strings.TrimSpace(input.OrderBy) != "" {
		orderBy := strings.TrimSpace(input.OrderBy)
		opt.OrderBy = &orderBy
	}
	if strings.TrimSpace(input.Sort) != "" {
		sort := strings.TrimSpace(input.Sort)
		opt.Sort = &sort
	}
	return opt
}

func (c liveClient) ListMergeRequests(input MergeRequestListOptions) ([]MergeRequest, error) {
	if strings.TrimSpace(input.Project) != "" {
		opt := projectMergeRequestListOptions(input)
		if input.All {
			opt.PerPage = 100
		}
		var out []MergeRequest
		for {
			mrs, resp, err := c.client.MergeRequests.ListProjectMergeRequests(projectID(input.Project), opt)
			if err != nil {
				return nil, err
			}
			out = append(out, mergeRequestsFromAPI(mrs)...)
			if !input.All || resp == nil || resp.NextPage == 0 {
				return out, nil
			}
			opt.Page = resp.NextPage
		}
	}
	opt := mergeRequestListOptions(input)
	if input.All {
		opt.PerPage = 100
	}
	var out []MergeRequest
	for {
		mrs, resp, err := c.client.MergeRequests.ListMergeRequests(opt)
		if err != nil {
			return nil, err
		}
		out = append(out, mergeRequestsFromAPI(mrs)...)
		if !input.All || resp == nil || resp.NextPage == 0 {
			return out, nil
		}
		opt.Page = resp.NextPage
	}
}

func (c liveClient) GetMergeRequest(project any, iid int64) (MergeRequest, error) {
	mr, _, err := c.client.MergeRequests.GetMergeRequest(project, iid, nil)
	if err != nil {
		return MergeRequest{}, err
	}
	return mergeRequestFromAPI(mr), nil
}

func (c liveClient) CreateMergeRequest(project any, input MergeRequestCreateOptions) (MergeRequest, error) {
	opt := createMergeRequestOptions(input)
	mr, _, err := c.client.MergeRequests.CreateMergeRequest(project, opt)
	if err != nil {
		return MergeRequest{}, err
	}
	return mergeRequestFromAPI(mr), nil
}

func (c liveClient) ApproveMergeRequest(project any, iid int64, input MergeRequestApproveOptions) (MergeRequestApproval, error) {
	opt := &gitlabapi.ApproveMergeRequestOptions{}
	if strings.TrimSpace(input.SHA) != "" {
		sha := strings.TrimSpace(input.SHA)
		opt.SHA = &sha
	}
	approval, _, err := c.client.MergeRequestApprovals.ApproveMergeRequest(project, iid, opt)
	if err != nil {
		return MergeRequestApproval{}, err
	}
	return mergeRequestApprovalFromAPI(approval), nil
}

func (c liveClient) MergeMergeRequest(project any, iid int64, input MergeRequestMergeOptions) (MergeRequest, error) {
	opt := acceptMergeRequestOptions(input)
	mr, _, err := c.client.MergeRequests.AcceptMergeRequest(project, iid, opt)
	if err != nil {
		return MergeRequest{}, err
	}
	return mergeRequestFromAPI(mr), nil
}

func (c liveClient) CreateRepositoryTag(project any, input RepositoryTagCreateOptions) (RepositoryTag, error) {
	opt := &gitlabapi.CreateTagOptions{}
	tagName := strings.TrimSpace(input.TagName)
	ref := strings.TrimSpace(input.Ref)
	message := strings.TrimSpace(input.Message)
	opt.TagName = &tagName
	opt.Ref = &ref
	if message != "" {
		opt.Message = &message
	}
	tag, _, err := c.client.Tags.CreateTag(project, opt)
	if err != nil {
		return RepositoryTag{}, err
	}
	return repositoryTagFromAPI(tag), nil
}

func (c liveClient) CreateBranch(project any, input BranchCreateOptions) (Branch, error) {
	opt := &gitlabapi.CreateBranchOptions{
		Branch: gitlabapi.Ptr(input.Branch),
		Ref:    gitlabapi.Ptr(input.Ref),
	}
	branch, _, err := c.client.Branches.CreateBranch(project, opt)
	if err != nil {
		return Branch{}, err
	}
	return branchFromAPI(branch), nil
}

func (c liveClient) DeleteBranch(project any, branch string) error {
	_, err := c.client.Branches.DeleteBranch(project, branch)
	return err
}

func (c liveClient) DeleteMergedBranches(project any) error {
	_, err := c.client.Branches.DeleteMergedBranches(project)
	return err
}

func (c liveClient) CreateRepositoryFile(project any, input RepoFileCreateOptions) (RepoFile, error) {
	opt := &gitlabapi.CreateFileOptions{
		Branch:        gitlabapi.Ptr(input.Branch),
		Content:       gitlabapi.Ptr(input.Content),
		CommitMessage: gitlabapi.Ptr(input.CommitMessage),
	}
	if input.ExecuteFilemode != nil {
		opt.ExecuteFilemode = input.ExecuteFilemode
	}
	if input.StartBranch != "" {
		opt.StartBranch = gitlabapi.Ptr(input.StartBranch)
	}
	if input.Encoding != "" {
		opt.Encoding = gitlabapi.Ptr(input.Encoding)
	}
	if input.AuthorEmail != "" {
		opt.AuthorEmail = gitlabapi.Ptr(input.AuthorEmail)
	}
	if input.AuthorName != "" {
		opt.AuthorName = gitlabapi.Ptr(input.AuthorName)
	}
	info, _, err := c.client.RepositoryFiles.CreateFile(project, input.FilePath, opt)
	if err != nil {
		return RepoFile{}, err
	}
	return repoFileFromAPI(info), nil
}

func (c liveClient) UpdateRepositoryFile(project any, input RepoFileUpdateOptions) (RepoFile, error) {
	opt := &gitlabapi.UpdateFileOptions{
		Branch:        gitlabapi.Ptr(input.Branch),
		Content:       gitlabapi.Ptr(input.Content),
		CommitMessage: gitlabapi.Ptr(input.CommitMessage),
	}
	if input.ExecuteFilemode != nil {
		opt.ExecuteFilemode = input.ExecuteFilemode
	}
	if input.StartBranch != "" {
		opt.StartBranch = gitlabapi.Ptr(input.StartBranch)
	}
	if input.Encoding != "" {
		opt.Encoding = gitlabapi.Ptr(input.Encoding)
	}
	if input.AuthorEmail != "" {
		opt.AuthorEmail = gitlabapi.Ptr(input.AuthorEmail)
	}
	if input.AuthorName != "" {
		opt.AuthorName = gitlabapi.Ptr(input.AuthorName)
	}
	if input.LastCommitID != "" {
		opt.LastCommitID = gitlabapi.Ptr(input.LastCommitID)
	}
	info, _, err := c.client.RepositoryFiles.UpdateFile(project, input.FilePath, opt)
	if err != nil {
		return RepoFile{}, err
	}
	return repoFileFromAPI(info), nil
}

func (c liveClient) DeleteRepositoryFile(project any, input RepoFileDeleteOptions) error {
	opt := &gitlabapi.DeleteFileOptions{
		Branch:        gitlabapi.Ptr(input.Branch),
		CommitMessage: gitlabapi.Ptr(input.CommitMessage),
	}
	if input.StartBranch != "" {
		opt.StartBranch = gitlabapi.Ptr(input.StartBranch)
	}
	if input.AuthorEmail != "" {
		opt.AuthorEmail = gitlabapi.Ptr(input.AuthorEmail)
	}
	if input.AuthorName != "" {
		opt.AuthorName = gitlabapi.Ptr(input.AuthorName)
	}
	if input.LastCommitID != "" {
		opt.LastCommitID = gitlabapi.Ptr(input.LastCommitID)
	}
	_, err := c.client.RepositoryFiles.DeleteFile(project, input.FilePath, opt)
	return err
}

func (c liveClient) CreateCommit(project any, input CommitCreateOptions) (Commit, error) {
	opt := &gitlabapi.CreateCommitOptions{
		Branch:        gitlabapi.Ptr(input.Branch),
		CommitMessage: gitlabapi.Ptr(input.CommitMessage),
		Actions:       make([]*gitlabapi.CommitActionOptions, 0, len(input.Actions)),
	}
	if input.Force != nil {
		opt.Force = input.Force
	}
	if input.StartBranch != "" {
		opt.StartBranch = gitlabapi.Ptr(input.StartBranch)
	}
	if input.StartSHA != "" {
		opt.StartSHA = gitlabapi.Ptr(input.StartSHA)
	}
	if input.StartProject != "" {
		opt.StartProject = gitlabapi.Ptr(input.StartProject)
	}
	if input.AuthorEmail != "" {
		opt.AuthorEmail = gitlabapi.Ptr(input.AuthorEmail)
	}
	if input.AuthorName != "" {
		opt.AuthorName = gitlabapi.Ptr(input.AuthorName)
	}
	for _, action := range input.Actions {
		opt.Actions = append(opt.Actions, commitActionToAPI(action))
	}
	commit, _, err := c.client.Commits.CreateCommit(project, opt)
	if err != nil {
		return Commit{}, err
	}
	return commitFromAPI(commit), nil
}

func (c liveClient) CreateCIVariable(project any, input CIVariableCreateOptions) (CIVariable, error) {
	opt := &gitlabapi.CreateProjectVariableOptions{
		Key:   gitlabapi.Ptr(input.Key),
		Value: gitlabapi.Ptr(input.Value),
	}
	if input.Masked != nil {
		opt.Masked = input.Masked
	}
	if input.MaskedAndHidden != nil {
		opt.MaskedAndHidden = input.MaskedAndHidden
	}
	if input.Protected != nil {
		opt.Protected = input.Protected
	}
	if input.Raw != nil {
		opt.Raw = input.Raw
	}
	if input.Description != "" {
		opt.Description = gitlabapi.Ptr(input.Description)
	}
	if input.EnvironmentScope != "" {
		opt.EnvironmentScope = gitlabapi.Ptr(input.EnvironmentScope)
	}
	if typ := variableTypeValue(input.VariableType); typ != "" {
		opt.VariableType = &typ
	}
	variable, _, err := c.client.ProjectVariables.CreateVariable(project, opt)
	if err != nil {
		return CIVariable{}, err
	}
	return ciVariableFromAPI(variable), nil
}

func (c liveClient) UpdateCIVariable(project any, key string, input CIVariableUpdateOptions) (CIVariable, error) {
	opt := &gitlabapi.UpdateProjectVariableOptions{
		Value:  gitlabapi.Ptr(input.Value),
		Filter: variableFilter(input.EnvironmentScope),
	}
	if input.Masked != nil {
		opt.Masked = input.Masked
	}
	if input.Protected != nil {
		opt.Protected = input.Protected
	}
	if input.Raw != nil {
		opt.Raw = input.Raw
	}
	if input.Description != "" {
		opt.Description = gitlabapi.Ptr(input.Description)
	}
	if input.EnvironmentScope != "" {
		opt.EnvironmentScope = gitlabapi.Ptr(input.EnvironmentScope)
	}
	if typ := variableTypeValue(input.VariableType); typ != "" {
		opt.VariableType = &typ
	}
	variable, _, err := c.client.ProjectVariables.UpdateVariable(project, key, opt)
	if err != nil {
		return CIVariable{}, err
	}
	return ciVariableFromAPI(variable), nil
}

func (c liveClient) DeleteCIVariable(project any, key string, input CIVariableDeleteOptions) error {
	opt := &gitlabapi.RemoveProjectVariableOptions{Filter: variableFilter(input.EnvironmentScope)}
	_, err := c.client.ProjectVariables.RemoveVariable(project, key, opt)
	return err
}

func (c liveClient) CreatePipeline(project any, input PipelineCreateOptions) (Pipeline, error) {
	opt := &gitlabapi.CreatePipelineOptions{Ref: gitlabapi.Ptr(input.Ref)}
	if len(input.Variables) > 0 {
		variables := make([]*gitlabapi.PipelineVariableOptions, 0, len(input.Variables))
		for _, variable := range input.Variables {
			item := &gitlabapi.PipelineVariableOptions{
				Key:   gitlabapi.Ptr(variable.Key),
				Value: gitlabapi.Ptr(variable.Value),
			}
			if typ := variableTypeValue(variable.VariableType); typ != "" {
				item.VariableType = &typ
			}
			variables = append(variables, item)
		}
		opt.Variables = &variables
	}
	pipeline, _, err := c.client.Pipelines.CreatePipeline(project, opt)
	if err != nil {
		return Pipeline{}, err
	}
	return pipelineFromAPI(pipeline), nil
}

func (c liveClient) RetryPipeline(project any, pipeline int64) (Pipeline, error) {
	out, _, err := c.client.Pipelines.RetryPipelineBuild(project, pipeline)
	if err != nil {
		return Pipeline{}, err
	}
	return pipelineFromAPI(out), nil
}

func (c liveClient) CancelPipeline(project any, pipeline int64) (Pipeline, error) {
	out, _, err := c.client.Pipelines.CancelPipelineBuild(project, pipeline)
	if err != nil {
		return Pipeline{}, err
	}
	return pipelineFromAPI(out), nil
}

func (c liveClient) CreateSnippet(input SnippetCreateOptions) (Snippet, error) {
	files := make([]*gitlabapi.CreateSnippetFileOptions, 0, len(input.Files))
	for _, file := range input.Files {
		files = append(files, &gitlabapi.CreateSnippetFileOptions{
			FilePath: gitlabapi.Ptr(file.FilePath),
			Content:  gitlabapi.Ptr(file.Content),
		})
	}
	opt := &gitlabapi.CreateSnippetOptions{
		Title: gitlabapi.Ptr(input.Title),
		Files: &files,
	}
	if input.Description != "" {
		opt.Description = gitlabapi.Ptr(input.Description)
	}
	visibility := gitlabapi.VisibilityValue(input.Visibility)
	opt.Visibility = &visibility
	snippet, _, err := c.client.Snippets.CreateSnippet(opt)
	if err != nil {
		return Snippet{}, err
	}
	return snippetFromAPI(snippet), nil
}

func (c liveClient) DeleteSnippet(snippetID int64) error {
	_, err := c.client.Snippets.DeleteSnippet(snippetID)
	return err
}

func variableFilter(scope string) *gitlabapi.VariableFilter {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return nil
	}
	return &gitlabapi.VariableFilter{EnvironmentScope: scope}
}

func variableTypeValue(value string) gitlabapi.VariableTypeValue {
	switch strings.TrimSpace(value) {
	case "env_var":
		return gitlabapi.EnvVariableType
	case "file":
		return gitlabapi.FileVariableType
	default:
		return ""
	}
}

func commitActionToAPI(action CommitFileAction) *gitlabapi.CommitActionOptions {
	value := gitlabapi.FileActionValue(strings.ToLower(action.Action))
	opt := &gitlabapi.CommitActionOptions{
		Action:   &value,
		FilePath: gitlabapi.Ptr(action.FilePath),
	}
	if action.ExecuteFilemode != nil {
		opt.ExecuteFilemode = action.ExecuteFilemode
	}
	if action.PreviousPath != "" {
		opt.PreviousPath = gitlabapi.Ptr(action.PreviousPath)
	}
	if action.Content != "" {
		opt.Content = gitlabapi.Ptr(action.Content)
	}
	if action.Encoding != "" {
		opt.Encoding = gitlabapi.Ptr(action.Encoding)
	}
	if action.LastCommitID != "" {
		opt.LastCommitID = gitlabapi.Ptr(action.LastCommitID)
	}
	return opt
}

func branchFromAPI(branch *gitlabapi.Branch) Branch {
	if branch == nil {
		return Branch{}
	}
	return Branch{
		Name:               branch.Name,
		WebURL:             branch.WebURL,
		Merged:             branch.Merged,
		Protected:          branch.Protected,
		Default:            branch.Default,
		CanPush:            branch.CanPush,
		DevelopersCanPush:  branch.DevelopersCanPush,
		DevelopersCanMerge: branch.DevelopersCanMerge,
	}
}

func repoFileFromAPI(info *gitlabapi.FileInfo) RepoFile {
	if info == nil {
		return RepoFile{}
	}
	return RepoFile{FilePath: info.FilePath, Branch: info.Branch}
}

func commitFromAPI(commit *gitlabapi.Commit) Commit {
	if commit == nil {
		return Commit{}
	}
	return Commit{
		ID:            commit.ID,
		ShortID:       commit.ShortID,
		Title:         commit.Title,
		Message:       commit.Message,
		AuthorName:    commit.AuthorName,
		AuthorEmail:   commit.AuthorEmail,
		CreatedAt:     formatTime(commit.CreatedAt),
		CommittedDate: formatTime(commit.CommittedDate),
		WebURL:        commit.WebURL,
	}
}

func ciVariableFromAPI(variable *gitlabapi.ProjectVariable) CIVariable {
	if variable == nil {
		return CIVariable{}
	}
	return CIVariable{
		Key:              variable.Key,
		Value:            variable.Value,
		VariableType:     string(variable.VariableType),
		EnvironmentScope: variable.EnvironmentScope,
		Description:      variable.Description,
		Protected:        variable.Protected,
		Masked:           variable.Masked,
		Raw:              variable.Raw,
	}
}

func pipelineFromAPI(pipeline *gitlabapi.Pipeline) Pipeline {
	if pipeline == nil {
		return Pipeline{}
	}
	return Pipeline{
		ID:         pipeline.ID,
		ProjectID:  pipeline.ProjectID,
		Status:     pipeline.Status,
		Ref:        pipeline.Ref,
		SHA:        pipeline.SHA,
		WebURL:     pipeline.WebURL,
		Source:     string(pipeline.Source),
		CreatedAt:  formatTime(pipeline.CreatedAt),
		UpdatedAt:  formatTime(pipeline.UpdatedAt),
		StartedAt:  formatTime(pipeline.StartedAt),
		FinishedAt: formatTime(pipeline.FinishedAt),
		Duration:   pipeline.Duration,
	}
}

func snippetFromAPI(snippet *gitlabapi.Snippet) Snippet {
	if snippet == nil {
		return Snippet{}
	}
	return Snippet{
		ID:          snippet.ID,
		Title:       snippet.Title,
		Description: snippet.Description,
		Visibility:  snippet.Visibility,
		WebURL:      snippet.WebURL,
		RawURL:      snippet.RawURL,
		CreatedAt:   formatTime(snippet.CreatedAt),
		UpdatedAt:   formatTime(snippet.UpdatedAt),
	}
}

func mergeRequestListOptions(input MergeRequestListOptions) *gitlabapi.ListMergeRequestsOptions {
	opt := &gitlabapi.ListMergeRequestsOptions{}
	applyMergeRequestListOptions(&opt.ListOptions, &opt.State, &opt.Search, &opt.OrderBy, &opt.Sort, input)
	return opt
}

func projectMergeRequestListOptions(input MergeRequestListOptions) *gitlabapi.ListProjectMergeRequestsOptions {
	opt := &gitlabapi.ListProjectMergeRequestsOptions{}
	applyMergeRequestListOptions(&opt.ListOptions, &opt.State, &opt.Search, &opt.OrderBy, &opt.Sort, input)
	return opt
}

func applyMergeRequestListOptions(list *gitlabapi.ListOptions, stateField, searchField, orderByField, sortField **string, input MergeRequestListOptions) {
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	list.PerPage = int64(limit)
	list.Page = 1
	if strings.TrimSpace(input.State) != "" {
		state := strings.TrimSpace(input.State)
		*stateField = &state
	}
	if strings.TrimSpace(input.Search) != "" {
		search := strings.TrimSpace(input.Search)
		*searchField = &search
	}
	if strings.TrimSpace(input.OrderBy) != "" {
		orderBy := strings.TrimSpace(input.OrderBy)
		*orderByField = &orderBy
	}
	if strings.TrimSpace(input.Sort) != "" {
		sort := strings.TrimSpace(input.Sort)
		*sortField = &sort
	}
}

func createMergeRequestOptions(input MergeRequestCreateOptions) *gitlabapi.CreateMergeRequestOptions {
	opt := &gitlabapi.CreateMergeRequestOptions{}
	title := strings.TrimSpace(input.Title)
	sourceBranch := strings.TrimSpace(input.SourceBranch)
	targetBranch := strings.TrimSpace(input.TargetBranch)
	opt.Title = &title
	opt.SourceBranch = &sourceBranch
	opt.TargetBranch = &targetBranch
	if strings.TrimSpace(input.Description) != "" {
		description := strings.TrimSpace(input.Description)
		opt.Description = &description
	}
	if len(input.Labels) > 0 {
		labels := gitlabapi.LabelOptions(input.Labels)
		opt.Labels = &labels
	}
	if input.AssigneeID > 0 {
		opt.AssigneeID = &input.AssigneeID
	}
	if len(input.AssigneeIDs) > 0 {
		opt.AssigneeIDs = &input.AssigneeIDs
	}
	if len(input.ReviewerIDs) > 0 {
		opt.ReviewerIDs = &input.ReviewerIDs
	}
	if input.TargetProjectID > 0 {
		opt.TargetProjectID = &input.TargetProjectID
	}
	if input.MilestoneID > 0 {
		opt.MilestoneID = &input.MilestoneID
	}
	if input.RemoveSourceBranch != nil {
		opt.RemoveSourceBranch = input.RemoveSourceBranch
	}
	if input.Squash != nil {
		opt.Squash = input.Squash
	}
	if input.AllowCollaboration != nil {
		opt.AllowCollaboration = input.AllowCollaboration
	}
	return opt
}

func acceptMergeRequestOptions(input MergeRequestMergeOptions) *gitlabapi.AcceptMergeRequestOptions {
	opt := &gitlabapi.AcceptMergeRequestOptions{}
	if input.AutoMerge != nil {
		opt.AutoMerge = input.AutoMerge
	}
	if strings.TrimSpace(input.MergeCommitMessage) != "" {
		message := strings.TrimSpace(input.MergeCommitMessage)
		opt.MergeCommitMessage = &message
	}
	if strings.TrimSpace(input.SquashCommitMessage) != "" {
		message := strings.TrimSpace(input.SquashCommitMessage)
		opt.SquashCommitMessage = &message
	}
	if input.Squash != nil {
		opt.Squash = input.Squash
	}
	if input.ShouldRemoveSourceBranch != nil {
		opt.ShouldRemoveSourceBranch = input.ShouldRemoveSourceBranch
	}
	if strings.TrimSpace(input.SHA) != "" {
		sha := strings.TrimSpace(input.SHA)
		opt.SHA = &sha
	}
	return opt
}

func userFromAPI(user *gitlabapi.User) User {
	if user == nil {
		return User{}
	}
	return User{ID: user.ID, Username: user.Username, Name: user.Name, Email: user.Email, WebURL: user.WebURL, State: user.State}
}

func groupFromAPI(group *gitlabapi.Group) Group {
	if group == nil {
		return Group{}
	}
	return Group{
		ID:          group.ID,
		Name:        group.Name,
		Path:        group.Path,
		FullName:    group.FullName,
		FullPath:    group.FullPath,
		Description: group.Description,
		Visibility:  string(group.Visibility),
		WebURL:      group.WebURL,
		ParentID:    group.ParentID,
		CreatedAt:   formatTime(group.CreatedAt),
	}
}

func projectFromAPI(project *gitlabapi.Project) Project {
	if project == nil {
		return Project{}
	}
	return Project{
		ID:                project.ID,
		Name:              project.Name,
		NameWithNamespace: project.NameWithNamespace,
		Path:              project.Path,
		PathWithNamespace: project.PathWithNamespace,
		Description:       project.Description,
		DefaultBranch:     project.DefaultBranch,
		Visibility:        string(project.Visibility),
		SSHURL:            project.SSHURLToRepo,
		HTTPURL:           project.HTTPURLToRepo,
		WebURL:            project.WebURL,
		Topics:            project.Topics,
		Archived:          project.Archived,
		LastActivityAt:    formatTime(project.LastActivityAt),
		UpdatedAt:         formatTime(project.UpdatedAt),
	}
}

func issueFromAPI(issue *gitlabapi.Issue) Issue {
	if issue == nil {
		return Issue{}
	}
	author := ""
	if issue.Author != nil {
		author = issue.Author.Username
	}
	reference := ""
	if issue.References != nil {
		reference = issue.References.Full
		if reference == "" {
			reference = issue.References.Relative
		}
		if reference == "" {
			reference = issue.References.Short
		}
	}
	return Issue{
		ID:             issue.ID,
		IID:            issue.IID,
		ProjectID:      issue.ProjectID,
		Title:          issue.Title,
		State:          issue.State,
		WebURL:         issue.WebURL,
		AuthorUsername: author,
		Labels:         []string(issue.Labels),
		Reference:      reference,
		CreatedAt:      formatTime(issue.CreatedAt),
		UpdatedAt:      formatTime(issue.UpdatedAt),
		ClosedAt:       formatTime(issue.ClosedAt),
	}
}

func mergeRequestFromAPI(mr *gitlabapi.MergeRequest) MergeRequest {
	if mr == nil {
		return MergeRequest{}
	}
	author := ""
	if mr.Author != nil {
		author = mr.Author.Username
	}
	reference := mergeRequestReference(mr.References)
	return MergeRequest{
		ID:             mr.ID,
		IID:            mr.IID,
		ProjectID:      mr.ProjectID,
		Title:          mr.Title,
		Description:    mr.Description,
		State:          mr.State,
		SourceBranch:   mr.SourceBranch,
		TargetBranch:   mr.TargetBranch,
		WebURL:         mr.WebURL,
		AuthorUsername: author,
		Labels:         []string(mr.Labels),
		Reference:      reference,
		SHA:            mr.SHA,
		Draft:          mr.Draft,
		CreatedAt:      formatTime(mr.CreatedAt),
		UpdatedAt:      formatTime(mr.UpdatedAt),
	}
}

func mergeRequestsFromAPI(mrs []*gitlabapi.BasicMergeRequest) []MergeRequest {
	out := make([]MergeRequest, 0, len(mrs))
	for _, mr := range mrs {
		out = append(out, basicMergeRequestFromAPI(mr))
	}
	return out
}

func basicMergeRequestFromAPI(mr *gitlabapi.BasicMergeRequest) MergeRequest {
	if mr == nil {
		return MergeRequest{}
	}
	author := ""
	if mr.Author != nil {
		author = mr.Author.Username
	}
	reference := mergeRequestReference(mr.References)
	return MergeRequest{
		ID:             mr.ID,
		IID:            mr.IID,
		ProjectID:      mr.ProjectID,
		Title:          mr.Title,
		Description:    mr.Description,
		State:          mr.State,
		SourceBranch:   mr.SourceBranch,
		TargetBranch:   mr.TargetBranch,
		WebURL:         mr.WebURL,
		AuthorUsername: author,
		Labels:         []string(mr.Labels),
		Reference:      reference,
		SHA:            mr.SHA,
		Draft:          mr.Draft,
		CreatedAt:      formatTime(mr.CreatedAt),
		UpdatedAt:      formatTime(mr.UpdatedAt),
	}
}

func mergeRequestApprovalFromAPI(approval *gitlabapi.MergeRequestApprovals) MergeRequestApproval {
	if approval == nil {
		return MergeRequestApproval{}
	}
	return MergeRequestApproval{
		ID:                approval.ID,
		IID:               approval.IID,
		ProjectID:         approval.ProjectID,
		Title:             approval.Title,
		State:             approval.State,
		MergeStatus:       approval.MergeStatus,
		Approved:          approval.Approved,
		ApprovalsRequired: approval.ApprovalsRequired,
		ApprovalsLeft:     approval.ApprovalsLeft,
		UserHasApproved:   approval.UserHasApproved,
		UserCanApprove:    approval.UserCanApprove,
		HasApprovalRules:  approval.HasApprovalRules,
		CreatedAt:         formatTime(approval.CreatedAt),
		UpdatedAt:         formatTime(approval.UpdatedAt),
	}
}

func repositoryTagFromAPI(tag *gitlabapi.Tag) RepositoryTag {
	if tag == nil {
		return RepositoryTag{}
	}
	return RepositoryTag{
		Name:      tag.Name,
		Message:   tag.Message,
		Target:    tag.Target,
		Protected: tag.Protected,
		CreatedAt: formatTime(tag.CreatedAt),
		Commit:    repositoryTagCommitFromAPI(tag.Commit),
	}
}

func repositoryTagCommitFromAPI(commit *gitlabapi.Commit) RepositoryTagCommit {
	if commit == nil {
		return RepositoryTagCommit{}
	}
	return RepositoryTagCommit{
		ID:            commit.ID,
		ShortID:       commit.ShortID,
		Title:         commit.Title,
		CreatedAt:     formatTime(commit.CreatedAt),
		CommittedDate: formatTime(commit.CommittedDate),
		WebURL:        commit.WebURL,
	}
}

func mergeRequestReference(refs *gitlabapi.IssueReferences) string {
	if refs == nil {
		return ""
	}
	if refs.Full != "" {
		return refs.Full
	}
	if refs.Relative != "" {
		return refs.Relative
	}
	return refs.Short
}

func formatTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
