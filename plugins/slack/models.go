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

func userTitle(user User) string {
	for _, value := range []string{user.DisplayName, user.RealName, user.Name, user.ID} {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
