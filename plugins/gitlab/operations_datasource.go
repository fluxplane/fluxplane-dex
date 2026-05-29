package gitlab

import (
	"strconv"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type ProjectSearchResult = pluginbinding.DatasourceSearchResult[ProjectRecord]
type UserSearchResult = pluginbinding.DatasourceSearchResult[UserRecord]
type GroupSearchResult = pluginbinding.DatasourceSearchResult[GroupRecord]
type IssueSearchResult = pluginbinding.DatasourceSearchResult[IssueRecord]
type MergeRequestSearchResult = pluginbinding.DatasourceSearchResult[MergeRequestRecord]

type ProjectGetResult = pluginbinding.DatasourceGetResult[ProjectRecord]
type UserGetResult = pluginbinding.DatasourceGetResult[UserRecord]
type GroupGetResult = pluginbinding.DatasourceGetResult[GroupRecord]
type IssueGetResult = pluginbinding.DatasourceGetResult[IssueRecord]
type MergeRequestGetResult = pluginbinding.DatasourceGetResult[MergeRequestRecord]

func (s Service) ProjectDatasourceSearch(ctx pluginbinding.Context, input pluginbinding.DatasourceSearchInput) (ProjectSearchResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return ProjectSearchResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	projects, err := client.ListProjects(ProjectListOptions{Search: strings.TrimSpace(input.Query), Limit: datasourceLimit(input.Limit, 20), Membership: boolPtr(true)})
	if err != nil {
		return ProjectSearchResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	records := make([]ProjectRecord, 0, len(projects))
	for _, project := range projects {
		records = append(records, normalizeProjectRecord(ctx.DatasourceSource(), project))
	}
	return pluginbinding.NewDatasourceSearchResult(PluginName, input.Query, records), nil
}

func (s Service) ProjectDatasourceGet(ctx pluginbinding.Context, input pluginbinding.DatasourceGetInput) (ProjectGetResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return ProjectGetResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return ProjectGetResult{}, pluginbinding.Fail("bad_input", "id is required")
	}
	project, err := client.GetProject(projectID(id))
	if err != nil {
		return ProjectGetResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return pluginbinding.NewDatasourceGetResult(PluginName, normalizeProjectRecord(ctx.DatasourceSource(), project)), nil
}

func (s Service) UserDatasourceSearch(ctx pluginbinding.Context, input pluginbinding.DatasourceSearchInput) (UserSearchResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return UserSearchResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	users, err := client.ListUsers(UserListOptions{Search: strings.TrimSpace(input.Query), Limit: datasourceLimit(input.Limit, 20), Active: boolPtr(true)})
	if err != nil {
		return UserSearchResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	records := make([]UserRecord, 0, len(users))
	for _, user := range users {
		records = append(records, normalizeUserRecord(ctx.DatasourceSource(), user))
	}
	return pluginbinding.NewDatasourceSearchResult(PluginName, input.Query, records), nil
}

func (s Service) UserDatasourceGet(ctx pluginbinding.Context, input pluginbinding.DatasourceGetInput) (UserGetResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return UserGetResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return UserGetResult{}, pluginbinding.Fail("bad_input", "id is required")
	}
	users, err := client.ListUsers(UserListOptions{Search: id, Limit: 100})
	if err != nil {
		return UserGetResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	for _, user := range users {
		record := normalizeUserRecord(ctx.DatasourceSource(), user)
		if record.ID == id || strconv.FormatInt(user.ID, 10) == id {
			return pluginbinding.NewDatasourceGetResult(PluginName, record), nil
		}
	}
	return UserGetResult{}, pluginbinding.Fail("not_found", "GitLab user not found: "+id)
}

func (s Service) GroupDatasourceSearch(ctx pluginbinding.Context, input pluginbinding.DatasourceSearchInput) (GroupSearchResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return GroupSearchResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	groups, err := client.ListGroups(GroupListOptions{Search: strings.TrimSpace(input.Query), Limit: datasourceLimit(input.Limit, 20), Active: boolPtr(true)})
	if err != nil {
		return GroupSearchResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	records := make([]GroupRecord, 0, len(groups))
	for _, group := range groups {
		records = append(records, normalizeGroupRecord(ctx.DatasourceSource(), group))
	}
	return pluginbinding.NewDatasourceSearchResult(PluginName, input.Query, records), nil
}

func (s Service) GroupDatasourceGet(ctx pluginbinding.Context, input pluginbinding.DatasourceGetInput) (GroupGetResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return GroupGetResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return GroupGetResult{}, pluginbinding.Fail("bad_input", "id is required")
	}
	groups, err := client.ListGroups(GroupListOptions{Search: id, Limit: 100})
	if err != nil {
		return GroupGetResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	for _, group := range groups {
		record := normalizeGroupRecord(ctx.DatasourceSource(), group)
		if record.ID == id || strconv.FormatInt(group.ID, 10) == id {
			return pluginbinding.NewDatasourceGetResult(PluginName, record), nil
		}
	}
	return GroupGetResult{}, pluginbinding.Fail("not_found", "GitLab group not found: "+id)
}

func (s Service) IssueDatasourceSearch(ctx pluginbinding.Context, input pluginbinding.DatasourceSearchInput) (IssueSearchResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return IssueSearchResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	issues, err := client.ListIssues(IssueListOptions{Search: strings.TrimSpace(input.Query), Limit: datasourceLimit(input.Limit, 20), State: "all"})
	if err != nil {
		return IssueSearchResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	records := make([]IssueRecord, 0, len(issues))
	for _, issue := range issues {
		records = append(records, normalizeIssueRecord(ctx.DatasourceSource(), issue))
	}
	return pluginbinding.NewDatasourceSearchResult(PluginName, input.Query, records), nil
}

func (s Service) IssueDatasourceGet(ctx pluginbinding.Context, input pluginbinding.DatasourceGetInput) (IssueGetResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return IssueGetResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return IssueGetResult{}, pluginbinding.Fail("bad_input", "id is required")
	}
	issues, err := client.ListIssues(IssueListOptions{Search: id, Limit: 100, State: "all"})
	if err != nil {
		return IssueGetResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	for _, issue := range issues {
		record := normalizeIssueRecord(ctx.DatasourceSource(), issue)
		if record.ID == id || strconv.FormatInt(issue.ID, 10) == id {
			return pluginbinding.NewDatasourceGetResult(PluginName, record), nil
		}
	}
	return IssueGetResult{}, pluginbinding.Fail("not_found", "GitLab issue not found: "+id)
}

func (s Service) MergeRequestDatasourceSearch(ctx pluginbinding.Context, input pluginbinding.DatasourceSearchInput) (MergeRequestSearchResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return MergeRequestSearchResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	mrs, err := client.ListMergeRequests(MergeRequestListOptions{Search: strings.TrimSpace(input.Query), Limit: datasourceLimit(input.Limit, 20), State: "all"})
	if err != nil {
		return MergeRequestSearchResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	records := make([]MergeRequestRecord, 0, len(mrs))
	for _, mr := range mrs {
		records = append(records, normalizeMergeRequestRecord(ctx.DatasourceSource(), mr))
	}
	return pluginbinding.NewDatasourceSearchResult(PluginName, input.Query, records), nil
}

func (s Service) MergeRequestDatasourceGet(ctx pluginbinding.Context, input pluginbinding.DatasourceGetInput) (MergeRequestGetResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return MergeRequestGetResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return MergeRequestGetResult{}, pluginbinding.Fail("bad_input", "id is required")
	}
	project, iid, err := parseMergeRequestRef(id)
	if err == nil {
		mr, err := client.GetMergeRequest(projectID(project), iid)
		if err != nil {
			return MergeRequestGetResult{}, pluginbinding.Errorf("gitlab", "%s", err)
		}
		return pluginbinding.NewDatasourceGetResult(PluginName, normalizeMergeRequestRecord(ctx.DatasourceSource(), mr)), nil
	}
	mrs, err := client.ListMergeRequests(MergeRequestListOptions{Search: id, Limit: 100, State: "all"})
	if err != nil {
		return MergeRequestGetResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	for _, mr := range mrs {
		record := normalizeMergeRequestRecord(ctx.DatasourceSource(), mr)
		if record.ID == id || strconv.FormatInt(mr.ID, 10) == id {
			return pluginbinding.NewDatasourceGetResult(PluginName, record), nil
		}
	}
	return MergeRequestGetResult{}, pluginbinding.Fail("not_found", "GitLab merge request not found: "+id)
}

func datasourceLimit(limit, fallback int) int {
	if limit <= 0 {
		limit = fallback
	}
	if limit > 100 {
		return 100
	}
	return limit
}
