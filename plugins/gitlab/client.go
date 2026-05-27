package gitlab

import (
	"fmt"
	"strings"
	"time"

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
}

type ClientFactory func(reqSecretSet SecretSet) (Client, error)

func NewLiveClient(secrets SecretSet) (Client, error) {
	baseURL := strings.TrimSpace(secrets.GitLabURL.Value)
	token := strings.TrimSpace(secrets.AccessToken.Value)
	if baseURL == "" {
		return nil, fmt.Errorf("gitlab_url is empty")
	}
	if token == "" {
		return nil, fmt.Errorf("access_token is empty")
	}
	client, err := gitlabapi.NewClient(token, gitlabapi.WithBaseURL(baseURL))
	if err != nil {
		return nil, err
	}
	return liveClient{client: client}, nil
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
		State:          mr.State,
		SourceBranch:   mr.SourceBranch,
		TargetBranch:   mr.TargetBranch,
		WebURL:         mr.WebURL,
		AuthorUsername: author,
		Labels:         []string(mr.Labels),
		Reference:      reference,
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
		State:          mr.State,
		SourceBranch:   mr.SourceBranch,
		TargetBranch:   mr.TargetBranch,
		WebURL:         mr.WebURL,
		AuthorUsername: author,
		Labels:         []string(mr.Labels),
		Reference:      reference,
		Draft:          mr.Draft,
		CreatedAt:      formatTime(mr.CreatedAt),
		UpdatedAt:      formatTime(mr.UpdatedAt),
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
