package gitlab

import (
	"fmt"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type BranchCreateInput struct {
	Project   string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID string `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path      string `json:"path,omitempty" jsonschema:"description=Alias for project"`
	Branch    string `json:"branch,omitempty" jsonschema:"description=Branch name to create"`
	Name      string `json:"name,omitempty" jsonschema:"description=Alias for branch"`
	Ref       string `json:"ref,omitempty" jsonschema:"description=Source ref (commit SHA\\, branch\\, or tag)"`
}

type BranchDeleteInput struct {
	Project   string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID string `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path      string `json:"path,omitempty" jsonschema:"description=Alias for project"`
	Branch    string `json:"branch,omitempty" jsonschema:"description=Branch name to delete"`
	Name      string `json:"name,omitempty" jsonschema:"description=Alias for branch"`
}

type BranchDeleteMergedInput struct {
	Project   string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID string `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path      string `json:"path,omitempty" jsonschema:"description=Alias for project"`
}

type BranchCreateOptions struct {
	Project string `json:"project,omitempty"`
	Branch  string `json:"branch,omitempty"`
	Ref     string `json:"ref,omitempty"`
}

type Branch struct {
	Name               string `json:"name,omitempty"`
	WebURL             string `json:"web_url,omitempty"`
	Merged             bool   `json:"merged,omitempty"`
	Protected          bool   `json:"protected,omitempty"`
	Default            bool   `json:"default,omitempty"`
	CanPush            bool   `json:"can_push,omitempty"`
	DevelopersCanPush  bool   `json:"developers_can_push,omitempty"`
	DevelopersCanMerge bool   `json:"developers_can_merge,omitempty"`
}

type BranchActionResult struct {
	Project string `json:"project,omitempty"`
	Branch  string `json:"branch,omitempty"`
	WebURL  string `json:"web_url,omitempty"`
	Message string `json:"message,omitempty"`
}

func (s Service) BranchCreate(ctx pluginbinding.Context, input BranchCreateInput) (Branch, error) {
	client, err := s.client(ctx)
	if err != nil {
		return Branch{}, pluginbinding.Errorf("secret", "%s", err)
	}
	options, err := branchCreateOptionsFromInput(pluginbinding.InputMap(input))
	if err != nil {
		return Branch{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	branch, err := client.CreateBranch(projectID(options.Project), options)
	if err != nil {
		return Branch{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return branch, nil
}

func (s Service) BranchDelete(ctx pluginbinding.Context, input BranchDeleteInput) (BranchActionResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return BranchActionResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	values := pluginbinding.InputMap(input)
	project := strings.TrimSpace(pluginbinding.FirstString(values, "project", "project_id", "path"))
	branch := strings.TrimSpace(pluginbinding.FirstString(values, "branch", "name"))
	if project == "" {
		return BranchActionResult{}, pluginbinding.Fail("bad_input", "project is required")
	}
	if branch == "" {
		return BranchActionResult{}, pluginbinding.Fail("bad_input", "branch is required")
	}
	if err := client.DeleteBranch(projectID(project), branch); err != nil {
		return BranchActionResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return BranchActionResult{Project: project, Branch: branch, Message: "branch deleted"}, nil
}

func (s Service) BranchDeleteMerged(ctx pluginbinding.Context, input BranchDeleteMergedInput) (BranchActionResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return BranchActionResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	values := pluginbinding.InputMap(input)
	project := strings.TrimSpace(pluginbinding.FirstString(values, "project", "project_id", "path"))
	if project == "" {
		return BranchActionResult{}, pluginbinding.Fail("bad_input", "project is required")
	}
	if err := client.DeleteMergedBranches(projectID(project)); err != nil {
		return BranchActionResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return BranchActionResult{Project: project, Message: "merged branches deletion requested"}, nil
}

func branchCreateOptionsFromInput(input map[string]any) (BranchCreateOptions, error) {
	options := BranchCreateOptions{
		Project: strings.TrimSpace(pluginbinding.FirstString(input, "project", "project_id", "path")),
		Branch:  strings.TrimSpace(pluginbinding.FirstString(input, "branch", "name")),
		Ref:     strings.TrimSpace(pluginbinding.StringFromInput(input, "ref")),
	}
	if options.Project == "" {
		return BranchCreateOptions{}, fmt.Errorf("project is required")
	}
	if options.Branch == "" {
		return BranchCreateOptions{}, fmt.Errorf("branch is required")
	}
	if options.Ref == "" {
		return BranchCreateOptions{}, fmt.Errorf("ref is required")
	}
	return options, nil
}
