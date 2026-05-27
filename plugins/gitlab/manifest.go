package gitlab

import "github.com/fluxplane/fluxplane-dex/core"

const PluginName = "gitlab"

func Manifest() core.PluginManifest {
	return core.PluginManifest{
		Name:        PluginName,
		Version:     "0.1.0",
		Description: "GitLab operations, datasources, indexes, and reverse lookups.",
		Aliases:     []string{"gl", "gitlab"},
		Auth: []core.AuthMethod{{
			Name:        "personal_access_token",
			Kind:        "bearer_token",
			Description: "GitLab personal access token resolved by dex secret broker.",
			Env:         []string{"GITLAB_PERSONAL_TOKEN", "GITLAB_ACCESS_TOKEN", "GITLAB_TOKEN"},
			Fields: []core.AuthField{
				{Name: "access_token", Required: true, Sensitive: true, Secret: true, Description: "GitLab personal access token", Env: []string{"GITLAB_PERSONAL_TOKEN", "GITLAB_ACCESS_TOKEN", "GITLAB_TOKEN"}},
				{Name: "gitlab_url", Required: true, Description: "GitLab base URL", Env: []string{"GITLAB_URL"}},
			},
		}},
		Operations: []core.OperationSpec{
			{Name: "gitlab.auth.test", Description: "Test GitLab authentication by fetching the current user.", ReadOnly: true, SecretPurposes: []string{"access_token", "gitlab_url"}},
			{Name: "gitlab.index.build", Description: "Build GitLab index records.", ReadOnly: true, SecretPurposes: []string{"access_token", "gitlab_url"}},
			{Name: "gitlab.project.list", Description: "List accessible GitLab projects.", ReadOnly: true, Compact: true, SecretPurposes: []string{"access_token", "gitlab_url"}},
			{Name: "gitlab.project.show", Description: "Show one GitLab project.", ReadOnly: true, SecretPurposes: []string{"access_token", "gitlab_url"}},
			{Name: "gitlab.mr.list", Description: "List GitLab merge requests.", ReadOnly: true, Compact: true, SecretPurposes: []string{"access_token", "gitlab_url"}},
			{Name: "gitlab.mr.show", Description: "Show one GitLab merge request.", ReadOnly: true, SecretPurposes: []string{"access_token", "gitlab_url"}},
		},
		Datasources: []core.DatasourceSpec{
			{Name: "gitlab.projects", Entity: "gitlab.project", Description: "GitLab projects.", Capabilities: []string{"search", "lookup", "get", "index"}},
			{Name: "gitlab.users", Entity: "gitlab.user", Description: "GitLab users.", Capabilities: []string{"search", "lookup", "get", "index"}},
			{Name: "gitlab.groups", Entity: "gitlab.group", Description: "GitLab groups and namespaces.", Capabilities: []string{"search", "lookup", "get", "index"}},
			{Name: "gitlab.issues", Entity: "gitlab.issue", Description: "GitLab issues.", Capabilities: []string{"search", "lookup", "get", "index"}},
			{Name: "gitlab.merge_requests", Entity: "gitlab.merge_request", Description: "GitLab merge requests.", Capabilities: []string{"search", "lookup", "get", "index"}},
		},
		Context:   []core.ContextSpec{{Name: "gitlab.context", Description: "GitLab context blocks.", Kinds: []string{"text", "reference"}}},
		Endpoints: []core.EndpointSpec{{Name: "gitlab.endpoint", Description: "Configured GitLab API endpoint.", Products: []string{"gitlab"}}},
		Indexes: []core.IndexSpec{
			{Name: "gitlab.projects", Description: "Project metadata and reverse lookup index.", Entities: []string{"gitlab.project"}},
			{Name: "gitlab.users", Description: "User metadata and reverse lookup index.", Entities: []string{"gitlab.user"}},
			{Name: "gitlab.groups", Description: "Group and namespace reverse lookup index.", Entities: []string{"gitlab.group"}},
			{Name: "gitlab.issues", Description: "Issue metadata and reverse lookup index.", Entities: []string{"gitlab.issue"}},
			{Name: "gitlab.merge_requests", Description: "Merge request metadata and reverse lookup index.", Entities: []string{"gitlab.merge_request"}},
		},
	}
}
