package slack

import (
	"context"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type Service struct {
	SecretGetter  pluginbinding.SecretGetter
	ClientFactory ClientFactory
}

func NewService() Service {
	return Service{ClientFactory: NewLiveClient}
}

type IndexBuildInput = pluginbinding.IndexBuildInput

type LookupInput = pluginbinding.DatasourceLookupInput
type LookupResult = pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]

type MessageSendInput struct {
	Channel string `json:"channel,omitempty" jsonschema:"required,description=Slack channel ID or name"`
	Text    string `json:"text,omitempty" jsonschema:"required,description=Message text"`
}

type MessageSendResult struct {
	Channel string `json:"channel,omitempty"`
	TS      string `json:"ts,omitempty"`
	OK      bool   `json:"ok"`
}

type SearchInput struct {
	Query string `json:"query,omitempty" jsonschema:"required,description=Slack search query"`
	Limit int    `json:"limit,omitempty" jsonschema:"description=Maximum messages to return"`
}

type SearchResult struct {
	Count    int             `json:"count"`
	Messages []SearchMessage `json:"messages,omitempty"`
}

type SearchMessage struct {
	Channel string `json:"channel,omitempty"`
	TS      string `json:"ts,omitempty"`
	User    string `json:"user,omitempty"`
	Text    string `json:"text,omitempty"`
}

type ThreadInput struct {
	Channel string `json:"channel,omitempty" jsonschema:"required,description=Slack channel ID"`
	TS      string `json:"ts,omitempty" jsonschema:"required,description=Slack message timestamp"`
	Limit   int    `json:"limit,omitempty" jsonschema:"description=Maximum replies to return"`
}

type ThreadResult struct {
	Channel  string          `json:"channel,omitempty"`
	TS       string          `json:"ts,omitempty"`
	Count    int             `json:"count"`
	Messages []ThreadMessage `json:"messages,omitempty"`
}

type ThreadMessage struct {
	TS   string `json:"ts,omitempty"`
	User string `json:"user,omitempty"`
	Text string `json:"text,omitempty"`
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

func (s Service) SendMessage(ctx pluginbinding.Context, input MessageSendInput) (MessageSendResult, error) {
	channel := strings.TrimSpace(input.Channel)
	text := strings.TrimSpace(input.Text)
	if channel == "" {
		return MessageSendResult{}, pluginbinding.Fail("bad_input", "channel is required")
	}
	if text == "" {
		return MessageSendResult{}, pluginbinding.Fail("bad_input", "text is required")
	}
	material, err := ctx.Secret(AuthPurposeBot)
	if err != nil {
		return MessageSendResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	client, err := s.client(material)
	if err != nil {
		return MessageSendResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	ts, err := client.SendMessage(context.Background(), channel, text)
	if err != nil {
		return MessageSendResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	return MessageSendResult{Channel: channel, TS: ts, OK: true}, nil
}

func (s Service) Search(ctx pluginbinding.Context, input SearchInput) (SearchResult, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return SearchResult{}, pluginbinding.Fail("bad_input", "query is required")
	}
	messages, _, err := pluginbinding.ReadWithPreferredSecrets[Client, searchMessagesOutput](ctx, []string{AuthPurposeUser, AuthPurposeBot}, s.client, func(client Client, _ string) (searchMessagesOutput, error) {
		messages, total, err := client.SearchMessages(context.Background(), query, input.Limit)
		return searchMessagesOutput{Messages: messages, Total: total}, err
	}, fallbackableSlackError)
	if err != nil {
		return SearchResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	return SearchResult{Count: messages.Total, Messages: messages.Messages}, nil
}

func (s Service) Thread(ctx pluginbinding.Context, input ThreadInput) (ThreadResult, error) {
	channel := strings.TrimSpace(input.Channel)
	ts := strings.TrimSpace(input.TS)
	if channel == "" {
		return ThreadResult{}, pluginbinding.Fail("bad_input", "channel is required")
	}
	if ts == "" {
		return ThreadResult{}, pluginbinding.Fail("bad_input", "ts is required")
	}
	messages, _, err := pluginbinding.ReadWithPreferredSecrets[Client, []ThreadMessage](ctx, []string{AuthPurposeUser, AuthPurposeBot}, s.client, func(client Client, _ string) ([]ThreadMessage, error) {
		return client.GetThread(context.Background(), channel, ts, input.Limit)
	}, fallbackableSlackError)
	if err != nil {
		return ThreadResult{}, pluginbinding.Errorf("slack", "%s", err)
	}
	return ThreadResult{Channel: channel, TS: ts, Count: len(messages), Messages: messages}, nil
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
	return pluginbinding.ReadWithPreferredSecrets[Client, []User](ctx, []string{AuthPurposeUser, AuthPurposeBot}, s.client, func(client Client, _ string) ([]User, error) {
		return client.ListUsers(context.Background())
	}, fallbackableSlackError)
}

func (s Service) listChannels(ctx pluginbinding.Context) ([]Channel, string, error) {
	return pluginbinding.ReadWithPreferredSecrets[Client, []Channel](ctx, []string{AuthPurposeUser, AuthPurposeBot}, s.client, func(client Client, _ string) ([]Channel, error) {
		return client.ListChannels(context.Background())
	}, fallbackableSlackError)
}

func (s Service) client(material pluginbinding.SecretMaterial) (Client, error) {
	factory := s.ClientFactory
	if factory == nil {
		factory = NewLiveClient
	}
	return factory(material)
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
