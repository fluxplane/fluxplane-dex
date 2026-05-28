package confluence

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/internal/atlassian"
)

type Service struct {
	SecretGetter  pluginbinding.SecretGetter
	ClientFactory ClientFactory
}

func NewService() Service {
	return Service{ClientFactory: NewLiveClient}
}

func (s Service) client(ctx pluginbinding.Context) (Client, atlassian.Credentials, error) {
	credentials, err := resolveCredentials(ctx)
	if err != nil {
		return nil, atlassian.Credentials{}, err
	}
	factory := s.ClientFactory
	if factory == nil {
		factory = NewLiveClient
	}
	client, err := factory(credentials)
	return client, credentials, err
}

type NoInput struct{}

type LookupInput = pluginbinding.DatasourceLookupInput
type LookupResult = pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]
type PageDatasourceResult = pluginbinding.DatasourceSearchResult[PageRecord]
type UserDatasourceResult = pluginbinding.DatasourceSearchResult[UserRecord]
type PageSearchResult = pluginbinding.ListResult[Page]
type UserSearchResult = pluginbinding.ListResult[User]

type AuthTestResult struct {
	Text   string `json:"text"`
	Status string `json:"status"`
	User   User   `json:"user"`
}

type PageSearchInput struct {
	pluginbinding.DatasourceSearchInput
	CQL      string `json:"cql,omitempty" jsonschema:"description=Confluence CQL query"`
	Title    string `json:"title,omitempty" jsonschema:"description=Exact page title filter"`
	SpaceKey string `json:"space_key,omitempty" jsonschema:"description=Confluence space key filter (e.g. OPS, DEV)"`
	Status   string `json:"status,omitempty" jsonschema:"description=Page status"`
}

type PageShowInput struct {
	ID     string `json:"id,omitempty" jsonschema:"description=Page ID"`
	PageID string `json:"page_id,omitempty" jsonschema:"description=Alias for id"`
}

type PageCreateInput struct {
	SpaceKey    string `json:"space_key,omitempty" jsonschema:"required,description=Confluence space key."`
	Title       string `json:"title,omitempty" jsonschema:"required,description=Page title."`
	BodyStorage string `json:"body_storage,omitempty" jsonschema:"description=Confluence storage-format XHTML body. Defaults to a minimal paragraph."`
	ParentID    string `json:"parent_id,omitempty" jsonschema:"description=Optional parent page ID."`
}

type PageDeleteInput struct {
	ID     string `json:"id,omitempty" jsonschema:"description=Page ID"`
	PageID string `json:"page_id,omitempty" jsonschema:"description=Alias for id"`
}

type AttachmentAddInput struct {
	PageID       string `json:"page_id,omitempty" jsonschema:"required,description=Confluence page ID."`
	ID           string `json:"id,omitempty" jsonschema:"description=Alias for page_id."`
	FilePath     string `json:"file_path,omitempty" jsonschema:"description=Local file path to upload. Mutually exclusive with content_bytes."`
	ContentBytes []byte `json:"content_bytes,omitempty" jsonschema:"description=Base64-encoded inline bytes. Mutually exclusive with file_path."`
	Filename     string `json:"filename,omitempty" jsonschema:"description=Filename shown in Confluence. Defaults to file_path basename."`
	ContentType  string `json:"content_type,omitempty" jsonschema:"description=Attachment MIME type."`
}

type AttachmentListInput struct {
	PageID string `json:"page_id,omitempty" jsonschema:"required,description=Confluence page ID."`
	ID     string `json:"id,omitempty" jsonschema:"description=Alias for page_id."`
}

type AttachmentGetInput struct {
	AttachmentID string `json:"attachment_id,omitempty" jsonschema:"required,description=Confluence attachment ID."`
	PageID       string `json:"page_id,omitempty" jsonschema:"description=Optional page ID used for Confluence's attachment download endpoint."`
	Download     bool   `json:"download,omitempty" jsonschema:"description=Download attachment bytes into content_bytes. Enabled automatically when output_path is set."`
	OutputPath   string `json:"output_path,omitempty" jsonschema:"description=Local file path or directory to write the attachment. If omitted, content_bytes is returned."`
}

type AttachmentDeleteInput struct {
	AttachmentID string `json:"attachment_id,omitempty" jsonschema:"required,description=Confluence attachment ID."`
}

type UserSearchInput struct {
	pluginbinding.DatasourceSearchInput
	CQL string `json:"cql,omitempty" jsonschema:"description=Confluence user CQL query"`
}

type IndexBuildInput struct {
	pluginbinding.IndexBuildInput
	PageLimit int    `json:"page_limit,omitempty" jsonschema:"description=Page fetch page size"`
	PageQuery string `json:"page_query,omitempty" jsonschema:"description=Page text query"`
	PageCQL   string `json:"page_cql,omitempty" jsonschema:"description=Page CQL query"`
	Title     string `json:"title,omitempty" jsonschema:"description=Exact page title filter"`
	SpaceKey  string `json:"space_key,omitempty" jsonschema:"description=Confluence space key filter"`
	UserLimit int    `json:"user_limit,omitempty" jsonschema:"description=User fetch page size"`
	UserQuery string `json:"user_query,omitempty" jsonschema:"description=User search query"`
	UserCQL   string `json:"user_cql,omitempty" jsonschema:"description=User CQL query"`
}

func (s Service) AuthTest(ctx pluginbinding.Context, _ NoInput) (AuthTestResult, error) {
	client, _, err := s.client(ctx)
	if err != nil {
		return AuthTestResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	user, err := client.CurrentUser(context.Background())
	if err != nil {
		return AuthTestResult{}, pluginbinding.Errorf("confluence", "%s", err)
	}
	return AuthTestResult{Text: "Confluence auth OK", Status: "ok", User: user}, nil
}

func (s Service) PageSearch(ctx pluginbinding.Context, input PageSearchInput) (PageSearchResult, error) {
	client, _, err := s.client(ctx)
	if err != nil {
		return PageSearchResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	pages, err := client.SearchPages(context.Background(), pageSearchOptions(pluginbinding.InputMap(input), 20))
	if err != nil {
		return PageSearchResult{}, pluginbinding.Errorf("confluence", "%s", err)
	}
	return pluginbinding.NewListResult(pages), nil
}

func (s Service) PageShow(ctx pluginbinding.Context, input PageShowInput) (pluginbinding.ShowResult[Page], error) {
	client, _, err := s.client(ctx)
	if err != nil {
		return pluginbinding.ShowResult[Page]{}, pluginbinding.Errorf("secret", "%s", err)
	}
	id := strings.TrimSpace(pluginbinding.FirstString(pluginbinding.InputMap(input), "id", "page_id"))
	if id == "" {
		return pluginbinding.ShowResult[Page]{}, pluginbinding.Fail("bad_input", "page id is required")
	}
	page, err := client.GetPage(context.Background(), id)
	if err != nil {
		return pluginbinding.ShowResult[Page]{}, pluginbinding.Errorf("confluence", "%s", err)
	}
	if attachments, attachErr := client.ListPageAttachments(context.Background(), id); attachErr == nil {
		page.Attachments = attachments.Attachments
	}
	return pluginbinding.NewShowResult(page, map[string]any{"id": id}), nil
}

func (s Service) PageCreate(ctx pluginbinding.Context, input PageCreateInput) (PageMutationResult, error) {
	client, _, err := s.client(ctx)
	if err != nil {
		return PageMutationResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	request, err := pageCreateRequest(input)
	if err != nil {
		return PageMutationResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	result, err := client.CreatePage(context.Background(), request)
	if err != nil {
		return PageMutationResult{}, pluginbinding.Errorf("confluence", "%s", err)
	}
	return result, nil
}

func (s Service) PageDelete(ctx pluginbinding.Context, input PageDeleteInput) (PageMutationResult, error) {
	client, _, err := s.client(ctx)
	if err != nil {
		return PageMutationResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	id := strings.TrimSpace(pluginbinding.FirstString(pluginbinding.InputMap(input), "id", "page_id"))
	if id == "" {
		return PageMutationResult{}, pluginbinding.Fail("bad_input", "page id is required")
	}
	result, err := client.DeletePage(context.Background(), id)
	if err != nil {
		return PageMutationResult{}, pluginbinding.Errorf("confluence", "%s", err)
	}
	return result, nil
}

func (s Service) AttachmentAdd(ctx pluginbinding.Context, input AttachmentAddInput) (AttachmentUploadResult, error) {
	client, _, err := s.client(ctx)
	if err != nil {
		return AttachmentUploadResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	pageID := strings.TrimSpace(pluginbinding.FirstString(pluginbinding.InputMap(input), "page_id", "id"))
	if pageID == "" {
		return AttachmentUploadResult{}, pluginbinding.Fail("bad_input", "page_id is required")
	}
	request, err := attachmentUploadRequest(input.FilePath, input.ContentBytes, input.Filename, input.ContentType)
	if err != nil {
		return AttachmentUploadResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	result, err := client.UploadPageAttachment(context.Background(), pageID, request)
	if err != nil {
		return AttachmentUploadResult{}, pluginbinding.Errorf("confluence", "%s", err)
	}
	return result, nil
}

func (s Service) AttachmentList(ctx pluginbinding.Context, input AttachmentListInput) (AttachmentListResult, error) {
	client, _, err := s.client(ctx)
	if err != nil {
		return AttachmentListResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	pageID := strings.TrimSpace(pluginbinding.FirstString(pluginbinding.InputMap(input), "page_id", "id"))
	if pageID == "" {
		return AttachmentListResult{}, pluginbinding.Fail("bad_input", "page_id is required")
	}
	result, err := client.ListPageAttachments(context.Background(), pageID)
	if err != nil {
		return AttachmentListResult{}, pluginbinding.Errorf("confluence", "%s", err)
	}
	return result, nil
}

func (s Service) AttachmentGet(ctx pluginbinding.Context, input AttachmentGetInput) (AttachmentGetResult, error) {
	client, _, err := s.client(ctx)
	if err != nil {
		return AttachmentGetResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	attachmentID := strings.TrimSpace(input.AttachmentID)
	if attachmentID == "" {
		return AttachmentGetResult{}, pluginbinding.Fail("bad_input", "attachment_id is required")
	}
	outputPath := strings.TrimSpace(input.OutputPath)
	result, err := client.GetAttachment(context.Background(), attachmentID, input.PageID, input.Download || outputPath != "")
	if err != nil {
		return AttachmentGetResult{}, pluginbinding.Errorf("confluence", "%s", err)
	}
	if outputPath != "" && len(result.ContentBytes) > 0 {
		path, err := atlassian.WriteAttachment(outputPath, firstNonEmpty(result.Filename, attachmentID), result.ContentBytes)
		if err != nil {
			return AttachmentGetResult{}, pluginbinding.Errorf("filesystem", "%s", err)
		}
		result.OutputPath = path
		result.ContentBytes = nil
	}
	return result, nil
}

func (s Service) AttachmentDelete(ctx pluginbinding.Context, input AttachmentDeleteInput) (AttachmentDeleteResult, error) {
	client, _, err := s.client(ctx)
	if err != nil {
		return AttachmentDeleteResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	attachmentID := strings.TrimSpace(input.AttachmentID)
	if attachmentID == "" {
		return AttachmentDeleteResult{}, pluginbinding.Fail("bad_input", "attachment_id is required")
	}
	result, err := client.DeleteAttachment(context.Background(), attachmentID)
	if err != nil {
		return AttachmentDeleteResult{}, pluginbinding.Errorf("confluence", "%s", err)
	}
	return result, nil
}

func (s Service) UserSearch(ctx pluginbinding.Context, input UserSearchInput) (UserSearchResult, error) {
	client, _, err := s.client(ctx)
	if err != nil {
		return UserSearchResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	users, err := client.SearchUsers(context.Background(), userSearchOptions(pluginbinding.InputMap(input), 20))
	if err != nil {
		return UserSearchResult{}, pluginbinding.Errorf("confluence", "%s", err)
	}
	return pluginbinding.NewListResult(users), nil
}

func (s Service) PageDatasource(ctx pluginbinding.Context, input PageSearchInput) (PageDatasourceResult, error) {
	client, credentials, err := s.client(ctx)
	if err != nil {
		return PageDatasourceResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	pages, err := client.SearchPages(context.Background(), pageSearchOptions(pluginbinding.InputMap(input), 20))
	if err != nil {
		return PageDatasourceResult{}, pluginbinding.Errorf("confluence", "%s", err)
	}
	return pluginbinding.NewDatasourceSearchResult(DatasourcePages, pageSearchDisplayQuery(pluginbinding.InputMap(input)), pageRecords(ctx.DatasourceSource(), credentials.BaseURL, pages)), nil
}

func (s Service) UserDatasource(ctx pluginbinding.Context, input UserSearchInput) (UserDatasourceResult, error) {
	client, _, err := s.client(ctx)
	if err != nil {
		return UserDatasourceResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	users, err := client.SearchUsers(context.Background(), userSearchOptions(pluginbinding.InputMap(input), 20))
	if err != nil {
		return UserDatasourceResult{}, pluginbinding.Errorf("confluence", "%s", err)
	}
	return pluginbinding.NewDatasourceSearchResult(DatasourceUsers, strings.TrimSpace(input.Query), userRecords(ctx.DatasourceSource(), users)), nil
}

func (s Service) IndexBuild(ctx pluginbinding.Context, input IndexBuildInput) (pluginbinding.IndexBuildResult, error) {
	client, credentials, err := s.client(ctx)
	if err != nil {
		return pluginbinding.IndexBuildResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	values := pluginbinding.InputMap(input)
	selector, err := indexBuildSelector(values)
	if err != nil {
		return pluginbinding.IndexBuildResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	pageOptions := pageIndexOptions(values, 100)
	userOptions := userIndexOptions(values, 100)
	return pluginbinding.RunIndexJobs(ctx, selector, "confluence",
		pluginbinding.NewIndexJob(DatasourcePages, EntityPage, OperationIndexBuild, func() ([]Page, error) {
			return client.SearchPages(context.Background(), pageOptions)
		}, func(source pluginbinding.DatasourceSource, page Page) (PageRecord, bool) {
			return normalizePageRecord(source, credentials.BaseURL, page)
		}, pluginbinding.IndexBuildMetadata(EntityPage, OperationIndexBuild, map[string]any{"query": pageOptions.Query, "cql": pageOptions.CQL, "title": pageOptions.Title, "space_key": pageOptions.SpaceKey, "limit": pageOptions.Limit})),
		pluginbinding.NewIndexJob(DatasourceUsers, EntityUser, OperationIndexBuild, func() ([]User, error) {
			return client.SearchUsers(context.Background(), userOptions)
		}, normalizeUserRecord, pluginbinding.IndexBuildMetadata(EntityUser, OperationIndexBuild, map[string]any{"query": userOptions.Query, "cql": userOptions.CQL, "limit": userOptions.Limit})),
	)
}

func (s Service) Lookup(ctx pluginbinding.Context, input LookupInput) (LookupResult, error) {
	client, credentials, err := s.client(ctx)
	if err != nil {
		return LookupResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	var candidates []pluginbinding.LookupCandidate
	if input.Entity == "" || input.Entity == EntityPage {
		for _, id := range lookupPageIDs(input) {
			if page, err := client.GetPage(context.Background(), id); err == nil {
				record, ok := normalizePageRecord(ctx.DatasourceSource(), credentials.BaseURL, page)
				if ok {
					candidates = append(candidates, pluginbinding.NewExactLookupCandidate(ctx.LookupSource(PluginName, DatasourcePages), record.Entity, record.ID, 1200, []string{"page_id"}, record, pageLookupValues(record)))
				}
			}
		}
		for _, term := range lookupSearchTerms(input) {
			pages, err := client.SearchPages(context.Background(), PageSearchOptions{Query: term, Limit: pluginbinding.LookupLimit(input, 20, 100)})
			if err != nil {
				return LookupResult{}, pluginbinding.Errorf("confluence", "%s", err)
			}
			for _, page := range pages {
				record, ok := normalizePageRecord(ctx.DatasourceSource(), credentials.BaseURL, page)
				if ok {
					candidates = append(candidates, pluginbinding.NewLookupCandidate(ctx.LookupSource(PluginName, DatasourcePages), record.Entity, record.ID, record, pageLookupValues(record)))
				}
			}
		}
	}
	if input.Entity == "" || input.Entity == EntityUser {
		for _, term := range lookupSearchTerms(input) {
			users, err := client.SearchUsers(context.Background(), UserSearchOptions{Query: term, Limit: pluginbinding.LookupLimit(input, 20, 100)})
			if err != nil {
				return LookupResult{}, pluginbinding.Errorf("confluence", "%s", err)
			}
			for _, user := range users {
				record, ok := normalizeUserRecord(ctx.DatasourceSource(), user)
				if ok {
					candidates = append(candidates, pluginbinding.NewLookupCandidate(ctx.LookupSource(PluginName, DatasourceUsers), record.Entity, record.ID, record, userLookupValues(record)))
				}
			}
		}
	}
	return pluginbinding.NewDatasourceLookupResultFromCandidates(PluginName, input, candidates), nil
}

func pageSearchOptions(input map[string]any, defaultLimit int) PageSearchOptions {
	return PageSearchOptions{
		Query:   pluginbinding.StringFromInput(input, "query", "search"),
		CQL:     pluginbinding.StringFromInput(input, "cql"),
		Title:   pluginbinding.StringFromInput(input, "title"),
		SpaceKey: pluginbinding.StringFromInput(input, "space_key", "space_id"),
		Status:  pluginbinding.DefaultStringFromInput(input, "current", "status"),
		Limit:   pluginbinding.BoundedIntFromInput(input, "limit", defaultLimit, 100),
	}
}

func pageIndexOptions(input map[string]any, defaultLimit int) PageSearchOptions {
	options := PageSearchOptions{
		Query:   pluginbinding.StringFromInput(input, "page_query", "query"),
		CQL:     pluginbinding.StringFromInput(input, "page_cql", "cql"),
		Title:   pluginbinding.StringFromInput(input, "title"),
		SpaceKey: pluginbinding.StringFromInput(input, "space_key", "space_id"),
		Status:  pluginbinding.DefaultStringFromInput(input, "current", "status"),
		Limit:   pluginbinding.BoundedIntFromInput(input, "page_limit", defaultLimit, 100),
		All:     true,
	}
	return options
}

func userSearchOptions(input map[string]any, defaultLimit int) UserSearchOptions {
	return UserSearchOptions{
		Query: pluginbinding.StringFromInput(input, "query", "search", "user_query"),
		CQL:   pluginbinding.StringFromInput(input, "cql", "user_cql"),
		Limit: pluginbinding.BoundedIntFromInput(input, "limit", defaultLimit, 100),
	}
}

func userIndexOptions(input map[string]any, defaultLimit int) UserSearchOptions {
	return UserSearchOptions{
		Query: pluginbinding.StringFromInput(input, "user_query", "query"),
		CQL:   pluginbinding.StringFromInput(input, "user_cql", "cql"),
		Limit: pluginbinding.BoundedIntFromInput(input, "user_limit", defaultLimit, 100),
		All:   true,
	}
}

func pageCreateRequest(input PageCreateInput) (PageCreateRequest, error) {
	spaceKey := strings.TrimSpace(input.SpaceKey)
	title := strings.TrimSpace(input.Title)
	if spaceKey == "" || title == "" {
		return PageCreateRequest{}, fmt.Errorf("space_key and title are required")
	}
	body := strings.TrimSpace(input.BodyStorage)
	if body == "" {
		body = "<p>Created by dex.</p>"
	}
	return PageCreateRequest{SpaceKey: spaceKey, Title: title, BodyStorage: body, ParentID: strings.TrimSpace(input.ParentID)}, nil
}

func pageCQL(input PageSearchOptions) string {
	if strings.TrimSpace(input.CQL) != "" {
		return strings.TrimSpace(input.CQL)
	}
	var parts []string
	parts = append(parts, "type = page")
	if input.Query != "" {
		parts = append(parts, "text ~ "+cqlString(input.Query))
	}
	if input.Title != "" {
		parts = append(parts, "title ~ "+cqlString(input.Title))
	}
	return strings.Join(parts, " and ")
}

func userCQL(input UserSearchOptions) string {
	if strings.TrimSpace(input.CQL) != "" {
		return strings.TrimSpace(input.CQL)
	}
	return "user.fullname ~ " + cqlString(input.Query)
}

func cqlString(value string) string {
	escaped := strings.ReplaceAll(strings.TrimSpace(value), `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

func pageSearchDisplayQuery(input map[string]any) string {
	if cql := pluginbinding.StringFromInput(input, "cql"); cql != "" {
		return cql
	}
	return pluginbinding.StringFromInput(input, "query", "search", "title")
}

func pageRecords(source pluginbinding.DatasourceSource, baseURL string, pages []Page) []PageRecord {
	records := make([]PageRecord, 0, len(pages))
	for _, page := range pages {
		record, ok := normalizePageRecord(source, baseURL, page)
		if ok {
			records = append(records, record)
		}
	}
	return records
}

func userRecords(source pluginbinding.DatasourceSource, users []User) []UserRecord {
	records := make([]UserRecord, 0, len(users))
	for _, user := range users {
		record, ok := normalizeUserRecord(source, user)
		if ok {
			records = append(records, record)
		}
	}
	return records
}

func indexBuildSelector(input map[string]any) (pluginbinding.IndexSelector, error) {
	known := map[string]string{
		DatasourcePages: DatasourcePages,
		EntityPage:      DatasourcePages,
		"page":          DatasourcePages,
		"pages":         DatasourcePages,
		DatasourceUsers: DatasourceUsers,
		EntityUser:      DatasourceUsers,
		"user":          DatasourceUsers,
		"users":         DatasourceUsers,
	}
	return pluginbinding.NewIndexSelector(input, known, "Confluence")
}

func lookupSearchTerms(input LookupInput) []string {
	return pluginbinding.FilterLookupTerms(input, 3, func(term string) bool {
		return !strings.Contains(term, "://") && !strings.Contains(term, "/pages/")
	})
}

func lookupPageIDs(input LookupInput) []string {
	seen := map[string]bool{}
	var out []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		out = append(out, value)
	}
	for _, term := range pluginbinding.LookupTerms(input) {
		if parsed, err := url.Parse(strings.TrimSpace(term)); err == nil && parsed.Host != "" {
			if matches := pageIDURLPattern.FindStringSubmatch(parsed.Path); len(matches) == 2 {
				add(matches[1])
			}
		}
	}
	return out
}

func pageLookupValues(record PageRecord) map[string]string {
	return map[string]string{
		"id":               record.ID,
		"title":            record.Title,
		"record.page_id":   record.PageID,
		"record.title":     record.Title,
		"record.space_id":  record.SpaceID,
		"record.space_key": record.SpaceKey,
		"record.web_url":   record.WebURL,
		"record.author_id": record.AuthorID,
		"record.updated":   record.UpdatedAt,
	}
}

func userLookupValues(record UserRecord) map[string]string {
	return map[string]string{
		"id":                  record.ID,
		"record.account_id":   record.AccountID,
		"record.display_name": record.DisplayName,
		"record.public_name":  record.PublicName,
		"record.email":        record.Email,
	}
}

func attachmentUploadRequest(filePath string, contentBytes []byte, filename, contentType string) (AttachmentUploadRequest, error) {
	out, err := atlassian.BuildAttachmentUploadRequest(filePath, contentBytes, filename, contentType)
	if err != nil {
		return AttachmentUploadRequest{}, err
	}
	return AttachmentUploadRequest{Filename: out.Filename, ContentType: out.ContentType, Data: out.Data}, nil
}
