package gitlab

import (
	"fmt"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

func BuildContext(_ pluginbinding.Context, input pluginbinding.ContextBuildInput) (pluginbinding.ContextBuildResult, error) {
	query := strings.TrimSpace(input.Query)
	lines := []string{
		"GitLab plugin context.",
		"Read operations: projects, merge requests, auth test, index build.",
		"Indexed entities: project, user, group, issue, merge request.",
		"Use datasource lookup for pasted GitLab URLs, project paths, users, issues, and merge requests.",
	}
	if query != "" {
		lines = append(lines, fmt.Sprintf("Query: %s", query))
	}
	return pluginbinding.ContextBuildResult{
		Blocks: []core.ContextBlock{{
			ID:       ContextName,
			Kind:     pluginbinding.ContextKindText,
			Title:    "GitLab context",
			Content:  strings.Join(lines, "\n"),
			Priority: 40,
			Metadata: map[string]string{
				"operations":  strings.Join([]string{OperationProjectList, OperationProjectShow, OperationMRList, OperationMRShow, OperationIndexBuild}, ","),
				"datasources": strings.Join([]string{DatasourceProjects, DatasourceUsers, DatasourceGroups, DatasourceIssues, DatasourceMergeRequests}, ","),
			},
		}},
	}, nil
}
