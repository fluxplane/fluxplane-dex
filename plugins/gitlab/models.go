package gitlab

import "github.com/fluxplane/fluxplane-dex/core/pluginbinding"

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username,omitempty"`
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
	WebURL   string `json:"web_url,omitempty"`
	State    string `json:"state,omitempty"`
}

type UserRecord struct {
	pluginbinding.DatasourceRecord
	UserID   int64  `json:"user_id" datasource:"id"`
	Username string `json:"username,omitempty" datasource:"completion,view=compact|lookup|table"`
	Name     string `json:"name,omitempty" datasource:"title,completion,view=compact|lookup|table"`
	Email    string `json:"email,omitempty"`
	State    string `json:"state,omitempty"`
	WebURL   string `json:"web_url,omitempty"`
}

type Group struct {
	ID          int64  `json:"id"`
	Name        string `json:"name,omitempty"`
	Path        string `json:"path,omitempty"`
	FullName    string `json:"full_name,omitempty"`
	FullPath    string `json:"full_path,omitempty"`
	Description string `json:"description,omitempty"`
	Visibility  string `json:"visibility,omitempty"`
	WebURL      string `json:"web_url,omitempty"`
	ParentID    int64  `json:"parent_id,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

type GroupRecord struct {
	pluginbinding.DatasourceRecord
	GroupID     int64  `json:"group_id"`
	Name        string `json:"name,omitempty"`
	FullName    string `json:"full_name,omitempty"`
	FullPath    string `json:"full_path,omitempty" datasource:"id,title,completion,view=compact|lookup|table"`
	Description string `json:"description,omitempty"`
	Visibility  string `json:"visibility,omitempty"`
	WebURL      string `json:"web_url,omitempty"`
	ParentID    int64  `json:"parent_id,omitempty" datasource:"relation=gitlab.group:parent"`
	CreatedAt   string `json:"created_at,omitempty"`
}

type Project struct {
	ID                int64    `json:"id"`
	Name              string   `json:"name,omitempty"`
	NameWithNamespace string   `json:"name_with_namespace,omitempty"`
	Path              string   `json:"path,omitempty"`
	PathWithNamespace string   `json:"path_with_namespace,omitempty"`
	Description       string   `json:"description,omitempty"`
	DefaultBranch     string   `json:"default_branch,omitempty"`
	Visibility        string   `json:"visibility,omitempty"`
	SSHURL            string   `json:"ssh_url_to_repo,omitempty"`
	HTTPURL           string   `json:"http_url_to_repo,omitempty"`
	WebURL            string   `json:"web_url,omitempty"`
	Topics            []string `json:"topics,omitempty"`
	Archived          bool     `json:"archived,omitempty"`
	LastActivityAt    string   `json:"last_activity_at,omitempty"`
	UpdatedAt         string   `json:"updated_at,omitempty"`
}

type ProjectRecord struct {
	pluginbinding.DatasourceRecord
	ProjectID         int64    `json:"project_id"`
	Name              string   `json:"name,omitempty" datasource:"completion,view=compact|lookup|table"`
	NameWithNamespace string   `json:"name_with_namespace,omitempty" datasource:"title,completion,view=compact|lookup|table"`
	PathWithNamespace string   `json:"path_with_namespace,omitempty" datasource:"id,completion,view=compact|lookup|table"`
	DefaultBranch     string   `json:"default_branch,omitempty"`
	Visibility        string   `json:"visibility,omitempty"`
	WebURL            string   `json:"web_url,omitempty"`
	Topics            []string `json:"topics,omitempty"`
	Archived          bool     `json:"archived,omitempty"`
	LastActivityAt    string   `json:"last_activity_at,omitempty"`
}

type Issue struct {
	ID             int64    `json:"id"`
	IID            int64    `json:"iid"`
	ProjectID      int64    `json:"project_id"`
	Title          string   `json:"title,omitempty"`
	State          string   `json:"state,omitempty"`
	WebURL         string   `json:"web_url,omitempty"`
	AuthorUsername string   `json:"author_username,omitempty"`
	Labels         []string `json:"labels,omitempty"`
	Reference      string   `json:"reference,omitempty"`
	CreatedAt      string   `json:"created_at,omitempty"`
	UpdatedAt      string   `json:"updated_at,omitempty"`
	ClosedAt       string   `json:"closed_at,omitempty"`
}

type IssueRecord struct {
	pluginbinding.DatasourceRecord
	IssueID        int64    `json:"issue_id"`
	IID            int64    `json:"iid"`
	ProjectID      int64    `json:"project_id" datasource:"relation=gitlab.project:project"`
	Title          string   `json:"title,omitempty" datasource:"title,completion,view=compact|lookup|table"`
	State          string   `json:"state,omitempty"`
	WebURL         string   `json:"web_url,omitempty"`
	AuthorUsername string   `json:"author_username,omitempty"`
	Labels         []string `json:"labels,omitempty"`
	Reference      string   `json:"reference,omitempty"`
	CreatedAt      string   `json:"created_at,omitempty"`
	UpdatedAt      string   `json:"updated_at,omitempty"`
}

type MergeRequest struct {
	ID             int64    `json:"id"`
	IID            int64    `json:"iid"`
	ProjectID      int64    `json:"project_id"`
	Title          string   `json:"title,omitempty"`
	Description    string   `json:"description,omitempty"`
	State          string   `json:"state,omitempty"`
	SourceBranch   string   `json:"source_branch,omitempty"`
	TargetBranch   string   `json:"target_branch,omitempty"`
	WebURL         string   `json:"web_url,omitempty"`
	AuthorUsername string   `json:"author_username,omitempty"`
	Labels         []string `json:"labels,omitempty"`
	Reference      string   `json:"reference,omitempty"`
	SHA            string   `json:"sha,omitempty"`
	Draft          bool     `json:"draft,omitempty"`
	CreatedAt      string   `json:"created_at,omitempty"`
	UpdatedAt      string   `json:"updated_at,omitempty"`
}

type MergeRequestRecord struct {
	pluginbinding.DatasourceRecord
	MergeRequestID int64    `json:"merge_request_id"`
	IID            int64    `json:"iid"`
	ProjectID      int64    `json:"project_id" datasource:"relation=gitlab.project:project"`
	Title          string   `json:"title,omitempty" datasource:"title,completion,view=compact|lookup|table"`
	State          string   `json:"state,omitempty"`
	SourceBranch   string   `json:"source_branch,omitempty"`
	TargetBranch   string   `json:"target_branch,omitempty"`
	WebURL         string   `json:"web_url,omitempty"`
	AuthorUsername string   `json:"author_username,omitempty"`
	Labels         []string `json:"labels,omitempty"`
	Reference      string   `json:"reference,omitempty"`
	Draft          bool     `json:"draft,omitempty"`
	CreatedAt      string   `json:"created_at,omitempty"`
	UpdatedAt      string   `json:"updated_at,omitempty"`
}

type MergeRequestListOptions struct {
	Project string `json:"project,omitempty"`
	Limit   int    `json:"limit,omitempty"`
	All     bool   `json:"all,omitempty"`
	State   string `json:"state,omitempty"`
	Search  string `json:"search,omitempty"`
	OrderBy string `json:"order_by,omitempty"`
	Sort    string `json:"sort,omitempty"`
}

type MergeRequestCreateOptions struct {
	Project            string   `json:"project,omitempty"`
	Title              string   `json:"title,omitempty"`
	SourceBranch       string   `json:"source_branch,omitempty"`
	TargetBranch       string   `json:"target_branch,omitempty"`
	Description        string   `json:"description,omitempty"`
	Labels             []string `json:"labels,omitempty"`
	AssigneeID         int64    `json:"assignee_id,omitempty"`
	AssigneeIDs        []int64  `json:"assignee_ids,omitempty"`
	ReviewerIDs        []int64  `json:"reviewer_ids,omitempty"`
	TargetProjectID    int64    `json:"target_project_id,omitempty"`
	MilestoneID        int64    `json:"milestone_id,omitempty"`
	RemoveSourceBranch *bool    `json:"remove_source_branch,omitempty"`
	Squash             *bool    `json:"squash,omitempty"`
	AllowCollaboration *bool    `json:"allow_collaboration,omitempty"`
}

type MergeRequestApproveOptions struct {
	SHA string `json:"sha,omitempty"`
}

type MergeRequestMergeOptions struct {
	AutoMerge                *bool  `json:"auto_merge,omitempty"`
	MergeCommitMessage       string `json:"merge_commit_message,omitempty"`
	SquashCommitMessage      string `json:"squash_commit_message,omitempty"`
	Squash                   *bool  `json:"squash,omitempty"`
	ShouldRemoveSourceBranch *bool  `json:"should_remove_source_branch,omitempty"`
	SHA                      string `json:"sha,omitempty"`
}

type MergeRequestApproval struct {
	ID                int64  `json:"id"`
	IID               int64  `json:"iid"`
	ProjectID         int64  `json:"project_id"`
	Title             string `json:"title,omitempty"`
	State             string `json:"state,omitempty"`
	MergeStatus       string `json:"merge_status,omitempty"`
	Approved          bool   `json:"approved"`
	ApprovalsRequired int64  `json:"approvals_required,omitempty"`
	ApprovalsLeft     int64  `json:"approvals_left,omitempty"`
	UserHasApproved   bool   `json:"user_has_approved,omitempty"`
	UserCanApprove    bool   `json:"user_can_approve,omitempty"`
	HasApprovalRules  bool   `json:"has_approval_rules,omitempty"`
	CreatedAt         string `json:"created_at,omitempty"`
	UpdatedAt         string `json:"updated_at,omitempty"`
}

type RepositoryTagCreateOptions struct {
	Project string `json:"project,omitempty"`
	TagName string `json:"tag_name,omitempty"`
	Ref     string `json:"ref,omitempty"`
	Message string `json:"message,omitempty"`
}

type RepositoryTag struct {
	Name      string              `json:"name"`
	Message   string              `json:"message,omitempty"`
	Target    string              `json:"target,omitempty"`
	Protected bool                `json:"protected"`
	CreatedAt string              `json:"created_at,omitempty"`
	Commit    RepositoryTagCommit `json:"commit,omitempty"`
}

type RepositoryTagCommit struct {
	ID            string `json:"id,omitempty"`
	ShortID       string `json:"short_id,omitempty"`
	Title         string `json:"title,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
	CommittedDate string `json:"committed_date,omitempty"`
	WebURL        string `json:"web_url,omitempty"`
}

type ProjectListOptions struct {
	Limit      int    `json:"limit,omitempty"`
	All        bool   `json:"all,omitempty"`
	Search     string `json:"search,omitempty"`
	OrderBy    string `json:"order_by,omitempty"`
	Sort       string `json:"sort,omitempty"`
	Membership *bool  `json:"membership,omitempty"`
}

type UserListOptions struct {
	Limit  int    `json:"limit,omitempty"`
	All    bool   `json:"all,omitempty"`
	Search string `json:"search,omitempty"`
	Active *bool  `json:"active,omitempty"`
}

type GroupListOptions struct {
	Limit      int    `json:"limit,omitempty"`
	All        bool   `json:"all,omitempty"`
	Search     string `json:"search,omitempty"`
	OrderBy    string `json:"order_by,omitempty"`
	Sort       string `json:"sort,omitempty"`
	Active     *bool  `json:"active,omitempty"`
	TopLevel   *bool  `json:"top_level,omitempty"`
	AllVisible *bool  `json:"all_visible,omitempty"`
}

type IssueListOptions struct {
	Limit   int    `json:"limit,omitempty"`
	All     bool   `json:"all,omitempty"`
	Search  string `json:"search,omitempty"`
	State   string `json:"state,omitempty"`
	OrderBy string `json:"order_by,omitempty"`
	Sort    string `json:"sort,omitempty"`
}

func normalizeProjectRecord(source pluginbinding.DatasourceSource, project Project) ProjectRecord {
	id := project.PathWithNamespace
	if id == "" {
		id = project.WebURL
	}
	return ProjectRecord{
		DatasourceRecord:  pluginbinding.NewDatasourceRecord(source, EntityProject, id, pluginbinding.RecordTitle(project.NameWithNamespace), pluginbinding.RecordLink("self", project.WebURL)),
		ProjectID:         project.ID,
		Name:              project.Name,
		NameWithNamespace: project.NameWithNamespace,
		PathWithNamespace: project.PathWithNamespace,
		DefaultBranch:     project.DefaultBranch,
		Visibility:        project.Visibility,
		WebURL:            project.WebURL,
		Topics:            project.Topics,
		Archived:          project.Archived,
		LastActivityAt:    project.LastActivityAt,
	}
}

func normalizeGroupRecord(source pluginbinding.DatasourceSource, group Group) GroupRecord {
	id := group.FullPath
	if id == "" {
		id = group.WebURL
	}
	return GroupRecord{
		DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityGroup, id, pluginbinding.RecordTitle(group.FullName), pluginbinding.RecordLink("self", group.WebURL)),
		GroupID:          group.ID,
		Name:             group.Name,
		FullName:         group.FullName,
		FullPath:         group.FullPath,
		Description:      group.Description,
		Visibility:       group.Visibility,
		WebURL:           group.WebURL,
		ParentID:         group.ParentID,
		CreatedAt:        group.CreatedAt,
	}
}

func normalizeUserRecord(source pluginbinding.DatasourceSource, user User) UserRecord {
	id := user.Username
	if id == "" {
		id = user.WebURL
	}
	return UserRecord{
		DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityUser, id, pluginbinding.RecordTitle(user.Name), pluginbinding.RecordLink("self", user.WebURL)),
		UserID:           user.ID,
		Username:         user.Username,
		Name:             user.Name,
		Email:            user.Email,
		State:            user.State,
		WebURL:           user.WebURL,
	}
}

func normalizeIssueRecord(source pluginbinding.DatasourceSource, issue Issue) IssueRecord {
	id := issue.Reference
	if id == "" {
		id = issue.WebURL
	}
	return IssueRecord{
		DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityIssue, id, pluginbinding.RecordTitle(issue.Title), pluginbinding.RecordLink("self", issue.WebURL)),
		IssueID:          issue.ID,
		IID:              issue.IID,
		ProjectID:        issue.ProjectID,
		Title:            issue.Title,
		State:            issue.State,
		WebURL:           issue.WebURL,
		AuthorUsername:   issue.AuthorUsername,
		Labels:           issue.Labels,
		Reference:        issue.Reference,
		CreatedAt:        issue.CreatedAt,
		UpdatedAt:        issue.UpdatedAt,
	}
}

func normalizeMergeRequestRecord(source pluginbinding.DatasourceSource, mr MergeRequest) MergeRequestRecord {
	id := mr.Reference
	if id == "" {
		id = mr.WebURL
	}
	return MergeRequestRecord{
		DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityMergeRequest, id, pluginbinding.RecordTitle(mr.Title), pluginbinding.RecordLink("self", mr.WebURL)),
		MergeRequestID:   mr.ID,
		IID:              mr.IID,
		ProjectID:        mr.ProjectID,
		Title:            mr.Title,
		State:            mr.State,
		SourceBranch:     mr.SourceBranch,
		TargetBranch:     mr.TargetBranch,
		WebURL:           mr.WebURL,
		AuthorUsername:   mr.AuthorUsername,
		Labels:           mr.Labels,
		Reference:        mr.Reference,
		Draft:            mr.Draft,
		CreatedAt:        mr.CreatedAt,
		UpdatedAt:        mr.UpdatedAt,
	}
}
