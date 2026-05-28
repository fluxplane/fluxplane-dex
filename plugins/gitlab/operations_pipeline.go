package gitlab

import (
	"fmt"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type PipelineCreateInput struct {
	Project   string                     `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID string                     `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path      string                     `json:"path,omitempty" jsonschema:"description=Alias for project"`
	Ref       string                     `json:"ref,omitempty" jsonschema:"description=Git ref to run the pipeline on"`
	Variables []PipelineVariableArgument `json:"variables,omitempty" jsonschema:"description=Pipeline variables"`
}

type PipelineRetryInput struct {
	Project    string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID  string `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path       string `json:"path,omitempty" jsonschema:"description=Alias for project"`
	PipelineID int64  `json:"pipeline_id,omitempty" jsonschema:"description=Pipeline id"`
}

type PipelineCancelInput struct {
	Project    string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID  string `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path       string `json:"path,omitempty" jsonschema:"description=Alias for project"`
	PipelineID int64  `json:"pipeline_id,omitempty" jsonschema:"description=Pipeline id"`
}

type PipelineVariableArgument struct {
	Key          string `json:"key,omitempty" jsonschema:"description=Variable key"`
	Value        string `json:"value,omitempty" jsonschema:"description=Variable value"`
	VariableType string `json:"variable_type,omitempty" jsonschema:"description=Variable type,enum=env_var,enum=file"`
}

type PipelineCreateOptions struct {
	Project   string
	Ref       string
	Variables []PipelineVariableArgument
}

type Pipeline struct {
	ID         int64  `json:"id,omitempty"`
	ProjectID  int64  `json:"project_id,omitempty"`
	Status     string `json:"status,omitempty"`
	Ref        string `json:"ref,omitempty"`
	SHA        string `json:"sha,omitempty"`
	WebURL     string `json:"web_url,omitempty"`
	Source     string `json:"source,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	Duration   int64  `json:"duration,omitempty"`
}

func (s Service) PipelineCreate(ctx pluginbinding.Context, input PipelineCreateInput) (Pipeline, error) {
	client, err := s.client(ctx)
	if err != nil {
		return Pipeline{}, pluginbinding.Errorf("secret", "%s", err)
	}
	options, err := pipelineCreateOptionsFromInput(input)
	if err != nil {
		return Pipeline{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	pipeline, err := client.CreatePipeline(projectID(options.Project), options)
	if err != nil {
		return Pipeline{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return pipeline, nil
}

func (s Service) PipelineRetry(ctx pluginbinding.Context, input PipelineRetryInput) (Pipeline, error) {
	client, err := s.client(ctx)
	if err != nil {
		return Pipeline{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := strings.TrimSpace(firstNonEmpty(input.Project, input.ProjectID, input.Path))
	if project == "" {
		return Pipeline{}, pluginbinding.Fail("bad_input", "project is required")
	}
	if input.PipelineID <= 0 {
		return Pipeline{}, pluginbinding.Fail("bad_input", "pipeline_id must be a positive integer")
	}
	pipeline, err := client.RetryPipeline(projectID(project), input.PipelineID)
	if err != nil {
		return Pipeline{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return pipeline, nil
}

func (s Service) PipelineCancel(ctx pluginbinding.Context, input PipelineCancelInput) (Pipeline, error) {
	client, err := s.client(ctx)
	if err != nil {
		return Pipeline{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := strings.TrimSpace(firstNonEmpty(input.Project, input.ProjectID, input.Path))
	if project == "" {
		return Pipeline{}, pluginbinding.Fail("bad_input", "project is required")
	}
	if input.PipelineID <= 0 {
		return Pipeline{}, pluginbinding.Fail("bad_input", "pipeline_id must be a positive integer")
	}
	pipeline, err := client.CancelPipeline(projectID(project), input.PipelineID)
	if err != nil {
		return Pipeline{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return pipeline, nil
}

func pipelineCreateOptionsFromInput(input PipelineCreateInput) (PipelineCreateOptions, error) {
	project := strings.TrimSpace(firstNonEmpty(input.Project, input.ProjectID, input.Path))
	if project == "" {
		return PipelineCreateOptions{}, fmt.Errorf("project is required")
	}
	ref := strings.TrimSpace(input.Ref)
	if ref == "" {
		return PipelineCreateOptions{}, fmt.Errorf("ref is required")
	}
	variables := make([]PipelineVariableArgument, 0, len(input.Variables))
	for i, variable := range input.Variables {
		key := strings.TrimSpace(variable.Key)
		if key == "" {
			return PipelineCreateOptions{}, fmt.Errorf("variables[%d]: key is required", i)
		}
		variableType, err := validateVariableType(variable.VariableType)
		if err != nil {
			return PipelineCreateOptions{}, fmt.Errorf("variables[%d]: %w", i, err)
		}
		variables = append(variables, PipelineVariableArgument{Key: key, Value: variable.Value, VariableType: variableType})
	}
	return PipelineCreateOptions{Project: project, Ref: ref, Variables: variables}, nil
}
