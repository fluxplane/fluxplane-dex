package slack

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	slackapi "github.com/slack-go/slack"
)

type Client interface {
	ListUsers(context.Context) ([]User, error)
	ListChannels(context.Context) ([]Channel, error)
	SendMessage(context.Context, string, string) (string, error)
	SearchMessages(context.Context, string, int) ([]SearchMessage, int, error)
	GetThread(context.Context, string, string, int) ([]ThreadMessage, error)
}

type ClientFactory func(pluginbinding.SecretMaterial) (Client, error)

func NewLiveClient(material pluginbinding.SecretMaterial) (Client, error) {
	token := strings.TrimSpace(material.Value)
	if token == "" {
		return nil, fmt.Errorf("%s is empty", material.Purpose)
	}
	return liveClient{client: slackapi.New(token), purpose: material.Purpose}, nil
}

type liveClient struct {
	client  *slackapi.Client
	purpose string
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

func (c liveClient) SendMessage(ctx context.Context, channel, text string) (string, error) {
	_, ts, err := c.client.PostMessageContext(ctx, channel, slackapi.MsgOptionText(text, false))
	return ts, err
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
			Channel: message.Channel.ID,
			TS:      message.Timestamp,
			User:    firstNonEmpty(message.User, message.Username),
			Text:    strings.TrimSpace(message.Text),
		})
	}
	return out, messages.Total, nil
}

func (c liveClient) GetThread(ctx context.Context, channel, ts string, limit int) ([]ThreadMessage, error) {
	if limit <= 0 {
		limit = 100
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
		out = append(out, ThreadMessage{
			TS:   reply.Timestamp,
			User: reply.User,
			Text: strings.TrimSpace(reply.Text),
		})
	}
	return out, nil
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
