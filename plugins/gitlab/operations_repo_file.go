package gitlab

import (
	"fmt"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type RepoFileCreateInput struct {
	Project         string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID       string `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path            string `json:"path,omitempty" jsonschema:"description=Alias for project"`
	FilePath        string `json:"file_path,omitempty" jsonschema:"description=Repository file path"`
	Branch          string `json:"branch,omitempty" jsonschema:"description=Target branch"`
	Content         string `json:"content,omitempty" jsonschema:"description=File content"`
	CommitMessage   string `json:"commit_message,omitempty" jsonschema:"description=Commit message"`
	StartBranch     string `json:"start_branch,omitempty" jsonschema:"description=Optional source branch when creating target branch"`
	Encoding        string `json:"encoding,omitempty" jsonschema:"description=Content encoding, such as text or base64"`
	AuthorEmail     string `json:"author_email,omitempty" jsonschema:"description=Commit author email"`
	AuthorName      string `json:"author_name,omitempty" jsonschema:"description=Commit author name"`
	ExecuteFilemode *bool  `json:"execute_filemode,omitempty" jsonschema:"description=Whether the file should be executable"`
}

type RepoFileUpdateInput struct {
	Project         string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID       string `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path            string `json:"path,omitempty" jsonschema:"description=Alias for project"`
	FilePath        string `json:"file_path,omitempty" jsonschema:"description=Repository file path"`
	Branch          string `json:"branch,omitempty" jsonschema:"description=Target branch"`
	Content         string `json:"content,omitempty" jsonschema:"description=File content"`
	CommitMessage   string `json:"commit_message,omitempty" jsonschema:"description=Commit message"`
	StartBranch     string `json:"start_branch,omitempty" jsonschema:"description=Optional source branch when creating target branch"`
	Encoding        string `json:"encoding,omitempty" jsonschema:"description=Content encoding, such as text or base64"`
	AuthorEmail     string `json:"author_email,omitempty" jsonschema:"description=Commit author email"`
	AuthorName      string `json:"author_name,omitempty" jsonschema:"description=Commit author name"`
	LastCommitID    string `json:"last_commit_id,omitempty" jsonschema:"description=Expected last commit id"`
	ExecuteFilemode *bool  `json:"execute_filemode,omitempty" jsonschema:"description=Whether the file should be executable"`
}

type RepoFileDeleteInput struct {
	Project       string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID     string `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path          string `json:"path,omitempty" jsonschema:"description=Alias for project"`
	FilePath      string `json:"file_path,omitempty" jsonschema:"description=Repository file path"`
	Branch        string `json:"branch,omitempty" jsonschema:"description=Target branch"`
	CommitMessage string `json:"commit_message,omitempty" jsonschema:"description=Commit message"`
	StartBranch   string `json:"start_branch,omitempty" jsonschema:"description=Optional source branch when creating target branch"`
	AuthorEmail   string `json:"author_email,omitempty" jsonschema:"description=Commit author email"`
	AuthorName    string `json:"author_name,omitempty" jsonschema:"description=Commit author name"`
	LastCommitID  string `json:"last_commit_id,omitempty" jsonschema:"description=Expected last commit id"`
}

type RepoFileCreateOptions struct {
	Project         string
	FilePath        string
	Branch          string
	Content         string
	CommitMessage   string
	StartBranch     string
	Encoding        string
	AuthorEmail     string
	AuthorName      string
	ExecuteFilemode *bool
}

type RepoFileUpdateOptions struct {
	Project         string
	FilePath        string
	Branch          string
	Content         string
	CommitMessage   string
	StartBranch     string
	Encoding        string
	AuthorEmail     string
	AuthorName      string
	LastCommitID    string
	ExecuteFilemode *bool
}

type RepoFileDeleteOptions struct {
	Project       string
	FilePath      string
	Branch        string
	CommitMessage string
	StartBranch   string
	AuthorEmail   string
	AuthorName    string
	LastCommitID  string
}

type RepoFile struct {
	FilePath string `json:"file_path,omitempty"`
	Branch   string `json:"branch,omitempty"`
}

type RepoFileActionResult struct {
	Project  string `json:"project,omitempty"`
	FilePath string `json:"file_path,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Message  string `json:"message,omitempty"`
}

func (s Service) RepoFileCreate(ctx pluginbinding.Context, input RepoFileCreateInput) (RepoFile, error) {
	client, err := s.client(ctx)
	if err != nil {
		return RepoFile{}, pluginbinding.Errorf("secret", "%s", err)
	}
	options, err := repoFileCreateOptionsFromInput(pluginbinding.InputMap(input))
	if err != nil {
		return RepoFile{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	file, err := client.CreateRepositoryFile(projectID(options.Project), options)
	if err != nil {
		return RepoFile{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return file, nil
}

func (s Service) RepoFileUpdate(ctx pluginbinding.Context, input RepoFileUpdateInput) (RepoFile, error) {
	client, err := s.client(ctx)
	if err != nil {
		return RepoFile{}, pluginbinding.Errorf("secret", "%s", err)
	}
	options, err := repoFileUpdateOptionsFromInput(pluginbinding.InputMap(input))
	if err != nil {
		return RepoFile{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	file, err := client.UpdateRepositoryFile(projectID(options.Project), options)
	if err != nil {
		return RepoFile{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return file, nil
}

func (s Service) RepoFileDelete(ctx pluginbinding.Context, input RepoFileDeleteInput) (RepoFileActionResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return RepoFileActionResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	options, err := repoFileDeleteOptionsFromInput(pluginbinding.InputMap(input))
	if err != nil {
		return RepoFileActionResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	if err := client.DeleteRepositoryFile(projectID(options.Project), options); err != nil {
		return RepoFileActionResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return RepoFileActionResult{Project: options.Project, FilePath: options.FilePath, Branch: options.Branch, Message: "repository file deleted"}, nil
}

func repoFileCreateOptionsFromInput(input map[string]any) (RepoFileCreateOptions, error) {
	options := RepoFileCreateOptions{
		Project:         strings.TrimSpace(pluginbinding.FirstString(input, "project", "project_id", "path")),
		FilePath:        strings.TrimSpace(pluginbinding.StringFromInput(input, "file_path")),
		Branch:          strings.TrimSpace(pluginbinding.StringFromInput(input, "branch")),
		Content:         pluginbinding.StringFromInput(input, "content"),
		CommitMessage:   strings.TrimSpace(pluginbinding.StringFromInput(input, "commit_message")),
		StartBranch:     strings.TrimSpace(pluginbinding.StringFromInput(input, "start_branch")),
		Encoding:        strings.TrimSpace(pluginbinding.StringFromInput(input, "encoding")),
		AuthorEmail:     strings.TrimSpace(pluginbinding.StringFromInput(input, "author_email")),
		AuthorName:      strings.TrimSpace(pluginbinding.StringFromInput(input, "author_name")),
		ExecuteFilemode: boolPtrFromInput(input, "execute_filemode"),
	}
	if err := requireRepoFileFields(options.Project, options.FilePath, options.Branch, options.CommitMessage); err != nil {
		return RepoFileCreateOptions{}, err
	}
	if options.Content == "" {
		return RepoFileCreateOptions{}, fmt.Errorf("content is required for create")
	}
	return options, nil
}

func repoFileUpdateOptionsFromInput(input map[string]any) (RepoFileUpdateOptions, error) {
	options := RepoFileUpdateOptions{
		Project:         strings.TrimSpace(pluginbinding.FirstString(input, "project", "project_id", "path")),
		FilePath:        strings.TrimSpace(pluginbinding.StringFromInput(input, "file_path")),
		Branch:          strings.TrimSpace(pluginbinding.StringFromInput(input, "branch")),
		Content:         pluginbinding.StringFromInput(input, "content"),
		CommitMessage:   strings.TrimSpace(pluginbinding.StringFromInput(input, "commit_message")),
		StartBranch:     strings.TrimSpace(pluginbinding.StringFromInput(input, "start_branch")),
		Encoding:        strings.TrimSpace(pluginbinding.StringFromInput(input, "encoding")),
		AuthorEmail:     strings.TrimSpace(pluginbinding.StringFromInput(input, "author_email")),
		AuthorName:      strings.TrimSpace(pluginbinding.StringFromInput(input, "author_name")),
		LastCommitID:    strings.TrimSpace(pluginbinding.StringFromInput(input, "last_commit_id")),
		ExecuteFilemode: boolPtrFromInput(input, "execute_filemode"),
	}
	if err := requireRepoFileFields(options.Project, options.FilePath, options.Branch, options.CommitMessage); err != nil {
		return RepoFileUpdateOptions{}, err
	}
	if options.Content == "" {
		return RepoFileUpdateOptions{}, fmt.Errorf("content is required for update")
	}
	return options, nil
}

func repoFileDeleteOptionsFromInput(input map[string]any) (RepoFileDeleteOptions, error) {
	options := RepoFileDeleteOptions{
		Project:       strings.TrimSpace(pluginbinding.FirstString(input, "project", "project_id", "path")),
		FilePath:      strings.TrimSpace(pluginbinding.StringFromInput(input, "file_path")),
		Branch:        strings.TrimSpace(pluginbinding.StringFromInput(input, "branch")),
		CommitMessage: strings.TrimSpace(pluginbinding.StringFromInput(input, "commit_message")),
		StartBranch:   strings.TrimSpace(pluginbinding.StringFromInput(input, "start_branch")),
		AuthorEmail:   strings.TrimSpace(pluginbinding.StringFromInput(input, "author_email")),
		AuthorName:    strings.TrimSpace(pluginbinding.StringFromInput(input, "author_name")),
		LastCommitID:  strings.TrimSpace(pluginbinding.StringFromInput(input, "last_commit_id")),
	}
	if err := requireRepoFileFields(options.Project, options.FilePath, options.Branch, options.CommitMessage); err != nil {
		return RepoFileDeleteOptions{}, err
	}
	return options, nil
}

func requireRepoFileFields(project, filePath, branch, commitMessage string) error {
	if project == "" {
		return fmt.Errorf("project is required")
	}
	if filePath == "" {
		return fmt.Errorf("file_path is required")
	}
	if branch == "" {
		return fmt.Errorf("branch is required")
	}
	if commitMessage == "" {
		return fmt.Errorf("commit_message is required")
	}
	return nil
}
