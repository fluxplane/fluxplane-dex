package gitlab

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/plugins/internal/pluginutil"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

type OperationRunner struct {
	SecretGetter  SecretGetter
	ClientFactory ClientFactory
}

func NewOperationRunner() OperationRunner {
	return OperationRunner{SecretGetter: defaultSecretGetter, ClientFactory: NewLiveClient}
}

func (r OperationRunner) Run(req protocol.Request, call protocol.OperationCall, cache map[string]pluginutil.SecretMaterial) protocol.OperationResult {
	raw, err := json.Marshal(call)
	if err != nil {
		return pluginbinding.OperationError(call, "marshal_call", err.Error())
	}
	req.Command = protocol.CommandOperationsCall
	req.Payload = raw
	plugin := NewPluginWithRunner(r)
	if cache != nil {
		plugin.Command(protocol.CommandOperationsCall, func(ctx pluginbinding.Context) protocol.Response {
			ctx.Cache.Set(secretCacheKey, cache)
			decoded, err := protocol.DecodePayload[protocol.OperationCall](ctx.Request.Payload)
			if err != nil {
				return protocol.Fail("bad_payload", err.Error())
			}
			result := plugin.RunOperation(ctx.Request, decoded, ctx.Cache)
			if !result.OK {
				return protocol.Response{Protocol: protocol.Version, OK: false, Error: result.Error}
			}
			return protocol.Response{Protocol: protocol.Version, OK: true, Result: result.Result}
		})
	}
	resp := plugin.Handle(req)
	if resp.OK {
		if call.ID == "" {
			call.ID = call.Name
		}
		return protocol.OperationResult{ID: call.ID, Name: call.Name, OK: true, Result: resp.Result}
	}
	errResp := resp.Error
	if errResp == nil {
		errResp = &protocol.Error{Code: "plugin_error", Message: "operation failed"}
	}
	return protocol.OperationResult{ID: call.ID, Name: call.Name, OK: false, Error: errResp}
}

func (r OperationRunner) client(req protocol.Request, cache map[string]pluginutil.SecretMaterial) (Client, error) {
	secrets, err := resolveSecrets(req, cache, r.SecretGetter)
	if err != nil {
		return nil, err
	}
	factory := r.ClientFactory
	if factory == nil {
		factory = NewLiveClient
	}
	return factory(secrets)
}

const secretCacheKey = "gitlab.secret_cache"

func secretCache(ctx pluginbinding.Context) map[string]pluginutil.SecretMaterial {
	if ctx.Cache == nil {
		return nil
	}
	if value, ok := ctx.Cache.Get(secretCacheKey); ok {
		if cache, ok := value.(map[string]pluginutil.SecretMaterial); ok {
			return cache
		}
	}
	cache := map[string]pluginutil.SecretMaterial{}
	ctx.Cache.Set(secretCacheKey, cache)
	return cache
}

type NoInput struct{}

type ProjectListInput struct {
	Limit      int    `json:"limit,omitempty" jsonschema:"description=Maximum projects to return"`
	Search     string `json:"search,omitempty" jsonschema:"description=Project search text"`
	Query      string `json:"query,omitempty" jsonschema:"description=Alias for search"`
	OrderBy    string `json:"order_by,omitempty" jsonschema:"description=GitLab order_by value"`
	Sort       string `json:"sort,omitempty" jsonschema:"description=Sort direction,enum=asc|desc"`
	Membership *bool  `json:"membership,omitempty" jsonschema:"description=Limit results to member projects"`
}

type ProjectShowInput struct {
	ID      string `json:"id,omitempty" jsonschema:"description=Numeric project ID or path"`
	Project string `json:"project,omitempty" jsonschema:"description=Alias for id"`
	Path    string `json:"path,omitempty" jsonschema:"description=Alias for id"`
}

type MergeRequestListInput struct {
	Project   string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID string `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path      string `json:"path,omitempty" jsonschema:"description=Alias for project"`
	Limit     int    `json:"limit,omitempty" jsonschema:"description=Maximum merge requests to return"`
	State     string `json:"state,omitempty" jsonschema:"description=Merge request state"`
	Search    string `json:"search,omitempty" jsonschema:"description=Merge request search text"`
	Query     string `json:"query,omitempty" jsonschema:"description=Alias for search"`
	OrderBy   string `json:"order_by,omitempty" jsonschema:"description=GitLab order_by value"`
	Sort      string `json:"sort,omitempty" jsonschema:"description=Sort direction,enum=asc|desc"`
}

type MergeRequestShowInput struct {
	Ref string `json:"ref,omitempty" jsonschema:"description=Merge request reference as PROJECT!IID"`
	ID  string `json:"id,omitempty" jsonschema:"description=Alias for ref"`
}

type IndexBuildInput struct {
	Index            string `json:"index,omitempty" jsonschema:"description=Index selector"`
	Indexes          string `json:"indexes,omitempty" jsonschema:"description=Comma-separated index selectors"`
	Entity           string `json:"entity,omitempty" jsonschema:"description=Entity selector"`
	Entities         string `json:"entities,omitempty" jsonschema:"description=Comma-separated entity selectors"`
	Limit            int    `json:"limit,omitempty" jsonschema:"description=Project fetch limit"`
	Search           string `json:"search,omitempty" jsonschema:"description=Project search text"`
	Query            string `json:"query,omitempty" jsonschema:"description=Alias for search"`
	OrderBy          string `json:"order_by,omitempty" jsonschema:"description=Project order_by value"`
	Sort             string `json:"sort,omitempty" jsonschema:"description=Project sort direction,enum=asc|desc"`
	UserLimit        int    `json:"user_limit,omitempty" jsonschema:"description=User fetch limit"`
	UserSearch       string `json:"user_search,omitempty" jsonschema:"description=User search text"`
	GroupLimit       int    `json:"group_limit,omitempty" jsonschema:"description=Group fetch limit"`
	GroupSearch      string `json:"group_search,omitempty" jsonschema:"description=Group search text"`
	GroupOrderBy     string `json:"group_order_by,omitempty" jsonschema:"description=Group order_by value"`
	GroupSort        string `json:"group_sort,omitempty" jsonschema:"description=Group sort direction,enum=asc|desc"`
	IssueLimit       int    `json:"issue_limit,omitempty" jsonschema:"description=Issue fetch limit"`
	IssueSearch      string `json:"issue_search,omitempty" jsonschema:"description=Issue search text"`
	IssueState       string `json:"issue_state,omitempty" jsonschema:"description=Issue state"`
	IssueOrderBy     string `json:"issue_order_by,omitempty" jsonschema:"description=Issue order_by value"`
	IssueSort        string `json:"issue_sort,omitempty" jsonschema:"description=Issue sort direction,enum=asc|desc"`
	MRProject        string `json:"mr_project,omitempty" jsonschema:"description=Merge request project path or ID"`
	MRLimit          int    `json:"mr_limit,omitempty" jsonschema:"description=Merge request fetch limit"`
	MRSearch         string `json:"mr_search,omitempty" jsonschema:"description=Merge request search text"`
	MRState          string `json:"mr_state,omitempty" jsonschema:"description=Merge request state"`
	MROrderBy        string `json:"mr_order_by,omitempty" jsonschema:"description=Merge request order_by value"`
	MRSort           string `json:"mr_sort,omitempty" jsonschema:"description=Merge request sort direction,enum=asc|desc"`
	Membership       *bool  `json:"membership,omitempty" jsonschema:"description=Limit projects to member projects"`
	ActiveUsers      *bool  `json:"active_users,omitempty" jsonschema:"description=Only include active users"`
	ActiveGroups     *bool  `json:"active_groups,omitempty" jsonschema:"description=Only include active groups"`
	AllVisibleGroups *bool  `json:"all_visible_groups,omitempty" jsonschema:"description=Include all visible groups"`
}

func inputMap(input any) map[string]any {
	raw, err := json.Marshal(input)
	if err != nil {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func (r OperationRunner) AuthTest(ctx pluginbinding.Context, _ NoInput) (map[string]any, error) {
	client, err := r.client(ctx.Request, secretCache(ctx))
	if err != nil {
		return nil, pluginbinding.Errorf("secret", "%s", err)
	}
	user, err := client.CurrentUser()
	if err != nil {
		return nil, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return map[string]any{"text": "GitLab auth OK", "status": "ok", "user": user}, nil
}

func (r OperationRunner) ProjectList(ctx pluginbinding.Context, input ProjectListInput) (map[string]any, error) {
	client, err := r.client(ctx.Request, secretCache(ctx))
	if err != nil {
		return nil, pluginbinding.Errorf("secret", "%s", err)
	}
	projects, err := client.ListProjects(projectListOptions(inputMap(input), 20))
	if err != nil {
		return nil, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return map[string]any{"projects": projects, "count": len(projects)}, nil
}

func (r OperationRunner) ProjectShow(ctx pluginbinding.Context, input ProjectShowInput) (map[string]any, error) {
	client, err := r.client(ctx.Request, secretCache(ctx))
	if err != nil {
		return nil, pluginbinding.Errorf("secret", "%s", err)
	}
	id := strings.TrimSpace(firstString(inputMap(input), "id", "project", "path"))
	if id == "" {
		return nil, pluginbinding.Fail("bad_input", "project id or path is required")
	}
	project, err := client.GetProject(projectID(id))
	if err != nil {
		return nil, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return map[string]any{"project": project}, nil
}

func (r OperationRunner) MergeRequestList(ctx pluginbinding.Context, input MergeRequestListInput) (map[string]any, error) {
	client, err := r.client(ctx.Request, secretCache(ctx))
	if err != nil {
		return nil, pluginbinding.Errorf("secret", "%s", err)
	}
	mrs, err := client.ListMergeRequests(mergeRequestListOptionsFromInput(inputMap(input)))
	if err != nil {
		return nil, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return map[string]any{"merge_requests": mrs, "count": len(mrs)}, nil
}

func (r OperationRunner) IndexBuild(ctx pluginbinding.Context, input IndexBuildInput) (map[string]any, error) {
	client, err := r.client(ctx.Request, secretCache(ctx))
	if err != nil {
		return nil, pluginbinding.Errorf("secret", "%s", err)
	}
	return r.indexBuild(client, inputMap(input))
}

func (r OperationRunner) MergeRequestShow(ctx pluginbinding.Context, input MergeRequestShowInput) (map[string]any, error) {
	client, err := r.client(ctx.Request, secretCache(ctx))
	if err != nil {
		return nil, pluginbinding.Errorf("secret", "%s", err)
	}
	ref := strings.TrimSpace(firstString(inputMap(input), "ref", "id"))
	project, iid, err := parseMergeRequestRef(ref)
	if err != nil {
		return nil, pluginbinding.Errorf("bad_input", "%s", err)
	}
	mr, err := client.GetMergeRequest(projectID(project), iid)
	if err != nil {
		return nil, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return map[string]any{"merge_request": mr, "ref": ref}, nil
}

func (r OperationRunner) indexBuild(client Client, input map[string]any) (map[string]any, error) {
	selector, err := indexBuildSelector(input)
	if err != nil {
		return nil, pluginbinding.Errorf("bad_input", "%s", err)
	}
	var indexes []map[string]any
	var records []ProjectRecord
	if selector.includesIndex("gitlab.projects") {
		options := projectListOptions(input, 100)
		options.All = true
		projects, err := client.ListProjects(options)
		if err != nil {
			return nil, pluginbinding.Errorf("gitlab", "%s", err)
		}
		records = make([]ProjectRecord, 0, len(projects))
		for _, project := range projects {
			records = append(records, normalizeProjectRecord(project))
		}
		indexes = append(indexes, map[string]any{"index": "gitlab.projects", "records": records, "count": len(records), "metadata": indexBuildMetadata("gitlab.project", projectIndexMetadata(options))})
	}
	if selector.includesIndex("gitlab.users") {
		userOptions := userListOptions(input, 100)
		userOptions.All = true
		users, err := client.ListUsers(userOptions)
		if err != nil {
			return nil, pluginbinding.Errorf("gitlab", "%s", err)
		}
		userRecords := make([]UserRecord, 0, len(users))
		for _, user := range users {
			userRecords = append(userRecords, normalizeUserRecord(user))
		}
		indexes = append(indexes, map[string]any{"index": "gitlab.users", "records": userRecords, "count": len(userRecords), "metadata": indexBuildMetadata("gitlab.user", userIndexMetadata(userOptions))})
	}
	if selector.includesIndex("gitlab.groups") {
		groupOptions := groupListOptions(input, 100)
		groupOptions.All = true
		groups, err := client.ListGroups(groupOptions)
		if err != nil {
			return nil, pluginbinding.Errorf("gitlab", "%s", err)
		}
		groupRecords := make([]GroupRecord, 0, len(groups))
		for _, group := range groups {
			groupRecords = append(groupRecords, normalizeGroupRecord(group))
		}
		indexes = append(indexes, map[string]any{"index": "gitlab.groups", "records": groupRecords, "count": len(groupRecords), "metadata": indexBuildMetadata("gitlab.group", groupIndexMetadata(groupOptions))})
	}
	if selector.includesIndex("gitlab.issues") {
		issueOptions := issueListOptions(input, 100)
		issueOptions.All = true
		issues, err := client.ListIssues(issueOptions)
		if err != nil {
			return nil, pluginbinding.Errorf("gitlab", "%s", err)
		}
		issueRecords := make([]IssueRecord, 0, len(issues))
		for _, issue := range issues {
			issueRecords = append(issueRecords, normalizeIssueRecord(issue))
		}
		indexes = append(indexes, map[string]any{"index": "gitlab.issues", "records": issueRecords, "count": len(issueRecords), "metadata": indexBuildMetadata("gitlab.issue", issueIndexMetadata(issueOptions))})
	}
	if selector.includesIndex("gitlab.merge_requests") {
		mrOptions := mergeRequestIndexOptions(input, 100)
		mrOptions.All = true
		mrs, err := client.ListMergeRequests(mrOptions)
		if err != nil {
			return nil, pluginbinding.Errorf("gitlab", "%s", err)
		}
		mrRecords := make([]MergeRequestRecord, 0, len(mrs))
		for _, mr := range mrs {
			mrRecords = append(mrRecords, normalizeMergeRequestRecord(mr))
		}
		indexes = append(indexes, map[string]any{"index": "gitlab.merge_requests", "records": mrRecords, "count": len(mrRecords), "metadata": indexBuildMetadata("gitlab.merge_request", mergeRequestIndexMetadata(mrOptions))})
	}
	firstIndex := ""
	if len(indexes) > 0 {
		firstIndex, _ = indexes[0]["index"].(string)
	}
	return map[string]any{
		"index":   firstIndex,
		"records": records,
		"count":   len(records),
		"indexes": indexes,
	}, nil
}

func projectListOptions(input map[string]any, defaultLimit int) ProjectListOptions {
	limit := intFromInput(input, "limit", defaultLimit)
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > 100 {
		limit = 100
	}
	membership := true
	if raw, ok := input["membership"].(bool); ok {
		membership = raw
	}
	orderBy := strings.TrimSpace(firstString(input, "order_by"))
	if orderBy == "" {
		orderBy = "last_activity_at"
	}
	sort := strings.TrimSpace(firstString(input, "sort"))
	if sort == "" {
		sort = "desc"
	}
	return ProjectListOptions{
		Limit:      limit,
		Search:     strings.TrimSpace(firstString(input, "search", "query")),
		OrderBy:    orderBy,
		Sort:       sort,
		Membership: &membership,
	}
}

func userListOptions(input map[string]any, defaultLimit int) UserListOptions {
	limit := intFromInput(input, "user_limit", defaultLimit)
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > 100 {
		limit = 100
	}
	active := true
	if raw, ok := input["active_users"].(bool); ok {
		active = raw
	}
	return UserListOptions{
		Limit:  limit,
		Search: strings.TrimSpace(firstString(input, "user_search")),
		Active: &active,
	}
}

func groupListOptions(input map[string]any, defaultLimit int) GroupListOptions {
	limit := intFromInput(input, "group_limit", defaultLimit)
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > 100 {
		limit = 100
	}
	active := true
	if raw, ok := input["active_groups"].(bool); ok {
		active = raw
	}
	allVisible := false
	if raw, ok := input["all_visible_groups"].(bool); ok {
		allVisible = raw
	}
	orderBy := strings.TrimSpace(firstString(input, "group_order_by"))
	if orderBy == "" {
		orderBy = "name"
	}
	sort := strings.TrimSpace(firstString(input, "group_sort"))
	if sort == "" {
		sort = "asc"
	}
	return GroupListOptions{
		Limit:      limit,
		Search:     strings.TrimSpace(firstString(input, "group_search")),
		OrderBy:    orderBy,
		Sort:       sort,
		Active:     &active,
		AllVisible: &allVisible,
	}
}

func issueListOptions(input map[string]any, defaultLimit int) IssueListOptions {
	limit := intFromInput(input, "issue_limit", defaultLimit)
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > 100 {
		limit = 100
	}
	state := strings.TrimSpace(firstString(input, "issue_state"))
	if state == "" {
		state = "all"
	}
	orderBy := strings.TrimSpace(firstString(input, "issue_order_by"))
	if orderBy == "" {
		orderBy = "updated_at"
	}
	sort := strings.TrimSpace(firstString(input, "issue_sort"))
	if sort == "" {
		sort = "desc"
	}
	return IssueListOptions{
		Limit:   limit,
		Search:  strings.TrimSpace(firstString(input, "issue_search")),
		State:   state,
		OrderBy: orderBy,
		Sort:    sort,
	}
}

func mergeRequestListOptionsFromInput(input map[string]any) MergeRequestListOptions {
	limit := intFromInput(input, "limit", 20)
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	state := strings.TrimSpace(firstString(input, "state"))
	if state == "" {
		state = "opened"
	}
	orderBy := strings.TrimSpace(firstString(input, "order_by"))
	if orderBy == "" {
		orderBy = "updated_at"
	}
	sort := strings.TrimSpace(firstString(input, "sort"))
	if sort == "" {
		sort = "desc"
	}
	return MergeRequestListOptions{
		Project: strings.TrimSpace(firstString(input, "project", "project_id", "path")),
		Limit:   limit,
		State:   state,
		Search:  strings.TrimSpace(firstString(input, "search", "query")),
		OrderBy: orderBy,
		Sort:    sort,
	}
}

func mergeRequestIndexOptions(input map[string]any, defaultLimit int) MergeRequestListOptions {
	limit := intFromInput(input, "mr_limit", defaultLimit)
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > 100 {
		limit = 100
	}
	state := strings.TrimSpace(firstString(input, "mr_state"))
	if state == "" {
		state = "all"
	}
	orderBy := strings.TrimSpace(firstString(input, "mr_order_by"))
	if orderBy == "" {
		orderBy = "updated_at"
	}
	sort := strings.TrimSpace(firstString(input, "mr_sort"))
	if sort == "" {
		sort = "desc"
	}
	return MergeRequestListOptions{
		Project: strings.TrimSpace(firstString(input, "mr_project", "project", "project_id", "path")),
		Limit:   limit,
		State:   state,
		Search:  strings.TrimSpace(firstString(input, "mr_search")),
		OrderBy: orderBy,
		Sort:    sort,
	}
}

type indexSelector struct {
	indexes map[string]bool
}

func (s indexSelector) includesIndex(index string) bool {
	if len(s.indexes) == 0 {
		return true
	}
	return s.indexes[index]
}

func indexBuildSelector(input map[string]any) (indexSelector, error) {
	values := splitSelectorValues(firstString(input, "index", "indexes"), firstString(input, "entity", "entities"))
	if len(values) == 0 {
		return indexSelector{}, nil
	}
	known := map[string]string{
		"gitlab.projects":       "gitlab.projects",
		"gitlab.project":        "gitlab.projects",
		"project":               "gitlab.projects",
		"projects":              "gitlab.projects",
		"gitlab.users":          "gitlab.users",
		"gitlab.user":           "gitlab.users",
		"user":                  "gitlab.users",
		"users":                 "gitlab.users",
		"gitlab.groups":         "gitlab.groups",
		"gitlab.group":          "gitlab.groups",
		"group":                 "gitlab.groups",
		"groups":                "gitlab.groups",
		"gitlab.issues":         "gitlab.issues",
		"gitlab.issue":          "gitlab.issues",
		"issue":                 "gitlab.issues",
		"issues":                "gitlab.issues",
		"gitlab.merge_requests": "gitlab.merge_requests",
		"gitlab.merge_request":  "gitlab.merge_requests",
		"merge_request":         "gitlab.merge_requests",
		"merge_requests":        "gitlab.merge_requests",
		"mr":                    "gitlab.merge_requests",
		"mrs":                   "gitlab.merge_requests",
	}
	selector := indexSelector{indexes: map[string]bool{}}
	for _, value := range values {
		index, ok := known[value]
		if !ok {
			return indexSelector{}, fmt.Errorf("unknown GitLab index/entity selector %q", value)
		}
		selector.indexes[index] = true
	}
	return selector, nil
}

func splitSelectorValues(values ...string) []string {
	var out []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.ToLower(strings.TrimSpace(part))
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func indexBuildMetadata(entity string, input map[string]any) map[string]any {
	metadata := map[string]any{
		"entity":     entity,
		"source":     "gitlab.index.build",
		"fetch_mode": "all_pages",
	}
	for key, value := range input {
		metadata[key] = value
	}
	return metadata
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

func firstString(input map[string]any, keys ...string) string {
	for _, key := range keys {
		switch value := input[key].(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return value
			}
		case float64:
			if value != 0 {
				return strconv.FormatInt(int64(value), 10)
			}
		}
	}
	return ""
}

func intFromInput(input map[string]any, key string, fallback int) int {
	switch value := input[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			return parsed
		}
	}
	return fallback
}

func opOK(call protocol.OperationCall, value any) protocol.OperationResult {
	raw, _ := json.Marshal(value)
	return protocol.OperationResult{ID: call.ID, Name: call.Name, OK: true, Result: raw}
}

func opError(call protocol.OperationCall, code, message string) protocol.OperationResult {
	return protocol.OperationResult{ID: call.ID, Name: call.Name, OK: false, Error: &protocol.Error{Code: code, Message: message}}
}
