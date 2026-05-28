package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	slackapi "github.com/slack-go/slack"
)

const defaultThreadImageMaxBytes = 10 * 1024 * 1024

type Client interface {
	AuthTest(context.Context) (AuthInfo, error)
	ListUsers(context.Context) ([]User, error)
	ListChannels(context.Context) ([]Channel, error)
	ListChannelMembers(context.Context, string, int) ([]User, error)
	ListEmojis(context.Context, bool) (EmojiSet, error)
	ListBookmarks(context.Context, string) ([]Bookmark, error)
	AddBookmark(context.Context, BookmarkAddRequest) (Bookmark, error)
	EditBookmark(context.Context, BookmarkEditRequest) (Bookmark, error)
	DeleteBookmark(context.Context, BookmarkDeleteRequest) error
	GetPresence(context.Context, string) (Presence, error)
	SetPresence(context.Context, string) error
	SendMessage(context.Context, MessageSendRequest) (string, error)
	EditMessage(context.Context, MessageEditRequest) (string, error)
	DeleteMessage(context.Context, MessageRefRequest) error
	AddReaction(context.Context, ReactionAddRequest) error
	RemoveReaction(context.Context, ReactionAddRequest) error
	JoinChannel(context.Context, ChannelJoinRequest) error
	MarkRead(context.Context, MessageRefRequest) error
	LatestMessageTS(context.Context, string) (string, error)
	ListFiles(context.Context, FileListRequest) ([]FileRecord, error)
	GetFileInfo(context.Context, string) (FileRecord, error)
	DownloadFile(context.Context, FileDownloadRequest) (FileDownloadResult, error)
	DeleteFile(context.Context, string) error
	ListUnreads(context.Context, UnreadsRequest) ([]UnreadChannel, error)
	UploadFile(context.Context, FileUploadRequest) (FileUploadResult, error)
	SearchMessages(context.Context, string, int) ([]SearchMessage, int, error)
	GetThread(context.Context, string, string, int, int) ([]ThreadMessage, error)
}

type ClientFactory func(pluginbinding.SecretMaterial) (Client, error)

func NewLiveClient(material pluginbinding.SecretMaterial) (Client, error) {
	token := strings.TrimSpace(material.Value)
	if token == "" {
		return nil, fmt.Errorf("%s is empty", material.Purpose)
	}
	return liveClient{client: slackapi.New(token), token: token, purpose: material.Purpose}, nil
}

type liveClient struct {
	client  *slackapi.Client
	token   string
	purpose string
}

func (c liveClient) AuthTest(ctx context.Context) (AuthInfo, error) {
	response, err := c.client.AuthTestContext(ctx)
	if err != nil {
		return AuthInfo{}, err
	}
	return AuthInfo{
		URL:          strings.TrimSpace(response.URL),
		Team:         strings.TrimSpace(response.Team),
		User:         strings.TrimSpace(response.User),
		TeamID:       strings.TrimSpace(response.TeamID),
		UserID:       strings.TrimSpace(response.UserID),
		BotID:        strings.TrimSpace(response.BotID),
		EnterpriseID: strings.TrimSpace(response.EnterpriseID),
	}, nil
}

func (c liveClient) ListUsers(ctx context.Context) ([]User, error) {
	users, err := c.client.GetUsersContext(ctx, slackapi.GetUsersOptionLimit(200))
	if err != nil {
		return nil, err
	}
	out := make([]User, 0, len(users))
	for _, user := range users {
		out = append(out, userFromAPI(user))
	}
	return out, nil
}

func (c liveClient) ListChannels(ctx context.Context) ([]Channel, error) {
	channels, err := c.client.GetAllConversationsContext(
		ctx,
		slackapi.GetConversationsOptionLimit(200),
		slackapi.GetConversationsOptionExcludeArchived(false),
		slackapi.GetConversationsOptionTypes([]string{"public_channel", "private_channel", "mpim", "im"}),
	)
	if err != nil {
		return nil, err
	}
	out := make([]Channel, 0, len(channels))
	for _, channel := range channels {
		out = append(out, channelFromAPI(channel))
	}
	return out, nil
}

func (c liveClient) ListChannelMembers(ctx context.Context, channel string, limit int) ([]User, error) {
	if limit <= 0 {
		limit = 200
	}
	out := make([]User, 0, limit)
	cursor := ""
	for len(out) < limit {
		pageLimit := limit - len(out)
		if pageLimit > 1000 {
			pageLimit = 1000
		}
		members, nextCursor, err := c.client.GetUsersInConversationContext(ctx, &slackapi.GetUsersInConversationParameters{
			ChannelID: channel,
			Cursor:    cursor,
			Limit:     pageLimit,
		})
		if err != nil {
			return nil, err
		}
		for _, member := range members {
			member = strings.TrimSpace(member)
			if member != "" {
				out = append(out, User{ID: member})
			}
		}
		if strings.TrimSpace(nextCursor) == "" {
			break
		}
		cursor = nextCursor
	}
	return out, nil
}

func (c liveClient) ListEmojis(ctx context.Context, includeCategories bool) (EmojiSet, error) {
	if !includeCategories {
		emojis, err := c.client.GetEmojiContext(ctx)
		return EmojiSet{Custom: emojis}, err
	}
	values := url.Values{}
	values.Set("include_categories", "true")
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, slackapi.APIURL+"emoji.list", strings.NewReader(values.Encode()))
	if err != nil {
		return EmojiSet{}, err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return EmojiSet{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return EmojiSet{}, fmt.Errorf("emoji.list: %s", response.Status)
	}
	var payload struct {
		OK         bool              `json:"ok"`
		Error      string            `json:"error"`
		Emoji      map[string]string `json:"emoji"`
		Categories []struct {
			Name       string   `json:"name"`
			EmojiNames []string `json:"emoji_names"`
		} `json:"categories"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return EmojiSet{}, err
	}
	if !payload.OK {
		return EmojiSet{}, slackapi.SlackErrorResponse{Err: payload.Error}
	}
	categories := make([]EmojiCategory, 0, len(payload.Categories))
	for _, category := range payload.Categories {
		categories = append(categories, EmojiCategory{Name: category.Name, EmojiNames: category.EmojiNames})
	}
	return EmojiSet{Custom: payload.Emoji, Categories: categories}, nil
}

func (c liveClient) ListBookmarks(ctx context.Context, channel string) ([]Bookmark, error) {
	bookmarks, err := c.client.ListBookmarksContext(ctx, channel)
	if err != nil {
		return nil, err
	}
	out := make([]Bookmark, 0, len(bookmarks))
	for _, bookmark := range bookmarks {
		out = append(out, bookmarkFromAPI(bookmark))
	}
	return out, nil
}

func (c liveClient) AddBookmark(ctx context.Context, request BookmarkAddRequest) (Bookmark, error) {
	bookmark, err := c.client.AddBookmarkContext(ctx, request.Channel, slackapi.AddBookmarkParameters{
		Title: request.Title,
		Type:  "link",
		Link:  request.Link,
		Emoji: strings.Trim(request.Emoji, ":"),
	})
	if err != nil {
		return Bookmark{}, err
	}
	return bookmarkFromAPI(bookmark), nil
}

func (c liveClient) EditBookmark(ctx context.Context, request BookmarkEditRequest) (Bookmark, error) {
	var title *string
	var emoji *string
	if strings.TrimSpace(request.Title) != "" {
		value := strings.TrimSpace(request.Title)
		title = &value
	}
	if strings.TrimSpace(request.Emoji) != "" {
		value := strings.Trim(strings.TrimSpace(request.Emoji), ":")
		emoji = &value
	}
	bookmark, err := c.client.EditBookmarkContext(ctx, request.Channel, request.BookmarkID, slackapi.EditBookmarkParameters{
		Title: title,
		Link:  strings.TrimSpace(request.Link),
		Emoji: emoji,
	})
	if err != nil {
		return Bookmark{}, err
	}
	return bookmarkFromAPI(bookmark), nil
}

func (c liveClient) DeleteBookmark(ctx context.Context, request BookmarkDeleteRequest) error {
	return c.client.RemoveBookmarkContext(ctx, request.Channel, request.BookmarkID)
}

func (c liveClient) GetPresence(ctx context.Context, user string) (Presence, error) {
	presence, err := c.client.GetUserPresenceContext(ctx, strings.TrimSpace(user))
	if err != nil {
		return Presence{}, err
	}
	return Presence{
		Presence:        strings.TrimSpace(presence.Presence),
		Online:          presence.Online,
		AutoAway:        presence.AutoAway,
		ManualAway:      presence.ManualAway,
		ConnectionCount: presence.ConnectionCount,
		LastActivity:    int64(presence.LastActivity),
	}, nil
}

func (c liveClient) SetPresence(ctx context.Context, presence string) error {
	return c.client.SetUserPresenceContext(ctx, strings.TrimSpace(presence))
}

func (c liveClient) SendMessage(ctx context.Context, request MessageSendRequest) (string, error) {
	options := []slackapi.MsgOption{slackapi.MsgOptionText(request.Text, false)}
	if len(request.Blocks) > 0 {
		blocks, err := slackBlocksFromJSON(request.Blocks)
		if err != nil {
			return "", err
		}
		options = append(options, slackapi.MsgOptionBlocks(blocks...))
	}
	if request.ThreadTS != "" {
		options = append(options, slackapi.MsgOptionTS(request.ThreadTS))
	}
	if request.ReplyBroadcast {
		options = append(options, slackapi.MsgOptionBroadcast())
	}
	options = append(options, slackMessageOptions(request.UnfurlLinks, request.UnfurlMedia, request.Parse)...)
	_, ts, err := c.client.PostMessageContext(ctx, request.Channel, options...)
	return ts, err
}

func (c liveClient) EditMessage(ctx context.Context, request MessageEditRequest) (string, error) {
	options := []slackapi.MsgOption{slackapi.MsgOptionText(request.Text, false)}
	if len(request.Blocks) > 0 {
		blocks, err := slackBlocksFromJSON(request.Blocks)
		if err != nil {
			return "", err
		}
		options = append(options, slackapi.MsgOptionBlocks(blocks...))
	}
	options = append(options, slackMessageOptions(request.UnfurlLinks, request.UnfurlMedia, request.Parse)...)
	_, ts, _, err := c.client.UpdateMessageContext(ctx, request.Channel, request.TS, options...)
	return ts, err
}

func slackBlocksFromJSON(rawBlocks []json.RawMessage) ([]slackapi.Block, error) {
	blocks := make([]slackapi.Block, 0, len(rawBlocks))
	for _, raw := range rawBlocks {
		block, err := slackapi.BlockFromJSON(string(raw))
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

func slackMessageOptions(unfurlLinks, unfurlMedia *bool, parse string) []slackapi.MsgOption {
	var options []slackapi.MsgOption
	if unfurlLinks != nil {
		if *unfurlLinks {
			options = append(options, slackapi.MsgOptionEnableLinkUnfurl())
		} else {
			options = append(options, slackapi.MsgOptionDisableLinkUnfurl())
		}
	}
	if unfurlMedia != nil && !*unfurlMedia {
		options = append(options, slackapi.MsgOptionDisableMediaUnfurl())
	}
	parse = strings.TrimSpace(parse)
	if parse != "" {
		params := slackapi.NewPostMessageParameters()
		params.Parse = parse
		options = append(options, slackapi.MsgOptionPostMessageParameters(params))
	}
	return options
}

func (c liveClient) DeleteMessage(ctx context.Context, request MessageRefRequest) error {
	_, _, err := c.client.DeleteMessageContext(ctx, request.Channel, request.TS)
	return err
}

func (c liveClient) AddReaction(ctx context.Context, request ReactionAddRequest) error {
	return c.client.AddReactionContext(ctx, strings.Trim(request.Emoji, ":"), slackapi.NewRefToMessage(request.Channel, request.TS))
}

func (c liveClient) RemoveReaction(ctx context.Context, request ReactionAddRequest) error {
	return c.client.RemoveReactionContext(ctx, strings.Trim(request.Emoji, ":"), slackapi.NewRefToMessage(request.Channel, request.TS))
}

func (c liveClient) JoinChannel(ctx context.Context, request ChannelJoinRequest) error {
	_, _, _, err := c.client.JoinConversationContext(ctx, request.Channel)
	return err
}

func (c liveClient) MarkRead(ctx context.Context, request MessageRefRequest) error {
	return c.client.MarkConversationContext(ctx, request.Channel, request.TS)
}

func (c liveClient) LatestMessageTS(ctx context.Context, channel string) (string, error) {
	history, err := c.client.GetConversationHistoryContext(ctx, &slackapi.GetConversationHistoryParameters{
		ChannelID: strings.TrimSpace(channel),
		Limit:     1,
	})
	if err != nil {
		return "", err
	}
	for _, message := range history.Messages {
		ts := strings.TrimSpace(message.Timestamp)
		if ts != "" {
			return ts, nil
		}
	}
	return "", nil
}

func (c liveClient) ListFiles(ctx context.Context, request FileListRequest) ([]FileRecord, error) {
	limit := request.Limit
	if limit <= 0 {
		limit = 100
	}
	params := slackapi.GetFilesParameters{
		Channel: request.Channel,
		User:    request.User,
		Types:   firstNonEmpty(request.Types, "all"),
		Count:   limit,
		Page:    1,
	}
	files, _, err := c.client.GetFilesContext(ctx, params)
	if err != nil {
		return nil, err
	}
	out := make([]FileRecord, 0, len(files))
	for _, file := range files {
		out = append(out, fileRecordFromAPI(file))
	}
	return out, nil
}

func (c liveClient) GetFileInfo(ctx context.Context, fileID string) (FileRecord, error) {
	file, _, _, err := c.client.GetFileInfoContext(ctx, fileID, 0, 0)
	if err != nil {
		return FileRecord{}, err
	}
	return fileRecordFromAPI(*file), nil
}

func (c liveClient) DownloadFile(ctx context.Context, request FileDownloadRequest) (FileDownloadResult, error) {
	file, err := c.GetFileInfo(ctx, request.FileID)
	if err != nil {
		return FileDownloadResult{}, err
	}
	downloadURL := strings.TrimSpace(file.downloadURL)
	if downloadURL == "" {
		return FileDownloadResult{}, errors.New("file has no private download URL")
	}
	outputPath := strings.TrimSpace(request.OutputPath)
	if outputPath == "" {
		return FileDownloadResult{}, errors.New("output_path is required")
	}
	if info, err := os.Stat(outputPath); err == nil && info.IsDir() {
		outputPath = filepath.Join(outputPath, firstNonEmpty(file.Name, file.Title, file.ID))
	}
	if dir := filepath.Dir(outputPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return FileDownloadResult{}, err
		}
	}
	out, err := os.Create(outputPath)
	if err != nil {
		return FileDownloadResult{}, err
	}
	defer out.Close()
	if err := c.client.GetFileContext(ctx, downloadURL, out); err != nil {
		return FileDownloadResult{}, err
	}
	info, err := out.Stat()
	if err != nil {
		return FileDownloadResult{}, err
	}
	return FileDownloadResult{OK: true, FileID: file.ID, Path: outputPath, Size: int(info.Size()), File: file}, nil
}

func (c liveClient) DeleteFile(ctx context.Context, fileID string) error {
	return c.client.DeleteFileContext(ctx, fileID)
}

func (c liveClient) ListUnreads(ctx context.Context, request UnreadsRequest) ([]UnreadChannel, error) {
	channels, err := c.unreadChannels(ctx, request.Channel)
	if err != nil {
		return nil, err
	}
	out := make([]UnreadChannel, 0, len(channels))
	for _, channel := range channels {
		lastRead := strings.TrimSpace(channel.LastRead)
		if lastRead == "" && channel.Latest != nil {
			lastRead = strings.TrimSpace(channel.Latest.Timestamp)
		}
		if lastRead == "" {
			lastRead = "0"
		}
		oldest := lastRead
		if request.Since > 0 {
			sinceTS := fmt.Sprintf("%d.000000", request.Since)
			if sinceTS > oldest {
				oldest = sinceTS
			}
		}
		limit := request.Limit
		if limit <= 0 {
			limit = 100
		}
		history, err := c.client.GetConversationHistoryContext(ctx, &slackapi.GetConversationHistoryParameters{
			ChannelID: channel.ID,
			Oldest:    oldest,
			Limit:     limit,
			Inclusive: false,
		})
		if err != nil {
			continue
		}
		if len(history.Messages) == 0 {
			continue
		}
		messages := unreadMessagesFromAPI(history.Messages)
		name := channel.Name
		if channel.IsIM {
			name = channel.User
		}
		out = append(out, UnreadChannel{
			ID:          channel.ID,
			Name:        name,
			IsPrivate:   channel.IsPrivate,
			IsDM:        channel.IsIM,
			UnreadCount: len(messages),
			LastRead:    lastRead,
			Messages:    messages,
		})
	}
	return out, nil
}

func (c liveClient) unreadChannels(ctx context.Context, channelFilter string) ([]slackapi.Channel, error) {
	channelFilter = strings.TrimSpace(channelFilter)
	var out []slackapi.Channel
	cursor := ""
	for {
		channels, nextCursor, err := c.client.GetConversationsContext(ctx, &slackapi.GetConversationsParameters{
			Cursor:          cursor,
			Limit:           200,
			ExcludeArchived: true,
			Types:           []string{"public_channel", "private_channel", "im", "mpim"},
		})
		if err != nil {
			return nil, err
		}
		for _, channel := range channels {
			if channelFilter != "" && !strings.EqualFold(channel.ID, channelFilter) && !strings.EqualFold(channel.Name, channelFilter) {
				continue
			}
			if channel.IsMember || (channel.IsIM && channel.IsOpen) || (channel.IsMpIM && channel.IsOpen) {
				out = append(out, channel)
			}
		}
		if strings.TrimSpace(nextCursor) == "" || (channelFilter != "" && len(out) > 0) {
			break
		}
		cursor = nextCursor
	}
	if channelFilter != "" && len(out) == 0 {
		return nil, fmt.Errorf("channel %q not found or not visible to user token", channelFilter)
	}
	return out, nil
}

func (c liveClient) UploadFile(ctx context.Context, request FileUploadRequest) (FileUploadResult, error) {
	summary, err := c.client.UploadFileContext(ctx, slackapi.UploadFileParameters{
		FileSize:        len(request.Content),
		Reader:          bytes.NewReader(request.Content),
		Filename:        request.Filename,
		Title:           request.Filename,
		InitialComment:  request.InitialComment,
		Channel:         request.Channel,
		ThreadTimestamp: request.ThreadTS,
		AltTxt:          request.AltText,
	})
	if err != nil {
		return FileUploadResult{}, err
	}
	result := FileUploadResult{
		OK:       true,
		Channel:  request.Channel,
		ThreadTS: request.ThreadTS,
		FileID:   summary.ID,
		Title:    firstNonEmpty(summary.Title, request.Filename),
		Filename: request.Filename,
		Size:     len(request.Content),
	}
	file, _, _, err := c.client.GetFileInfoContext(ctx, summary.ID, 0, 0)
	if err != nil {
		result.Warning = "uploaded file, but could not fetch permalink: " + err.Error()
		return result, nil
	}
	result.Permalink = strings.TrimSpace(file.Permalink)
	return result, nil
}

func (c liveClient) SearchMessages(ctx context.Context, query string, limit int) ([]SearchMessage, int, error) {
	if limit <= 0 {
		limit = 20
	}
	params := slackapi.NewSearchParameters()
	params.Count = limit
	messages, err := c.client.SearchMessagesContext(ctx, query, params)
	if err != nil {
		return nil, 0, err
	}
	out := make([]SearchMessage, 0, len(messages.Matches))
	for _, message := range messages.Matches {
		out = append(out, SearchMessage{
			Channel:   message.Channel.ID,
			TS:        message.Timestamp,
			ThreadTS:  extractSlackThreadTS(message.Permalink),
			User:      firstNonEmpty(message.User, message.Username),
			Text:      strings.TrimSpace(message.Text),
			Permalink: strings.TrimSpace(message.Permalink),
		})
	}
	return out, messages.Total, nil
}

func (c liveClient) GetThread(ctx context.Context, channel, ts string, limit, maxBytes int) ([]ThreadMessage, error) {
	if limit <= 0 {
		limit = 100
	}
	if maxBytes <= 0 {
		maxBytes = defaultThreadImageMaxBytes
	}
	replies, _, _, err := c.client.GetConversationRepliesContext(ctx, &slackapi.GetConversationRepliesParameters{
		ChannelID: channel,
		Timestamp: ts,
		Limit:     limit,
		Inclusive: true,
	})
	if err != nil {
		return nil, err
	}
	out := make([]ThreadMessage, 0, len(replies))
	for _, reply := range replies {
		files, err := c.threadMessageFiles(ctx, reply, maxBytes)
		if err != nil {
			return nil, err
		}
		out = append(out, ThreadMessage{
			TS:        reply.Timestamp,
			User:      reply.User,
			Text:      strings.TrimSpace(reply.Text),
			Files:     files,
			Reactions: reactionsFromAPI(reply.Reactions),
		})
	}
	return limitThreadMessages(out, limit), nil
}

func (c liveClient) downloadFileToTemp(ctx context.Context, file slackapi.File, url string, maxBytes int) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download Slack file: %s", resp.Status)
	}
	temp, err := os.CreateTemp("", "dex-slack-image-*"+imageExtension(file))
	if err != nil {
		return "", err
	}
	path := temp.Name()
	n, copyErr := io.Copy(temp, io.LimitReader(resp.Body, int64(maxBytes)+1))
	closeErr := temp.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return "", closeErr
	}
	if n > int64(maxBytes) {
		_ = os.Remove(path)
		return "", fmt.Errorf("downloaded Slack image exceeds max_bytes %d", maxBytes)
	}
	return path, nil
}

func (c liveClient) threadMessageFiles(ctx context.Context, message slackapi.Message, maxBytes int) ([]SlackFile, error) {
	files := make([]SlackFile, 0, len(message.Files))
	for _, file := range message.Files {
		out := SlackFile{
			FileID:    file.ID,
			Name:      file.Name,
			Title:     firstNonEmpty(file.Title, file.Name),
			Mimetype:  file.Mimetype,
			Filetype:  file.Filetype,
			Permalink: file.Permalink,
			Width:     file.OriginalW,
			Height:    file.OriginalH,
			Size:      file.Size,
		}
		if isSlackImageFile(file) {
			url := firstNonEmpty(file.URLPrivateDownload, file.URLPrivate)
			if url != "" {
				path, err := c.downloadFileToTemp(ctx, file, url, maxBytes)
				if err != nil {
					return nil, err
				}
				out.FilePath = path
			}
		}
		files = append(files, out)
	}
	return files, nil
}

func isSlackImageFile(file slackapi.File) bool {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(file.Mimetype)), "image/") {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(file.Filetype)) {
	case "jpg", "jpeg", "png", "gif", "webp":
		return true
	default:
		return false
	}
}

func imageExtension(file slackapi.File) string {
	if ext := filepath.Ext(strings.TrimSpace(file.Name)); ext != "" {
		return ext
	}
	switch strings.ToLower(strings.TrimSpace(file.Mimetype)) {
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".jpg"
	}
}

func userFromAPI(user slackapi.User) User {
	displayName := strings.TrimSpace(user.Profile.DisplayName)
	if displayName == "" {
		displayName = strings.TrimSpace(user.Profile.DisplayNameNormalized)
	}
	realName := strings.TrimSpace(user.RealName)
	if realName == "" {
		realName = strings.TrimSpace(user.Profile.RealName)
	}
	return User{
		ID:          user.ID,
		TeamID:      user.TeamID,
		Name:        user.Name,
		RealName:    realName,
		DisplayName: displayName,
		Email:       user.Profile.Email,
		TZ:          user.TZ,
		IsBot:       user.IsBot,
		IsAppUser:   user.IsAppUser,
		Deleted:     user.Deleted,
	}
}

func channelFromAPI(channel slackapi.Channel) Channel {
	return Channel{
		ID:          channel.ID,
		Name:        channel.Name,
		IsChannel:   channel.IsChannel,
		IsGroup:     channel.IsGroup,
		IsPrivate:   channel.IsPrivate,
		IsArchived:  channel.IsArchived,
		IsIM:        channel.IsIM,
		IsMPIM:      channel.IsMpIM,
		IsMember:    channel.IsMember,
		NumMembers:  channel.NumMembers,
		User:        channel.User,
		Topic:       channel.Topic.Value,
		Purpose:     channel.Purpose.Value,
		ContextTeam: channel.ContextTeamID,
	}
}

func bookmarkFromAPI(bookmark slackapi.Bookmark) Bookmark {
	return Bookmark{
		ID:       strings.TrimSpace(bookmark.ID),
		Channel:  strings.TrimSpace(bookmark.ChannelID),
		Title:    strings.TrimSpace(bookmark.Title),
		Link:     strings.TrimSpace(bookmark.Link),
		Emoji:    strings.TrimSpace(bookmark.Emoji),
		IconURL:  strings.TrimSpace(bookmark.IconURL),
		Type:     strings.TrimSpace(bookmark.Type),
		EntityID: strings.TrimSpace(bookmark.EntityID),
		AppID:    strings.TrimSpace(bookmark.AppID),
	}
}

func fileRecordFromAPI(file slackapi.File) FileRecord {
	return FileRecord{
		ID:              strings.TrimSpace(file.ID),
		Name:            strings.TrimSpace(file.Name),
		Title:           strings.TrimSpace(file.Title),
		Mimetype:        strings.TrimSpace(file.Mimetype),
		Filetype:        strings.TrimSpace(file.Filetype),
		PrettyType:      strings.TrimSpace(file.PrettyType),
		User:            strings.TrimSpace(file.User),
		Size:            file.Size,
		Permalink:       strings.TrimSpace(file.Permalink),
		PermalinkPublic: strings.TrimSpace(file.PermalinkPublic),
		Channels:        append([]string(nil), file.Channels...),
		Groups:          append([]string(nil), file.Groups...),
		IMs:             append([]string(nil), file.IMs...),
		Created:         int64(file.Created),
		Timestamp:       int64(file.Timestamp),
		downloadURL:     firstNonEmpty(file.URLPrivateDownload, file.URLPrivate),
	}
}

func reactionsFromAPI(reactions []slackapi.ItemReaction) []Reaction {
	out := make([]Reaction, 0, len(reactions))
	for _, reaction := range reactions {
		out = append(out, Reaction{
			Name:  strings.TrimSpace(reaction.Name),
			Users: append([]string(nil), reaction.Users...),
		})
	}
	return out
}

func unreadMessagesFromAPI(messages []slackapi.Message) []UnreadMessage {
	out := make([]UnreadMessage, 0, len(messages))
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		out = append(out, UnreadMessage{
			TS:        strings.TrimSpace(message.Timestamp),
			User:      strings.TrimSpace(message.User),
			Text:      strings.TrimSpace(message.Text),
			ThreadTS:  strings.TrimSpace(message.ThreadTimestamp),
			Files:     slackFilesFromAPI(message.Files),
			Reactions: reactionsFromAPI(message.Reactions),
		})
	}
	return out
}

func slackFilesFromAPI(files []slackapi.File) []SlackFile {
	out := make([]SlackFile, 0, len(files))
	for _, file := range files {
		out = append(out, SlackFile{
			FileID:    strings.TrimSpace(file.ID),
			Name:      strings.TrimSpace(file.Name),
			Title:     firstNonEmpty(file.Title, file.Name),
			Mimetype:  strings.TrimSpace(file.Mimetype),
			Filetype:  strings.TrimSpace(file.Filetype),
			Permalink: strings.TrimSpace(file.Permalink),
			Width:     file.OriginalW,
			Height:    file.OriginalH,
			Size:      file.Size,
		})
	}
	return out
}

func fallbackableSlackError(err error) bool {
	if err == nil {
		return false
	}
	var slackErr slackapi.SlackErrorResponse
	if !errors.As(err, &slackErr) {
		return false
	}
	switch slackErr.Err {
	case "missing_scope", "invalid_auth", "not_authed", "token_revoked", "account_inactive", "no_permission", "not_allowed_token_type":
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
