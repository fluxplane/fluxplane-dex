package slack

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type Service struct {
	ClientFactory ClientFactory
}

func NewService() Service {
	return Service{ClientFactory: NewLiveClient}
}

type IndexBuildInput = pluginbinding.IndexBuildInput

type NoInput struct{}

const (
	SlackRoleBot  = "bot"
	SlackRoleUser = "user"
)

type LookupInput = pluginbinding.DatasourceLookupInput
type LookupResult = pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]
type MessageDatasourceResult = pluginbinding.DatasourceSearchResult[MessageRecord]
type ThreadMessagesDatasourceResult = pluginbinding.DatasourceSearchResult[ThreadMessageRecord]
type ChannelMembersDatasourceResult = pluginbinding.DatasourceSearchResult[ChannelMemberRecord]

type AuthTestResult struct {
	Status string            `json:"status"`
	Count  int               `json:"count"`
	Tokens []TokenInfoResult `json:"tokens,omitempty"`
}

type InfoResult struct {
	Status string            `json:"status"`
	Count  int               `json:"count"`
	Tokens []TokenInfoResult `json:"tokens,omitempty"`
}

type TokenInfoResult struct {
	Role   string `json:"role"`
	Source string `json:"source,omitempty"`
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
	AuthInfo
}

type SlackRoleInput struct {
	Role string `json:"role,omitempty" jsonschema:"description=Slack token role to use for this write operation,enum=bot,enum=user"`
}

type ListInput struct {
	Query string `json:"query,omitempty" jsonschema:"description=Optional case-insensitive substring filter"`
	Limit int    `json:"limit,omitempty" jsonschema:"description=Maximum records to return"`
}

type UserListInput = ListInput
type ChannelListInput = ListInput

type BookmarkListInput struct {
	Channel string `json:"channel,omitempty" jsonschema:"required,description=Slack channel ID or name"`
	Query   string `json:"query,omitempty" jsonschema:"description=Optional case-insensitive substring filter"`
	Limit   int    `json:"limit,omitempty" jsonschema:"description=Maximum bookmarks to return"`
}

type BookmarkAddInput struct {
	SlackRoleInput
	Channel string `json:"channel,omitempty" jsonschema:"required,description=Slack channel ID or name"`
	Title   string `json:"title,omitempty" jsonschema:"required,description=Bookmark title"`
	Link    string `json:"link,omitempty" jsonschema:"required,description=Bookmark URL"`
	Emoji   string `json:"emoji,omitempty" jsonschema:"description=Optional bookmark emoji"`
}

type BookmarkEditInput struct {
	SlackRoleInput
	Channel    string `json:"channel,omitempty" jsonschema:"required,description=Slack channel ID or name"`
	BookmarkID string `json:"bookmark_id,omitempty" jsonschema:"required,description=Slack bookmark ID"`
	Title      string `json:"title,omitempty" jsonschema:"description=Replacement bookmark title"`
	Link       string `json:"link,omitempty" jsonschema:"description=Replacement bookmark URL"`
	Emoji      string `json:"emoji,omitempty" jsonschema:"description=Replacement bookmark emoji"`
}

type BookmarkDeleteInput struct {
	SlackRoleInput
	Channel    string `json:"channel,omitempty" jsonschema:"required,description=Slack channel ID or name"`
	BookmarkID string `json:"bookmark_id,omitempty" jsonschema:"required,description=Slack bookmark ID"`
}

type EmojiListInput struct {
	Query          string `json:"query,omitempty" jsonschema:"description=Optional case-insensitive emoji name filter"`
	Limit          int    `json:"limit,omitempty" jsonschema:"description=Maximum emoji records to return"`
	Mode           string `json:"mode,omitempty" jsonschema:"description=Emoji source mode. Defaults to custom.,enum=custom,enum=builtin,enum=all"`
	IncludeAliases bool   `json:"include_aliases,omitempty" jsonschema:"description=Include Slack emoji aliases. Defaults to false."`
}

type PresenceGetInput struct {
	User string `json:"user,omitempty" jsonschema:"description=Slack user ID, mention, or name. Empty asks Slack for the authenticated user's presence when supported."`
}

type PresenceSetInput struct {
	SlackRoleInput
	Presence string `json:"presence,omitempty" jsonschema:"required,description=Presence to set, either auto or away,enum=auto,enum=away"`
}

type MessageSendInput struct {
	SlackRoleInput
	Channel        string            `json:"channel,omitempty" jsonschema:"required,description=Slack channel ID or name"`
	Text           string            `json:"text,omitempty" jsonschema:"description=Message text or fallback text when blocks are provided"`
	Markdown       string            `json:"markdown,omitempty" jsonschema:"description=Message markdown rendered as a Block Kit mrkdwn section"`
	Blocks         []json.RawMessage `json:"blocks,omitempty" jsonschema:"description=Raw Slack Block Kit block JSON array"`
	ThreadTS       string            `json:"thread_ts,omitempty" jsonschema:"description=Parent Slack message timestamp for replying in a thread."`
	ReplyBroadcast bool              `json:"reply_broadcast,omitempty" jsonschema:"description=Also broadcast the thread reply into the channel."`
	UnfurlLinks    *bool             `json:"unfurl_links,omitempty" jsonschema:"description=Slack chat.postMessage unfurl_links option"`
	UnfurlMedia    *bool             `json:"unfurl_media,omitempty" jsonschema:"description=Slack chat.postMessage unfurl_media option"`
	Parse          string            `json:"parse,omitempty" jsonschema:"description=Slack chat.postMessage parse option"`
}

type MessageEditInput struct {
	SlackRoleInput
	Ref         string            `json:"ref,omitempty" jsonschema:"description=Slack message reference as URL or channel:timestamp"`
	Channel     string            `json:"channel,omitempty" jsonschema:"description=Slack channel ID or name"`
	TS          string            `json:"ts,omitempty" jsonschema:"description=Slack message timestamp"`
	Text        string            `json:"text,omitempty" jsonschema:"description=Replacement message text or fallback text when blocks are provided"`
	Markdown    string            `json:"markdown,omitempty" jsonschema:"description=Replacement markdown rendered as a Block Kit mrkdwn section"`
	Blocks      []json.RawMessage `json:"blocks,omitempty" jsonschema:"description=Raw Slack Block Kit block JSON array"`
	UnfurlLinks *bool             `json:"unfurl_links,omitempty" jsonschema:"description=Slack chat.update unfurl_links option"`
	UnfurlMedia *bool             `json:"unfurl_media,omitempty" jsonschema:"description=Slack chat.update unfurl_media option"`
	Parse       string            `json:"parse,omitempty" jsonschema:"description=Slack chat.update parse option"`
}

type MessageDeleteInput struct {
	SlackRoleInput
	Ref     string `json:"ref,omitempty" jsonschema:"description=Slack message reference as URL or channel:timestamp"`
	Channel string `json:"channel,omitempty" jsonschema:"description=Slack channel ID or name"`
	TS      string `json:"ts,omitempty" jsonschema:"description=Slack message timestamp"`
}

type ReactionAddInput struct {
	SlackRoleInput
	Ref     string `json:"ref,omitempty" jsonschema:"description=Slack message reference as URL or channel:timestamp"`
	Channel string `json:"channel,omitempty" jsonschema:"description=Slack channel ID or name"`
	TS      string `json:"ts,omitempty" jsonschema:"description=Slack message timestamp"`
	Emoji   string `json:"emoji,omitempty" jsonschema:"required,description=Emoji reaction name without colons"`
}

type ChannelJoinInput struct {
	SlackRoleInput
	Channel string `json:"channel,omitempty" jsonschema:"required,description=Slack public channel ID or name"`
}

type ChannelMarkInput struct {
	SlackRoleInput
	Ref     string `json:"ref,omitempty" jsonschema:"description=Slack message reference as URL or channel:timestamp"`
	Channel string `json:"channel,omitempty" jsonschema:"description=Slack channel ID or name"`
	TS      string `json:"ts,omitempty" jsonschema:"description=Slack message timestamp to mark through"`
}

type FileUploadInput struct {
	SlackRoleInput
	Channel        string `json:"channel,omitempty" jsonschema:"required,description=Slack channel ID or DM ID"`
	ThreadTS       string `json:"thread_ts,omitempty" jsonschema:"description=Slack thread timestamp to upload into"`
	BlobRef        string `json:"blob_ref,omitempty" jsonschema:"description=Host blob ref to upload. Mutually exclusive with content_bytes."`
	ContentBytes   []byte `json:"content_bytes,omitempty" jsonschema:"description=Base64-encoded inline file bytes. Mutually exclusive with blob_ref."`
	Filename       string `json:"filename,omitempty" jsonschema:"description=Filename shown in Slack. Defaults to host blob filename when using blob_ref."`
	InitialComment string `json:"initial_comment,omitempty" jsonschema:"description=Optional message text posted with the file."`
	AltText        string `json:"alt_text,omitempty" jsonschema:"description=Alt text for image uploads."`
}

type FileListInput struct {
	Channel string `json:"channel,omitempty" jsonschema:"description=Slack channel ID or name"`
	User    string `json:"user,omitempty" jsonschema:"description=Slack user ID, mention, or name"`
	Types   string `json:"types,omitempty" jsonschema:"description=Slack file types filter. Defaults to all."`
	Query   string `json:"query,omitempty" jsonschema:"description=Optional case-insensitive substring filter"`
	Limit   int    `json:"limit,omitempty" jsonschema:"description=Maximum files to return"`
}

type FileInfoInput struct {
	FileID string `json:"file_id,omitempty" jsonschema:"required,description=Slack file ID"`
}

type FileDownloadInput struct {
	SlackRoleInput
	FileID   string `json:"file_id,omitempty" jsonschema:"required,description=Slack file ID"`
	BlobRef  string `json:"blob_ref,omitempty" jsonschema:"description=Optional host blob ref for the downloaded file."`
	Filename string `json:"filename,omitempty" jsonschema:"description=Filename to use for the downloaded host blob."`
}

type FileDeleteInput struct {
	SlackRoleInput
	FileID string `json:"file_id,omitempty" jsonschema:"required,description=Slack file ID"`
}

type MessageSendResult struct {
	Channel  string `json:"channel,omitempty"`
	TS       string `json:"ts,omitempty"`
	ThreadTS string `json:"thread_ts,omitempty"`
	Role     string `json:"role,omitempty"`
	OK       bool   `json:"ok"`
}

type UserListResult struct {
	Count int    `json:"count"`
	Users []User `json:"users,omitempty"`
}

type ChannelListResult struct {
	Count    int       `json:"count"`
	Channels []Channel `json:"channels,omitempty"`
}

type BookmarkListResult struct {
	Count     int        `json:"count"`
	Channel   string     `json:"channel,omitempty"`
	Bookmarks []Bookmark `json:"bookmarks,omitempty"`
}

type BookmarkResult struct {
	OK       bool     `json:"ok"`
	Channel  string   `json:"channel,omitempty"`
	Bookmark Bookmark `json:"bookmark,omitempty"`
	Role     string   `json:"role,omitempty"`
}

type BookmarkDeleteResult struct {
	OK         bool   `json:"ok"`
	Channel    string `json:"channel,omitempty"`
	BookmarkID string `json:"bookmark_id,omitempty"`
	Role       string `json:"role,omitempty"`
}

type EmojiListResult struct {
	Count  int     `json:"count"`
	Emojis []Emoji `json:"emojis,omitempty"`
}

type Presence struct {
	Presence        string `json:"presence,omitempty"`
	Online          bool   `json:"online,omitempty"`
	AutoAway        bool   `json:"auto_away,omitempty"`
	ManualAway      bool   `json:"manual_away,omitempty"`
	ConnectionCount int    `json:"connection_count,omitempty"`
	LastActivity    int64  `json:"last_activity,omitempty"`
}

type PresenceGetResult struct {
	User string `json:"user,omitempty"`
	Presence
}

type PresenceSetResult struct {
	OK       bool   `json:"ok"`
	Presence string `json:"presence,omitempty"`
	Role     string `json:"role,omitempty"`
}

type MessageEditResult struct {
	OK      bool   `json:"ok"`
	Channel string `json:"channel,omitempty"`
	TS      string `json:"ts,omitempty"`
	Role    string `json:"role,omitempty"`
}

type MessageDeleteResult struct {
	OK      bool   `json:"ok"`
	Channel string `json:"channel,omitempty"`
	TS      string `json:"ts,omitempty"`
	Role    string `json:"role,omitempty"`
}

type ReactionAddResult struct {
	OK      bool   `json:"ok"`
	Channel string `json:"channel,omitempty"`
	TS      string `json:"ts,omitempty"`
	Emoji   string `json:"emoji,omitempty"`
	Role    string `json:"role,omitempty"`
}

type ChannelJoinResult struct {
	OK      bool   `json:"ok"`
	Channel string `json:"channel,omitempty"`
	Role    string `json:"role,omitempty"`
}

type ChannelMarkResult struct {
	OK      bool   `json:"ok"`
	Channel string `json:"channel,omitempty"`
	TS      string `json:"ts,omitempty"`
	Role    string `json:"role,omitempty"`
}

type FileListResult struct {
	Count int          `json:"count"`
	Files []FileRecord `json:"files,omitempty"`
}

type FileInfoResult struct {
	File FileRecord `json:"file"`
}

type FileDownloadResult struct {
	OK      bool                  `json:"ok"`
	FileID  string                `json:"file_id,omitempty"`
	Path    string                `json:"path,omitempty"`
	Size    int                   `json:"size,omitempty"`
	Role    string                `json:"role,omitempty"`
	Blob    pluginbinding.BlobRef `json:"blob,omitempty"`
	File    FileRecord            `json:"file,omitempty"`
	content []byte
}

type FileDeleteResult struct {
	OK     bool   `json:"ok"`
	FileID string `json:"file_id,omitempty"`
	Role   string `json:"role,omitempty"`
}

type Emoji struct {
	Name     string `json:"name"`
	Source   string `json:"source"`
	URL      string `json:"url,omitempty"`
	AliasFor string `json:"alias_for,omitempty"`
	Category string `json:"category,omitempty"`
}

type EmojiSet struct {
	Custom     map[string]string
	Categories []EmojiCategory
}

type EmojiCategory struct {
	Name       string
	EmojiNames []string
}

type Bookmark struct {
	ID       string `json:"id,omitempty"`
	Channel  string `json:"channel,omitempty"`
	Title    string `json:"title,omitempty"`
	Link     string `json:"link,omitempty"`
	Emoji    string `json:"emoji,omitempty"`
	IconURL  string `json:"icon_url,omitempty"`
	Type     string `json:"type,omitempty"`
	EntityID string `json:"entity_id,omitempty"`
	AppID    string `json:"app_id,omitempty"`
}

type FileRecord struct {
	ID              string   `json:"id,omitempty"`
	Name            string   `json:"name,omitempty"`
	Title           string   `json:"title,omitempty"`
	Mimetype        string   `json:"mimetype,omitempty"`
	Filetype        string   `json:"filetype,omitempty"`
	PrettyType      string   `json:"pretty_type,omitempty"`
	User            string   `json:"user,omitempty"`
	Size            int      `json:"size,omitempty"`
	Permalink       string   `json:"permalink,omitempty"`
	PermalinkPublic string   `json:"permalink_public,omitempty"`
	Channels        []string `json:"channels,omitempty"`
	Groups          []string `json:"groups,omitempty"`
	IMs             []string `json:"ims,omitempty"`
	Created         int64    `json:"created,omitempty"`
	Timestamp       int64    `json:"timestamp,omitempty"`
	downloadURL     string
}

type MessageSendRequest struct {
	Channel        string
	Text           string
	Blocks         []json.RawMessage
	ThreadTS       string
	ReplyBroadcast bool
	UnfurlLinks    *bool
	UnfurlMedia    *bool
	Parse          string
}

type MessageRefRequest struct {
	Channel string
	TS      string
}

type MessageEditRequest struct {
	MessageRefRequest
	Text        string
	Blocks      []json.RawMessage
	UnfurlLinks *bool
	UnfurlMedia *bool
	Parse       string
}

type ReactionAddRequest struct {
	MessageRefRequest
	Emoji string
}

type ChannelJoinRequest struct {
	Channel string
}

type FileListRequest struct {
	Channel string
	User    string
	Types   string
	Limit   int
}

type FileDownloadRequest struct {
	FileID string
}

type UnreadsRequest struct {
	Channel string
	Since   int64
	Limit   int
}

type BookmarkAddRequest struct {
	Channel string
	Title   string
	Link    string
	Emoji   string
}

type BookmarkEditRequest struct {
	Channel    string
	BookmarkID string
	Title      string
	Link       string
	Emoji      string
}

type BookmarkDeleteRequest struct {
	Channel    string
	BookmarkID string
}

type FileUploadRequest struct {
	Channel        string
	ThreadTS       string
	Content        []byte
	Filename       string
	InitialComment string
	AltText        string
}

type FileUploadResult struct {
	OK        bool   `json:"ok"`
	Channel   string `json:"channel,omitempty"`
	ThreadTS  string `json:"thread_ts,omitempty"`
	Role      string `json:"role,omitempty"`
	FileID    string `json:"file_id,omitempty"`
	Filename  string `json:"filename,omitempty"`
	Title     string `json:"title,omitempty"`
	Permalink string `json:"permalink,omitempty"`
	Size      int    `json:"size,omitempty"`
	Warning   string `json:"warning,omitempty"`
}

type SearchInput struct {
	Query      string   `json:"query,omitempty" jsonschema:"required,description=Slack search query"`
	Limit      int      `json:"limit,omitempty" jsonschema:"description=Maximum messages to return"`
	Tickets    bool     `json:"tickets,omitempty" jsonschema:"description=Extract ticket references from matching messages"`
	TicketKeys []string `json:"ticket_keys,omitempty" jsonschema:"description=Optional ticket project keys to extract, for example DEV or TEL. Empty extracts uppercase issue keys."`
}

type MessageSearchInput struct {
	Datasource string `json:"datasource,omitempty" jsonschema:"description=Exact datasource name."`
	Query      string `json:"query,omitempty" jsonschema:"required,description=Slack search query"`
	Limit      int    `json:"limit,omitempty" jsonschema:"description=Maximum messages to return"`
	Entity     string `json:"entity,omitempty" jsonschema:"description=Datasource entity filter."`
}

type SearchResult struct {
	Count    int             `json:"count"`
	Messages []SearchMessage `json:"messages,omitempty"`
	Tickets  []TicketMention `json:"tickets,omitempty"`
}

type SearchMessage struct {
	Channel   string   `json:"channel,omitempty"`
	TS        string   `json:"ts,omitempty"`
	ThreadTS  string   `json:"thread_ts,omitempty"`
	User      string   `json:"user,omitempty"`
	Text      string   `json:"text,omitempty"`
	Permalink string   `json:"permalink,omitempty"`
	Tickets   []string `json:"tickets,omitempty"`
}

type TicketMention struct {
	Key        string   `json:"key"`
	Mentions   int      `json:"mentions"`
	Permalinks []string `json:"permalinks,omitempty"`
}

type MentionsInput struct {
	User       string   `json:"user,omitempty" jsonschema:"description=Slack user ID, mention, or name to search mentions for. Empty defaults to authenticated user or bot."`
	Bot        bool     `json:"bot,omitempty" jsonschema:"description=Search mentions of the bot token identity instead of the user token identity."`
	Since      string   `json:"since,omitempty" jsonschema:"description=Time window such as 1h, 7d, or 14d. Empty means today."`
	Limit      int      `json:"limit,omitempty" jsonschema:"description=Maximum mentions to return"`
	Unhandled  bool     `json:"unhandled,omitempty" jsonschema:"description=Only return pending mentions"`
	MaxThread  int      `json:"max_thread,omitempty" jsonschema:"description=Maximum thread messages to inspect for status classification"`
	Tickets    bool     `json:"tickets,omitempty" jsonschema:"description=Extract ticket references from mention text"`
	TicketKeys []string `json:"ticket_keys,omitempty" jsonschema:"description=Optional ticket project keys to extract, for example DEV or TEL. Empty extracts uppercase issue keys."`
}

type UnreadsInput struct {
	Channel string `json:"channel,omitempty" jsonschema:"description=Optional Slack channel ID or name"`
	Since   string `json:"since,omitempty" jsonschema:"description=Time window such as 1h, 7d, or 14d. Defaults to 14d."`
	Limit   int    `json:"limit,omitempty" jsonschema:"description=Maximum unread messages to fetch per channel"`
}

type MentionsResult struct {
	Target    string          `json:"target,omitempty"`
	Since     string          `json:"since,omitempty"`
	Count     int             `json:"count"`
	Total     int             `json:"total"`
	Unhandled bool            `json:"unhandled,omitempty"`
	Mentions  []MentionItem   `json:"mentions,omitempty"`
	Tickets   []TicketMention `json:"tickets,omitempty"`
}

type UnreadsResult struct {
	Since    string          `json:"since,omitempty"`
	Count    int             `json:"count"`
	Channels []UnreadChannel `json:"channels,omitempty"`
}

type UnreadChannel struct {
	ID          string          `json:"id,omitempty"`
	Name        string          `json:"name,omitempty"`
	IsPrivate   bool            `json:"is_private,omitempty"`
	IsDM        bool            `json:"is_dm,omitempty"`
	UnreadCount int             `json:"unread_count"`
	LastRead    string          `json:"last_read,omitempty"`
	Messages    []UnreadMessage `json:"messages,omitempty"`
}

type UnreadMessage struct {
	TS        string      `json:"ts,omitempty"`
	User      string      `json:"user,omitempty"`
	Text      string      `json:"text,omitempty"`
	ThreadTS  string      `json:"thread_ts,omitempty"`
	Files     []SlackFile `json:"files,omitempty"`
	Reactions []Reaction  `json:"reactions,omitempty"`
}

type MentionItem struct {
	Channel   string      `json:"channel,omitempty"`
	TS        string      `json:"ts,omitempty"`
	ThreadTS  string      `json:"thread_ts,omitempty"`
	User      string      `json:"user,omitempty"`
	Text      string      `json:"text,omitempty"`
	Permalink string      `json:"permalink,omitempty"`
	Status    string      `json:"status,omitempty"`
	Files     []SlackFile `json:"files,omitempty"`
	Tickets   []string    `json:"tickets,omitempty"`
}

type Reaction struct {
	Name  string   `json:"name,omitempty"`
	Users []string `json:"users,omitempty"`
}

type ThreadInput struct {
	Ref      string `json:"ref,omitempty" jsonschema:"description=Slack message reference as URL or channel:timestamp"`
	Channel  string `json:"channel,omitempty" jsonschema:"description=Slack channel ID or name"`
	TS       string `json:"ts,omitempty" jsonschema:"description=Slack message timestamp"`
	Limit    int    `json:"limit,omitempty" jsonschema:"description=Maximum thread messages to return"`
	MaxBytes int    `json:"max_bytes,omitempty" jsonschema:"description=Maximum bytes to download per image. Defaults to 10485760."`
}

type ThreadMessagesInput struct {
	Datasource string         `json:"datasource,omitempty" jsonschema:"description=Exact datasource name."`
	Query      string         `json:"query,omitempty" jsonschema:"description=Slack message reference as URL or channel:timestamp. Used as ref when ref is empty."`
	Ref        string         `json:"ref,omitempty" jsonschema:"description=Slack message reference as URL or channel:timestamp"`
	Channel    string         `json:"channel,omitempty" jsonschema:"description=Slack channel ID or name"`
	TS         string         `json:"ts,omitempty" jsonschema:"description=Slack root message timestamp"`
	Limit      int            `json:"limit,omitempty" jsonschema:"description=Maximum thread messages to return"`
	Entity     string         `json:"entity,omitempty" jsonschema:"description=Datasource entity filter."`
	Filters    map[string]any `json:"filters,omitempty" jsonschema:"description=Optional generic datasource filters. Supports ref, channel, channel_id, ts, message_ts, thread_ts, and root_ts."`
}

type ThreadResult struct {
	Channel  string          `json:"channel,omitempty"`
	TS       string          `json:"ts,omitempty"`
	Count    int             `json:"count"`
	Messages []ThreadMessage `json:"messages,omitempty"`
}

type ThreadMessage struct {
	TS        string      `json:"ts,omitempty"`
	User      string      `json:"user,omitempty"`
	Text      string      `json:"text,omitempty"`
	Files     []SlackFile `json:"files,omitempty"`
	Reactions []Reaction  `json:"reactions,omitempty"`
}

type SlackFile struct {
	FileID    string `json:"file_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Title     string `json:"title,omitempty"`
	Mimetype  string `json:"mimetype,omitempty"`
	Filetype  string `json:"filetype,omitempty"`
	Permalink string `json:"permalink,omitempty"`
	FilePath  string `json:"file_path,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Size      int    `json:"size,omitempty"`
}

type ChannelMembersInput struct {
	Datasource string `json:"datasource,omitempty" jsonschema:"description=Exact datasource name."`
	Channel    string `json:"channel,omitempty" jsonschema:"required,description=Slack channel ID"`
	Query      string `json:"query,omitempty" jsonschema:"description=Optional member text filter"`
	Limit      int    `json:"limit,omitempty" jsonschema:"description=Maximum members to return"`
	Entity     string `json:"entity,omitempty" jsonschema:"description=Datasource entity filter."`
}

func (s Service) IndexBuild(ctx pluginbinding.Context, input IndexBuildInput) (pluginbinding.IndexBuildResult, error) {
	return s.indexBuild(ctx, pluginbinding.InputMap(input))
}

func (s Service) Lookup(ctx pluginbinding.Context, input LookupInput) (LookupResult, error) {
	entity := strings.TrimSpace(input.Entity)
	var candidates []pluginbinding.LookupCandidate
	if entity == "" || entity == EntityUser {
		users, _, err := s.listUsers(ctx)
		if err != nil {
			return LookupResult{}, pluginbinding.Errorf("slack", "%s", err)
		}
		for _, user := range users {
			record, ok := normalizeUserRecord(ctx.DatasourceSource(), user)
			if !ok {
				continue
			}
			candidates = append(candidates, pluginbinding.NewLookupCandidate(ctx.LookupSource(PluginName, DatasourceUsers), record.Entity, record.ID, record, userLookupValues(record)))
		}
	}
	if entity == "" || entity == EntityChannel {
		channels, _, err := s.listChannels(ctx)
		if err != nil {
			return LookupResult{}, pluginbinding.Errorf("slack", "%s", err)
		}
		for _, channel := range channels {
			record, ok := normalizeChannelRecord(ctx.DatasourceSource(), channel)
			if !ok {
				continue
			}
			candidates = append(candidates, pluginbinding.NewLookupCandidate(ctx.LookupSource(PluginName, DatasourceChannels), record.Entity, record.ID, record, channelLookupValues(record)))
		}
	}
	return pluginbinding.NewDatasourceLookupResultFromCandidates(PluginName, input, candidates), nil
}

func (s Service) AuthTest(ctx pluginbinding.Context, _ NoInput) (AuthTestResult, error) {
	status, results, err := s.tokenIdentityReport(ctx)
	if err != nil {
		return AuthTestResult{}, err
	}
	return AuthTestResult{Status: status, Count: len(results), Tokens: results}, nil
}

func (s Service) Info(ctx pluginbinding.Context, _ NoInput) (InfoResult, error) {
	status, results, err := s.tokenIdentityReport(ctx)
	if err != nil {
		return InfoResult{}, err
	}
	return InfoResult{Status: status, Count: len(results), Tokens: results}, nil
}

func (s Service) tokenIdentityReport(ctx pluginbinding.Context) (string, []TokenInfoResult, error) {
	results := make([]TokenInfoResult, 0, 2)
	for _, purpose := range []string{AuthPurposeUser, AuthPurposeBot} {
		result := TokenInfoResult{Role: slackRoleFromPurpose(purpose)}
		client, err := s.openClient(ctx, purpose)
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}
		info, err := client.AuthTest(context.Background())
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}
		result.OK = true
		result.AuthInfo = info
		results = append(results, result)
	}
	if len(results) == 0 {
		return "", nil, pluginbinding.Fail("slack", "no Slack auth purposes configured")
	}
	okCount := 0
	for _, result := range results {
		if result.OK {
			okCount++
		}
	}
	status := "error"
	if okCount == len(results) {
		status = "ok"
	} else if okCount > 0 {
		status = "partial"
	}
	return status, results, nil
}

func (s Service) ListUsers(ctx pluginbinding.Context, input UserListInput) (UserListResult, error) {
	users, _, err := s.listUsers(ctx)
	if err != nil {
		return UserListResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	users = filterUsers(users, strings.TrimSpace(input.Query))
	users = limitUsers(users, input.Limit)
	return UserListResult{Count: len(users), Users: users}, nil
}

func (s Service) ListChannels(ctx pluginbinding.Context, input ChannelListInput) (ChannelListResult, error) {
	channels, _, err := s.listChannels(ctx)
	if err != nil {
		return ChannelListResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	channels = filterChannels(channels, strings.TrimSpace(input.Query))
	channels = limitChannels(channels, input.Limit)
	return ChannelListResult{Count: len(channels), Channels: channels}, nil
}

func (s Service) ListEmojis(ctx pluginbinding.Context, input EmojiListInput) (EmojiListResult, error) {
	mode := emojiListMode(input.Mode)
	if mode == "" {
		return EmojiListResult{}, pluginbinding.Fail("bad_input", "mode must be custom, builtin, or all")
	}
	includeCategories := mode == "builtin" || mode == "all"
	emojis, _, err := pluginbinding.ReadWithPreferredAuthPurposes[Client, EmojiSet]([]string{AuthPurposeUser, AuthPurposeBot}, s.openClientForContext(ctx), func(client Client, _ string) (EmojiSet, error) {
		return client.ListEmojis(context.Background(), includeCategories)
	}, fallbackableSlackError)
	if err != nil {
		return EmojiListResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	records := emojiRecords(emojis, mode, input.IncludeAliases, strings.TrimSpace(input.Query), input.Limit)
	return EmojiListResult{Count: len(records), Emojis: records}, nil
}

func (s Service) ListBookmarks(ctx pluginbinding.Context, input BookmarkListInput) (BookmarkListResult, error) {
	channel, err := s.resolveChannel(ctx, input.Channel)
	if err != nil {
		return BookmarkListResult{}, err
	}
	bookmarks, _, err := pluginbinding.ReadWithPreferredAuthPurposes[Client, []Bookmark]([]string{AuthPurposeUser, AuthPurposeBot}, s.openClientForContext(ctx), func(client Client, _ string) ([]Bookmark, error) {
		return client.ListBookmarks(context.Background(), channel)
	}, fallbackableSlackError)
	if err != nil {
		return BookmarkListResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	bookmarks = filterBookmarks(bookmarks, strings.TrimSpace(input.Query))
	bookmarks = limitBookmarks(bookmarks, input.Limit)
	return BookmarkListResult{Count: len(bookmarks), Channel: channel, Bookmarks: bookmarks}, nil
}

func (s Service) AddBookmark(ctx pluginbinding.Context, input BookmarkAddInput) (BookmarkResult, error) {
	channel, err := s.resolveChannel(ctx, input.Channel)
	if err != nil {
		return BookmarkResult{}, err
	}
	title := strings.TrimSpace(input.Title)
	link := strings.TrimSpace(input.Link)
	if title == "" {
		return BookmarkResult{}, pluginbinding.Fail("bad_input", "title is required")
	}
	if link == "" {
		return BookmarkResult{}, pluginbinding.Fail("bad_input", "link is required")
	}
	role, purpose, err := slackWriteRole(input.Role, SlackRoleBot)
	if err != nil {
		return BookmarkResult{}, err
	}
	client, err := s.clientForPurpose(ctx, purpose)
	if err != nil {
		return BookmarkResult{}, err
	}
	bookmark, err := client.AddBookmark(context.Background(), BookmarkAddRequest{Channel: channel, Title: title, Link: link, Emoji: input.Emoji})
	if err != nil {
		return BookmarkResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	return BookmarkResult{OK: true, Channel: channel, Bookmark: bookmark, Role: role}, nil
}

func (s Service) EditBookmark(ctx pluginbinding.Context, input BookmarkEditInput) (BookmarkResult, error) {
	channel, err := s.resolveChannel(ctx, input.Channel)
	if err != nil {
		return BookmarkResult{}, err
	}
	bookmarkID := strings.TrimSpace(input.BookmarkID)
	if bookmarkID == "" {
		return BookmarkResult{}, pluginbinding.Fail("bad_input", "bookmark_id is required")
	}
	if strings.TrimSpace(input.Title) == "" && strings.TrimSpace(input.Link) == "" && strings.TrimSpace(input.Emoji) == "" {
		return BookmarkResult{}, pluginbinding.Fail("bad_input", "title, link, or emoji is required")
	}
	role, purpose, err := slackWriteRole(input.Role, SlackRoleBot)
	if err != nil {
		return BookmarkResult{}, err
	}
	client, err := s.clientForPurpose(ctx, purpose)
	if err != nil {
		return BookmarkResult{}, err
	}
	bookmark, err := client.EditBookmark(context.Background(), BookmarkEditRequest{
		Channel:    channel,
		BookmarkID: bookmarkID,
		Title:      input.Title,
		Link:       input.Link,
		Emoji:      input.Emoji,
	})
	if err != nil {
		return BookmarkResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	return BookmarkResult{OK: true, Channel: channel, Bookmark: bookmark, Role: role}, nil
}

func (s Service) DeleteBookmark(ctx pluginbinding.Context, input BookmarkDeleteInput) (BookmarkDeleteResult, error) {
	channel, err := s.resolveChannel(ctx, input.Channel)
	if err != nil {
		return BookmarkDeleteResult{}, err
	}
	bookmarkID := strings.TrimSpace(input.BookmarkID)
	if bookmarkID == "" {
		return BookmarkDeleteResult{}, pluginbinding.Fail("bad_input", "bookmark_id is required")
	}
	role, purpose, err := slackWriteRole(input.Role, SlackRoleBot)
	if err != nil {
		return BookmarkDeleteResult{}, err
	}
	client, err := s.clientForPurpose(ctx, purpose)
	if err != nil {
		return BookmarkDeleteResult{}, err
	}
	if err := client.DeleteBookmark(context.Background(), BookmarkDeleteRequest{Channel: channel, BookmarkID: bookmarkID}); err != nil {
		return BookmarkDeleteResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	return BookmarkDeleteResult{OK: true, Channel: channel, BookmarkID: bookmarkID, Role: role}, nil
}

func (s Service) GetPresence(ctx pluginbinding.Context, input PresenceGetInput) (PresenceGetResult, error) {
	user, err := s.resolveOptionalUser(ctx, input.User)
	if err != nil {
		return PresenceGetResult{}, err
	}
	presence, _, err := pluginbinding.ReadWithPreferredAuthPurposes[Client, Presence]([]string{AuthPurposeUser, AuthPurposeBot}, s.openClientForContext(ctx), func(client Client, _ string) (Presence, error) {
		return client.GetPresence(context.Background(), user)
	}, fallbackableSlackError)
	if err != nil {
		return PresenceGetResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	return PresenceGetResult{User: user, Presence: presence}, nil
}

func (s Service) SetPresence(ctx pluginbinding.Context, input PresenceSetInput) (PresenceSetResult, error) {
	presence := strings.ToLower(strings.TrimSpace(input.Presence))
	if presence != "auto" && presence != "away" {
		return PresenceSetResult{}, pluginbinding.Fail("bad_input", "presence must be auto or away")
	}
	role, purpose, err := slackWriteRole(input.Role, SlackRoleUser)
	if err != nil {
		return PresenceSetResult{}, err
	}
	client, err := s.clientForPurpose(ctx, purpose)
	if err != nil {
		return PresenceSetResult{}, err
	}
	if err := client.SetPresence(context.Background(), presence); err != nil {
		return PresenceSetResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	return PresenceSetResult{OK: true, Presence: presence, Role: role}, nil
}

func (s Service) SendMessage(ctx pluginbinding.Context, input MessageSendInput) (MessageSendResult, error) {
	channel, err := s.resolveChannel(ctx, input.Channel)
	if err != nil {
		return MessageSendResult{}, err
	}
	content, err := s.messageContent(ctx, input.Text, input.Markdown, input.Blocks, input.UnfurlLinks, input.UnfurlMedia, input.Parse)
	if err != nil {
		return MessageSendResult{}, err
	}
	role, purpose, err := slackWriteRole(input.Role, SlackRoleBot)
	if err != nil {
		return MessageSendResult{}, err
	}
	client, err := s.clientForPurpose(ctx, purpose)
	if err != nil {
		return MessageSendResult{}, err
	}
	request := MessageSendRequest{
		Channel:        channel,
		Text:           content.Text,
		Blocks:         content.Blocks,
		ThreadTS:       normalizeSlackTimestamp(input.ThreadTS),
		ReplyBroadcast: input.ReplyBroadcast,
		UnfurlLinks:    content.UnfurlLinks,
		UnfurlMedia:    content.UnfurlMedia,
		Parse:          content.Parse,
	}
	ts, err := client.SendMessage(context.Background(), request)
	if err != nil {
		return MessageSendResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	return MessageSendResult{Channel: channel, TS: ts, ThreadTS: request.ThreadTS, Role: role, OK: true}, nil
}

func (s Service) EditMessage(ctx pluginbinding.Context, input MessageEditInput) (MessageEditResult, error) {
	ref, err := s.resolveMessageRef(ctx, input.Ref, input.Channel, input.TS)
	if err != nil {
		return MessageEditResult{}, err
	}
	content, err := s.messageContent(ctx, input.Text, input.Markdown, input.Blocks, input.UnfurlLinks, input.UnfurlMedia, input.Parse)
	if err != nil {
		return MessageEditResult{}, err
	}
	role, purpose, err := slackWriteRole(input.Role, SlackRoleBot)
	if err != nil {
		return MessageEditResult{}, err
	}
	client, err := s.clientForPurpose(ctx, purpose)
	if err != nil {
		return MessageEditResult{}, err
	}
	ts, err := client.EditMessage(context.Background(), MessageEditRequest{
		MessageRefRequest: MessageRefRequest{Channel: ref.Channel, TS: ref.TS},
		Text:              content.Text,
		Blocks:            content.Blocks,
		UnfurlLinks:       content.UnfurlLinks,
		UnfurlMedia:       content.UnfurlMedia,
		Parse:             content.Parse,
	})
	if err != nil {
		return MessageEditResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	return MessageEditResult{OK: true, Channel: ref.Channel, TS: ts, Role: role}, nil
}

func (s Service) DeleteMessage(ctx pluginbinding.Context, input MessageDeleteInput) (MessageDeleteResult, error) {
	ref, err := s.resolveMessageRef(ctx, input.Ref, input.Channel, input.TS)
	if err != nil {
		return MessageDeleteResult{}, err
	}
	role, purpose, err := slackWriteRole(input.Role, SlackRoleBot)
	if err != nil {
		return MessageDeleteResult{}, err
	}
	client, err := s.clientForPurpose(ctx, purpose)
	if err != nil {
		return MessageDeleteResult{}, err
	}
	if err := client.DeleteMessage(context.Background(), MessageRefRequest{Channel: ref.Channel, TS: ref.TS}); err != nil {
		return MessageDeleteResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	return MessageDeleteResult{OK: true, Channel: ref.Channel, TS: ref.TS, Role: role}, nil
}

func (s Service) AddReaction(ctx pluginbinding.Context, input ReactionAddInput) (ReactionAddResult, error) {
	ref, err := s.resolveMessageRef(ctx, input.Ref, input.Channel, input.TS)
	if err != nil {
		return ReactionAddResult{}, err
	}
	emoji := strings.Trim(strings.TrimSpace(input.Emoji), ":")
	if emoji == "" {
		return ReactionAddResult{}, pluginbinding.Fail("bad_input", "emoji is required")
	}
	role, purpose, err := slackWriteRole(input.Role, SlackRoleBot)
	if err != nil {
		return ReactionAddResult{}, err
	}
	client, err := s.clientForPurpose(ctx, purpose)
	if err != nil {
		return ReactionAddResult{}, err
	}
	request := ReactionAddRequest{MessageRefRequest: MessageRefRequest{Channel: ref.Channel, TS: ref.TS}, Emoji: emoji}
	if err := client.AddReaction(context.Background(), request); err != nil {
		return ReactionAddResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	return ReactionAddResult{OK: true, Channel: ref.Channel, TS: ref.TS, Emoji: emoji, Role: role}, nil
}

func (s Service) RemoveReaction(ctx pluginbinding.Context, input ReactionAddInput) (ReactionAddResult, error) {
	ref, err := s.resolveMessageRef(ctx, input.Ref, input.Channel, input.TS)
	if err != nil {
		return ReactionAddResult{}, err
	}
	emoji := strings.Trim(strings.TrimSpace(input.Emoji), ":")
	if emoji == "" {
		return ReactionAddResult{}, pluginbinding.Fail("bad_input", "emoji is required")
	}
	role, purpose, err := slackWriteRole(input.Role, SlackRoleBot)
	if err != nil {
		return ReactionAddResult{}, err
	}
	client, err := s.clientForPurpose(ctx, purpose)
	if err != nil {
		return ReactionAddResult{}, err
	}
	request := ReactionAddRequest{MessageRefRequest: MessageRefRequest{Channel: ref.Channel, TS: ref.TS}, Emoji: emoji}
	if err := client.RemoveReaction(context.Background(), request); err != nil {
		return ReactionAddResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	return ReactionAddResult{OK: true, Channel: ref.Channel, TS: ref.TS, Emoji: emoji, Role: role}, nil
}

func (s Service) JoinChannel(ctx pluginbinding.Context, input ChannelJoinInput) (ChannelJoinResult, error) {
	channel, err := s.resolveChannel(ctx, input.Channel)
	if err != nil {
		return ChannelJoinResult{}, err
	}
	role, purpose, err := slackWriteRole(input.Role, SlackRoleBot)
	if err != nil {
		return ChannelJoinResult{}, err
	}
	client, err := s.clientForPurpose(ctx, purpose)
	if err != nil {
		return ChannelJoinResult{}, err
	}
	if err := client.JoinChannel(context.Background(), ChannelJoinRequest{Channel: channel}); err != nil {
		return ChannelJoinResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	return ChannelJoinResult{OK: true, Channel: channel, Role: role}, nil
}

func (s Service) MarkRead(ctx pluginbinding.Context, input ChannelMarkInput) (ChannelMarkResult, error) {
	ref, err := s.resolveMessageRef(ctx, input.Ref, input.Channel, input.TS)
	if err != nil {
		return ChannelMarkResult{}, err
	}
	role, purpose, err := slackWriteRole(input.Role, SlackRoleUser)
	if err != nil {
		return ChannelMarkResult{}, err
	}
	client, err := s.clientForPurpose(ctx, purpose)
	if err != nil {
		return ChannelMarkResult{}, err
	}
	if strings.EqualFold(ref.TS, "latest") {
		latest, err := client.LatestMessageTS(context.Background(), ref.Channel)
		if err != nil {
			return ChannelMarkResult{}, pluginbinding.Errorf("slack", "%s", err)
		}
		if strings.TrimSpace(latest) == "" {
			return ChannelMarkResult{}, pluginbinding.Fail("empty_channel", "no latest message exists in channel")
		}
		ref.TS = latest
	}
	if err := client.MarkRead(context.Background(), MessageRefRequest{Channel: ref.Channel, TS: ref.TS}); err != nil {
		return ChannelMarkResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	return ChannelMarkResult{OK: true, Channel: ref.Channel, TS: ref.TS, Role: role}, nil
}

func (s Service) ListFiles(ctx pluginbinding.Context, input FileListInput) (FileListResult, error) {
	channel := strings.TrimSpace(input.Channel)
	if channel != "" {
		resolved, err := s.resolveChannel(ctx, channel)
		if err != nil {
			return FileListResult{}, err
		}
		channel = resolved
	}
	user, err := s.resolveOptionalUser(ctx, input.User)
	if err != nil {
		return FileListResult{}, err
	}
	files, _, err := pluginbinding.ReadWithPreferredAuthPurposes[Client, []FileRecord]([]string{AuthPurposeUser, AuthPurposeBot}, s.openClientForContext(ctx), func(client Client, _ string) ([]FileRecord, error) {
		return client.ListFiles(context.Background(), FileListRequest{Channel: channel, User: user, Types: strings.TrimSpace(input.Types), Limit: input.Limit})
	}, fallbackableSlackError)
	if err != nil {
		return FileListResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	files = filterFiles(files, strings.TrimSpace(input.Query))
	files = limitFiles(files, input.Limit)
	return FileListResult{Count: len(files), Files: files}, nil
}

func (s Service) FileInfo(ctx pluginbinding.Context, input FileInfoInput) (FileInfoResult, error) {
	fileID := strings.TrimSpace(input.FileID)
	if fileID == "" {
		return FileInfoResult{}, pluginbinding.Fail("bad_input", "file_id is required")
	}
	file, _, err := pluginbinding.ReadWithPreferredAuthPurposes[Client, FileRecord]([]string{AuthPurposeUser, AuthPurposeBot}, s.openClientForContext(ctx), func(client Client, _ string) (FileRecord, error) {
		return client.GetFileInfo(context.Background(), fileID)
	}, fallbackableSlackError)
	if err != nil {
		return FileInfoResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	return FileInfoResult{File: file}, nil
}

func (s Service) DownloadFile(ctx pluginbinding.Context, input FileDownloadInput) (FileDownloadResult, error) {
	fileID := strings.TrimSpace(input.FileID)
	if fileID == "" {
		return FileDownloadResult{}, pluginbinding.Fail("bad_input", "file_id is required")
	}
	role, purpose, err := slackWriteRole(input.Role, SlackRoleBot)
	if err != nil {
		return FileDownloadResult{}, err
	}
	client, err := s.clientForPurpose(ctx, purpose)
	if err != nil {
		return FileDownloadResult{}, err
	}
	result, err := client.DownloadFile(context.Background(), FileDownloadRequest{FileID: fileID})
	if err != nil {
		return FileDownloadResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	if len(result.content) > 0 {
		filename := firstNonEmpty(input.Filename, result.File.Name, result.File.Title, result.File.ID)
		blob, err := ctx.Host.BlobWrite(pluginbinding.BlobWriteRequest{
			Ref:       strings.TrimSpace(input.BlobRef),
			Content:   result.content,
			Filename:  filename,
			MediaType: result.File.Mimetype,
			Metadata: map[string]string{
				"source":  "slack",
				"file_id": result.FileID,
			},
		})
		if err != nil {
			return FileDownloadResult{}, pluginbinding.Errorf("blob", "%s", err)
		}
		result.Blob = blob
		result.Size = len(result.content)
		result.content = nil
	}
	result.Role = role
	return result, nil
}

func (s Service) DeleteFile(ctx pluginbinding.Context, input FileDeleteInput) (FileDeleteResult, error) {
	fileID := strings.TrimSpace(input.FileID)
	if fileID == "" {
		return FileDeleteResult{}, pluginbinding.Fail("bad_input", "file_id is required")
	}
	role, purpose, err := slackWriteRole(input.Role, SlackRoleBot)
	if err != nil {
		return FileDeleteResult{}, err
	}
	client, err := s.clientForPurpose(ctx, purpose)
	if err != nil {
		return FileDeleteResult{}, err
	}
	if err := client.DeleteFile(context.Background(), fileID); err != nil {
		return FileDeleteResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	return FileDeleteResult{OK: true, FileID: fileID, Role: role}, nil
}

func (s Service) UploadFile(ctx pluginbinding.Context, input FileUploadInput) (FileUploadResult, error) {
	request, err := fileUploadRequest(ctx, input)
	if err != nil {
		return FileUploadResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	request.Channel, err = s.resolveChannel(ctx, request.Channel)
	if err != nil {
		return FileUploadResult{}, err
	}
	request.ThreadTS = normalizeSlackTimestamp(request.ThreadTS)
	request.InitialComment = s.resolveMessageText(ctx, request.InitialComment)
	role, purpose, err := slackWriteRole(input.Role, SlackRoleBot)
	if err != nil {
		return FileUploadResult{}, err
	}
	client, err := s.clientForPurpose(ctx, purpose)
	if err != nil {
		return FileUploadResult{}, err
	}
	result, err := client.UploadFile(context.Background(), request)
	if err != nil {
		return FileUploadResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	result.Role = role
	return result, nil
}

func (s Service) Search(ctx pluginbinding.Context, input SearchInput) (SearchResult, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return SearchResult{}, pluginbinding.Fail("bad_input", "query is required")
	}
	messages, _, err := pluginbinding.ReadWithPreferredAuthPurposes[Client, searchMessagesOutput]([]string{AuthPurposeUser, AuthPurposeBot}, s.openClientForContext(ctx), func(client Client, _ string) (searchMessagesOutput, error) {
		messages, total, err := client.SearchMessages(context.Background(), query, input.Limit)
		return searchMessagesOutput{Messages: messages, Total: total}, err
	}, fallbackableSlackError)
	if err != nil {
		return SearchResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	if input.Tickets {
		addTicketsToMessages(messages.Messages, input.TicketKeys)
	}
	return SearchResult{Count: messages.Total, Messages: messages.Messages, Tickets: collectTicketMentionsFromSearch(messages.Messages)}, nil
}

func (s Service) Mentions(ctx pluginbinding.Context, input MentionsInput) (MentionsResult, error) {
	target, err := s.mentionTarget(ctx, input)
	if err != nil {
		return MentionsResult{}, err
	}
	since, sinceQuery, err := mentionSince(input.Since)
	if err != nil {
		return MentionsResult{}, err
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	query := "<@" + target + ">"
	if sinceQuery != "" {
		query += " after:" + sinceQuery
	}
	messages, _, err := pluginbinding.ReadWithPreferredAuthPurposes[Client, searchMessagesOutput]([]string{AuthPurposeUser, AuthPurposeBot}, s.openClientForContext(ctx), func(client Client, _ string) (searchMessagesOutput, error) {
		messages, total, err := client.SearchMessages(context.Background(), query, limit)
		return searchMessagesOutput{Messages: messages, Total: total}, err
	}, fallbackableSlackError)
	if err != nil {
		return MentionsResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	ownIDs := s.ownSlackUserIDs(ctx)
	mentions := make([]MentionItem, 0, len(messages.Messages))
	for _, message := range messages.Messages {
		if since > 0 && slackTimestampUnix(message.TS) < since {
			continue
		}
		status, files := s.classifyMention(ctx, message, ownIDs, input.MaxThread)
		if input.Unhandled && status != "pending" {
			continue
		}
		item := MentionItem{
			Channel:   message.Channel,
			TS:        message.TS,
			ThreadTS:  message.ThreadTS,
			User:      message.User,
			Text:      message.Text,
			Permalink: message.Permalink,
			Status:    status,
			Files:     files,
		}
		if input.Tickets {
			item.Tickets = extractTickets(item.Text, input.TicketKeys)
		}
		mentions = append(mentions, item)
	}
	return MentionsResult{
		Target:    target,
		Since:     input.Since,
		Count:     len(mentions),
		Total:     messages.Total,
		Unhandled: input.Unhandled,
		Mentions:  mentions,
		Tickets:   collectTicketMentionsFromMentions(mentions),
	}, nil
}

func (s Service) Unreads(ctx pluginbinding.Context, input UnreadsInput) (UnreadsResult, error) {
	channel := strings.TrimSpace(input.Channel)
	if channel != "" {
		resolved, err := s.resolveChannel(ctx, channel)
		if err != nil {
			return UnreadsResult{}, err
		}
		channel = resolved
	}
	since, sinceLabel, err := unreadSince(input.Since)
	if err != nil {
		return UnreadsResult{}, err
	}
	client, err := s.clientForPurpose(ctx, AuthPurposeUser)
	if err != nil {
		return UnreadsResult{}, err
	}
	channels, err := client.ListUnreads(context.Background(), UnreadsRequest{Channel: channel, Since: since, Limit: input.Limit})
	if err != nil {
		return UnreadsResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	return UnreadsResult{Since: sinceLabel, Count: len(channels), Channels: channels}, nil
}

func (s Service) SearchMessagesDatasource(ctx pluginbinding.Context, input MessageSearchInput) (MessageDatasourceResult, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return MessageDatasourceResult{}, pluginbinding.Fail("bad_input", "query is required")
	}
	messages, _, err := pluginbinding.ReadWithPreferredAuthPurposes[Client, searchMessagesOutput]([]string{AuthPurposeUser, AuthPurposeBot}, s.openClientForContext(ctx), func(client Client, _ string) (searchMessagesOutput, error) {
		messages, total, err := client.SearchMessages(context.Background(), query, input.Limit)
		return searchMessagesOutput{Messages: messages, Total: total}, err
	}, fallbackableSlackError)
	if err != nil {
		return MessageDatasourceResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	records := make([]MessageRecord, 0, len(messages.Messages))
	for _, message := range messages.Messages {
		record, ok := normalizeMessageRecord(ctx.DatasourceSource(), message)
		if ok {
			records = append(records, record)
		}
	}
	return pluginbinding.NewDatasourceSearchResult(DatasourceMessages, query, records), nil
}

func (s Service) Thread(ctx pluginbinding.Context, input ThreadInput) (ThreadResult, error) {
	ref, err := s.resolveMessageRef(ctx, input.Ref, input.Channel, input.TS)
	if err != nil {
		return ThreadResult{}, err
	}
	messages, _, err := pluginbinding.ReadWithPreferredAuthPurposes[Client, []ThreadMessage]([]string{AuthPurposeUser, AuthPurposeBot}, s.openClientForContext(ctx), func(client Client, _ string) ([]ThreadMessage, error) {
		return client.GetThread(context.Background(), ref.Channel, ref.TS, input.Limit, input.MaxBytes)
	}, fallbackableSlackError)
	if err != nil {
		return ThreadResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	messages = limitThreadMessages(messages, input.Limit)
	return ThreadResult{Channel: ref.Channel, TS: ref.TS, Count: len(messages), Messages: messages}, nil
}

func (s Service) ThreadMessagesDatasource(ctx pluginbinding.Context, input ThreadMessagesInput) (ThreadMessagesDatasourceResult, error) {
	ref, channel, ts := threadMessagesRefInput(input)
	msgRef, err := s.resolveMessageRef(ctx, ref, channel, ts)
	if err != nil {
		return ThreadMessagesDatasourceResult{}, err
	}
	messages, _, err := pluginbinding.ReadWithPreferredAuthPurposes[Client, []ThreadMessage]([]string{AuthPurposeUser, AuthPurposeBot}, s.openClientForContext(ctx), func(client Client, _ string) ([]ThreadMessage, error) {
		return client.GetThread(context.Background(), msgRef.Channel, msgRef.TS, input.Limit, 0)
	}, fallbackableSlackError)
	if err != nil {
		return ThreadMessagesDatasourceResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	messages = limitThreadMessages(messages, input.Limit)
	records := make([]ThreadMessageRecord, 0, len(messages))
	for _, message := range messages {
		record, ok := normalizeThreadMessageRecord(ctx.DatasourceSource(), msgRef.Channel, msgRef.TS, message)
		if ok {
			records = append(records, record)
		}
	}
	return pluginbinding.NewDatasourceSearchResult(DatasourceThreadMessages, msgRef.TS, records), nil
}

func threadMessagesRefInput(input ThreadMessagesInput) (ref, channel, ts string) {
	ref = strings.TrimSpace(input.Ref)
	channel = strings.TrimSpace(input.Channel)
	ts = strings.TrimSpace(input.TS)
	if ref == "" {
		ref = filterString(input.Filters, "ref")
	}
	if channel == "" {
		channel = firstFilterString(input.Filters, "channel", "channel_id", "channel_ref")
	}
	if ts == "" {
		ts = firstFilterString(input.Filters, "ts", "message_ts", "thread_ts", "root_ts")
	}
	query := strings.TrimSpace(input.Query)
	if ref == "" && channel == "" && ts == "" {
		ref = query
	} else if ref == "" && query != "" {
		if parsed, ok := parseSlackMessageRef(query); ok {
			ref = parsed.Channel + ":" + parsed.TS
		}
	}
	return ref, channel, ts
}

func firstFilterString(filters map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := filterString(filters, key); value != "" {
			return value
		}
	}
	return ""
}

func filterString(filters map[string]any, key string) string {
	if len(filters) == 0 {
		return ""
	}
	value, ok := filters[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.Trim(strings.TrimSpace(jsonNumberString(typed)), "\"")
	}
}

func jsonNumberString(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

func (s Service) ChannelMembersDatasource(ctx pluginbinding.Context, input ChannelMembersInput) (ChannelMembersDatasourceResult, error) {
	channel, err := s.resolveChannel(ctx, input.Channel)
	if err != nil {
		return ChannelMembersDatasourceResult{}, err
	}
	query := strings.TrimSpace(input.Query)
	readLimit := input.Limit
	if query != "" {
		readLimit = 0
	}
	members, _, err := pluginbinding.ReadWithPreferredAuthPurposes[Client, []User]([]string{AuthPurposeUser, AuthPurposeBot}, s.openClientForContext(ctx), func(client Client, _ string) ([]User, error) {
		return client.ListChannelMembers(context.Background(), channel, readLimit)
	}, fallbackableSlackError)
	if err != nil {
		return ChannelMembersDatasourceResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	members = filterChannelMembers(members, query)
	records := make([]ChannelMemberRecord, 0, len(members))
	for _, member := range members {
		record, ok := normalizeChannelMemberRecord(ctx.DatasourceSource(), channel, member)
		if ok {
			records = append(records, record)
		}
	}
	return pluginbinding.NewDatasourceSearchResult(DatasourceChannelMembers, query, limitChannelMemberRecords(records, input.Limit)), nil
}

type searchMessagesOutput struct {
	Messages []SearchMessage
	Total    int
}

func (s Service) indexBuild(ctx pluginbinding.Context, input map[string]any) (pluginbinding.IndexBuildResult, error) {
	selector, err := indexBuildSelector(input)
	if err != nil {
		return pluginbinding.IndexBuildResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	var userSource string
	var channelSource string
	return pluginbinding.RunIndexJobs(ctx, selector, "slack",
		pluginbinding.NewDynamicIndexJob(DatasourceUsers, EntityUser, OperationIndexBuild, func() ([]User, error) {
			users, source, err := s.listUsers(ctx)
			userSource = source
			return users, err
		}, normalizeUserRecord, func() map[string]any {
			return indexBuildMetadata(userSource, map[string]any{"include_deleted": false, "include_bots": true, "include_app_users": true})
		}),
		pluginbinding.NewDynamicIndexJob(DatasourceChannels, EntityChannel, OperationIndexBuild, func() ([]Channel, error) {
			channels, source, err := s.listChannels(ctx)
			channelSource = source
			return channels, err
		}, normalizeChannelRecord, func() map[string]any {
			return indexBuildMetadata(channelSource, map[string]any{"types": []string{"public_channel", "private_channel", "mpim", "im"}, "exclude_archived": false})
		}),
	)
}

func (s Service) listUsers(ctx pluginbinding.Context) ([]User, string, error) {
	return pluginbinding.ReadWithPreferredAuthPurposes[Client, []User]([]string{AuthPurposeUser, AuthPurposeBot}, s.openClientForContext(ctx), func(client Client, _ string) ([]User, error) {
		return client.ListUsers(context.Background())
	}, fallbackableSlackError)
}

func (s Service) listChannels(ctx pluginbinding.Context) ([]Channel, string, error) {
	return pluginbinding.ReadWithPreferredAuthPurposes[Client, []Channel]([]string{AuthPurposeUser, AuthPurposeBot}, s.openClientForContext(ctx), func(client Client, _ string) ([]Channel, error) {
		return client.ListChannels(context.Background())
	}, fallbackableSlackError)
}

func (s Service) openClientForContext(ctx pluginbinding.Context) func(string) (Client, error) {
	return func(purpose string) (Client, error) {
		return s.openClient(ctx, purpose)
	}
}

func (s Service) openClient(ctx pluginbinding.Context, purpose string) (Client, error) {
	factory := s.ClientFactory
	if factory == nil {
		factory = NewLiveClient
	}
	return factory(ctx, purpose)
}

func (s Service) clientForPurpose(ctx pluginbinding.Context, purpose string) (Client, error) {
	client, err := s.openClient(ctx, purpose)
	if err != nil {
		return nil, pluginbinding.Errorf("slack", "%s", err)
	}
	return client, nil
}

func indexBuildSelector(input map[string]any) (pluginbinding.IndexSelector, error) {
	known := map[string]string{
		DatasourceUsers:    DatasourceUsers,
		EntityUser:         DatasourceUsers,
		"user":             DatasourceUsers,
		"users":            DatasourceUsers,
		DatasourceChannels: DatasourceChannels,
		EntityChannel:      DatasourceChannels,
		"channel":          DatasourceChannels,
		"channels":         DatasourceChannels,
	}
	return pluginbinding.NewIndexSelector(input, known, "Slack")
}

func indexBuildMetadata(source string, extra map[string]any) map[string]any {
	metadata := map[string]any{}
	metadata["token_source"] = source
	for key, value := range extra {
		metadata[key] = value
	}
	return metadata
}

func slackWriteRole(role, defaultRole string) (string, string, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		role = strings.ToLower(strings.TrimSpace(defaultRole))
	}
	switch role {
	case SlackRoleBot:
		return role, AuthPurposeBot, nil
	case SlackRoleUser:
		return role, AuthPurposeUser, nil
	default:
		return "", "", pluginbinding.Fail("bad_input", "role must be bot or user")
	}
}

func slackRoleFromPurpose(purpose string) string {
	switch strings.TrimSpace(purpose) {
	case AuthPurposeBot:
		return SlackRoleBot
	case AuthPurposeUser:
		return SlackRoleUser
	default:
		return strings.TrimSpace(purpose)
	}
}

func (s Service) mentionTarget(ctx pluginbinding.Context, input MentionsInput) (string, error) {
	if strings.TrimSpace(input.User) != "" {
		return s.resolveOptionalUser(ctx, input.User)
	}
	purpose := AuthPurposeUser
	if input.Bot {
		purpose = AuthPurposeBot
	}
	client, err := s.clientForPurpose(ctx, purpose)
	if err != nil {
		return "", err
	}
	info, err := client.AuthTest(context.Background())
	if err != nil {
		return "", pluginbinding.Errorf("slack", "%s", err)
	}
	if strings.TrimSpace(info.UserID) == "" {
		return "", pluginbinding.Fail("slack", "Slack token identity has no user_id")
	}
	return strings.TrimSpace(info.UserID), nil
}

func mentionSince(raw string) (int64, string, error) {
	raw = strings.TrimSpace(raw)
	now := time.Now()
	var since time.Time
	if raw == "" {
		since = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	} else {
		duration, err := parseSlackDuration(raw)
		if err != nil {
			return 0, "", err
		}
		since = now.Add(-duration)
	}
	return since.Unix(), since.AddDate(0, 0, -1).Format("2006-01-02"), nil
}

func unreadSince(raw string) (int64, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "14d"
	}
	duration, err := parseSlackDuration(raw)
	if err != nil {
		return 0, "", err
	}
	return time.Now().Add(-duration).Unix(), raw, nil
}

func parseSlackDuration(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	if strings.HasSuffix(raw, "d") {
		days := strings.TrimSuffix(raw, "d")
		duration, err := time.ParseDuration(days + "h")
		if err != nil {
			return 0, pluginbinding.Errorf("bad_input", "invalid since duration %q", raw)
		}
		return duration * 24, nil
	}
	duration, err := time.ParseDuration(raw)
	if err != nil {
		return 0, pluginbinding.Errorf("bad_input", "invalid since duration %q", raw)
	}
	return duration, nil
}

func (s Service) ownSlackUserIDs(ctx pluginbinding.Context) map[string]bool {
	ids := map[string]bool{}
	for _, purpose := range []string{AuthPurposeUser, AuthPurposeBot} {
		client, err := s.openClient(ctx, purpose)
		if err != nil {
			continue
		}
		info, err := client.AuthTest(context.Background())
		if err != nil {
			continue
		}
		if strings.TrimSpace(info.UserID) != "" {
			ids[strings.TrimSpace(info.UserID)] = true
		}
	}
	return ids
}

func (s Service) classifyMention(ctx pluginbinding.Context, message SearchMessage, ownIDs map[string]bool, maxThread int) (string, []SlackFile) {
	rootTS := firstNonEmpty(message.ThreadTS, message.TS)
	if rootTS == "" || message.Channel == "" {
		return "pending", nil
	}
	if maxThread <= 0 {
		maxThread = 50
	}
	thread, _, err := pluginbinding.ReadWithPreferredAuthPurposes[Client, []ThreadMessage]([]string{AuthPurposeUser, AuthPurposeBot}, s.openClientForContext(ctx), func(client Client, _ string) ([]ThreadMessage, error) {
		return client.GetThread(context.Background(), message.Channel, rootTS, maxThread, 0)
	}, fallbackableSlackError)
	if err != nil || len(thread) == 0 {
		return "pending", nil
	}
	var files []SlackFile
	for index, reply := range thread {
		if reply.TS == message.TS {
			files = reply.Files
			if ownIDs[strings.TrimSpace(reply.User)] {
				return "replied", files
			}
			for _, reaction := range reply.Reactions {
				for _, user := range reaction.Users {
					if ownIDs[strings.TrimSpace(user)] {
						return "acked", files
					}
				}
			}
		}
		if index > 0 && ownIDs[strings.TrimSpace(reply.User)] {
			return "replied", files
		}
	}
	return "pending", files
}

func addTicketsToMessages(messages []SearchMessage, keys []string) {
	for index := range messages {
		messages[index].Tickets = extractTickets(messages[index].Text, keys)
	}
}

func collectTicketMentionsFromSearch(messages []SearchMessage) []TicketMention {
	seen := map[string]map[string]bool{}
	for _, message := range messages {
		for _, ticket := range message.Tickets {
			if seen[ticket] == nil {
				seen[ticket] = map[string]bool{}
			}
			if message.Permalink != "" {
				seen[ticket][message.Permalink] = true
			}
		}
	}
	return ticketMentionRecords(seen)
}

func collectTicketMentionsFromMentions(mentions []MentionItem) []TicketMention {
	seen := map[string]map[string]bool{}
	for _, mention := range mentions {
		for _, ticket := range mention.Tickets {
			if seen[ticket] == nil {
				seen[ticket] = map[string]bool{}
			}
			if mention.Permalink != "" {
				seen[ticket][mention.Permalink] = true
			}
		}
	}
	return ticketMentionRecords(seen)
}

func ticketMentionRecords(seen map[string]map[string]bool) []TicketMention {
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]TicketMention, 0, len(keys))
	for _, key := range keys {
		permalinks := make([]string, 0, len(seen[key]))
		for permalink := range seen[key] {
			permalinks = append(permalinks, permalink)
		}
		sort.Strings(permalinks)
		out = append(out, TicketMention{Key: key, Mentions: len(permalinks), Permalinks: permalinks})
	}
	return out
}

func extractTickets(text string, keys []string) []string {
	var pattern string
	cleanKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		key = strings.ToUpper(strings.TrimSpace(key))
		if key != "" {
			cleanKeys = append(cleanKeys, regexp.QuoteMeta(key))
		}
	}
	if len(cleanKeys) == 0 {
		pattern = `\b[A-Z][A-Z0-9]+-\d+\b`
	} else {
		pattern = `(?i)\b(` + strings.Join(cleanKeys, "|") + `)-\d+\b`
	}
	matches := regexp.MustCompile(pattern).FindAllString(text, -1)
	seen := map[string]bool{}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		match = strings.ToUpper(strings.TrimSpace(match))
		if !seen[match] {
			seen[match] = true
			out = append(out, match)
		}
	}
	sort.Strings(out)
	return out
}

func slackTimestampUnix(ts string) int64 {
	ts = normalizeSlackTimestamp(ts)
	if before, _, ok := strings.Cut(ts, "."); ok {
		ts = before
	}
	value, err := time.ParseDuration(ts + "s")
	if err != nil {
		return 0
	}
	return int64(value / time.Second)
}

func filterUsers(users []User, query string) []User {
	if query == "" {
		return users
	}
	out := make([]User, 0, len(users))
	for _, user := range users {
		if containsFold(user.ID, query) || containsFold(user.Name, query) || containsFold(user.RealName, query) || containsFold(user.DisplayName, query) || containsFold(user.Email, query) {
			out = append(out, user)
		}
	}
	return out
}

func filterChannels(channels []Channel, query string) []Channel {
	if query == "" {
		return channels
	}
	out := make([]Channel, 0, len(channels))
	for _, channel := range channels {
		if containsFold(channel.ID, query) || containsFold(channel.Name, query) || containsFold(channel.Topic, query) || containsFold(channel.Purpose, query) {
			out = append(out, channel)
		}
	}
	return out
}

func filterBookmarks(bookmarks []Bookmark, query string) []Bookmark {
	if query == "" {
		return bookmarks
	}
	out := make([]Bookmark, 0, len(bookmarks))
	for _, bookmark := range bookmarks {
		if containsFold(bookmark.ID, query) || containsFold(bookmark.Title, query) || containsFold(bookmark.Link, query) || containsFold(bookmark.Type, query) {
			out = append(out, bookmark)
		}
	}
	return out
}

func filterFiles(files []FileRecord, query string) []FileRecord {
	if query == "" {
		return files
	}
	out := make([]FileRecord, 0, len(files))
	for _, file := range files {
		if containsFold(file.ID, query) || containsFold(file.Name, query) || containsFold(file.Title, query) || containsFold(file.Mimetype, query) || containsFold(file.Filetype, query) || containsFold(file.PrettyType, query) || containsFold(file.User, query) {
			out = append(out, file)
		}
	}
	return out
}

func limitUsers(users []User, limit int) []User {
	if limit <= 0 || len(users) <= limit {
		return users
	}
	return users[:limit]
}

func limitChannels(channels []Channel, limit int) []Channel {
	if limit <= 0 || len(channels) <= limit {
		return channels
	}
	return channels[:limit]
}

func limitBookmarks(bookmarks []Bookmark, limit int) []Bookmark {
	if limit <= 0 || len(bookmarks) <= limit {
		return bookmarks
	}
	return bookmarks[:limit]
}

func limitFiles(files []FileRecord, limit int) []FileRecord {
	if limit <= 0 || len(files) <= limit {
		return files
	}
	return files[:limit]
}

type messageContent struct {
	Text        string
	Blocks      []json.RawMessage
	UnfurlLinks *bool
	UnfurlMedia *bool
	Parse       string
}

func (s Service) messageContent(ctx pluginbinding.Context, text, markdown string, blocks []json.RawMessage, unfurlLinks, unfurlMedia *bool, parse string) (messageContent, error) {
	text = strings.TrimSpace(text)
	markdown = strings.TrimSpace(markdown)
	hasBlocks := len(blocks) > 0
	if hasBlocks && len(blocks) > 50 {
		return messageContent{}, pluginbinding.Fail("bad_input", "blocks cannot contain more than 50 items")
	}
	switch {
	case hasBlocks:
		if markdown != "" {
			return messageContent{}, pluginbinding.Fail("bad_input", "blocks cannot be combined with markdown")
		}
		if text == "" {
			return messageContent{}, pluginbinding.Fail("bad_input", "text fallback is required when blocks are provided")
		}
		return messageContent{Text: s.resolveMessageText(ctx, text), Blocks: blocks, UnfurlLinks: unfurlLinks, UnfurlMedia: unfurlMedia, Parse: strings.TrimSpace(parse)}, nil
	case text != "" && markdown != "":
		return messageContent{}, pluginbinding.Fail("bad_input", "exactly one of text, markdown, or blocks is required")
	case markdown != "":
		block, err := markdownSectionBlock(markdown)
		if err != nil {
			return messageContent{}, pluginbinding.Errorf("bad_input", "%s", err)
		}
		resolved := s.resolveMessageText(ctx, markdown)
		return messageContent{Text: resolved, Blocks: []json.RawMessage{block}, UnfurlLinks: unfurlLinks, UnfurlMedia: unfurlMedia, Parse: strings.TrimSpace(parse)}, nil
	case text != "":
		return messageContent{Text: s.resolveMessageText(ctx, text), UnfurlLinks: unfurlLinks, UnfurlMedia: unfurlMedia, Parse: strings.TrimSpace(parse)}, nil
	default:
		return messageContent{}, pluginbinding.Fail("bad_input", "exactly one of text, markdown, or blocks is required")
	}
}

func markdownSectionBlock(markdown string) (json.RawMessage, error) {
	block := map[string]any{
		"type": "section",
		"text": map[string]any{
			"type": "mrkdwn",
			"text": markdown,
		},
	}
	return json.Marshal(block)
}

func emojiListMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "custom":
		return "custom"
	case "builtin":
		return "builtin"
	case "all":
		return "all"
	default:
		return ""
	}
}

func emojiRecords(emojis EmojiSet, mode string, includeAliases bool, query string, limit int) []Emoji {
	var out []Emoji
	if mode == "custom" || mode == "all" {
		names := make([]string, 0, len(emojis.Custom))
		for name := range emojis.Custom {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			value := strings.TrimSpace(emojis.Custom[name])
			record := Emoji{Name: name, Source: "custom"}
			if aliasFor := strings.TrimPrefix(value, "alias:"); aliasFor != value {
				if !includeAliases {
					continue
				}
				record.AliasFor = strings.TrimSpace(aliasFor)
			} else {
				record.URL = value
			}
			if query == "" || containsFold(record.Name, query) {
				out = append(out, record)
			}
		}
	}
	if mode == "builtin" || mode == "all" {
		for _, category := range emojis.Categories {
			for _, name := range category.EmojiNames {
				name = strings.Trim(strings.TrimSpace(name), ":")
				if name == "" || (query != "" && !containsFold(name, query)) {
					continue
				}
				out = append(out, Emoji{Name: name, Source: "builtin", Category: strings.TrimSpace(category.Name)})
			}
		}
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].Name != out[j].Name {
				return out[i].Name < out[j].Name
			}
			if out[i].Source != out[j].Source {
				return out[i].Source < out[j].Source
			}
			return out[i].Category < out[j].Category
		})
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func containsFold(value, query string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(value)), strings.ToLower(query))
}

func userLookupValues(record UserRecord) map[string]string {
	return map[string]string{
		"id":                  record.ID,
		"title":               record.Title,
		"links.self":          record.Links["self"],
		"record.user_id":      record.UserID,
		"record.name":         record.Name,
		"record.real_name":    record.RealName,
		"record.display_name": record.DisplayName,
		"record.email":        record.Email,
		"record.web_url":      record.WebURL,
	}
}

func channelLookupValues(record ChannelRecord) map[string]string {
	return map[string]string{
		"id":                record.ID,
		"title":             record.Title,
		"links.self":        record.Links["self"],
		"record.channel_id": record.ChannelID,
		"record.name":       record.Name,
		"record.topic":      record.Topic,
		"record.purpose":    record.Purpose,
		"record.web_url":    record.WebURL,
	}
}

func filterChannelMembers(members []User, query string) []User {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return members
	}
	out := make([]User, 0, len(members))
	for _, member := range members {
		values := []string{member.ID, member.Name, member.RealName, member.DisplayName, member.Email}
		for _, value := range values {
			if strings.Contains(strings.ToLower(strings.TrimSpace(value)), query) {
				out = append(out, member)
				break
			}
		}
	}
	return out
}

func limitChannelMemberRecords(records []ChannelMemberRecord, limit int) []ChannelMemberRecord {
	if limit <= 0 || len(records) <= limit {
		return records
	}
	return records[:limit]
}

func limitThreadMessages(messages []ThreadMessage, limit int) []ThreadMessage {
	if limit <= 0 || len(messages) <= limit {
		return messages
	}
	return messages[:limit]
}

func fileUploadRequest(ctx pluginbinding.Context, input FileUploadInput) (FileUploadRequest, error) {
	channel := strings.TrimSpace(input.Channel)
	if channel == "" {
		return FileUploadRequest{}, pluginbinding.Fail("bad_input", "channel is required")
	}
	blobRef := strings.TrimSpace(input.BlobRef)
	hasBlobRef := blobRef != ""
	hasContent := len(input.ContentBytes) > 0
	if hasBlobRef == hasContent {
		return FileUploadRequest{}, pluginbinding.Fail("bad_input", "provide exactly one of blob_ref or content_bytes")
	}
	filename := strings.TrimSpace(input.Filename)
	content := append([]byte(nil), input.ContentBytes...)
	if hasBlobRef {
		blob, err := ctx.Host.BlobRead(pluginbinding.BlobReadRequest{Ref: blobRef})
		if err != nil {
			return FileUploadRequest{}, err
		}
		content = append([]byte(nil), blob.Content...)
		if filename == "" {
			filename = firstNonEmpty(blob.Blob.Filename, blobPathFilename(blob.Blob.Path), blob.Blob.Ref)
		}
	}
	if len(content) == 0 {
		return FileUploadRequest{}, pluginbinding.Fail("bad_input", "file content is empty")
	}
	if filename == "" {
		return FileUploadRequest{}, pluginbinding.Fail("bad_input", "filename is required when using content_bytes or unnamed blob_ref")
	}
	return FileUploadRequest{
		Channel:        channel,
		ThreadTS:       strings.TrimSpace(input.ThreadTS),
		Content:        content,
		Filename:       filename,
		InitialComment: strings.TrimSpace(input.InitialComment),
		AltText:        strings.TrimSpace(input.AltText),
	}, nil
}

func blobPathFilename(path string) string {
	path = strings.TrimRight(strings.TrimSpace(path), "/")
	if path == "" {
		return ""
	}
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return strings.TrimSpace(path[idx+1:])
	}
	return path
}
