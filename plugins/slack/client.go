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
