package slack

import (
	"net/url"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type slackMessageRef struct {
	Channel string
	TS      string
}

func normalizeSlackTimestamp(ts string) string {
	ts = strings.TrimSpace(ts)
	if ts == "" {
		return ""
	}
	if strings.HasPrefix(ts, "p") && len(ts) > 1 {
		ts = ts[1:]
	}
	if idx := strings.IndexAny(ts, "?#"); idx >= 0 {
		ts = ts[:idx]
	}
	if !strings.Contains(ts, ".") && len(ts) > 10 && allDigits(ts) {
		return ts[:10] + "." + ts[10:]
	}
	return ts
}

func parseSlackMessageRef(ref string) (slackMessageRef, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return slackMessageRef{}, false
	}
	if parsed, err := url.Parse(ref); err == nil && parsed.Scheme != "" {
		if parsed.Scheme == "http" || parsed.Scheme == "https" {
			parts := splitPath(parsed.Path)
			for i := 0; i+2 < len(parts); i++ {
				if parts[i] == "archives" {
					return slackMessageRef{Channel: parts[i+1], TS: normalizeSlackTimestamp(parts[i+2])}, true
				}
			}
		}
		if parsed.Scheme == "slack" && parsed.Host == "channel" {
			parts := splitPath(parsed.Path)
			if len(parts) >= 3 && parts[1] == "message" {
				return slackMessageRef{Channel: parts[0], TS: normalizeSlackTimestamp(parts[2])}, true
			}
		}
	}
	if channel, ts, ok := strings.Cut(ref, ":"); ok {
		channel = strings.TrimSpace(channel)
		ts = normalizeSlackTimestamp(ts)
		if channel != "" && ts != "" {
			return slackMessageRef{Channel: channel, TS: ts}, true
		}
	}
	return slackMessageRef{}, false
}

func extractSlackThreadTS(ref string) string {
	parsed, err := url.Parse(strings.TrimSpace(ref))
	if err != nil {
		return ""
	}
	return normalizeSlackTimestamp(parsed.Query().Get("thread_ts"))
}

func (s Service) resolveMessageRef(ctx pluginbinding.Context, ref, channel, ts string) (slackMessageRef, error) {
	if parsed, ok := parseSlackMessageRef(ref); ok {
		channel = parsed.Channel
		ts = parsed.TS
	}
	channel = strings.TrimSpace(channel)
	ts = normalizeSlackTimestamp(ts)
	if channel == "" {
		return slackMessageRef{}, pluginbinding.Fail("bad_input", "channel or ref is required")
	}
	if ts == "" {
		return slackMessageRef{}, pluginbinding.Fail("bad_input", "ts or ref is required")
	}
	resolved, err := s.resolveChannel(ctx, channel)
	if err != nil {
		return slackMessageRef{}, err
	}
	return slackMessageRef{Channel: resolved, TS: ts}, nil
}

func (s Service) resolveChannel(ctx pluginbinding.Context, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", pluginbinding.Fail("bad_input", "channel is required")
	}
	if id, ok := slackChannelID(raw); ok {
		return id, nil
	}
	id, err := resolveIndexedID(ctx, EntityChannel, strings.TrimPrefix(raw, "#"), raw)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s Service) resolveMessageText(ctx pluginbinding.Context, text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if hasResolvableUserMention(text) {
		text = resolveUserMentionsInText(ctx, text)
	}
	if hasResolvableChannelMention(text) {
		text = resolveChannelMentionsInText(ctx, text)
	}
	return text
}

func (s Service) resolveOptionalUser(ctx pluginbinding.Context, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if id, ok := slackUserID(raw); ok {
		return id, nil
	}
	return resolveIndexedID(ctx, EntityUser, strings.TrimPrefix(raw, "@"), raw)
}

func resolveUserMentionsInText(ctx pluginbinding.Context, text string) string {
	return replaceSlackToken(text, '@', func(token string) (string, bool) {
		id, err := resolveIndexedID(ctx, EntityUser, token, "@"+token)
		if err != nil {
			return "", false
		}
		return "<@" + id + ">", true
	})
}

func resolveChannelMentionsInText(ctx pluginbinding.Context, text string) string {
	return replaceSlackToken(text, '#', func(token string) (string, bool) {
		id, err := resolveIndexedID(ctx, EntityChannel, token, "#"+token)
		if err != nil {
			return "", false
		}
		return "<#" + id + ">", true
	})
}

func resolveIndexedID(ctx pluginbinding.Context, entity, term, label string) (string, error) {
	term = strings.TrimSpace(term)
	label = strings.TrimSpace(label)
	if term == "" {
		return "", pluginbinding.Fail("bad_input", "empty Slack reference")
	}
	result, err := ctx.Host.Lookup(pluginbinding.DatasourceLookupInput{
		Text:   label,
		Terms:  []string{term},
		Entity: entity,
		Limit:  1,
	})
	if err != nil {
		return "", pluginbinding.Errorf("host_lookup", "%s", err)
	}
	if len(result.Matches) == 0 || strings.TrimSpace(result.Matches[0].ID) == "" {
		return "", pluginbinding.Fail("bad_input", "unknown Slack reference "+label+"; run dex slack index build")
	}
	return strings.TrimSpace(result.Matches[0].ID), nil
}

func slackUserMatches(user User, token string) bool {
	token = strings.TrimSpace(token)
	for _, value := range []string{user.ID, user.Name, user.RealName, user.DisplayName, user.Email} {
		if strings.EqualFold(strings.TrimSpace(value), token) {
			return true
		}
	}
	return false
}

func replaceSlackToken(text string, marker byte, resolve func(string) (string, bool)) string {
	var out strings.Builder
	out.Grow(len(text))
	for i := 0; i < len(text); {
		if text[i] != marker || !tokenStart(text, i) {
			out.WriteByte(text[i])
			i++
			continue
		}
		end := i + 1
		for end < len(text) && slackTokenChar(text[end]) {
			end++
		}
		if end == i+1 {
			out.WriteByte(text[i])
			i++
			continue
		}
		token := text[i+1 : end]
		if replacement, ok := resolve(token); ok {
			out.WriteString(replacement)
		} else {
			out.WriteString(text[i:end])
		}
		i = end
	}
	return out.String()
}

func hasResolvableUserMention(text string) bool {
	return hasResolvableToken(text, '@')
}

func hasResolvableChannelMention(text string) bool {
	return hasResolvableToken(text, '#')
}

func hasResolvableToken(text string, marker byte) bool {
	for i := 0; i < len(text); i++ {
		if text[i] == marker && tokenStart(text, i) && i+1 < len(text) && slackTokenChar(text[i+1]) {
			return true
		}
	}
	return false
}

func tokenStart(text string, i int) bool {
	if i > 0 {
		prev := text[i-1]
		if prev == '<' || slackTokenChar(prev) {
			return false
		}
	}
	return true
}

func slackTokenChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-'
}

func slackChannelID(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "<#") {
		end := strings.Index(raw, ">")
		if end > 2 {
			id := raw[2:end]
			if pipe := strings.Index(id, "|"); pipe >= 0 {
				id = id[:pipe]
			}
			if isSlackConversationID(id) {
				return id, true
			}
		}
	}
	if isSlackConversationID(raw) {
		return raw, true
	}
	return "", false
}

func slackUserID(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "<@") {
		end := strings.Index(raw, ">")
		if end > 2 {
			id := raw[2:end]
			if pipe := strings.Index(id, "|"); pipe >= 0 {
				id = id[:pipe]
			}
			if isSlackUserID(id) {
				return id, true
			}
		}
	}
	if isSlackUserID(raw) {
		return raw, true
	}
	return "", false
}

func isSlackConversationID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return false
	}
	switch value[0] {
	case 'C', 'G', 'D':
		return allSlackIDChars(value[1:])
	default:
		return false
	}
}

func isSlackUserID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return false
	}
	switch value[0] {
	case 'U', 'W':
		return allSlackIDChars(value[1:])
	default:
		return false
	}
}

func splitPath(path string) []string {
	raw := strings.Split(strings.Trim(path, "/"), "/")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func allSlackIDChars(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}
