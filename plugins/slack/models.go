package slack

import "strings"

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
	Entity      string `json:"entity"`
	ID          string `json:"id"`
	Title       string `json:"title,omitempty"`
	UserID      string `json:"user_id"`
	Name        string `json:"name,omitempty"`
	RealName    string `json:"real_name,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
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
	Entity      string `json:"entity"`
	ID          string `json:"id"`
	Title       string `json:"title,omitempty"`
	ChannelID   string `json:"channel_id"`
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
	WebURL      string `json:"web_url,omitempty"`
}

type IndexOptions struct {
	Index  string `json:"index,omitempty"`
	Entity string `json:"entity,omitempty"`
}

func normalizeUserRecord(user User) (UserRecord, bool) {
	if strings.TrimSpace(user.ID) == "" || user.Deleted {
		return UserRecord{}, false
	}
	return UserRecord{
		Entity:      "slack.user",
		ID:          user.ID,
		Title:       userTitle(user),
		UserID:      user.ID,
		Name:        user.Name,
		RealName:    user.RealName,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		TeamID:      user.TeamID,
		TZ:          user.TZ,
		IsBot:       user.IsBot,
		IsAppUser:   user.IsAppUser,
		WebURL:      "slack://user/" + user.ID,
	}, true
}

func normalizeChannelRecord(channel Channel) (ChannelRecord, bool) {
	if strings.TrimSpace(channel.ID) == "" {
		return ChannelRecord{}, false
	}
	return ChannelRecord{
		Entity:      "slack.channel",
		ID:          channel.ID,
		Title:       channel.Name,
		ChannelID:   channel.ID,
		Name:        channel.Name,
		IsChannel:   channel.IsChannel,
		IsGroup:     channel.IsGroup,
		IsPrivate:   channel.IsPrivate,
		IsArchived:  channel.IsArchived,
		IsIM:        channel.IsIM,
		IsMPIM:      channel.IsMPIM,
		IsMember:    channel.IsMember,
		NumMembers:  channel.NumMembers,
		User:        channel.User,
		Topic:       channel.Topic,
		Purpose:     channel.Purpose,
		ContextTeam: channel.ContextTeam,
		WebURL:      "slack://channel/" + channel.ID,
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
