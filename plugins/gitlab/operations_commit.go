package gitlab

import (
	"fmt"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type CommitCreateInput struct {
	Project       string             `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID     string             `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path          string             `json:"path,omitempty" jsonschema:"description=Alias for project"`
	Branch        string             `json:"branch,omitempty" jsonschema:"description=Target branch"`
	CommitMessage string             `json:"commit_message,omitempty" jsonschema:"description=Commit message"`
	Actions       []CommitFileAction `json:"actions,omitempty" jsonschema:"description=File actions for this commit"`
	StartBranch   string             `json:"start_branch,omitempty" jsonschema:"description=Optional source branch"`
	StartSHA      string             `json:"start_sha,omitempty" jsonschema:"description=Optional source SHA"`
	StartProject  string             `json:"start_project,omitempty" jsonschema:"description=Optional source project"`
	AuthorEmail   string             `json:"author_email,omitempty" jsonschema:"description=Commit author email"`
	AuthorName    string             `json:"author_name,omitempty" jsonschema:"description=Commit author name"`
	Force         *bool              `json:"force,omitempty" jsonschema:"description=Whether to force update the branch"`
}

type CommitFileAction struct {
	Action          string `json:"action,omitempty" jsonschema:"description=File action,enum=create,enum=update,enum=delete,enum=move,enum=chmod"`
	FilePath        string `json:"file_path,omitempty" jsonschema:"description=File path"`
	PreviousPath    string `json:"previous_path,omitempty" jsonschema:"description=Previous path for move"`
	Content         string `json:"content,omitempty" jsonschema:"description=File content"`
	Encoding        string `json:"encoding,omitempty" jsonschema:"description=Content encoding"`
	LastCommitID    string `json:"last_commit_id,omitempty" jsonschema:"description=Expected last commit id"`
	ExecuteFilemode *bool  `json:"execute_filemode,omitempty" jsonschema:"description=Whether the file should be executable"`
}

type CommitCreateOptions struct {
	Project       string
	Branch        string
	CommitMessage string
	Actions       []CommitFileAction
	StartBranch   string
	StartSHA      string
	StartProject  string
	AuthorEmail   string
	AuthorName    string
	Force         *bool
}

type Commit struct {
	ID            string `json:"id,omitempty"`
	ShortID       string `json:"short_id,omitempty"`
	Title         string `json:"title,omitempty"`
	Message       string `json:"message,omitempty"`
	AuthorName    string `json:"author_name,omitempty"`
	AuthorEmail   string `json:"author_email,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
	CommittedDate string `json:"committed_date,omitempty"`
	WebURL        string `json:"web_url,omitempty"`
}

func (s Service) CommitCreate(ctx pluginbinding.Context, input CommitCreateInput) (Commit, error) {
	client, err := s.client(ctx)
	if err != nil {
		return Commit{}, pluginbinding.Errorf("secret", "%s", err)
	}
	options, err := commitCreateOptionsFromInput(input)
	if err != nil {
		return Commit{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	commit, err := client.CreateCommit(projectID(options.Project), options)
	if err != nil {
		return Commit{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return commit, nil
}

func commitCreateOptionsFromInput(input CommitCreateInput) (CommitCreateOptions, error) {
	project := strings.TrimSpace(firstNonEmpty(input.Project, input.ProjectID, input.Path))
	if project == "" {
		return CommitCreateOptions{}, fmt.Errorf("project is required")
	}
	branch := strings.TrimSpace(input.Branch)
	if branch == "" {
		return CommitCreateOptions{}, fmt.Errorf("branch is required")
	}
	commitMessage := strings.TrimSpace(input.CommitMessage)
	if commitMessage == "" {
		return CommitCreateOptions{}, fmt.Errorf("commit_message is required")
	}
	if len(input.Actions) == 0 {
		return CommitCreateOptions{}, fmt.Errorf("actions is required")
	}
	actions := make([]CommitFileAction, 0, len(input.Actions))
	for i, action := range input.Actions {
		normalized := normalizeCommitFileAction(action)
		if err := validateCommitFileAction(normalized); err != nil {
			return CommitCreateOptions{}, fmt.Errorf("actions[%d]: %w", i, err)
		}
		actions = append(actions, normalized)
	}
	return CommitCreateOptions{
		Project:       project,
		Branch:        branch,
		CommitMessage: commitMessage,
		Actions:       actions,
		StartBranch:   strings.TrimSpace(input.StartBranch),
		StartSHA:      strings.TrimSpace(input.StartSHA),
		StartProject:  strings.TrimSpace(input.StartProject),
		AuthorEmail:   strings.TrimSpace(input.AuthorEmail),
		AuthorName:    strings.TrimSpace(input.AuthorName),
		Force:         input.Force,
	}, nil
}

func normalizeCommitFileAction(action CommitFileAction) CommitFileAction {
	action.Action = strings.ToLower(strings.TrimSpace(action.Action))
	action.FilePath = strings.TrimSpace(action.FilePath)
	action.PreviousPath = strings.TrimSpace(action.PreviousPath)
	action.Encoding = strings.TrimSpace(action.Encoding)
	action.LastCommitID = strings.TrimSpace(action.LastCommitID)
	return action
}

func validateCommitFileAction(action CommitFileAction) error {
	switch action.Action {
	case "create", "update", "delete", "move", "chmod":
	case "":
		return fmt.Errorf("action is required")
	default:
		return fmt.Errorf("unsupported action %q", action.Action)
	}
	if action.FilePath == "" {
		return fmt.Errorf("file_path is required")
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
