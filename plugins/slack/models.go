package slack

import (
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type User struct {
	ID          string `json:"id"`
	TeamID      string `json:"team_id,omitempty"`
	Name        string `json:"name,omitempty"`
	RealName    string `json:"real_name,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Email       string `json:"email,omitempty"`
	TZ          string `json:"tz,omitempty"`
	IsBot       bool   `json:"is_bot,omitempty"`
	IsAppUser   bool   `json:"is_app_user,omitempty"`
	Deleted     bool   `json:"deleted,omitempty"`
}

type AuthInfo struct {
	URL          string `json:"url,omitempty"`
	Team         string `json:"team,omitempty"`
	User         string `json:"user,omitempty"`
	TeamID       string `json:"team_id,omitempty"`
	UserID       string `json:"user_id,omitempty"`
	BotID        string `json:"bot_id,omitempty"`
	EnterpriseID string `json:"enterprise_id,omitempty"`
}

type UserRecord struct {
	pluginbinding.DatasourceRecord
	Title       string `json:"title,omitempty" datasource:"title,view=compact|lookup|table"`
	UserID      string `json:"user_id" datasource:"id"`
	Name        string `json:"name,omitempty" datasource:"completion,view=compact|lookup|table"`
	RealName    string `json:"real_name,omitempty" datasource:"completion,view=compact|lookup|table"`
	DisplayName string `json:"display_name,omitempty" datasource:"completion,view=compact|lookup|table"`
	Email       string `json:"email,omitempty"`
	TeamID      string `json:"team_id,omitempty"`
	TZ          string `json:"tz,omitempty"`
	IsBot       bool   `json:"is_bot,omitempty"`
	IsAppUser   bool   `json:"is_app_user,omitempty"`
	WebURL      string `json:"web_url,omitempty"`
}

type Channel struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	IsChannel   bool   `json:"is_channel,omitempty"`
	IsGroup     bool   `json:"is_group,omitempty"`
	IsPrivate   bool   `json:"is_private,omitempty"`
	IsArchived  bool   `json:"is_archived,omitempty"`
	IsIM        bool   `json:"is_im,omitempty"`
	IsMPIM      bool   `json:"is_mpim,omitempty"`
	IsMember    bool   `json:"is_member,omitempty"`
	NumMembers  int    `json:"num_members,omitempty"`
	User        string `json:"user,omitempty"`
	Topic       string `json:"topic,omitempty"`
	Purpose     string `json:"purpose,omitempty"`
	ContextTeam string `json:"context_team_id,omitempty"`
}

type ChannelRecord struct {
	pluginbinding.DatasourceRecord
	Title       string `json:"title,omitempty" datasource:"title,view=compact|lookup|table"`
	ChannelID   string `json:"channel_id" datasource:"id"`
	Name        string `json:"name,omitempty" datasource:"completion,view=compact|lookup|table"`
	IsChannel   bool   `json:"is_channel,omitempty"`
	IsGroup     bool   `json:"is_group,omitempty"`
	IsPrivate   bool   `json:"is_private,omitempty"`
	IsArchived  bool   `json:"is_archived,omitempty"`
	IsIM        bool   `json:"is_im,omitempty"`
	IsMPIM      bool   `json:"is_mpim,omitempty"`
	IsMember    bool   `json:"is_member,omitempty"`
	NumMembers  int    `json:"num_members,omitempty"`
	User        string `json:"user,omitempty"`
	Topic       string `json:"topic,omitempty"`
	Purpose     string `json:"purpose,omitempty"`
	ContextTeam string `json:"context_team_id,omitempty"`
	WebURL      string `json:"web_url,omitempty"`
}

type MessageRecord struct {
	pluginbinding.DatasourceRecord
	Title     string `json:"title,omitempty" datasource:"title,view=compact|lookup|table"`
	MessageID string `json:"message_id" datasource:"id"`
	Channel   string `json:"channel" datasource:"completion,view=compact|lookup|table,relation=slack.channel:channel"`
	TS        string `json:"ts" datasource:"view=compact|lookup|table"`
	User      string `json:"user,omitempty" datasource:"completion,view=compact|lookup|table,relation=slack.user:user"`
	Text      string `json:"text,omitempty" datasource:"completion,view=compact|lookup|table"`
	WebURL    string `json:"web_url,omitempty"`
}

type ThreadMessageRecord struct {
	pluginbinding.DatasourceRecord
	Title           string `json:"title,omitempty" datasource:"title,view=compact|lookup|table"`
	ThreadMessageID string `json:"thread_message_id" datasource:"id"`
	Channel         string `json:"channel" datasource:"completion,view=compact|lookup|table,relation=slack.channel:channel"`
	RootTS          string `json:"root_ts" datasource:"completion,view=compact|lookup|table"`
	ReplyTS         string `json:"reply_ts" datasource:"completion,view=compact|lookup|table"`
	User            string `json:"user,omitempty" datasource:"completion,view=compact|lookup|table,relation=slack.user:user"`
	Text            string `json:"text,omitempty" datasource:"completion,view=compact|lookup|table"`
	WebURL          string `json:"web_url,omitempty"`
	ThreadURL       string `json:"thread_url,omitempty"`
}

type ChannelMemberRecord struct {
	pluginbinding.DatasourceRecord
	Title           string `json:"title,omitempty" datasource:"title,view=compact|lookup|table"`
	ChannelMemberID string `json:"channel_member_id" datasource:"id"`
	Channel         string `json:"channel" datasource:"completion,view=compact|lookup|table,relation=slack.channel:channel"`
	UserID          string `json:"user_id" datasource:"completion,view=compact|lookup|table,relation=slack.user:user"`
	Name            string `json:"name,omitempty" datasource:"completion,view=compact|lookup|table"`
	RealName        string `json:"real_name,omitempty" datasource:"completion,view=compact|lookup|table"`
	DisplayName     string `json:"display_name,omitempty" datasource:"completion,view=compact|lookup|table"`
	Email           string `json:"email,omitempty" datasource:"completion,view=lookup|table"`
	WebURL          string `json:"web_url,omitempty"`
	ChannelURL      string `json:"channel_url,omitempty"`
}

type IndexOptions struct {
	Index  string `json:"index,omitempty"`
	Entity string `json:"entity,omitempty"`
}

const (
	slackUserURLPrefix    = "slack://user/"
	slackChannelURLPrefix = "slack://channel/"
)

func normalizeUserRecord(source pluginbinding.DatasourceSource, user User) (UserRecord, bool) {
	if strings.TrimSpace(user.ID) == "" || user.Deleted {
		return UserRecord{}, false
	}
	webURL := slackUserURLPrefix + user.ID
	title := userTitle(user)
	return UserRecord{
		DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityUser, user.ID, pluginbinding.RecordTitle(title), pluginbinding.RecordLink("self", webURL)),
		Title:            title,
		UserID:           user.ID,
		Name:             user.Name,
		RealName:         user.RealName,
		DisplayName:      user.DisplayName,
		Email:            user.Email,
		TeamID:           user.TeamID,
		TZ:               user.TZ,
		IsBot:            user.IsBot,
		IsAppUser:        user.IsAppUser,
		WebURL:           webURL,
	}, true
}

func normalizeChannelRecord(source pluginbinding.DatasourceSource, channel Channel) (ChannelRecord, bool) {
	if strings.TrimSpace(channel.ID) == "" {
		return ChannelRecord{}, false
	}
	webURL := slackChannelURLPrefix + channel.ID
	return ChannelRecord{
		DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityChannel, channel.ID, pluginbinding.RecordTitle(channel.Name), pluginbinding.RecordLink("self", webURL)),
		Title:            channel.Name,
		ChannelID:        channel.ID,
		Name:             channel.Name,
		IsChannel:        channel.IsChannel,
		IsGroup:          channel.IsGroup,
		IsPrivate:        channel.IsPrivate,
		IsArchived:       channel.IsArchived,
		IsIM:             channel.IsIM,
		IsMPIM:           channel.IsMPIM,
		IsMember:         channel.IsMember,
		NumMembers:       channel.NumMembers,
		User:             channel.User,
		Topic:            channel.Topic,
		Purpose:          channel.Purpose,
		ContextTeam:      channel.ContextTeam,
		WebURL:           webURL,
	}, true
}

func normalizeMessageRecord(source pluginbinding.DatasourceSource, message SearchMessage) (MessageRecord, bool) {
	channel := strings.TrimSpace(message.Channel)
	ts := strings.TrimSpace(message.TS)
	if channel == "" || ts == "" {
		return MessageRecord{}, false
	}
	id := messageRecordID(channel, ts)
	webURL := slackMessageURL(channel, ts)
	title := messageTitle(message.Text, ts)
	return MessageRecord{
		DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityMessage, id, pluginbinding.RecordTitle(title), pluginbinding.RecordLink("self", webURL)),
		Title:            title,
		MessageID:        id,
		Channel:          channel,
		TS:               ts,
		User:             strings.TrimSpace(message.User),
		Text:             strings.TrimSpace(message.Text),
		WebURL:           webURL,
	}, true
}

func normalizeThreadMessageRecord(source pluginbinding.DatasourceSource, channel, rootTS string, message ThreadMessage) (ThreadMessageRecord, bool) {
	channel = strings.TrimSpace(channel)
	rootTS = strings.TrimSpace(rootTS)
	replyTS := strings.TrimSpace(message.TS)
	if channel == "" || rootTS == "" || replyTS == "" {
		return ThreadMessageRecord{}, false
	}
	id := threadMessageRecordID(channel, rootTS, replyTS)
	webURL := slackMessageURL(channel, replyTS)
	threadURL := slackMessageURL(channel, rootTS)
	title := messageTitle(message.Text, replyTS)
	return ThreadMessageRecord{
		DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityThreadMessage, id, pluginbinding.RecordTitle(title), pluginbinding.RecordLink("self", webURL), pluginbinding.RecordLink("thread", threadURL)),
		Title:            title,
		ThreadMessageID:  id,
		Channel:          channel,
		RootTS:           rootTS,
		ReplyTS:          replyTS,
		User:             strings.TrimSpace(message.User),
		Text:             strings.TrimSpace(message.Text),
		WebURL:           webURL,
		ThreadURL:        threadURL,
	}, true
}

func normalizeChannelMemberRecord(source pluginbinding.DatasourceSource, channel string, user User) (ChannelMemberRecord, bool) {
	channel = strings.TrimSpace(channel)
	user.ID = strings.TrimSpace(user.ID)
	if channel == "" || user.ID == "" || user.Deleted {
		return ChannelMemberRecord{}, false
	}
	id := channelMemberRecordID(channel, user.ID)
	webURL := slackUserURLPrefix + user.ID
	channelURL := slackChannelURLPrefix + channel
	title := userTitle(user)
	return ChannelMemberRecord{
		DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityChannelMember, id, pluginbinding.RecordTitle(title), pluginbinding.RecordLink("self", webURL), pluginbinding.RecordLink("channel", channelURL)),
		Title:            title,
		ChannelMemberID:  id,
		Channel:          channel,
		UserID:           user.ID,
		Name:             user.Name,
		RealName:         user.RealName,
		DisplayName:      user.DisplayName,
		Email:            user.Email,
		WebURL:           webURL,
		ChannelURL:       channelURL,
	}, true
}

func userTitle(user User) string {
	for _, value := range []string{user.DisplayName, user.RealName, user.Name, user.ID} {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func messageTitle(text, fallback string) string {
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if len(runes) > 80 {
		text = strings.TrimSpace(string(runes[:80]))
	}
	if text != "" {
		return text
	}
	return strings.TrimSpace(fallback)
}

func messageRecordID(channel, ts string) string {
	return strings.TrimSpace(channel) + ":" + strings.TrimSpace(ts)
}

func threadMessageRecordID(channel, rootTS, replyTS string) string {
	return strings.TrimSpace(channel) + ":" + strings.TrimSpace(rootTS) + ":" + strings.TrimSpace(replyTS)
}

func channelMemberRecordID(channel, userID string) string {
	return strings.TrimSpace(channel) + ":" + strings.TrimSpace(userID)
}

func slackMessageURL(channel, ts string) string {
	return slackChannelURLPrefix + strings.TrimSpace(channel) + "/message/" + strings.TrimSpace(ts)
}
