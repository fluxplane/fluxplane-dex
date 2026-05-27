package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fluxplane/fluxplane-dex/plugins/internal/pluginutil"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

type OperationRunner struct {
	SecretGetter  SecretGetter
	ClientFactory ClientFactory
}

func NewOperationRunner() OperationRunner {
	return OperationRunner{SecretGetter: defaultSecretGetter, ClientFactory: NewLiveClient}
}

func (r OperationRunner) Run(req protocol.Request, call protocol.OperationCall, cache map[string]pluginutil.SecretMaterial) protocol.OperationResult {
	if call.ID == "" {
		call.ID = call.Name
	}
	input, err := operationInput(call)
	if err != nil {
		return opError(call, "bad_input", err.Error())
	}
	switch call.Name {
	case "slack.index.build":
		return r.indexBuild(req, call, cache, input)
	case "slack.message.send", "slack.search", "slack.thread":
		return opError(call, "not_implemented", call.Name+" requires live Slack client migration")
	default:
		return opError(call, "unknown_operation", "unknown Slack operation "+call.Name)
	}
}

func (r OperationRunner) indexBuild(req protocol.Request, call protocol.OperationCall, cache map[string]pluginutil.SecretMaterial, input map[string]any) protocol.OperationResult {
	selector, err := indexBuildSelector(input)
	if err != nil {
		return opError(call, "bad_input", err.Error())
	}
	var indexes []map[string]any
	if selector.includesIndex("slack.users") {
		users, source, err := r.listUsers(req, cache)
		if err != nil {
			return opError(call, "slack", err.Error())
		}
		records := make([]UserRecord, 0, len(users))
		for _, user := range users {
			if record, ok := normalizeUserRecord(user); ok {
				records = append(records, record)
			}
		}
		indexes = append(indexes, map[string]any{"index": "slack.users", "records": records, "count": len(records), "metadata": indexBuildMetadata("slack.user", source, map[string]any{"include_deleted": false, "include_bots": true, "include_app_users": true})})
	}
	if selector.includesIndex("slack.channels") {
		channels, source, err := r.listChannels(req, cache)
		if err != nil {
			return opError(call, "slack", err.Error())
		}
		records := make([]ChannelRecord, 0, len(channels))
		for _, channel := range channels {
			if record, ok := normalizeChannelRecord(channel); ok {
				records = append(records, record)
			}
		}
		indexes = append(indexes, map[string]any{"index": "slack.channels", "records": records, "count": len(records), "metadata": indexBuildMetadata("slack.channel", source, map[string]any{"types": []string{"public_channel", "private_channel", "mpim", "im"}, "exclude_archived": false})})
	}
	firstIndex := ""
	if len(indexes) > 0 {
		firstIndex, _ = indexes[0]["index"].(string)
	}
	return opOK(call, map[string]any{"index": firstIndex, "indexes": indexes})
}

func (r OperationRunner) listUsers(req protocol.Request, cache map[string]pluginutil.SecretMaterial) ([]User, string, error) {
	client, source, err := r.readClient(req, cache, "user_token")
	if err == nil {
		users, listErr := client.ListUsers(context.Background())
		if listErr == nil {
			return users, source, nil
		}
		if !fallbackableSlackError(listErr) {
			return nil, source, listErr
		}
		err = listErr
	}
	return r.listUsersWithBot(req, cache, err)
}

func (r OperationRunner) listUsersWithBot(req protocol.Request, cache map[string]pluginutil.SecretMaterial, userErr error) ([]User, string, error) {
	client, source, err := r.readClient(req, cache, "bot_token")
	if err != nil {
		return nil, "", combineReadErrors("users", userErr, err)
	}
	users, err := client.ListUsers(context.Background())
	if err != nil {
		return nil, source, combineReadErrors("users", userErr, err)
	}
	return users, source, nil
}

func (r OperationRunner) listChannels(req protocol.Request, cache map[string]pluginutil.SecretMaterial) ([]Channel, string, error) {
	client, source, err := r.readClient(req, cache, "user_token")
	if err == nil {
		channels, listErr := client.ListChannels(context.Background())
		if listErr == nil {
			return channels, source, nil
		}
		if !fallbackableSlackError(listErr) {
			return nil, source, listErr
		}
		err = listErr
	}
	return r.listChannelsWithBot(req, cache, err)
}

func (r OperationRunner) listChannelsWithBot(req protocol.Request, cache map[string]pluginutil.SecretMaterial, userErr error) ([]Channel, string, error) {
	client, source, err := r.readClient(req, cache, "bot_token")
	if err != nil {
		return nil, "", combineReadErrors("channels", userErr, err)
	}
	channels, err := client.ListChannels(context.Background())
	if err != nil {
		return nil, source, combineReadErrors("channels", userErr, err)
	}
	return channels, source, nil
}

func (r OperationRunner) readClient(req protocol.Request, cache map[string]pluginutil.SecretMaterial, purpose string) (Client, string, error) {
	material, ok := optionalSecret(req, purpose, cache, r.SecretGetter)
	if !ok {
		return nil, "", fmt.Errorf("%s not available", purpose)
	}
	material.Purpose = purpose
	factory := r.ClientFactory
	if factory == nil {
		factory = NewLiveClient
	}
	client, err := factory(material)
	if err != nil {
		return nil, "", err
	}
	return client, purpose, nil
}

func combineReadErrors(kind string, userErr, botErr error) error {
	if userErr == nil {
		return fmt.Errorf("read Slack %s with bot_token: %w", kind, botErr)
	}
	return fmt.Errorf("read Slack %s failed with user_token (%v) and bot_token (%v)", kind, userErr, botErr)
}

type indexSelector struct {
	indexes map[string]bool
}

func (s indexSelector) includesIndex(index string) bool {
	if len(s.indexes) == 0 {
		return true
	}
	return s.indexes[index]
}

func indexBuildSelector(input map[string]any) (indexSelector, error) {
	values := splitSelectorValues(firstString(input, "index", "indexes"), firstString(input, "entity", "entities"))
	if len(values) == 0 {
		return indexSelector{}, nil
	}
	known := map[string]string{
		"slack.users":    "slack.users",
		"slack.user":     "slack.users",
		"user":           "slack.users",
		"users":          "slack.users",
		"slack.channels": "slack.channels",
		"slack.channel":  "slack.channels",
		"channel":        "slack.channels",
		"channels":       "slack.channels",
	}
	selector := indexSelector{indexes: map[string]bool{}}
	for _, value := range values {
		index, ok := known[value]
		if !ok {
			return indexSelector{}, fmt.Errorf("unknown Slack index/entity selector %q", value)
		}
		selector.indexes[index] = true
	}
	return selector, nil
}

func splitSelectorValues(values ...string) []string {
	var out []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.ToLower(strings.TrimSpace(part))
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func indexBuildMetadata(entity, source string, extra map[string]any) map[string]any {
	metadata := map[string]any{
		"entity":       entity,
		"source":       "slack.index.build",
		"fetch_mode":   "all_pages",
		"token_source": source,
	}
	for key, value := range extra {
		metadata[key] = value
	}
	return metadata
}

func operationInput(call protocol.OperationCall) (map[string]any, error) {
	input := map[string]any{}
	if len(call.Input) == 0 {
		return input, nil
	}
	if err := json.Unmarshal(call.Input, &input); err != nil {
		return nil, fmt.Errorf("decode operation input: %w", err)
	}
	return input, nil
}

func firstString(input map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := input[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func opOK(call protocol.OperationCall, value any) protocol.OperationResult {
	raw, _ := json.Marshal(value)
	return protocol.OperationResult{ID: call.ID, Name: call.Name, OK: true, Result: raw}
}

func opError(call protocol.OperationCall, code, message string) protocol.OperationResult {
	return protocol.OperationResult{ID: call.ID, Name: call.Name, OK: false, Error: &protocol.Error{Code: code, Message: message}}
}
