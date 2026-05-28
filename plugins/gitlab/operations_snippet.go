package gitlab

import (
	"fmt"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type SnippetCreateInput struct {
	Title       string                `json:"title,omitempty" jsonschema:"description=Snippet title"`
	Description string                `json:"description,omitempty" jsonschema:"description=Snippet description"`
	Visibility  string                `json:"visibility,omitempty" jsonschema:"description=Snippet visibility (defaults to private when omitted),enum=private,enum=internal,enum=public"`
	Files       []SnippetFileArgument `json:"files,omitempty" jsonschema:"description=Snippet files"`
}

type SnippetDeleteInput struct {
	SnippetID int64 `json:"snippet_id,omitempty" jsonschema:"description=Snippet id"`
	ID        int64 `json:"id,omitempty" jsonschema:"description=Alias for snippet_id"`
}

type SnippetFileArgument struct {
	FilePath string `json:"file_path,omitempty" jsonschema:"description=Snippet file path"`
	Content  string `json:"content,omitempty" jsonschema:"description=Snippet file content"`
}

type SnippetCreateOptions struct {
	Title       string
	Description string
	Visibility  string
	Files       []SnippetFileArgument
}

type Snippet struct {
	ID          int64  `json:"id,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Visibility  string `json:"visibility,omitempty"`
	WebURL      string `json:"web_url,omitempty"`
	RawURL      string `json:"raw_url,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

type SnippetActionResult struct {
	SnippetID int64  `json:"snippet_id,omitempty"`
	Message   string `json:"message,omitempty"`
}

func (s Service) SnippetCreate(ctx pluginbinding.Context, input SnippetCreateInput) (Snippet, error) {
	client, err := s.client(ctx)
	if err != nil {
		return Snippet{}, pluginbinding.Errorf("secret", "%s", err)
	}
	options, err := snippetCreateOptionsFromInput(input)
	if err != nil {
		return Snippet{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	snippet, err := client.CreateSnippet(options)
	if err != nil {
		return Snippet{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return snippet, nil
}

func (s Service) SnippetDelete(ctx pluginbinding.Context, input SnippetDeleteInput) (SnippetActionResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return SnippetActionResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	snippetID := input.SnippetID
	if snippetID <= 0 {
		snippetID = input.ID
	}
	if snippetID <= 0 {
		return SnippetActionResult{}, pluginbinding.Fail("bad_input", "snippet_id must be a positive integer")
	}
	if err := client.DeleteSnippet(snippetID); err != nil {
		return SnippetActionResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return SnippetActionResult{SnippetID: snippetID, Message: "snippet deleted"}, nil
}

func snippetCreateOptionsFromInput(input SnippetCreateInput) (SnippetCreateOptions, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return SnippetCreateOptions{}, fmt.Errorf("title is required")
	}
	visibility, err := snippetVisibility(input.Visibility)
	if err != nil {
		return SnippetCreateOptions{}, err
	}
	if len(input.Files) == 0 {
		return SnippetCreateOptions{}, fmt.Errorf("files is required")
	}
	files := make([]SnippetFileArgument, 0, len(input.Files))
	for i, file := range input.Files {
		path := strings.TrimSpace(file.FilePath)
		if path == "" {
			return SnippetCreateOptions{}, fmt.Errorf("files[%d]: file_path is required", i)
		}
		files = append(files, SnippetFileArgument{FilePath: path, Content: file.Content})
	}
	return SnippetCreateOptions{
		Title:       title,
		Description: strings.TrimSpace(input.Description),
		Visibility:  visibility,
		Files:       files,
	}, nil
}

func snippetVisibility(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "private", nil
	}
	switch value {
	case "private", "internal", "public":
		return value, nil
	default:
		return "", fmt.Errorf("invalid visibility %q", value)
	}
}
