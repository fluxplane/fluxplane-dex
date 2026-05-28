package gitlab

import (
	"fmt"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type CIVariableCreateInput struct {
	Project          string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID        string `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path             string `json:"path,omitempty" jsonschema:"description=Alias for project"`
	Key              string `json:"key,omitempty" jsonschema:"description=Variable key"`
	Value            string `json:"value,omitempty" jsonschema:"description=Variable value"`
	Description      string `json:"description,omitempty" jsonschema:"description=Variable description"`
	EnvironmentScope string `json:"environment_scope,omitempty" jsonschema:"description=Environment scope"`
	Masked           *bool  `json:"masked,omitempty" jsonschema:"description=Whether the variable is masked"`
	MaskedAndHidden  *bool  `json:"masked_and_hidden,omitempty" jsonschema:"description=Whether the variable is masked and hidden"`
	Protected        *bool  `json:"protected,omitempty" jsonschema:"description=Whether the variable is protected"`
	Raw              *bool  `json:"raw,omitempty" jsonschema:"description=Whether the variable is raw"`
	VariableType     string `json:"variable_type,omitempty" jsonschema:"description=Variable type,enum=env_var,enum=file"`
}

type CIVariableUpdateInput struct {
	Project          string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID        string `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path             string `json:"path,omitempty" jsonschema:"description=Alias for project"`
	Key              string `json:"key,omitempty" jsonschema:"description=Variable key"`
	Value            string `json:"value,omitempty" jsonschema:"description=Variable value"`
	Description      string `json:"description,omitempty" jsonschema:"description=Variable description"`
	EnvironmentScope string `json:"environment_scope,omitempty" jsonschema:"description=Environment scope"`
	Masked           *bool  `json:"masked,omitempty" jsonschema:"description=Whether the variable is masked"`
	Protected        *bool  `json:"protected,omitempty" jsonschema:"description=Whether the variable is protected"`
	Raw              *bool  `json:"raw,omitempty" jsonschema:"description=Whether the variable is raw"`
	VariableType     string `json:"variable_type,omitempty" jsonschema:"description=Variable type,enum=env_var,enum=file"`
}

type CIVariableDeleteInput struct {
	Project          string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID        string `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path             string `json:"path,omitempty" jsonschema:"description=Alias for project"`
	Key              string `json:"key,omitempty" jsonschema:"description=Variable key"`
	EnvironmentScope string `json:"environment_scope,omitempty" jsonschema:"description=Environment scope"`
}

type CIVariableCreateOptions struct {
	Project          string
	Key              string
	Value            string
	Description      string
	EnvironmentScope string
	Masked           *bool
	MaskedAndHidden  *bool
	Protected        *bool
	Raw              *bool
	VariableType     string
}

type CIVariableUpdateOptions struct {
	Project          string
	Key              string
	Value            string
	Description      string
	EnvironmentScope string
	Masked           *bool
	Protected        *bool
	Raw              *bool
	VariableType     string
}

type CIVariableDeleteOptions struct {
	Project          string
	Key              string
	EnvironmentScope string
}

type CIVariable struct {
	Key              string `json:"key,omitempty"`
	Value            string `json:"value,omitempty"`
	VariableType     string `json:"variable_type,omitempty"`
	EnvironmentScope string `json:"environment_scope,omitempty"`
	Description      string `json:"description,omitempty"`
	Protected        bool   `json:"protected,omitempty"`
	Masked           bool   `json:"masked,omitempty"`
	Raw              bool   `json:"raw,omitempty"`
}

type CIVariableActionResult struct {
	Project          string `json:"project,omitempty"`
	Key              string `json:"key,omitempty"`
	EnvironmentScope string `json:"environment_scope,omitempty"`
	Message          string `json:"message,omitempty"`
}

func (s Service) CIVariableCreate(ctx pluginbinding.Context, input CIVariableCreateInput) (CIVariable, error) {
	client, err := s.client(ctx)
	if err != nil {
		return CIVariable{}, pluginbinding.Errorf("secret", "%s", err)
	}
	options, err := ciVariableCreateOptionsFromInput(input)
	if err != nil {
		return CIVariable{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	variable, err := client.CreateCIVariable(projectID(options.Project), options)
	if err != nil {
		return CIVariable{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return variable, nil
}

func (s Service) CIVariableUpdate(ctx pluginbinding.Context, input CIVariableUpdateInput) (CIVariable, error) {
	client, err := s.client(ctx)
	if err != nil {
		return CIVariable{}, pluginbinding.Errorf("secret", "%s", err)
	}
	options, err := ciVariableUpdateOptionsFromInput(input)
	if err != nil {
		return CIVariable{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	variable, err := client.UpdateCIVariable(projectID(options.Project), options.Key, options)
	if err != nil {
		return CIVariable{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return variable, nil
}

func (s Service) CIVariableDelete(ctx pluginbinding.Context, input CIVariableDeleteInput) (CIVariableActionResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return CIVariableActionResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	options, err := ciVariableDeleteOptionsFromInput(input)
	if err != nil {
		return CIVariableActionResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	if err := client.DeleteCIVariable(projectID(options.Project), options.Key, options); err != nil {
		return CIVariableActionResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return CIVariableActionResult{Project: options.Project, Key: options.Key, EnvironmentScope: options.EnvironmentScope, Message: "ci variable deleted"}, nil
}

func ciVariableCreateOptionsFromInput(input CIVariableCreateInput) (CIVariableCreateOptions, error) {
	project := strings.TrimSpace(firstNonEmpty(input.Project, input.ProjectID, input.Path))
	key := strings.TrimSpace(input.Key)
	if project == "" {
		return CIVariableCreateOptions{}, fmt.Errorf("project is required")
	}
	if key == "" {
		return CIVariableCreateOptions{}, fmt.Errorf("key is required")
	}
	if input.Value == "" {
		return CIVariableCreateOptions{}, fmt.Errorf("value is required for create")
	}
	variableType, err := validateVariableType(input.VariableType)
	if err != nil {
		return CIVariableCreateOptions{}, err
	}
	return CIVariableCreateOptions{
		Project:          project,
		Key:              key,
		Value:            input.Value,
		Description:      strings.TrimSpace(input.Description),
		EnvironmentScope: strings.TrimSpace(input.EnvironmentScope),
		Masked:           input.Masked,
		MaskedAndHidden:  input.MaskedAndHidden,
		Protected:        input.Protected,
		Raw:              input.Raw,
		VariableType:     variableType,
	}, nil
}

func ciVariableUpdateOptionsFromInput(input CIVariableUpdateInput) (CIVariableUpdateOptions, error) {
	project := strings.TrimSpace(firstNonEmpty(input.Project, input.ProjectID, input.Path))
	key := strings.TrimSpace(input.Key)
	if project == "" {
		return CIVariableUpdateOptions{}, fmt.Errorf("project is required")
	}
	if key == "" {
		return CIVariableUpdateOptions{}, fmt.Errorf("key is required")
	}
	if input.Value == "" {
		return CIVariableUpdateOptions{}, fmt.Errorf("value is required for update")
	}
	variableType, err := validateVariableType(input.VariableType)
	if err != nil {
		return CIVariableUpdateOptions{}, err
	}
	return CIVariableUpdateOptions{
		Project:          project,
		Key:              key,
		Value:            input.Value,
		Description:      strings.TrimSpace(input.Description),
		EnvironmentScope: strings.TrimSpace(input.EnvironmentScope),
		Masked:           input.Masked,
		Protected:        input.Protected,
		Raw:              input.Raw,
		VariableType:     variableType,
	}, nil
}

func ciVariableDeleteOptionsFromInput(input CIVariableDeleteInput) (CIVariableDeleteOptions, error) {
	project := strings.TrimSpace(firstNonEmpty(input.Project, input.ProjectID, input.Path))
	key := strings.TrimSpace(input.Key)
	if project == "" {
		return CIVariableDeleteOptions{}, fmt.Errorf("project is required")
	}
	if key == "" {
		return CIVariableDeleteOptions{}, fmt.Errorf("key is required")
	}
	return CIVariableDeleteOptions{
		Project:          project,
		Key:              key,
		EnvironmentScope: strings.TrimSpace(input.EnvironmentScope),
	}, nil
}

func validateVariableType(value string) (string, error) {
	value = strings.TrimSpace(value)
	switch value {
	case "", "env_var", "file":
		return value, nil
	default:
		return "", fmt.Errorf("invalid variable_type %q", value)
	}
}
