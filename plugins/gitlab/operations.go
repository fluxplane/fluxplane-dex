package gitlab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type Service struct {
	SecretGetter  pluginbinding.SecretGetter
	ClientFactory ClientFactory
}

func NewService() Service {
	return Service{ClientFactory: NewLiveClient}
}

func (s Service) client(ctx pluginbinding.Context) (Client, error) {
	secrets, err := resolveSecrets(ctx)
	if err != nil {
		return nil, err
	}
	factory := s.ClientFactory
	if factory == nil {
		factory = NewLiveClient
	}
	return factory(secrets)
}

type NoInput struct{}

type LookupInput = pluginbinding.DatasourceLookupInput
type LookupResult = pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]

type AuthTestResult struct {
	Text   string `json:"text"`
	Status string `json:"status"`
	User   User   `json:"user"`
}

type ProjectListInput struct {
	pluginbinding.ListInput
	Membership *bool `json:"membership,omitempty" jsonschema:"description=Limit results to member projects"`
}

type ProjectShowInput struct {
	ID      string `json:"id,omitempty" jsonschema:"description=Numeric project ID or path"`
	Project string `json:"project,omitempty" jsonschema:"description=Alias for id"`
	Path    string `json:"path,omitempty" jsonschema:"description=Alias for id"`
}

func (input *ProjectShowInput) UnmarshalJSON(raw []byte) error {
	var values map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&values); err != nil {
		return err
	}
	input.ID = flexibleJSONString(values["id"])
	input.Project = flexibleJSONString(values["project"])
	input.Path = flexibleJSONString(values["path"])
	return nil
}

type MergeRequestListInput struct {
	pluginbinding.ListInput
	Project   string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID string `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path      string `json:"path,omitempty" jsonschema:"description=Alias for project"`
	State     string `json:"state,omitempty" jsonschema:"description=Merge request state"`
}

type MergeRequestShowInput struct {
	Ref string `json:"ref,omitempty" jsonschema:"description=Merge request reference as PROJECT!IID"`
	ID  string `json:"id,omitempty" jsonschema:"description=Alias for ref"`
}

type MergeRequestCreateInput struct {
	Project            string   `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID          string   `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path               string   `json:"path,omitempty" jsonschema:"description=Alias for project"`
	Title              string   `json:"title,omitempty" jsonschema:"description=Merge request title"`
	SourceBranch       string   `json:"source_branch,omitempty" jsonschema:"description=Source branch name"`
	TargetBranch       string   `json:"target_branch,omitempty" jsonschema:"description=Target branch name"`
	Description        string   `json:"description,omitempty" jsonschema:"description=Merge request description"`
	Labels             []string `json:"labels,omitempty" jsonschema:"description=Labels to set"`
	AssigneeID         int64    `json:"assignee_id,omitempty" jsonschema:"description=Single assignee user ID"`
	AssigneeIDs        []int64  `json:"assignee_ids,omitempty" jsonschema:"description=Assignee user IDs"`
	ReviewerIDs        []int64  `json:"reviewer_ids,omitempty" jsonschema:"description=Reviewer user IDs"`
	TargetProjectID    int64    `json:"target_project_id,omitempty" jsonschema:"description=Target project ID for fork merge requests"`
	MilestoneID        int64    `json:"milestone_id,omitempty" jsonschema:"description=Milestone ID"`
	RemoveSourceBranch *bool    `json:"remove_source_branch,omitempty" jsonschema:"description=Remove source branch after merge"`
	Squash             *bool    `json:"squash,omitempty" jsonschema:"description=Squash commits when merging"`
	AllowCollaboration *bool    `json:"allow_collaboration,omitempty" jsonschema:"description=Allow upstream members to collaborate"`
}

type MergeRequestApproveInput struct {
	Ref             string `json:"ref,omitempty" jsonschema:"description=Merge request reference as PROJECT!IID"`
	ID              string `json:"id,omitempty" jsonschema:"description=Alias for ref"`
	Project         string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID       string `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path            string `json:"path,omitempty" jsonschema:"description=Alias for project"`
	IID             int64  `json:"iid,omitempty" jsonschema:"description=Merge request IID"`
	MergeRequestIID int64  `json:"merge_request_iid,omitempty" jsonschema:"description=Alias for iid"`
	SHA             string `json:"sha,omitempty" jsonschema:"description=Expected merge request HEAD SHA"`
}

type MergeRequestMergeInput struct {
	Ref                      string `json:"ref,omitempty" jsonschema:"description=Merge request reference as PROJECT!IID"`
	ID                       string `json:"id,omitempty" jsonschema:"description=Alias for ref"`
	Project                  string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID                string `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path                     string `json:"path,omitempty" jsonschema:"description=Alias for project"`
	IID                      int64  `json:"iid,omitempty" jsonschema:"description=Merge request IID"`
	MergeRequestIID          int64  `json:"merge_request_iid,omitempty" jsonschema:"description=Alias for iid"`
	AutoMerge                *bool  `json:"auto_merge,omitempty" jsonschema:"description=Merge automatically when checks allow it"`
	MergeCommitMessage       string `json:"merge_commit_message,omitempty" jsonschema:"description=Custom merge commit message"`
	SquashCommitMessage      string `json:"squash_commit_message,omitempty" jsonschema:"description=Custom squash commit message"`
	Squash                   *bool  `json:"squash,omitempty" jsonschema:"description=Squash commits"`
	ShouldRemoveSourceBranch *bool  `json:"should_remove_source_branch,omitempty" jsonschema:"description=Remove source branch after merge"`
	RemoveSourceBranch       *bool  `json:"remove_source_branch,omitempty" jsonschema:"description=Alias for should_remove_source_branch"`
	SHA                      string `json:"sha,omitempty" jsonschema:"description=Expected merge request HEAD SHA"`
}

type RepositoryTagCreateInput struct {
	Project   string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID string `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path      string `json:"path,omitempty" jsonschema:"description=Alias for project"`
	TagName   string `json:"tag_name,omitempty" jsonschema:"description=Tag name to create"`
	Name      string `json:"name,omitempty" jsonschema:"description=Alias for tag_name"`
	Ref       string `json:"ref,omitempty" jsonschema:"description=Commit SHA, branch name, or existing tag name"`
	Message   string `json:"message,omitempty" jsonschema:"description=Optional annotated tag message"`
}

type IndexBuildInput struct {
	pluginbinding.IndexBuildInput
	Limit            int    `json:"limit,omitempty" jsonschema:"description=Project fetch limit"`
	Search           string `json:"search,omitempty" jsonschema:"description=Project search text"`
	Query            string `json:"query,omitempty" jsonschema:"description=Alias for search"`
	OrderBy          string `json:"order_by,omitempty" jsonschema:"description=Project order_by value"`
	Sort             string `json:"sort,omitempty" jsonschema:"description=Project sort direction,enum=asc,enum=desc"`
	UserLimit        int    `json:"user_limit,omitempty" jsonschema:"description=User fetch limit"`
	UserSearch       string `json:"user_search,omitempty" jsonschema:"description=User search text"`
	GroupLimit       int    `json:"group_limit,omitempty" jsonschema:"description=Group fetch limit"`
	GroupSearch      string `json:"group_search,omitempty" jsonschema:"description=Group search text"`
	GroupOrderBy     string `json:"group_order_by,omitempty" jsonschema:"description=Group order_by value"`
	GroupSort        string `json:"group_sort,omitempty" jsonschema:"description=Group sort direction,enum=asc,enum=desc"`
	IssueLimit       int    `json:"issue_limit,omitempty" jsonschema:"description=Issue fetch limit"`
	IssueSearch      string `json:"issue_search,omitempty" jsonschema:"description=Issue search text"`
	IssueState       string `json:"issue_state,omitempty" jsonschema:"description=Issue state"`
	IssueOrderBy     string `json:"issue_order_by,omitempty" jsonschema:"description=Issue order_by value"`
	IssueSort        string `json:"issue_sort,omitempty" jsonschema:"description=Issue sort direction,enum=asc,enum=desc"`
	MRProject        string `json:"mr_project,omitempty" jsonschema:"description=Merge request project path or ID"`
	MRLimit          int    `json:"mr_limit,omitempty" jsonschema:"description=Merge request fetch limit"`
	MRSearch         string `json:"mr_search,omitempty" jsonschema:"description=Merge request search text"`
	MRState          string `json:"mr_state,omitempty" jsonschema:"description=Merge request state"`
	MROrderBy        string `json:"mr_order_by,omitempty" jsonschema:"description=Merge request order_by value"`
	MRSort           string `json:"mr_sort,omitempty" jsonschema:"description=Merge request sort direction,enum=asc,enum=desc"`
	Membership       *bool  `json:"membership,omitempty" jsonschema:"description=Limit projects to member projects"`
	ActiveUsers      *bool  `json:"active_users,omitempty" jsonschema:"description=Only include active users"`
	ActiveGroups     *bool  `json:"active_groups,omitempty" jsonschema:"description=Only include active groups"`
	AllVisibleGroups *bool  `json:"all_visible_groups,omitempty" jsonschema:"description=Include all visible groups"`
}

func (s Service) AuthTest(ctx pluginbinding.Context, _ NoInput) (AuthTestResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return AuthTestResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	user, err := client.CurrentUser()
	if err != nil {
		return AuthTestResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return AuthTestResult{Text: "GitLab auth OK", Status: "ok", User: user}, nil
}

func (s Service) ProjectList(ctx pluginbinding.Context, input ProjectListInput) (pluginbinding.ListResult[Project], error) {
	client, err := s.client(ctx)
	if err != nil {
		return pluginbinding.ListResult[Project]{}, pluginbinding.Errorf("secret", "%s", err)
	}
	projects, err := client.ListProjects(projectListOptions(pluginbinding.InputMap(input), 20))
	if err != nil {
		return pluginbinding.ListResult[Project]{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return pluginbinding.NewListResult(projects), nil
}

func (s Service) ProjectShow(ctx pluginbinding.Context, input ProjectShowInput) (Project, error) {
	client, err := s.client(ctx)
	if err != nil {
		return Project{}, pluginbinding.Errorf("secret", "%s", err)
	}
	id := strings.TrimSpace(pluginbinding.FirstString(pluginbinding.InputMap(input), "id", "project", "path"))
	if id == "" {
		return Project{}, pluginbinding.Fail("bad_input", "project id or path is required")
	}
	project, err := client.GetProject(projectID(id))
	if err != nil {
		return Project{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return project, nil
}

func (s Service) MergeRequestList(ctx pluginbinding.Context, input MergeRequestListInput) (pluginbinding.ListResult[MergeRequest], error) {
	client, err := s.client(ctx)
	if err != nil {
		return pluginbinding.ListResult[MergeRequest]{}, pluginbinding.Errorf("secret", "%s", err)
	}
	mrs, err := client.ListMergeRequests(mergeRequestListOptionsFromInput(pluginbinding.InputMap(input)))
	if err != nil {
		return pluginbinding.ListResult[MergeRequest]{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return pluginbinding.NewListResult(mrs), nil
}

func (s Service) IndexBuild(ctx pluginbinding.Context, input IndexBuildInput) (pluginbinding.IndexBuildResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return pluginbinding.IndexBuildResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	return s.indexBuild(ctx, client, pluginbinding.InputMap(input))
}

func (s Service) MergeRequestShow(ctx pluginbinding.Context, input MergeRequestShowInput) (pluginbinding.ShowResult[MergeRequest], error) {
	client, err := s.client(ctx)
	if err != nil {
		return pluginbinding.ShowResult[MergeRequest]{}, pluginbinding.Errorf("secret", "%s", err)
	}
	ref := strings.TrimSpace(pluginbinding.FirstString(pluginbinding.InputMap(input), "ref", "id"))
	project, iid, err := parseMergeRequestRef(ref)
	if err != nil {
		return pluginbinding.ShowResult[MergeRequest]{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	mr, err := client.GetMergeRequest(projectID(project), iid)
	if err != nil {
		return pluginbinding.ShowResult[MergeRequest]{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return pluginbinding.NewShowResult(mr, map[string]any{"ref": ref}), nil
}

func (s Service) MergeRequestCreate(ctx pluginbinding.Context, input MergeRequestCreateInput) (MergeRequest, error) {
	client, err := s.client(ctx)
	if err != nil {
		return MergeRequest{}, pluginbinding.Errorf("secret", "%s", err)
	}
	options, err := mergeRequestCreateOptionsFromInput(pluginbinding.InputMap(input))
	if err != nil {
		return MergeRequest{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	mr, err := client.CreateMergeRequest(projectID(options.Project), options)
	if err != nil {
		return MergeRequest{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return mr, nil
}

func (s Service) MergeRequestApprove(ctx pluginbinding.Context, input MergeRequestApproveInput) (MergeRequestApproval, error) {
	client, err := s.client(ctx)
	if err != nil {
		return MergeRequestApproval{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project, iid, err := mergeRequestAddressFromInput(pluginbinding.InputMap(input))
	if err != nil {
		return MergeRequestApproval{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	approval, err := client.ApproveMergeRequest(projectID(project), iid, MergeRequestApproveOptions{SHA: strings.TrimSpace(input.SHA)})
	if err != nil {
		return MergeRequestApproval{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return approval, nil
}

func (s Service) MergeRequestMerge(ctx pluginbinding.Context, input MergeRequestMergeInput) (MergeRequest, error) {
	client, err := s.client(ctx)
	if err != nil {
		return MergeRequest{}, pluginbinding.Errorf("secret", "%s", err)
	}
	values := pluginbinding.InputMap(input)
	project, iid, err := mergeRequestAddressFromInput(values)
	if err != nil {
		return MergeRequest{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	options := mergeRequestMergeOptionsFromInput(values)
	mr, err := client.MergeMergeRequest(projectID(project), iid, options)
	if err != nil {
		return MergeRequest{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return mr, nil
}

func (s Service) RepositoryTagCreate(ctx pluginbinding.Context, input RepositoryTagCreateInput) (RepositoryTag, error) {
	client, err := s.client(ctx)
	if err != nil {
		return RepositoryTag{}, pluginbinding.Errorf("secret", "%s", err)
	}
	options, err := repositoryTagCreateOptionsFromInput(pluginbinding.InputMap(input))
	if err != nil {
		return RepositoryTag{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	tag, err := client.CreateRepositoryTag(projectID(options.Project), options)
	if err != nil {
		return RepositoryTag{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return tag, nil
}

func (s Service) Lookup(ctx pluginbinding.Context, input LookupInput) (LookupResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return LookupResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	candidates, err := s.lookupCandidates(ctx, client, input)
	if err != nil {
		return LookupResult{}, err
	}
	return pluginbinding.NewDatasourceLookupResultFromCandidates(PluginName, input, candidates), nil
}

func (s Service) lookupCandidates(ctx pluginbinding.Context, client Client, input LookupInput) ([]pluginbinding.LookupCandidate, error) {
	source := ctx.DatasourceSource()
	var candidates []pluginbinding.LookupCandidate
	addProject := func(project Project, exact bool) {
		record := normalizeProjectRecord(source, project)
		if exact {
			candidates = append(candidates, pluginbinding.NewExactLookupCandidate(ctx.LookupSource(PluginName, DatasourceProjects), record.Entity, record.ID, 1200, []string{"id"}, record, projectLookupValues(record)))
		} else {
			candidates = append(candidates, pluginbinding.NewLookupCandidate(ctx.LookupSource(PluginName, DatasourceProjects), record.Entity, record.ID, record, projectLookupValues(record)))
		}
	}
	addUser := func(user User) {
		record := normalizeUserRecord(source, user)
		candidates = append(candidates, pluginbinding.NewLookupCandidate(ctx.LookupSource(PluginName, DatasourceUsers), record.Entity, record.ID, record, userLookupValues(record)))
	}
	addGroup := func(group Group) {
		record := normalizeGroupRecord(source, group)
		candidates = append(candidates, pluginbinding.NewLookupCandidate(ctx.LookupSource(PluginName, DatasourceGroups), record.Entity, record.ID, record, groupLookupValues(record)))
	}
	addIssue := func(issue Issue) {
		record := normalizeIssueRecord(source, issue)
		candidates = append(candidates, pluginbinding.NewLookupCandidate(ctx.LookupSource(PluginName, DatasourceIssues), record.Entity, record.ID, record, issueLookupValues(record)))
	}
	addMergeRequest := func(mr MergeRequest, exact bool) {
		record := normalizeMergeRequestRecord(source, mr)
		if exact {
			candidates = append(candidates, pluginbinding.NewExactLookupCandidate(ctx.LookupSource(PluginName, DatasourceMergeRequests), record.Entity, record.ID, 1200, []string{"record.reference"}, record, mergeRequestLookupValues(record)))
		} else {
			candidates = append(candidates, pluginbinding.NewLookupCandidate(ctx.LookupSource(PluginName, DatasourceMergeRequests), record.Entity, record.ID, record, mergeRequestLookupValues(record)))
		}
	}

	entity := strings.TrimSpace(input.Entity)
	terms := lookupSearchTerms(input)
	if entity == "" || entity == EntityProject {
		for _, project := range lookupProjectCandidates(input) {
			if value, err := client.GetProject(projectID(project)); err == nil {
				addProject(value, true)
			}
		}
		for _, term := range terms {
			projects, err := client.ListProjects(ProjectListOptions{Search: term, Limit: pluginbinding.LookupLimit(input, 20, 100), Membership: boolPtr(true)})
			if err != nil {
				return nil, pluginbinding.Errorf("gitlab", "%s", err)
			}
			for _, project := range projects {
				addProject(project, false)
			}
		}
	}
	if entity == "" || entity == EntityUser {
		for _, term := range terms {
			users, err := client.ListUsers(UserListOptions{Search: term, Limit: pluginbinding.LookupLimit(input, 20, 100), Active: boolPtr(true)})
			if err != nil {
				return nil, pluginbinding.Errorf("gitlab", "%s", err)
			}
			for _, user := range users {
				addUser(user)
			}
		}
	}
	if entity == "" || entity == EntityGroup {
		for _, term := range terms {
			groups, err := client.ListGroups(GroupListOptions{Search: term, Limit: pluginbinding.LookupLimit(input, 20, 100), Active: boolPtr(true)})
			if err != nil {
				return nil, pluginbinding.Errorf("gitlab", "%s", err)
			}
			for _, group := range groups {
				addGroup(group)
			}
		}
	}
	if entity == "" || entity == EntityIssue {
		for _, term := range terms {
			issues, err := client.ListIssues(IssueListOptions{Search: term, Limit: pluginbinding.LookupLimit(input, 20, 100), State: "all"})
			if err != nil {
				return nil, pluginbinding.Errorf("gitlab", "%s", err)
			}
			for _, issue := range issues {
				addIssue(issue)
			}
		}
	}
	if entity == "" || entity == EntityMergeRequest {
		for _, ref := range lookupMergeRequestRefs(input) {
			project, iid, err := parseMergeRequestRef(ref)
			if err != nil {
				continue
			}
			if mr, err := client.GetMergeRequest(projectID(project), iid); err == nil {
				addMergeRequest(mr, true)
			}
		}
		for _, term := range terms {
			mrs, err := client.ListMergeRequests(MergeRequestListOptions{Search: term, Limit: pluginbinding.LookupLimit(input, 20, 100), State: "all"})
			if err != nil {
				return nil, pluginbinding.Errorf("gitlab", "%s", err)
			}
			for _, mr := range mrs {
				addMergeRequest(mr, false)
			}
		}
	}
	return candidates, nil
}

func (s Service) indexBuild(ctx pluginbinding.Context, client Client, input map[string]any) (pluginbinding.IndexBuildResult, error) {
	selector, err := indexBuildSelector(input)
	if err != nil {
		return pluginbinding.IndexBuildResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	return pluginbinding.RunIndexJobs(ctx, selector, "gitlab",
		pluginbinding.NewRequiredIndexJob(DatasourceProjects, EntityProject, OperationIndexBuild, func() ([]Project, error) {
			options := projectListOptions(input, 100)
			options.All = true
			return client.ListProjects(options)
		}, normalizeProjectRecord, projectIndexMetadata(projectListOptionsWithAll(input, 100))),
		pluginbinding.NewRequiredIndexJob(DatasourceUsers, EntityUser, OperationIndexBuild, func() ([]User, error) {
			options := userListOptions(input, 100)
			options.All = true
			return client.ListUsers(options)
		}, normalizeUserRecord, userIndexMetadata(userListOptionsWithAll(input, 100))),
		pluginbinding.NewRequiredIndexJob(DatasourceGroups, EntityGroup, OperationIndexBuild, func() ([]Group, error) {
			options := groupListOptions(input, 100)
			options.All = true
			return client.ListGroups(options)
		}, normalizeGroupRecord, groupIndexMetadata(groupListOptionsWithAll(input, 100))),
		pluginbinding.NewRequiredIndexJob(DatasourceIssues, EntityIssue, OperationIndexBuild, func() ([]Issue, error) {
			options := issueListOptions(input, 100)
			options.All = true
			return client.ListIssues(options)
		}, normalizeIssueRecord, issueIndexMetadata(issueListOptionsWithAll(input, 100))),
		pluginbinding.NewRequiredIndexJob(DatasourceMergeRequests, EntityMergeRequest, OperationIndexBuild, func() ([]MergeRequest, error) {
			options := mergeRequestIndexOptions(input, 100)
			options.All = true
			return client.ListMergeRequests(options)
		}, normalizeMergeRequestRecord, mergeRequestIndexMetadata(mergeRequestIndexOptionsWithAll(input, 100))),
	)
}

func projectListOptionsWithAll(input map[string]any, defaultLimit int) ProjectListOptions {
	options := projectListOptions(input, defaultLimit)
	options.All = true
	return options
}

func userListOptionsWithAll(input map[string]any, defaultLimit int) UserListOptions {
	options := userListOptions(input, defaultLimit)
	options.All = true
	return options
}

func groupListOptionsWithAll(input map[string]any, defaultLimit int) GroupListOptions {
	options := groupListOptions(input, defaultLimit)
	options.All = true
	return options
}

func issueListOptionsWithAll(input map[string]any, defaultLimit int) IssueListOptions {
	options := issueListOptions(input, defaultLimit)
	options.All = true
	return options
}

func mergeRequestIndexOptionsWithAll(input map[string]any, defaultLimit int) MergeRequestListOptions {
	options := mergeRequestIndexOptions(input, defaultLimit)
	options.All = true
	return options
}

func projectListOptions(input map[string]any, defaultLimit int) ProjectListOptions {
	membership := pluginbinding.BoolPtrFromInput(input, "membership", true)
	return ProjectListOptions{
		Limit:      pluginbinding.BoundedIntFromInput(input, "limit", defaultLimit, 100),
		Search:     pluginbinding.StringFromInput(input, "search", "query"),
		OrderBy:    pluginbinding.DefaultStringFromInput(input, "last_activity_at", "order_by"),
		Sort:       pluginbinding.DefaultStringFromInput(input, "desc", "sort"),
		Membership: membership,
	}
}

func userListOptions(input map[string]any, defaultLimit int) UserListOptions {
	return UserListOptions{
		Limit:  pluginbinding.BoundedIntFromInput(input, "user_limit", defaultLimit, 100),
		Search: pluginbinding.StringFromInput(input, "user_search"),
		Active: pluginbinding.BoolPtrFromInput(input, "active_users", true),
	}
}

func groupListOptions(input map[string]any, defaultLimit int) GroupListOptions {
	return GroupListOptions{
		Limit:      pluginbinding.BoundedIntFromInput(input, "group_limit", defaultLimit, 100),
		Search:     pluginbinding.StringFromInput(input, "group_search"),
		OrderBy:    pluginbinding.DefaultStringFromInput(input, "name", "group_order_by"),
		Sort:       pluginbinding.DefaultStringFromInput(input, "asc", "group_sort"),
		Active:     pluginbinding.BoolPtrFromInput(input, "active_groups", true),
		AllVisible: pluginbinding.BoolPtrFromInput(input, "all_visible_groups", false),
	}
}

func issueListOptions(input map[string]any, defaultLimit int) IssueListOptions {
	return IssueListOptions{
		Limit:   pluginbinding.BoundedIntFromInput(input, "issue_limit", defaultLimit, 100),
		Search:  pluginbinding.StringFromInput(input, "issue_search"),
		State:   pluginbinding.DefaultStringFromInput(input, "all", "issue_state"),
		OrderBy: pluginbinding.DefaultStringFromInput(input, "updated_at", "issue_order_by"),
		Sort:    pluginbinding.DefaultStringFromInput(input, "desc", "issue_sort"),
	}
}

func mergeRequestListOptionsFromInput(input map[string]any) MergeRequestListOptions {
	return MergeRequestListOptions{
		Project: pluginbinding.StringFromInput(input, "project", "project_id", "path"),
		Limit:   pluginbinding.BoundedIntFromInput(input, "limit", 20, 100),
		State:   pluginbinding.DefaultStringFromInput(input, "opened", "state"),
		Search:  pluginbinding.StringFromInput(input, "search", "query"),
		OrderBy: pluginbinding.DefaultStringFromInput(input, "updated_at", "order_by"),
		Sort:    pluginbinding.DefaultStringFromInput(input, "desc", "sort"),
	}
}

func mergeRequestCreateOptionsFromInput(input map[string]any) (MergeRequestCreateOptions, error) {
	options := MergeRequestCreateOptions{
		Project:            pluginbinding.StringFromInput(input, "project", "project_id", "path"),
		Title:              pluginbinding.StringFromInput(input, "title"),
		SourceBranch:       pluginbinding.StringFromInput(input, "source_branch"),
		TargetBranch:       pluginbinding.StringFromInput(input, "target_branch"),
		Description:        pluginbinding.StringFromInput(input, "description"),
		Labels:             stringSliceFromInput(input, "labels"),
		AssigneeID:         int64FromInput(input, "assignee_id"),
		AssigneeIDs:        int64SliceFromInput(input, "assignee_ids"),
		ReviewerIDs:        int64SliceFromInput(input, "reviewer_ids"),
		TargetProjectID:    int64FromInput(input, "target_project_id"),
		MilestoneID:        int64FromInput(input, "milestone_id"),
		RemoveSourceBranch: boolPtrFromInput(input, "remove_source_branch"),
		Squash:             boolPtrFromInput(input, "squash"),
		AllowCollaboration: boolPtrFromInput(input, "allow_collaboration"),
	}
	if strings.TrimSpace(options.Project) == "" {
		return MergeRequestCreateOptions{}, fmt.Errorf("project is required")
	}
	if strings.TrimSpace(options.Title) == "" {
		return MergeRequestCreateOptions{}, fmt.Errorf("title is required")
	}
	if strings.TrimSpace(options.SourceBranch) == "" {
		return MergeRequestCreateOptions{}, fmt.Errorf("source_branch is required")
	}
	if strings.TrimSpace(options.TargetBranch) == "" {
		return MergeRequestCreateOptions{}, fmt.Errorf("target_branch is required")
	}
	return options, nil
}

func mergeRequestAddressFromInput(input map[string]any) (string, int64, error) {
	ref := strings.TrimSpace(pluginbinding.FirstString(input, "ref", "id"))
	if ref != "" {
		return parseMergeRequestRef(ref)
	}
	project := strings.TrimSpace(pluginbinding.FirstString(input, "project", "project_id", "path"))
	if project == "" {
		return "", 0, fmt.Errorf("ref or project is required")
	}
	iid := int64FromInput(input, "iid", "merge_request_iid")
	if iid <= 0 {
		return "", 0, fmt.Errorf("iid must be a positive integer")
	}
	return project, iid, nil
}

func mergeRequestMergeOptionsFromInput(input map[string]any) MergeRequestMergeOptions {
	removeSourceBranch := boolPtrFromInput(input, "should_remove_source_branch")
	if removeSourceBranch == nil {
		removeSourceBranch = boolPtrFromInput(input, "remove_source_branch")
	}
	return MergeRequestMergeOptions{
		AutoMerge:                boolPtrFromInput(input, "auto_merge"),
		MergeCommitMessage:       pluginbinding.StringFromInput(input, "merge_commit_message"),
		SquashCommitMessage:      pluginbinding.StringFromInput(input, "squash_commit_message"),
		Squash:                   boolPtrFromInput(input, "squash"),
		ShouldRemoveSourceBranch: removeSourceBranch,
		SHA:                      pluginbinding.StringFromInput(input, "sha"),
	}
}

func repositoryTagCreateOptionsFromInput(input map[string]any) (RepositoryTagCreateOptions, error) {
	options := RepositoryTagCreateOptions{
		Project: pluginbinding.StringFromInput(input, "project", "project_id", "path"),
		TagName: pluginbinding.StringFromInput(input, "tag_name", "name"),
		Ref:     pluginbinding.StringFromInput(input, "ref"),
		Message: pluginbinding.StringFromInput(input, "message"),
	}
	if strings.TrimSpace(options.Project) == "" {
		return RepositoryTagCreateOptions{}, fmt.Errorf("project is required")
	}
	if strings.TrimSpace(options.TagName) == "" {
		return RepositoryTagCreateOptions{}, fmt.Errorf("tag_name is required")
	}
	if strings.TrimSpace(options.Ref) == "" {
		return RepositoryTagCreateOptions{}, fmt.Errorf("ref is required")
	}
	return options, nil
}

func mergeRequestIndexOptions(input map[string]any, defaultLimit int) MergeRequestListOptions {
	return MergeRequestListOptions{
		Project: pluginbinding.StringFromInput(input, "mr_project", "project", "project_id", "path"),
		Limit:   pluginbinding.BoundedIntFromInput(input, "mr_limit", defaultLimit, 100),
		State:   pluginbinding.DefaultStringFromInput(input, "all", "mr_state"),
		Search:  pluginbinding.StringFromInput(input, "mr_search"),
		OrderBy: pluginbinding.DefaultStringFromInput(input, "updated_at", "mr_order_by"),
		Sort:    pluginbinding.DefaultStringFromInput(input, "desc", "mr_sort"),
	}
}

func indexBuildSelector(input map[string]any) (pluginbinding.IndexSelector, error) {
	known := map[string]string{
		DatasourceProjects:      DatasourceProjects,
		EntityProject:           DatasourceProjects,
		"project":               DatasourceProjects,
		"projects":              DatasourceProjects,
		DatasourceUsers:         DatasourceUsers,
		EntityUser:              DatasourceUsers,
		"user":                  DatasourceUsers,
		"users":                 DatasourceUsers,
		DatasourceGroups:        DatasourceGroups,
		EntityGroup:             DatasourceGroups,
		"group":                 DatasourceGroups,
		"groups":                DatasourceGroups,
		DatasourceIssues:        DatasourceIssues,
		EntityIssue:             DatasourceIssues,
		"issue":                 DatasourceIssues,
		"issues":                DatasourceIssues,
		DatasourceMergeRequests: DatasourceMergeRequests,
		EntityMergeRequest:      DatasourceMergeRequests,
		"merge_request":         DatasourceMergeRequests,
		"merge_requests":        DatasourceMergeRequests,
		"mr":                    DatasourceMergeRequests,
		"mrs":                   DatasourceMergeRequests,
	}
	return pluginbinding.NewIndexSelector(input, known, "GitLab")
}

func indexBuildMetadata(entity string, input map[string]any) map[string]any {
	return pluginbinding.IndexBuildMetadata(entity, OperationIndexBuild, input)
}

func projectIndexMetadata(options ProjectListOptions) map[string]any {
	metadata := map[string]any{
		"limit":    options.Limit,
		"search":   options.Search,
		"order_by": options.OrderBy,
		"sort":     options.Sort,
	}
	if options.Membership != nil {
		metadata["membership"] = *options.Membership
	}
	return metadata
}

func userIndexMetadata(options UserListOptions) map[string]any {
	metadata := map[string]any{
		"limit":  options.Limit,
		"search": options.Search,
	}
	if options.Active != nil {
		metadata["active"] = *options.Active
	}
	return metadata
}

func groupIndexMetadata(options GroupListOptions) map[string]any {
	metadata := map[string]any{
		"limit":    options.Limit,
		"search":   options.Search,
		"order_by": options.OrderBy,
		"sort":     options.Sort,
	}
	if options.Active != nil {
		metadata["active"] = *options.Active
	}
	if options.TopLevel != nil {
		metadata["top_level"] = *options.TopLevel
	}
	if options.AllVisible != nil {
		metadata["all_visible"] = *options.AllVisible
	}
	return metadata
}

func issueIndexMetadata(options IssueListOptions) map[string]any {
	return map[string]any{
		"limit":    options.Limit,
		"search":   options.Search,
		"state":    options.State,
		"order_by": options.OrderBy,
		"sort":     options.Sort,
	}
}

func mergeRequestIndexMetadata(options MergeRequestListOptions) map[string]any {
	return map[string]any{
		"limit":    options.Limit,
		"project":  options.Project,
		"search":   options.Search,
		"state":    options.State,
		"order_by": options.OrderBy,
		"sort":     options.Sort,
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func boolPtrFromInput(input map[string]any, key string) *bool {
	value, ok := input[key].(bool)
	if !ok {
		return nil
	}
	return &value
}

func lookupSearchTerms(input LookupInput) []string {
	return pluginbinding.FilterLookupTerms(input, 3, func(term string) bool {
		return !strings.Contains(term, "://") && !strings.Contains(term, "/-/")
	})
}

func lookupProjectCandidates(input LookupInput) []string {
	seen := map[string]bool{}
	var out []string
	add := func(value string) {
		value = strings.Trim(strings.TrimSpace(value), "/")
		if value == "" || !strings.Contains(value, "/") || seen[value] {
			return
		}
		seen[value] = true
		out = append(out, value)
	}
	for _, term := range pluginbinding.LookupTerms(input) {
		if path := gitlabURLPath(term); path != "" {
			if project, _, ok := strings.Cut(path, "/-/"); ok {
				add(project)
			} else {
				add(path)
			}
			continue
		}
		if strings.Contains(term, "/") && !strings.Contains(term, "://") {
			add(term)
		}
	}
	return out
}

func lookupMergeRequestRefs(input LookupInput) []string {
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
		if strings.Contains(term, "!") {
			add(term)
			continue
		}
		path := gitlabURLPath(term)
		if path == "" {
			continue
		}
		project, rest, ok := strings.Cut(path, "/-/merge_requests/")
		if !ok {
			continue
		}
		iid := strings.Trim(strings.TrimSpace(rest), "/")
		if idx := strings.IndexByte(iid, '/'); idx >= 0 {
			iid = iid[:idx]
		}
		add(project + "!" + iid)
	}
	return out
}

func gitlabURLPath(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return strings.Trim(parsed.Path, "/")
}

func projectLookupValues(record ProjectRecord) map[string]string {
	return map[string]string{
		"id":                            record.ID,
		"title":                         record.Title,
		"links.self":                    record.Links["self"],
		"record.name":                   record.Name,
		"record.name_with_namespace":    record.NameWithNamespace,
		"record.path_with_namespace":    record.PathWithNamespace,
		"record.web_url":                record.WebURL,
		"record.default_branch":         record.DefaultBranch,
		"record.visibility":             record.Visibility,
		"record.last_activity_at":       record.LastActivityAt,
		"record.gitlab_project_id_text": strconv.FormatInt(record.ProjectID, 10),
	}
}

func userLookupValues(record UserRecord) map[string]string {
	return map[string]string{
		"id":              record.ID,
		"title":           record.Title,
		"links.self":      record.Links["self"],
		"record.username": record.Username,
		"record.name":     record.Name,
		"record.email":    record.Email,
		"record.state":    record.State,
		"record.web_url":  record.WebURL,
		"record.user_id":  strconv.FormatInt(record.UserID, 10),
	}
}

func groupLookupValues(record GroupRecord) map[string]string {
	return map[string]string{
		"id":                 record.ID,
		"title":              record.Title,
		"links.self":         record.Links["self"],
		"record.name":        record.Name,
		"record.full_name":   record.FullName,
		"record.full_path":   record.FullPath,
		"record.description": record.Description,
		"record.visibility":  record.Visibility,
		"record.web_url":     record.WebURL,
		"record.group_id":    strconv.FormatInt(record.GroupID, 10),
	}
}

func issueLookupValues(record IssueRecord) map[string]string {
	return map[string]string{
		"id":                       record.ID,
		"title":                    record.Title,
		"links.self":               record.Links["self"],
		"record.reference":         record.Reference,
		"record.author_username":   record.AuthorUsername,
		"record.state":             record.State,
		"record.web_url":           record.WebURL,
		"record.issue_id":          strconv.FormatInt(record.IssueID, 10),
		"record.iid":               strconv.FormatInt(record.IID, 10),
		"record.gitlab_project_id": strconv.FormatInt(record.ProjectID, 10),
	}
}

func mergeRequestLookupValues(record MergeRequestRecord) map[string]string {
	return map[string]string{
		"id":                       record.ID,
		"title":                    record.Title,
		"links.self":               record.Links["self"],
		"record.reference":         record.Reference,
		"record.author_username":   record.AuthorUsername,
		"record.state":             record.State,
		"record.source_branch":     record.SourceBranch,
		"record.target_branch":     record.TargetBranch,
		"record.web_url":           record.WebURL,
		"record.merge_request_id":  strconv.FormatInt(record.MergeRequestID, 10),
		"record.iid":               strconv.FormatInt(record.IID, 10),
		"record.gitlab_project_id": strconv.FormatInt(record.ProjectID, 10),
	}
}

func parseMergeRequestRef(ref string) (string, int64, error) {
	project, iidText, ok := strings.Cut(strings.TrimSpace(ref), "!")
	if !ok || strings.TrimSpace(project) == "" || strings.TrimSpace(iidText) == "" {
		return "", 0, fmt.Errorf("merge request ref must be PROJECT!IID")
	}
	iid, err := strconv.ParseInt(strings.TrimSpace(iidText), 10, 64)
	if err != nil || iid <= 0 {
		return "", 0, fmt.Errorf("merge request IID must be a positive integer")
	}
	return strings.TrimSpace(project), iid, nil
}

func projectID(value string) any {
	value = strings.TrimSpace(value)
	if id, err := strconv.ParseInt(value, 10, 64); err == nil {
		return id
	}
	return value
}

func flexibleJSONString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return strings.TrimSpace(typed.String())
	case float64:
		if typed == 0 {
			return ""
		}
		return strconv.FormatInt(int64(typed), 10)
	case int:
		if typed == 0 {
			return ""
		}
		return strconv.Itoa(typed)
	default:
		return ""
	}
}

func int64FromInput(input map[string]any, keys ...string) int64 {
	for _, key := range keys {
		switch value := input[key].(type) {
		case json.Number:
			if parsed, err := strconv.ParseInt(strings.TrimSpace(value.String()), 10, 64); err == nil {
				return parsed
			}
		case float64:
			return int64(value)
		case int:
			return int64(value)
		case int64:
			return value
		case string:
			if parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func int64SliceFromInput(input map[string]any, key string) []int64 {
	switch value := input[key].(type) {
	case []int64:
		return append([]int64(nil), value...)
	case []int:
		out := make([]int64, 0, len(value))
		for _, item := range value {
			out = append(out, int64(item))
		}
		return out
	case []float64:
		out := make([]int64, 0, len(value))
		for _, item := range value {
			out = append(out, int64(item))
		}
		return out
	case []any:
		out := make([]int64, 0, len(value))
		for _, item := range value {
			parsed := int64FromInput(map[string]any{"value": item}, "value")
			if parsed > 0 {
				out = append(out, parsed)
			}
		}
		return out
	case string:
		parts := strings.Split(value, ",")
		out := make([]int64, 0, len(parts))
		for _, part := range parts {
			if parsed, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64); err == nil && parsed > 0 {
				out = append(out, parsed)
			}
		}
		return out
	default:
		return nil
	}
}

func stringSliceFromInput(input map[string]any, key string) []string {
	switch value := input[key].(type) {
	case []string:
		return append([]string(nil), value...)
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			if text := flexibleJSONString(item); text != "" {
				out = append(out, text)
			}
		}
		return out
	case string:
		parts := strings.Split(value, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			if text := strings.TrimSpace(part); text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}
