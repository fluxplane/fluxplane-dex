package confluence

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type User struct {
	AccountID    string `json:"accountId,omitempty"`
	AccountType  string `json:"accountType,omitempty"`
	DisplayName  string `json:"displayName,omitempty"`
	Email        string `json:"email,omitempty"`
	EmailAddress string `json:"emailAddress,omitempty"`
	PublicName   string `json:"publicName,omitempty"`
	ProfileURL   string `json:"profilePicturePath,omitempty"`
	Type         string `json:"type,omitempty"`
}

type UserRecord struct {
	pluginbinding.DatasourceRecord
	AccountID   string `json:"account_id" datasource:"id,completion,view=compact|lookup|table"`
	DisplayName string `json:"display_name,omitempty" datasource:"title,completion,view=compact|lookup|table"`
	PublicName  string `json:"public_name,omitempty" datasource:"completion,view=lookup|table"`
	Email       string `json:"email,omitempty" datasource:"completion,view=lookup|table"`
	AccountType string `json:"account_type,omitempty"`
	ProfileURL  string `json:"profile_url,omitempty"`
}

type Page struct {
	ID          string       `json:"id,omitempty"`
	Status      string       `json:"status,omitempty"`
	Title       string       `json:"title,omitempty"`
	SpaceID     string       `json:"spaceId,omitempty"`
	SpaceKey    string       `json:"spaceKey,omitempty"`
	ParentID    string       `json:"parentId,omitempty"`
	AuthorID    string       `json:"authorId,omitempty"`
	CreatedAt   string       `json:"createdAt,omitempty"`
	Version     PageVersion  `json:"version,omitempty"`
	Links       PageLinks    `json:"_links,omitempty"`
	Body        any          `json:"body,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

type PageVersion struct {
	Number    int    `json:"number,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	AuthorID  string `json:"authorId,omitempty"`
}

type PageLinks struct {
	WebUI    string `json:"webui,omitempty"`
	Self     string `json:"self,omitempty"`
	Base     string `json:"base,omitempty"`
	Next     string `json:"next,omitempty"`
	Download string `json:"download,omitempty"`
}

type PageRecord struct {
	pluginbinding.DatasourceRecord
	PageID        string `json:"page_id" datasource:"id,completion,view=compact|lookup|table"`
	Title         string `json:"title,omitempty" datasource:"title,completion,view=compact|lookup|table"`
	Status        string `json:"status,omitempty" datasource:"completion,view=compact|lookup|table"`
	SpaceID       string `json:"space_id,omitempty" datasource:"completion,view=compact|lookup|table"`
	SpaceKey      string `json:"space_key,omitempty" datasource:"completion,view=compact|lookup|table"`
	ParentID      string `json:"parent_id,omitempty" datasource:"relation=confluence.page:parent"`
	AuthorID      string `json:"author_id,omitempty" datasource:"relation=confluence.user:author"`
	VersionNumber int    `json:"version_number,omitempty" datasource:"view=lookup|table"`
	CreatedAt     string `json:"created_at,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty" datasource:"view=compact|lookup|table"`
	WebURL        string `json:"web_url,omitempty" datasource:"completion,view=lookup|table"`
	Self          string `json:"self,omitempty"`
}

type PageSearchOptions struct {
	Query    string
	CQL      string
	Title    string
	SpaceKey string
	Status   string
	Limit    int
	All      bool
	Cursor   string
}

type PageCreateRequest struct {
	SpaceKey    string
	Title       string
	BodyStorage string
	ParentID    string
}

type PageMutationResult struct {
	OK   bool   `json:"ok"`
	ID   string `json:"id,omitempty"`
	Page *Page  `json:"page,omitempty"`
}

type UserSearchOptions struct {
	Query  string
	CQL    string
	Limit  int
	All    bool
	Cursor string
}

type Attachment struct {
	ID        string      `json:"id,omitempty"`
	Title     string      `json:"title,omitempty"`
	Filename  string      `json:"filename,omitempty"`
	MediaType string      `json:"mediaType,omitempty"`
	FileSize  int64       `json:"fileSize,omitempty"`
	Status    string      `json:"status,omitempty"`
	PageID    string      `json:"pageId,omitempty"`
	Version   PageVersion `json:"version,omitempty"`
	Links     PageLinks   `json:"_links,omitempty"`
}

type AttachmentUploadRequest struct {
	Filename    string
	ContentType string
	Data        []byte
}

type AttachmentUploadResult struct {
	OK          bool         `json:"ok"`
	PageID      string       `json:"page_id,omitempty"`
	Attachments []Attachment `json:"attachments"`
}

type AttachmentListResult struct {
	PageID      string       `json:"page_id,omitempty"`
	Count       int          `json:"count"`
	Attachments []Attachment `json:"attachments"`
}

type AttachmentGetResult struct {
	ID           string     `json:"id"`
	Filename     string     `json:"filename,omitempty"`
	MimeType     string     `json:"mime_type,omitempty"`
	Size         int        `json:"size"`
	ContentBytes []byte     `json:"content_bytes,omitempty"`
	OutputPath   string     `json:"output_path,omitempty"`
	Attachment   Attachment `json:"attachment,omitempty"`
}

type AttachmentDeleteResult struct {
	OK           bool   `json:"ok"`
	AttachmentID string `json:"attachment_id"`
}

type pageListResponse struct {
	Results []Page    `json:"results"`
	Links   PageLinks `json:"_links"`
}

type attachmentListResponse struct {
	Results []Attachment `json:"results"`
	Links   PageLinks    `json:"_links"`
}

type attachmentUploadResponse struct {
	Results []Attachment `json:"results"`
}

type searchResponse struct {
	Results []searchResult `json:"results"`
	Links   PageLinks      `json:"_links"`
}

type searchResult struct {
	Content searchContent `json:"content,omitempty"`
	User    User          `json:"user,omitempty"`
	URL     string        `json:"url,omitempty"`
	Title   string        `json:"title,omitempty"`
}

type searchContent struct {
	ID      string      `json:"id,omitempty"`
	Type    string      `json:"type,omitempty"`
	Status  string      `json:"status,omitempty"`
	Title   string      `json:"title,omitempty"`
	Space   searchSpace `json:"space,omitempty"`
	Links   PageLinks   `json:"_links,omitempty"`
	Version PageVersion `json:"version,omitempty"`
}

type searchSpace struct {
	ID  string `json:"id,omitempty"`
	Key string `json:"key,omitempty"`
}

func normalizePageRecord(source pluginbinding.DatasourceSource, baseURL string, page Page) (PageRecord, bool) {
	id := strings.TrimSpace(page.ID)
	if id == "" {
		return PageRecord{}, false
	}
	webURL := pageWebURL(baseURL, page)
	return PageRecord{
		DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityPage, id, pluginbinding.RecordTitle(page.Title), pluginbinding.RecordLink("self", webURL)),
		PageID:           id,
		Title:            page.Title,
		Status:           page.Status,
		SpaceID:          page.SpaceID,
		SpaceKey:         page.SpaceKey,
		ParentID:         page.ParentID,
		AuthorID:         firstNonEmpty(page.AuthorID, page.Version.AuthorID),
		VersionNumber:    page.Version.Number,
		CreatedAt:        page.CreatedAt,
		UpdatedAt:        page.Version.CreatedAt,
		WebURL:           webURL,
		Self:             page.Links.Self,
	}, true
}

func normalizeUserRecord(source pluginbinding.DatasourceSource, user User) (UserRecord, bool) {
	id := strings.TrimSpace(user.AccountID)
	if id == "" {
		return UserRecord{}, false
	}
	email := firstNonEmpty(user.EmailAddress, user.Email)
	return UserRecord{
		DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityUser, id, pluginbinding.RecordTitle(firstNonEmpty(user.DisplayName, user.PublicName)), pluginbinding.RecordLink("self", user.ProfileURL)),
		AccountID:        id,
		DisplayName:      user.DisplayName,
		PublicName:       user.PublicName,
		Email:            email,
		AccountType:      firstNonEmpty(user.AccountType, user.Type),
		ProfileURL:       user.ProfileURL,
	}, true
}

func pageWebURL(baseURL string, page Page) string {
	if self := strings.TrimSpace(page.Links.Self); self != "" {
		if parsed, err := url.Parse(self); err == nil && parsed.Scheme != "" && parsed.Host != "" {
			if idx := strings.Index(parsed.Path, "/rest/api/"); idx >= 0 {
				basePath := strings.TrimRight(parsed.Path[:idx], "/")
				webPath := "/" + strings.TrimLeft(page.Links.WebUI, "/")
				if basePath != "" && strings.HasPrefix(webPath, basePath+"/") {
					parsed.Path = webPath
				} else {
					parsed.Path = basePath + webPath
				}
				parsed.RawQuery = ""
				parsed.Fragment = ""
				return parsed.String()
			}
		}
	}
	for _, candidate := range []string{page.Links.WebUI, page.Links.Self} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if parsed, err := url.Parse(candidate); err == nil && parsed.Scheme != "" && parsed.Host != "" {
			return candidate
		}
		baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
		if baseURL != "" {
			return baseURL + "/" + strings.TrimLeft(candidate, "/")
		}
	}
	return ""
}

func pagesFromSearch(results []searchResult) []Page {
	pages := make([]Page, 0, len(results))
	for _, result := range results {
		content := result.Content
		if content.ID == "" {
			continue
		}
		page := Page{
			ID:       content.ID,
			Status:   content.Status,
			Title:    firstNonEmpty(content.Title, result.Title),
			SpaceID:  content.Space.ID,
			SpaceKey: content.Space.Key,
			Version:  content.Version,
			Links:    content.Links,
		}
		if page.Links.WebUI == "" {
			page.Links.WebUI = result.URL
		}
		pages = append(pages, page)
	}
	return pages
}

func usersFromSearch(results []searchResult) []User {
	users := make([]User, 0, len(results))
	for _, result := range results {
		if result.User.AccountID != "" {
			users = append(users, result.User)
		}
	}
	return users
}

func cursorFromNext(next string) string {
	parsed, err := url.Parse(strings.TrimSpace(next))
	if err != nil {
		return ""
	}
	return parsed.Query().Get("cursor")
}

func startFromNext(next string) string {
	parsed, err := url.Parse(strings.TrimSpace(next))
	if err != nil {
		return ""
	}
	return parsed.Query().Get("start")
}

func confluenceDownloadURL(attachment Attachment) string {
	download := strings.TrimSpace(attachment.Links.Download)
	if download == "" {
		return ""
	}
	if parsed, err := url.Parse(download); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return parsed.String()
	}
	if base := confluenceSiteBase(attachment.Links.Self); base != "" {
		if strings.HasPrefix(download, "/wiki/") {
			return base + download
		}
		return base + "/wiki/" + strings.TrimLeft(download, "/")
	}
	if strings.HasPrefix(download, "/wiki/") {
		return download
	}
	return "/wiki/" + strings.TrimLeft(download, "/")
}

func confluenceAttachmentPageID(attachment Attachment) string {
	if pageID := strings.TrimSpace(attachment.PageID); pageID != "" {
		return pageID
	}
	for _, candidate := range []string{attachment.Links.Download, attachment.Links.WebUI} {
		parsed, err := url.Parse(strings.TrimSpace(candidate))
		if err != nil {
			continue
		}
		if matches := attachmentDownloadPageIDPattern.FindStringSubmatch(parsed.Path); len(matches) == 2 {
			return matches[1]
		}
		if pageID := parsed.Query().Get("pageId"); strings.TrimSpace(pageID) != "" {
			return strings.TrimSpace(pageID)
		}
	}
	return ""
}

func confluenceSiteBase(self string) string {
	parsed, err := url.Parse(strings.TrimSpace(self))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

var pageIDURLPattern = regexp.MustCompile(`/pages/(\d+)`)
var attachmentDownloadPageIDPattern = regexp.MustCompile(`/download/attachments/(\d+)/`)
