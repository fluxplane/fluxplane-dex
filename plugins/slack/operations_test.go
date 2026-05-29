package slack

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
	slackapi "github.com/slack-go/slack"
)

func TestServiceIndexBuildUsesUserTokenFirst(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				users:    []User{{ID: "U1", Name: "timo", RealName: "Timo Friedl", DisplayName: "Timo"}},
				channels: []Channel{{ID: "C1", Name: "general", IsChannel: true}},
			},
		},
	}
	plugin := testPlugin(factory)

	out := plugintest.RunOK[struct {
		Indexes []struct {
			Index   string            `json:"index"`
			Records []json.RawMessage `json:"records"`
		} `json:"indexes"`
	}](t, plugin, OperationIndexBuild, map[string]any{})
	if len(out.Indexes) != 2 || out.Indexes[0].Index != "slack.users" || out.Indexes[1].Index != "slack.channels" {
		t.Fatalf("indexes = %#v", out.Indexes)
	}
	if len(out.Indexes[0].Records) != 1 || len(out.Indexes[1].Records) != 1 {
		t.Fatalf("records = %#v", out.Indexes)
	}
	var userRecord UserRecord
	if err := json.Unmarshal(out.Indexes[0].Records[0], &userRecord); err != nil {
		t.Fatal(err)
	}
	if userRecord.Source.Plugin != PluginName || userRecord.Source.Instance != "default" || userRecord.Links["self"] != "slack://user/U1" {
		t.Fatalf("unexpected user record source/links: %#v", userRecord)
	}
	if factory.created["bot_token"] != 0 {
		t.Fatalf("bot token should not be used: %#v", factory.created)
	}
}

func TestServiceIndexBuildFallsBackToBotToken(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {usersErr: slackapi.SlackErrorResponse{Err: "missing_scope"}, channelsErr: slackapi.SlackErrorResponse{Err: "missing_scope"}},
			"bot_token": {
				users:    []User{{ID: "U1", Name: "timo"}, {ID: "U2", Name: "deleted", Deleted: true}},
				channels: []Channel{{ID: "C1", Name: "general", IsChannel: true}},
			},
		},
	}
	plugin := testPlugin(factory)

	out := plugintest.RunOK[struct {
		Indexes []struct {
			Index   string            `json:"index"`
			Records []json.RawMessage `json:"records"`
		} `json:"indexes"`
	}](t, plugin, OperationIndexBuild, map[string]any{})
	if factory.created["user_token"] == 0 || factory.created["bot_token"] == 0 {
		t.Fatalf("expected user then bot token: %#v", factory.created)
	}
	if len(out.Indexes[0].Records) != 1 {
		t.Fatalf("deleted users should be filtered: %#v", out.Indexes[0].Records)
	}
}

func TestServiceIndexBuildFallsBackWhenUserTokenMissing(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"bot_token": {users: []User{{ID: "U1", Name: "timo"}}},
		},
	}
	plugin := testPlugin(factory)

	plugintest.RunOK[map[string]any](t, plugin, OperationIndexBuild, map[string]any{"entity": "slack.user"})
	if factory.created["bot_token"] != 1 {
		t.Fatalf("expected bot token fallback: %#v", factory.created)
	}
}

func TestServiceIndexBuildDoesNotFallbackOnNetworkError(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {usersErr: errors.New("network down")},
			"bot_token":  {users: []User{{ID: "U1"}}},
		},
	}
	plugin := testPlugin(factory)

	plugintest.RunError(t, plugin, OperationIndexBuild, map[string]any{"entity": "slack.user"})
	if factory.created["bot_token"] != 0 {
		t.Fatalf("bot token should not be used for non-auth error: %#v", factory.created)
	}
}

func TestServiceIndexBuildCanTargetOneIndex(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {channels: []Channel{{ID: "C1", Name: "general", IsChannel: true}}},
		},
	}
	plugin := testPlugin(factory)

	out := plugintest.RunOK[struct {
		Indexes []struct {
			Index   string            `json:"index"`
			Records []json.RawMessage `json:"records"`
		} `json:"indexes"`
	}](t, plugin, OperationIndexBuild, map[string]any{"index": "slack.channels"})
	if len(out.Indexes) != 1 || out.Indexes[0].Index != "slack.channels" || len(out.Indexes[0].Records) != 1 {
		t.Fatalf("targeted output = %#v", out.Indexes)
	}
	if factory.clients["user_token"].usersCalls != 0 || factory.clients["user_token"].channelsCalls != 1 {
		t.Fatalf("unexpected client calls: %#v", factory.clients["user_token"])
	}
}

func TestServiceLookupUsersAndChannels(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				users:    []User{{ID: "U1", Name: "timo", RealName: "Timo Friedl", DisplayName: "Timo"}},
				channels: []Channel{{ID: "C1", Name: "engineering", IsChannel: true}},
			},
		},
	}
	plugin := testPlugin(factory)

	out := plugintest.DatasourceLookupOK[LookupResult](t, plugin, map[string]any{"text": "ask #engineering and timo", "limit": 10}, plugintest.WithInstance("work"))
	if out.Source != PluginName || out.Count != 2 {
		t.Fatalf("lookup output = %#v", out)
	}
	if out.Matches[0].Source.Plugin != PluginName || out.Matches[0].Source.Instance != "work" {
		t.Fatalf("lookup source = %#v", out.Matches[0].Source)
	}
	if out.Matches[0].Entity != EntityChannel || out.Matches[0].ID != "C1" {
		t.Fatalf("first match = %#v", out.Matches[0])
	}
	if out.Matches[1].Entity != EntityUser || out.Matches[1].ID != "U1" {
		t.Fatalf("second match = %#v", out.Matches[1])
	}
}

func TestServiceLookupCanFilterEntity(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				users:    []User{{ID: "U1", Name: "timo"}},
				channels: []Channel{{ID: "C1", Name: "timo"}},
			},
		},
	}
	plugin := testPlugin(factory)

	out := plugintest.DatasourceLookupOK[LookupResult](t, plugin, map[string]any{"text": "timo", "entity": EntityUser})
	if out.Count != 1 || out.Matches[0].Entity != EntityUser || out.Matches[0].ID != "U1" {
		t.Fatalf("lookup output = %#v", out)
	}
	if factory.clients["user_token"].channelsCalls != 0 {
		t.Fatalf("entity-filtered lookup should not fetch channels")
	}
}

func TestServiceInfoReportsTokenIdentities(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				authInfo: AuthInfo{URL: "https://example.slack.com/", Team: "Example", User: "timo", TeamID: "T1", UserID: "U1"},
			},
			"bot_token": {
				authInfo: AuthInfo{URL: "https://example.slack.com/", Team: "Example", User: "dex", TeamID: "T1", UserID: "Ubot", BotID: "B1"},
			},
		},
	}
	plugin := testPlugin(factory)

	out := plugintest.RunOK[InfoResult](t, plugin, OperationInfo, map[string]any{})
	if out.Status != "ok" || out.Count != 2 || len(out.Tokens) != 2 {
		t.Fatalf("info result = %#v", out)
	}
	if out.Tokens[0].Role != SlackRoleUser || !out.Tokens[0].OK || out.Tokens[0].TeamID != "T1" || out.Tokens[0].UserID != "U1" {
		t.Fatalf("user token info = %#v", out.Tokens[0])
	}
	if out.Tokens[1].Role != SlackRoleBot || !out.Tokens[1].OK || out.Tokens[1].BotID != "B1" {
		t.Fatalf("bot token info = %#v", out.Tokens[1])
	}
	if factory.clients["user_token"].authCalls != 1 || factory.clients["bot_token"].authCalls != 1 {
		t.Fatalf("auth calls user=%d bot=%d", factory.clients["user_token"].authCalls, factory.clients["bot_token"].authCalls)
	}
}

func TestServiceAuthTestReportsTokenIdentities(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				authInfo: AuthInfo{URL: "https://example.slack.com/", Team: "Example", User: "timo", TeamID: "T1", UserID: "U1"},
			},
			"bot_token": {
				authInfo: AuthInfo{URL: "https://example.slack.com/", Team: "Example", User: "dex", TeamID: "T1", UserID: "Ubot", BotID: "B1"},
			},
		},
	}
	plugin := testPlugin(factory)

	out := plugintest.RunOK[AuthTestResult](t, plugin, OperationAuthTest, map[string]any{})
	if out.Status != "ok" || out.Count != 2 || len(out.Tokens) != 2 {
		t.Fatalf("auth test result = %#v", out)
	}
	if out.Tokens[0].Role != SlackRoleUser || !out.Tokens[0].OK || out.Tokens[0].UserID != "U1" {
		t.Fatalf("user token auth test = %#v", out.Tokens[0])
	}
	if out.Tokens[1].Role != SlackRoleBot || !out.Tokens[1].OK || out.Tokens[1].BotID != "B1" {
		t.Fatalf("bot token auth test = %#v", out.Tokens[1])
	}
}

func TestServiceInfoReportsPartialTokenFailure(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {authErr: slackapi.SlackErrorResponse{Err: "invalid_auth"}},
			"bot_token":  {authInfo: AuthInfo{Team: "Example", TeamID: "T1", UserID: "Ubot", BotID: "B1"}},
		},
	}
	plugin := testPlugin(factory)

	out := plugintest.RunOK[InfoResult](t, plugin, OperationInfo, map[string]any{})
	if out.Status != "partial" || out.Count != 2 {
		t.Fatalf("info result = %#v", out)
	}
	if out.Tokens[0].Role != SlackRoleUser || out.Tokens[0].OK || out.Tokens[0].Error == "" {
		t.Fatalf("user token failure = %#v", out.Tokens[0])
	}
	if out.Tokens[1].Role != SlackRoleBot || !out.Tokens[1].OK || out.Tokens[1].TeamID != "T1" {
		t.Fatalf("bot token info = %#v", out.Tokens[1])
	}
}

func TestServiceAuthTestReportsPartialTokenFailure(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {authErr: slackapi.SlackErrorResponse{Err: "invalid_auth"}},
			"bot_token":  {authInfo: AuthInfo{Team: "Example", TeamID: "T1", UserID: "Ubot", BotID: "B1"}},
		},
	}
	plugin := testPlugin(factory)

	out := plugintest.RunOK[AuthTestResult](t, plugin, OperationAuthTest, map[string]any{})
	if out.Status != "partial" || out.Count != 2 {
		t.Fatalf("auth test result = %#v", out)
	}
	if out.Tokens[0].Role != SlackRoleUser || out.Tokens[0].OK || out.Tokens[0].Error == "" {
		t.Fatalf("user token auth failure = %#v", out.Tokens[0])
	}
	if out.Tokens[1].Role != SlackRoleBot || !out.Tokens[1].OK {
		t.Fatalf("bot token auth test = %#v", out.Tokens[1])
	}
}

func TestServiceInfoReportsUnavailableAuthPurposes(t *testing.T) {
	factory := &capturingFactory{clients: map[string]*fakeClient{}}
	plugin := testPlugin(factory)

	out := plugintest.RunOK[InfoResult](t, plugin, OperationInfo, map[string]any{})
	if out.Status != "error" || out.Count != 2 {
		t.Fatalf("info = %#v", out)
	}
}

func TestSlackReferenceParsing(t *testing.T) {
	if got := normalizeSlackTimestamp("p1769777574026209"); got != "1769777574.026209" {
		t.Fatalf("url timestamp = %q", got)
	}
	if got := normalizeSlackTimestamp("1769777574.026209"); got != "1769777574.026209" {
		t.Fatalf("api timestamp = %q", got)
	}
	ref, ok := parseSlackMessageRef("https://example.slack.com/archives/C1/p1769777574026209?thread_ts=1769777574.026209")
	if !ok || ref.Channel != "C1" || ref.TS != "1769777574.026209" {
		t.Fatalf("url ref = %#v ok=%v", ref, ok)
	}
	ref, ok = parseSlackMessageRef("#engineering:p1769777574026209")
	if !ok || ref.Channel != "#engineering" || ref.TS != "1769777574.026209" {
		t.Fatalf("name ref = %#v ok=%v", ref, ok)
	}
	ref, ok = parseSlackMessageRef("slack://channel/C1/message/1769777574.026209")
	if !ok || ref.Channel != "C1" || ref.TS != "1769777574.026209" {
		t.Fatalf("synthetic ref = %#v ok=%v", ref, ok)
	}
}

func TestServiceSendSearchAndThreadUseLiveClient(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"bot_token": {
				sendTS: "1710000000.123456",
				thread: []ThreadMessage{{TS: "1710000000.123456", User: "U1", Text: "root"}},
			},
			"user_token": {
				searchMessages: []SearchMessage{{Channel: "C1", TS: "1710000000.123456", User: "U1", Text: "hello"}},
				searchTotal:    1,
				thread:         []ThreadMessage{{TS: "1710000000.123456", User: "U1", Text: "root"}},
			},
		},
	}
	plugin := testPlugin(factory)

	send := plugintest.RunOK[MessageSendResult](t, plugin, OperationMessageSend, map[string]any{"channel": "C1", "text": "hello"})
	if !send.OK || send.Role != SlackRoleBot || send.TS != "1710000000.123456" || factory.clients["bot_token"].sendCalls != 1 {
		t.Fatalf("send result = %#v calls=%d", send, factory.clients["bot_token"].sendCalls)
	}

	reply := plugintest.RunOK[MessageSendResult](t, plugin, OperationMessageSend, map[string]any{"channel": "C1", "text": "reply", "thread_ts": "1710000000.123456", "reply_broadcast": true})
	if !reply.OK || reply.ThreadTS != "1710000000.123456" || factory.clients["bot_token"].lastSend.ThreadTS != "1710000000.123456" || !factory.clients["bot_token"].lastSend.ReplyBroadcast {
		t.Fatalf("reply result = %#v request=%#v", reply, factory.clients["bot_token"].lastSend)
	}

	search := plugintest.RunOK[SearchResult](t, plugin, OperationSearch, map[string]any{"query": "hello", "limit": 5})
	if search.Count != 1 || len(search.Messages) != 1 || search.Messages[0].Channel != "C1" || factory.clients["user_token"].searchCalls != 1 {
		t.Fatalf("search result = %#v calls=%d", search, factory.clients["user_token"].searchCalls)
	}

	thread := plugintest.RunOK[ThreadResult](t, plugin, OperationThread, map[string]any{"channel": "C1", "ts": "1710000000.123456"})
	if thread.Count != 1 || thread.Messages[0].Text != "root" || factory.clients["user_token"].threadCalls != 1 {
		t.Fatalf("thread result = %#v calls=%d", thread, factory.clients["user_token"].threadCalls)
	}
}

func TestServiceSearchExtractsTicketsAndMentionsClassifyStatus(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Unix(), 10) + ".000001"
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				authInfo: AuthInfo{UserID: "U1"},
				searchMessages: []SearchMessage{{
					Channel:   "C1",
					TS:        ts,
					User:      "U2",
					Text:      "please check DEV-123 and tel-456 <@U1>",
					Permalink: "https://example.slack.com/archives/C1/p" + strings.ReplaceAll(ts, ".", ""),
				}},
				searchTotal: 1,
				thread: []ThreadMessage{
					{TS: ts, User: "U2", Text: "please check DEV-123 <@U1>", Reactions: []Reaction{{Name: "eyes", Users: []string{"U1"}}}},
				},
			},
			"bot_token": {authInfo: AuthInfo{UserID: "Ubot"}},
		},
	}
	plugin := testPlugin(factory)

	search := plugintest.RunOK[SearchResult](t, plugin, OperationSearch, map[string]any{"query": "DEV-", "tickets": true, "ticket_keys": []string{"DEV", "TEL"}})
	if search.Count != 1 || len(search.Messages) != 1 || len(search.Messages[0].Tickets) != 2 {
		t.Fatalf("search tickets = %#v", search)
	}
	if len(search.Tickets) != 2 || search.Tickets[0].Key != "DEV-123" || search.Tickets[1].Key != "TEL-456" {
		t.Fatalf("ticket aggregation = %#v", search.Tickets)
	}

	mentions := plugintest.RunOK[MentionsResult](t, plugin, OperationMentions, map[string]any{"user": "U1", "tickets": true, "ticket_keys": []string{"DEV"}, "limit": 5})
	if mentions.Count != 1 || mentions.Target != "U1" || mentions.Mentions[0].Status != "acked" {
		t.Fatalf("mentions result = %#v", mentions)
	}
	if len(mentions.Mentions[0].Tickets) != 1 || mentions.Mentions[0].Tickets[0] != "DEV-123" {
		t.Fatalf("mention tickets = %#v", mentions.Mentions[0].Tickets)
	}

	pendingOnly := plugintest.RunOK[MentionsResult](t, plugin, OperationMentions, map[string]any{"user": "U1", "unhandled": true, "limit": 5})
	if pendingOnly.Count != 0 {
		t.Fatalf("pending mentions = %#v", pendingOnly)
	}
}

func TestServiceSendMessageResolvesChannelAndMentions(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				users:    []User{{ID: "U1", Name: "timo"}, {ID: "U2", Name: "ada"}},
				channels: []Channel{{ID: "C1", Name: "engineering", IsChannel: true}},
			},
			"bot_token": {sendTS: "1710000000.123456"},
		},
	}
	plugin := testPlugin(factory)

	out := plugintest.RunOK[MessageSendResult](t, plugin, OperationMessageSend, map[string]any{"channel": "#engineering", "text": "hi @timo in #engineering, already <@U2>, mail a@b"}, withHost(factory))
	if !out.OK || out.Channel != "C1" {
		t.Fatalf("send result = %#v", out)
	}
	request := factory.clients["bot_token"].lastSend
	if request.Channel != "C1" || request.Text != "hi <@U1> in <#C1>, already <@U2>, mail a@b" {
		t.Fatalf("send request = %#v", request)
	}
}

func TestServiceSendMessageCanUseUserRole(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {sendTS: "1710000000.123456"},
			"bot_token":  {sendTS: "1710000001.123456"},
		},
	}
	plugin := testPlugin(factory)

	out := plugintest.RunOK[MessageSendResult](t, plugin, OperationMessageSend, map[string]any{"channel": "C1", "text": "hello", "role": "user"})
	if !out.OK || out.Role != SlackRoleUser || out.TS != "1710000000.123456" {
		t.Fatalf("send result = %#v", out)
	}
	if factory.clients["user_token"].sendCalls != 1 || factory.clients["bot_token"].sendCalls != 0 {
		t.Fatalf("send calls user=%d bot=%d", factory.clients["user_token"].sendCalls, factory.clients["bot_token"].sendCalls)
	}
}

func TestServiceSendAndEditRichMessages(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				users:    []User{{ID: "U1", Name: "timo"}},
				channels: []Channel{{ID: "C1", Name: "engineering"}},
			},
			"bot_token": {sendTS: "1710000000.123456", editTS: "1710000000.123456"},
		},
	}
	plugin := testPlugin(factory)

	send := plugintest.RunOK[MessageSendResult](t, plugin, OperationMessageSend, map[string]any{"channel": "#engineering", "markdown": "hi @timo in #engineering"}, withHost(factory))
	if !send.OK || send.Channel != "C1" {
		t.Fatalf("markdown send result = %#v", send)
	}
	if request := factory.clients["bot_token"].lastSend; request.Text != "hi <@U1> in <#C1>" || len(request.Blocks) != 1 || !strings.Contains(string(request.Blocks[0]), "hi @timo in #engineering") {
		t.Fatalf("markdown send request = %#v block=%s", request, request.Blocks[0])
	}

	rawBlocks := []map[string]any{{
		"type": "section",
		"text": map[string]any{"type": "mrkdwn", "text": "raw @timo"},
	}}
	edit := plugintest.RunOK[MessageEditResult](t, plugin, OperationMessageEdit, map[string]any{"channel": "C1", "ts": "1710000000.123456", "text": "fallback @timo", "blocks": rawBlocks, "unfurl_links": false, "parse": "none"}, withHost(factory))
	if !edit.OK {
		t.Fatalf("block edit result = %#v", edit)
	}
	if request := factory.clients["bot_token"].lastEdit; request.Text != "fallback <@U1>" || len(request.Blocks) != 1 || !strings.Contains(string(request.Blocks[0]), "raw @timo") || request.UnfurlLinks == nil || *request.UnfurlLinks || request.Parse != "none" {
		t.Fatalf("block edit request = %#v block=%s", request, request.Blocks[0])
	}
}

func TestServiceRichMessagesValidateContentPaths(t *testing.T) {
	plugin := testPlugin(&capturingFactory{clients: map[string]*fakeClient{"bot_token": {}}})

	if err := plugintest.RunError(t, plugin, OperationMessageSend, map[string]any{"channel": "C1", "text": "hello", "markdown": "hello"}); err == nil || err.Code != "bad_input" {
		t.Fatalf("conflicting content err = %#v", err)
	}
	if err := plugintest.RunError(t, plugin, OperationMessageSend, map[string]any{"channel": "C1", "blocks": []map[string]any{{"type": "divider"}}}); err == nil || err.Code != "bad_input" {
		t.Fatalf("missing fallback err = %#v", err)
	}
	if err := plugintest.RunError(t, plugin, OperationMessageSend, map[string]any{"channel": "C1"}); err == nil || err.Code != "bad_input" {
		t.Fatalf("missing content err = %#v", err)
	}
}

func TestServiceSendMessageRejectsInvalidRole(t *testing.T) {
	plugin := testPlugin(&capturingFactory{clients: map[string]*fakeClient{"bot_token": {}}})

	if err := plugintest.RunError(t, plugin, OperationMessageSend, map[string]any{"channel": "C1", "text": "hello", "role": "admin"}); err == nil || err.Code != "bad_input" {
		t.Fatalf("invalid role err = %#v", err)
	}
}

func TestServiceSendMessageDoesNotFallbackFromMissingDefaultRole(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {sendTS: "1710000000.123456"},
		},
	}
	plugin := testPlugin(factory)

	if err := plugintest.RunError(t, plugin, OperationMessageSend, map[string]any{"channel": "C1", "text": "hello"}); err == nil || err.Code != "slack" {
		t.Fatalf("missing default role err = %#v", err)
	}
	if factory.created[AuthPurposeUser] != 0 {
		t.Fatalf("send should not fallback to user role: %#v", factory.created)
	}
}

func TestServiceSendMessageFailsUnknownTargetChannel(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {channels: []Channel{{ID: "C1", Name: "engineering"}}},
			"bot_token":  {sendTS: "1710000000.123456"},
		},
	}
	plugin := testPlugin(factory)

	if err := plugintest.RunError(t, plugin, OperationMessageSend, map[string]any{"channel": "#missing", "text": "hello"}, withHost(factory)); err == nil || err.Code != "bad_input" {
		t.Fatalf("unknown channel err = %#v", err)
	}
	if factory.clients["bot_token"].sendCalls != 0 {
		t.Fatalf("unknown channel should not send: %#v", factory.clients["bot_token"])
	}
}

func TestServiceEditDeleteReactAndJoinUseResolvedReferences(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				channels: []Channel{{ID: "C1", Name: "engineering"}},
				users:    []User{{ID: "U1", Name: "timo"}},
			},
			"bot_token": {editTS: "1769777574.026209"},
		},
	}
	plugin := testPlugin(factory)

	edit := plugintest.RunOK[MessageEditResult](t, plugin, OperationMessageEdit, map[string]any{"ref": "#engineering:p1769777574026209", "text": "updated for @timo"}, withHost(factory))
	if !edit.OK || edit.Channel != "C1" || edit.TS != "1769777574.026209" || edit.Role != SlackRoleBot {
		t.Fatalf("edit result = %#v", edit)
	}
	if request := factory.clients["bot_token"].lastEdit; request.Channel != "C1" || request.TS != "1769777574.026209" || request.Text != "updated for <@U1>" {
		t.Fatalf("edit request = %#v", request)
	}

	del := plugintest.RunOK[MessageDeleteResult](t, plugin, OperationMessageDelete, map[string]any{"channel": "#engineering", "ts": "p1769777574026209"}, withHost(factory))
	if !del.OK || del.Channel != "C1" || del.TS != "1769777574.026209" || factory.clients["bot_token"].deleteCalls != 1 {
		t.Fatalf("delete result = %#v calls=%d", del, factory.clients["bot_token"].deleteCalls)
	}
	if request := factory.clients["bot_token"].lastDelete; request.Channel != "C1" || request.TS != "1769777574.026209" {
		t.Fatalf("delete request = %#v", request)
	}

	react := plugintest.RunOK[ReactionAddResult](t, plugin, OperationReactionAdd, map[string]any{"ref": "https://example.slack.com/archives/C1/p1769777574026209", "emoji": ":thumbsup:"})
	if !react.OK || react.Channel != "C1" || react.TS != "1769777574.026209" || react.Emoji != "thumbsup" {
		t.Fatalf("reaction result = %#v", react)
	}
	if request := factory.clients["bot_token"].lastReaction; request.Channel != "C1" || request.TS != "1769777574.026209" || request.Emoji != "thumbsup" {
		t.Fatalf("reaction request = %#v", request)
	}

	join := plugintest.RunOK[ChannelJoinResult](t, plugin, OperationChannelJoin, map[string]any{"channel": "#engineering"}, withHost(factory))
	if !join.OK || join.Channel != "C1" || factory.clients["bot_token"].joinCalls != 1 {
		t.Fatalf("join result = %#v calls=%d", join, factory.clients["bot_token"].joinCalls)
	}
	if factory.clients["bot_token"].lastJoin.Channel != "C1" {
		t.Fatalf("join request = %#v", factory.clients["bot_token"].lastJoin)
	}
}

func TestServiceListPresenceBookmarksEmojiAndMarkRead(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				users:     []User{{ID: "U1", Name: "timo", RealName: "Timo"}, {ID: "U2", Name: "ada"}},
				channels:  []Channel{{ID: "C1", Name: "engineering"}, {ID: "C2", Name: "random"}},
				emojis:    map[string]string{"shipit": "https://example/shipit.png", "thumbsup": "alias:+1"},
				bookmarks: []Bookmark{{ID: "B1", Channel: "C1", Title: "Runbook", Link: "https://example/runbook", Type: "link"}},
				presence:  Presence{Presence: "active", Online: true},
			},
			"bot_token": {},
		},
	}
	plugin := testPlugin(factory)

	users := plugintest.RunOK[UserListResult](t, plugin, OperationUserList, map[string]any{"query": "tim"})
	if users.Count != 1 || users.Users[0].ID != "U1" {
		t.Fatalf("users result = %#v", users)
	}

	channels := plugintest.RunOK[ChannelListResult](t, plugin, OperationChannelList, map[string]any{"query": "eng"})
	if channels.Count != 1 || channels.Channels[0].ID != "C1" {
		t.Fatalf("channels result = %#v", channels)
	}

	emojis := plugintest.RunOK[EmojiListResult](t, plugin, OperationEmojiList, map[string]any{"query": "thumb"})
	if emojis.Count != 0 {
		t.Fatalf("emoji aliases should be hidden by default: %#v", emojis)
	}
	emojis = plugintest.RunOK[EmojiListResult](t, plugin, OperationEmojiList, map[string]any{"query": "thumb", "include_aliases": true})
	if emojis.Count != 1 || emojis.Emojis[0].Name != "thumbsup" || emojis.Emojis[0].Source != "custom" || emojis.Emojis[0].AliasFor != "+1" {
		t.Fatalf("emoji result = %#v", emojis)
	}

	bookmarks := plugintest.RunOK[BookmarkListResult](t, plugin, OperationBookmarkList, map[string]any{"channel": "#engineering"}, withHost(factory))
	if bookmarks.Count != 1 || bookmarks.Channel != "C1" || factory.clients["user_token"].lastBookmarkChannel != "C1" {
		t.Fatalf("bookmarks result = %#v channel=%q", bookmarks, factory.clients["user_token"].lastBookmarkChannel)
	}

	presence := plugintest.RunOK[PresenceGetResult](t, plugin, OperationPresenceGet, map[string]any{"user": "@timo"}, withHost(factory))
	if presence.User != "U1" || presence.Presence.Presence != "active" || !presence.Online {
		t.Fatalf("presence result = %#v", presence)
	}

	mark := plugintest.RunOK[ChannelMarkResult](t, plugin, OperationChannelMark, map[string]any{"ref": "#engineering:p1769777574026209"}, withHost(factory))
	if !mark.OK || mark.Role != SlackRoleUser || factory.clients["user_token"].markReadCalls != 1 || factory.clients["bot_token"].markReadCalls != 0 {
		t.Fatalf("mark result = %#v user calls=%d bot calls=%d", mark, factory.clients["user_token"].markReadCalls, factory.clients["bot_token"].markReadCalls)
	}
}

func TestServiceListEmojiModes(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				emojis: map[string]string{"shipit": "https://example/shipit.png", "thumbsup": "alias:+1"},
				emojiCategories: []EmojiCategory{{
					Name:       "Smileys & Emotion",
					EmojiNames: []string{"grinning", "thumbsup"},
				}},
			},
		},
	}
	plugin := testPlugin(factory)

	custom := plugintest.RunOK[EmojiListResult](t, plugin, OperationEmojiList, map[string]any{})
	if custom.Count != 1 || custom.Emojis[0].Name != "shipit" || custom.Emojis[0].Source != "custom" || custom.Emojis[0].URL == "" {
		t.Fatalf("custom emoji result = %#v", custom)
	}
	if factory.clients["user_token"].lastEmojiIncludeCategories {
		t.Fatalf("custom mode should not request categories")
	}

	builtin := plugintest.RunOK[EmojiListResult](t, plugin, OperationEmojiList, map[string]any{"mode": "builtin", "query": "grin"})
	if builtin.Count != 1 || builtin.Emojis[0].Name != "grinning" || builtin.Emojis[0].Source != "builtin" || builtin.Emojis[0].Category != "Smileys & Emotion" {
		t.Fatalf("builtin emoji result = %#v", builtin)
	}
	if !factory.clients["user_token"].lastEmojiIncludeCategories {
		t.Fatalf("builtin mode should request categories")
	}

	all := plugintest.RunOK[EmojiListResult](t, plugin, OperationEmojiList, map[string]any{"mode": "all", "include_aliases": true, "query": "thumb"})
	if all.Count != 2 || all.Emojis[0].Source != "builtin" || all.Emojis[1].Source != "custom" {
		t.Fatalf("all emoji result = %#v", all)
	}
}

func TestServiceMarkReadLatest(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				channels: []Channel{{ID: "C1", Name: "engineering"}},
				latestTS: "1710000000.123456",
			},
		},
	}
	plugin := testPlugin(factory)

	mark := plugintest.RunOK[ChannelMarkResult](t, plugin, OperationChannelMark, map[string]any{"ref": "#engineering:latest"}, withHost(factory))
	if !mark.OK || mark.TS != "1710000000.123456" || factory.clients["user_token"].latestCalls != 1 || factory.clients["user_token"].lastMarkRead.TS != "1710000000.123456" {
		t.Fatalf("mark latest result = %#v client=%#v", mark, factory.clients["user_token"])
	}
}

func TestServiceMarkReadLatestEmptyChannel(t *testing.T) {
	factory := &capturingFactory{clients: map[string]*fakeClient{"user_token": {}}}
	plugin := testPlugin(factory)

	if err := plugintest.RunError(t, plugin, OperationChannelMark, map[string]any{"channel": "C1", "ts": "latest"}); err == nil || err.Code != "empty_channel" {
		t.Fatalf("empty channel err = %#v", err)
	}
}

func TestServiceFileLifecycleAndBookmarkWrites(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				users:    []User{{ID: "U1", Name: "timo"}},
				channels: []Channel{{ID: "C1", Name: "engineering"}},
				files: []FileRecord{
					{ID: "F1", Name: "runbook.md", Title: "Runbook", User: "U1", Size: 123},
					{ID: "F2", Name: "chart.png", Title: "Chart", User: "U2", Size: 456},
				},
				file:           FileRecord{ID: "F1", Name: "runbook.md", Title: "Runbook", Size: 123},
				downloadResult: FileDownloadResult{OK: true, FileID: "F1", Size: 123},
			},
			"bot_token": {},
		},
	}
	plugin := testPlugin(factory)

	files := plugintest.RunOK[FileListResult](t, plugin, OperationFileList, map[string]any{"channel": "#engineering", "user": "@timo", "query": "runbook", "limit": 1}, withHost(factory))
	if files.Count != 1 || files.Files[0].ID != "F1" {
		t.Fatalf("files result = %#v", files)
	}
	if request := factory.clients["user_token"].lastFileList; request.Channel != "C1" || request.User != "U1" || request.Limit != 1 {
		t.Fatalf("file list request = %#v", request)
	}

	info := plugintest.RunOK[FileInfoResult](t, plugin, OperationFileInfo, map[string]any{"file_id": "F1"})
	if info.File.ID != "F1" || factory.clients["user_token"].fileInfoCalls != 1 {
		t.Fatalf("file info = %#v calls=%d", info, factory.clients["user_token"].fileInfoCalls)
	}

	download := plugintest.RunOK[FileDownloadResult](t, plugin, OperationFileDownload, map[string]any{"file_id": "F1", "role": "user"})
	if !download.OK || download.Role != SlackRoleUser || factory.clients["user_token"].downloadCalls != 1 || factory.clients["bot_token"].downloadCalls != 0 {
		t.Fatalf("download = %#v user calls=%d bot calls=%d", download, factory.clients["user_token"].downloadCalls, factory.clients["bot_token"].downloadCalls)
	}

	topLevelDownload := plugintest.RunOK[FileDownloadResult](t, plugin, OperationDownload, map[string]any{"file_id": "F1", "role": "user"})
	if !topLevelDownload.OK || factory.clients["user_token"].downloadCalls != 2 {
		t.Fatalf("top-level download = %#v calls=%d", topLevelDownload, factory.clients["user_token"].downloadCalls)
	}

	deleted := plugintest.RunOK[FileDeleteResult](t, plugin, OperationFileDelete, map[string]any{"file_id": "F1", "role": "user"})
	if !deleted.OK || deleted.Role != SlackRoleUser || factory.clients["user_token"].lastDeleteFile != "F1" {
		t.Fatalf("delete file = %#v last=%q", deleted, factory.clients["user_token"].lastDeleteFile)
	}

	added := plugintest.RunOK[BookmarkResult](t, plugin, OperationBookmarkAdd, map[string]any{"channel": "#engineering", "title": "Runbook", "link": "https://example/runbook", "emoji": ":book:", "role": "user"}, withHost(factory))
	if !added.OK || added.Role != SlackRoleUser || added.Bookmark.ID != "BM1" {
		t.Fatalf("bookmark add = %#v", added)
	}
	if request := factory.clients["user_token"].lastBookmarkAdd; request.Channel != "C1" || request.Title != "Runbook" || request.Link != "https://example/runbook" {
		t.Fatalf("bookmark add request = %#v", request)
	}

	edited := plugintest.RunOK[BookmarkResult](t, plugin, OperationBookmarkEdit, map[string]any{"channel": "C1", "bookmark_id": "BM1", "title": "Runbook v2", "role": "user"})
	if !edited.OK || edited.Bookmark.ID != "BM1" || factory.clients["user_token"].lastBookmarkEdit.Title != "Runbook v2" {
		t.Fatalf("bookmark edit = %#v request=%#v", edited, factory.clients["user_token"].lastBookmarkEdit)
	}

	removed := plugintest.RunOK[BookmarkDeleteResult](t, plugin, OperationBookmarkDelete, map[string]any{"channel": "C1", "bookmark_id": "BM1", "role": "user"})
	if !removed.OK || removed.BookmarkID != "BM1" || factory.clients["user_token"].deleteBookmarkCalls != 1 {
		t.Fatalf("bookmark delete = %#v calls=%d", removed, factory.clients["user_token"].deleteBookmarkCalls)
	}
}

func TestServiceUnreadsUsesUserTokenAndResolvesChannel(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				channels: []Channel{{ID: "C1", Name: "engineering"}},
				unreads: []UnreadChannel{{
					ID:          "C1",
					Name:        "engineering",
					UnreadCount: 1,
					LastRead:    "1710000000.000000",
					Messages:    []UnreadMessage{{TS: "1710000001.000000", User: "U2", Text: "hello"}},
				}},
			},
			"bot_token": {},
		},
	}
	plugin := testPlugin(factory)

	out := plugintest.RunOK[UnreadsResult](t, plugin, OperationUnreads, map[string]any{"channel": "#engineering", "since": "1d", "limit": 10}, withHost(factory))
	if out.Count != 1 || out.Channels[0].ID != "C1" || out.Since != "1d" {
		t.Fatalf("unreads result = %#v", out)
	}
	if factory.clients["user_token"].unreadsCalls != 1 || factory.clients["bot_token"].unreadsCalls != 0 {
		t.Fatalf("unreads calls user=%d bot=%d", factory.clients["user_token"].unreadsCalls, factory.clients["bot_token"].unreadsCalls)
	}
	if request := factory.clients["user_token"].lastUnreads; request.Channel != "C1" || request.Limit != 10 || request.Since == 0 {
		t.Fatalf("unreads request = %#v", request)
	}
}

func TestServiceWriteActionsCanUseUserRole(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {editTS: "1710000000.123456"},
			"bot_token":  {editTS: "1710000001.123456"},
		},
	}
	plugin := testPlugin(factory)

	edit := plugintest.RunOK[MessageEditResult](t, plugin, OperationMessageEdit, map[string]any{"channel": "C1", "ts": "1710000000.123456", "text": "hello", "role": "user"})
	if edit.Role != SlackRoleUser || factory.clients["user_token"].editCalls != 1 || factory.clients["bot_token"].editCalls != 0 {
		t.Fatalf("edit result = %#v calls user=%d bot=%d", edit, factory.clients["user_token"].editCalls, factory.clients["bot_token"].editCalls)
	}

	react := plugintest.RunOK[ReactionAddResult](t, plugin, OperationReactionAdd, map[string]any{"channel": "C1", "ts": "1710000000.123456", "emoji": "eyes", "role": "user"})
	if react.Role != SlackRoleUser || factory.clients["user_token"].reactionCalls != 1 || factory.clients["bot_token"].reactionCalls != 0 {
		t.Fatalf("reaction result = %#v calls user=%d bot=%d", react, factory.clients["user_token"].reactionCalls, factory.clients["bot_token"].reactionCalls)
	}

	remove := plugintest.RunOK[ReactionAddResult](t, plugin, OperationReactionRemove, map[string]any{"channel": "C1", "ts": "1710000000.123456", "emoji": "eyes", "role": "user"})
	if remove.Role != SlackRoleUser || factory.clients["user_token"].removeReactionCalls != 1 || factory.clients["bot_token"].removeReactionCalls != 0 {
		t.Fatalf("remove reaction result = %#v calls user=%d bot=%d", remove, factory.clients["user_token"].removeReactionCalls, factory.clients["bot_token"].removeReactionCalls)
	}

	presence := plugintest.RunOK[PresenceSetResult](t, plugin, OperationPresenceSet, map[string]any{"presence": "away"})
	if presence.Role != SlackRoleUser || factory.clients["user_token"].setPresenceCalls != 1 || factory.clients["bot_token"].setPresenceCalls != 0 || factory.clients["user_token"].lastPresenceSet != "away" {
		t.Fatalf("presence result = %#v user calls=%d bot calls=%d value=%q", presence, factory.clients["user_token"].setPresenceCalls, factory.clients["bot_token"].setPresenceCalls, factory.clients["user_token"].lastPresenceSet)
	}
}

func TestServiceWriteActionsValidateInputs(t *testing.T) {
	plugin := testPlugin(&capturingFactory{clients: map[string]*fakeClient{"bot_token": {}}})

	if err := plugintest.RunError(t, plugin, OperationMessageEdit, map[string]any{"channel": "C1", "ts": "1710000000.123456"}); err == nil || err.Code != "bad_input" {
		t.Fatalf("missing edit text err = %#v", err)
	}
	if err := plugintest.RunError(t, plugin, OperationReactionAdd, map[string]any{"channel": "C1", "ts": "1710000000.123456"}); err == nil || err.Code != "bad_input" {
		t.Fatalf("missing emoji err = %#v", err)
	}
	if err := plugintest.RunError(t, plugin, OperationMessageDelete, map[string]any{"ref": "not-a-ref"}); err == nil || err.Code != "bad_input" {
		t.Fatalf("bad delete ref err = %#v", err)
	}
	if err := plugintest.RunError(t, plugin, OperationPresenceSet, map[string]any{"presence": "active"}); err == nil || err.Code != "bad_input" {
		t.Fatalf("bad presence err = %#v", err)
	}
	if err := plugintest.RunError(t, plugin, OperationFileDownload, map[string]any{}); err == nil || err.Code != "bad_input" {
		t.Fatalf("missing file id err = %#v", err)
	}
	if err := plugintest.RunError(t, plugin, OperationBookmarkAdd, map[string]any{"channel": "C1", "link": "https://example/runbook"}); err == nil || err.Code != "bad_input" {
		t.Fatalf("missing bookmark title err = %#v", err)
	}
	if err := plugintest.RunError(t, plugin, OperationBookmarkEdit, map[string]any{"channel": "C1", "bookmark_id": "BM1"}); err == nil || err.Code != "bad_input" {
		t.Fatalf("empty bookmark edit err = %#v", err)
	}
}

func TestServiceUploadFileUsesBotTokenAndBlobRef(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"bot_token": {
				uploadResult: FileUploadResult{OK: true, FileID: "F1", Permalink: "https://example.slack.com/files/F1"},
			},
		},
	}
	plugin := testPlugin(factory)
	host := hostForFactory(factory)
	host.blobs = map[string]pluginbinding.BlobReadResponse{
		"blob://chart": {
			Blob:    pluginbinding.BlobRef{Ref: "blob://chart", Filename: "chart.png"},
			Content: []byte("png bytes"),
		},
	}

	out := plugintest.RunOK[FileUploadResult](t, plugin, OperationFileUpload, map[string]any{"channel": "C1", "thread_ts": "1710000000.123456", "blob_ref": "blob://chart", "initial_comment": "graph", "alt_text": "Latency chart"}, plugintest.WithHost(host))
	if !out.OK || out.Role != SlackRoleBot || out.FileID != "F1" || factory.clients["bot_token"].uploadCalls != 1 {
		t.Fatalf("upload result = %#v calls=%d", out, factory.clients["bot_token"].uploadCalls)
	}
	request := factory.clients["bot_token"].lastUpload
	if request.Channel != "C1" || request.ThreadTS != "1710000000.123456" || request.Filename != "chart.png" || string(request.Content) != "png bytes" || request.InitialComment != "graph" || request.AltText != "Latency chart" {
		t.Fatalf("upload request = %#v", request)
	}
	if factory.created["user_token"] != 0 {
		t.Fatalf("file upload should only use bot token: %#v", factory.created)
	}
}

func TestServiceUploadFileResolvesChannelThreadAndComment(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				users:        []User{{ID: "U1", Name: "timo"}},
				channels:     []Channel{{ID: "C1", Name: "engineering"}},
				uploadResult: FileUploadResult{OK: true, FileID: "F2"},
			},
			"bot_token": {uploadResult: FileUploadResult{OK: true, FileID: "F1"}},
		},
	}
	plugin := testPlugin(factory)

	out := plugintest.RunOK[FileUploadResult](t, plugin, OperationFileUpload, map[string]any{"channel": "engineering", "thread_ts": "p1769777574026209", "filename": "chart.png", "content_bytes": "cG5n", "initial_comment": "for @timo"}, withHost(factory))
	if !out.OK || out.Channel != "C1" || out.ThreadTS != "1769777574.026209" {
		t.Fatalf("upload result = %#v", out)
	}
	request := factory.clients["bot_token"].lastUpload
	if request.Channel != "C1" || request.ThreadTS != "1769777574.026209" || request.InitialComment != "for <@U1>" {
		t.Fatalf("upload request = %#v", request)
	}
}

func TestServiceUploadFileCanUseUserRole(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {uploadResult: FileUploadResult{OK: true, FileID: "F2"}},
			"bot_token":  {uploadResult: FileUploadResult{OK: true, FileID: "F1"}},
		},
	}
	plugin := testPlugin(factory)

	out := plugintest.RunOK[FileUploadResult](t, plugin, OperationFileUpload, map[string]any{"channel": "C1", "filename": "chart.png", "content_bytes": "cG5nIGJ5dGVz", "role": "user"})
	if !out.OK || out.Role != SlackRoleUser || out.FileID != "F2" {
		t.Fatalf("upload result = %#v", out)
	}
	if factory.clients["user_token"].uploadCalls != 1 || factory.clients["bot_token"].uploadCalls != 0 {
		t.Fatalf("upload calls user=%d bot=%d", factory.clients["user_token"].uploadCalls, factory.clients["bot_token"].uploadCalls)
	}
}

func TestServiceUploadFileRejectsInvalidRole(t *testing.T) {
	plugin := testPlugin(&capturingFactory{clients: map[string]*fakeClient{"bot_token": {}}})

	if err := plugintest.RunError(t, plugin, OperationFileUpload, map[string]any{"channel": "C1", "filename": "chart.png", "content_bytes": "cG5n", "role": "admin"}); err == nil || err.Code != "bad_input" {
		t.Fatalf("invalid role err = %#v", err)
	}
}

func TestServiceUploadFileAcceptsBase64ContentBytes(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"bot_token": {uploadResult: FileUploadResult{OK: true, FileID: "F2"}},
		},
	}
	plugin := testPlugin(factory)

	out := plugintest.RunOK[FileUploadResult](t, plugin, OperationFileUpload, map[string]any{"channel": "C1", "filename": "chart.png", "content_bytes": "cG5nIGJ5dGVz"})
	if !out.OK || out.FileID != "F2" {
		t.Fatalf("upload result = %#v", out)
	}
	if string(factory.clients["bot_token"].lastUpload.Content) != "png bytes" || factory.clients["bot_token"].lastUpload.Filename != "chart.png" {
		t.Fatalf("upload request = %#v", factory.clients["bot_token"].lastUpload)
	}
}

func TestServiceUploadFileRequiresExactlyOneContentSource(t *testing.T) {
	plugin := testPlugin(&capturingFactory{clients: map[string]*fakeClient{"bot_token": {}}})

	if err := plugintest.RunError(t, plugin, OperationFileUpload, map[string]any{"channel": "C1", "filename": "chart.png"}); err == nil || err.Code != "bad_input" {
		t.Fatalf("missing content err = %#v", err)
	}
	if err := plugintest.RunError(t, plugin, OperationFileUpload, map[string]any{"channel": "C1", "blob_ref": "blob://chart", "filename": "chart.png", "content_bytes": "cG5n"}); err == nil || err.Code != "bad_input" {
		t.Fatalf("ambiguous content err = %#v", err)
	}
}

func TestServiceThreadLimitsTotalMessages(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				thread: []ThreadMessage{
					{TS: "1710000000.123456", User: "U1", Text: "root"},
					{TS: "1710000001.123456", User: "U2", Text: "reply 1"},
					{TS: "1710000002.123456", User: "U3", Text: "reply 2"},
				},
			},
		},
	}
	plugin := testPlugin(factory)

	thread := plugintest.RunOK[ThreadResult](t, plugin, OperationThread, map[string]any{"channel": "C1", "ts": "1710000000.123456", "limit": 2})
	if thread.Count != 2 || len(thread.Messages) != 2 {
		t.Fatalf("thread result = %#v", thread)
	}
	if thread.Messages[0].TS != "1710000000.123456" || thread.Messages[1].TS != "1710000001.123456" {
		t.Fatalf("thread messages = %#v", thread.Messages)
	}
}

func TestServiceThreadAcceptsSlackURLRef(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				thread: []ThreadMessage{{TS: "1769777574.026209", User: "U1", Text: "root"}},
			},
		},
	}
	plugin := testPlugin(factory)

	thread := plugintest.RunOK[ThreadResult](t, plugin, OperationThread, map[string]any{"ref": "https://example.slack.com/archives/C1/p1769777574026209"})
	if thread.Channel != "C1" || thread.TS != "1769777574.026209" || thread.Count != 1 {
		t.Fatalf("thread result = %#v", thread)
	}
}

func TestServiceMessagesDatasourceReturnsRecords(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				searchMessages: []SearchMessage{{Channel: "C1", TS: "1710000000.123456", User: "U1", Text: "incident update"}},
				searchTotal:    1,
			},
		},
	}
	plugin := testPlugin(factory)

	out := plugintest.DatasourceSearchOK[MessageDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceMessages, "query": "incident", "limit": 5}, plugintest.WithInstance("work"))
	if out.Source != DatasourceMessages || out.Query != "incident" || out.Count != 1 {
		t.Fatalf("message datasource result = %#v", out)
	}
	record := out.Records[0]
	if record.Entity != EntityMessage || record.ID != "C1:1710000000.123456" || record.Source.Instance != "work" {
		t.Fatalf("message record identity = %#v", record)
	}
	if record.Channel != "C1" || record.TS != "1710000000.123456" || record.User != "U1" || record.Links["self"] != "slack://channel/C1/message/1710000000.123456" {
		t.Fatalf("message record = %#v", record)
	}
}

func TestServiceThreadMessagesDatasourceRequiresChannelAndTS(t *testing.T) {
	plugin := testPlugin(&capturingFactory{clients: map[string]*fakeClient{"user_token": {}}})

	if err := plugintest.DatasourceSearchError(t, plugin, map[string]any{"datasource": DatasourceThreadMessages, "ts": "1710000000.123456"}); err == nil || err.Code != "bad_input" {
		t.Fatalf("missing channel err = %#v", err)
	}
	if err := plugintest.DatasourceSearchError(t, plugin, map[string]any{"datasource": DatasourceThreadMessages, "channel": "C1"}); err == nil || err.Code != "bad_input" {
		t.Fatalf("missing ts err = %#v", err)
	}
}

func TestServiceThreadMessagesDatasourceReturnsThreadRecords(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				thread: []ThreadMessage{
					{TS: "1710000000.123456", User: "U1", Text: "root"},
					{TS: "1710000001.123456", User: "U2", Text: "reply"},
					{TS: "1710000002.123456", User: "U3", Text: "later reply"},
				},
			},
		},
	}
	plugin := testPlugin(factory)

	out := plugintest.DatasourceSearchOK[ThreadMessagesDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceThreadMessages, "channel": "C1", "ts": "1710000000.123456", "limit": 2})
	if out.Source != DatasourceThreadMessages || out.Query != "1710000000.123456" || out.Count != 2 {
		t.Fatalf("thread datasource result = %#v", out)
	}
	if len(out.Records) != out.Count {
		t.Fatalf("thread datasource count mismatch = %#v", out)
	}
	reply := out.Records[1]
	if reply.Entity != EntityThreadMessage || reply.ID != "C1:1710000000.123456:1710000001.123456" {
		t.Fatalf("reply identity = %#v", reply)
	}
	if reply.Channel != "C1" || reply.RootTS != "1710000000.123456" || reply.ReplyTS != "1710000001.123456" || reply.Links["thread"] != "slack://channel/C1/message/1710000000.123456" {
		t.Fatalf("reply record = %#v", reply)
	}
}

func TestServiceThreadMessagesDatasourceAcceptsNameRef(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				channels: []Channel{{ID: "C1", Name: "engineering"}},
				thread:   []ThreadMessage{{TS: "1769777574.026209", User: "U1", Text: "root"}},
			},
		},
	}
	plugin := testPlugin(factory)

	out := plugintest.DatasourceSearchOK[ThreadMessagesDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceThreadMessages, "ref": "#engineering:p1769777574026209"}, withHost(factory))
	if out.Query != "1769777574.026209" || out.Count != 1 || out.Records[0].Channel != "C1" {
		t.Fatalf("thread datasource result = %#v", out)
	}
}

func TestServiceChannelMembersDatasourceRequiresChannelAndFilters(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				channelMembers: []User{
					{ID: "U1", Name: "timo", RealName: "Timo Friedl", DisplayName: "Timo"},
					{ID: "U2", Name: "ada", RealName: "Ada Lovelace", DisplayName: "Ada"},
				},
			},
		},
	}
	plugin := testPlugin(factory)

	if err := plugintest.DatasourceSearchError(t, plugin, map[string]any{"datasource": DatasourceChannelMembers, "query": "timo"}); err == nil || err.Code != "bad_input" {
		t.Fatalf("missing channel err = %#v", err)
	}

	out := plugintest.DatasourceSearchOK[ChannelMembersDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceChannelMembers, "channel": "C1", "query": "ada"})
	if out.Source != DatasourceChannelMembers || out.Query != "ada" || out.Count != 1 {
		t.Fatalf("channel members result = %#v", out)
	}
	member := out.Records[0]
	if member.Entity != EntityChannelMember || member.ID != "C1:U2" || member.UserID != "U2" || member.Channel != "C1" {
		t.Fatalf("member record = %#v", member)
	}
	if factory.clients["user_token"].channelMembersCalls != 1 || factory.clients["user_token"].lastMembersLimit != 0 {
		t.Fatalf("member calls = %#v", factory.clients["user_token"])
	}
}

func TestServiceChannelMembersDatasourceResolvesChannelName(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				channels:       []Channel{{ID: "C1", Name: "engineering"}},
				channelMembers: []User{{ID: "U1", Name: "timo"}},
			},
		},
	}
	plugin := testPlugin(factory)

	out := plugintest.DatasourceSearchOK[ChannelMembersDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceChannelMembers, "channel": "#engineering"}, withHost(factory))
	if out.Count != 1 || out.Records[0].Channel != "C1" || out.Records[0].ID != "C1:U1" {
		t.Fatalf("channel members result = %#v", out)
	}
}

func TestServiceChannelMembersDatasourceFallsBackToBotToken(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {channelMembersErr: slackapi.SlackErrorResponse{Err: "missing_scope"}},
			"bot_token":  {channelMembers: []User{{ID: "U1", Name: "timo"}}},
		},
	}
	plugin := testPlugin(factory)

	out := plugintest.DatasourceSearchOK[ChannelMembersDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceChannelMembers, "channel": "C1", "limit": 1})
	if out.Count != 1 || out.Records[0].ID != "C1:U1" {
		t.Fatalf("channel members result = %#v", out)
	}
	if factory.created["user_token"] == 0 || factory.created["bot_token"] == 0 {
		t.Fatalf("expected preferred token fallback: %#v", factory.created)
	}
}

func testPlugin(factory *capturingFactory) *pluginbinding.Plugin {
	return NewPluginWithService(Service{ClientFactory: factory.newClient})
}

type capturingFactory struct {
	clients map[string]*fakeClient
	created map[string]int
}

func (f *capturingFactory) newClient(_ pluginbinding.Context, purpose string) (Client, error) {
	if f.created == nil {
		f.created = map[string]int{}
	}
	f.created[purpose]++
	client := f.clients[purpose]
	if client == nil {
		return nil, errors.New("unexpected token " + purpose)
	}
	return client, nil
}

type fakeClient struct {
	authInfo                   AuthInfo
	users                      []User
	channels                   []Channel
	channelMembers             []User
	emojis                     map[string]string
	emojiCategories            []EmojiCategory
	bookmarks                  []Bookmark
	bookmark                   Bookmark
	files                      []FileRecord
	file                       FileRecord
	downloadResult             FileDownloadResult
	unreads                    []UnreadChannel
	presence                   Presence
	searchMessages             []SearchMessage
	thread                     []ThreadMessage
	sendTS                     string
	latestTS                   string
	searchTotal                int
	authErr                    error
	usersErr                   error
	channelsErr                error
	channelMembersErr          error
	emojisErr                  error
	bookmarksErr               error
	bookmarkErr                error
	filesErr                   error
	fileErr                    error
	downloadErr                error
	deleteFileErr              error
	unreadsErr                 error
	presenceErr                error
	setPresenceErr             error
	sendErr                    error
	editErr                    error
	deleteErr                  error
	reactionErr                error
	removeReactionErr          error
	joinErr                    error
	markReadErr                error
	uploadErr                  error
	searchErr                  error
	threadErr                  error
	authCalls                  int
	usersCalls                 int
	channelsCalls              int
	channelMembersCalls        int
	emojisCalls                int
	latestCalls                int
	bookmarksCalls             int
	addBookmarkCalls           int
	editBookmarkCalls          int
	deleteBookmarkCalls        int
	filesCalls                 int
	fileInfoCalls              int
	downloadCalls              int
	deleteFileCalls            int
	unreadsCalls               int
	presenceCalls              int
	setPresenceCalls           int
	lastMembersLimit           int
	sendCalls                  int
	editCalls                  int
	deleteCalls                int
	reactionCalls              int
	removeReactionCalls        int
	joinCalls                  int
	markReadCalls              int
	uploadCalls                int
	searchCalls                int
	threadCalls                int
	lastEmojiIncludeCategories bool
	lastSend                   MessageSendRequest
	lastEdit                   MessageEditRequest
	lastDelete                 MessageRefRequest
	lastReaction               ReactionAddRequest
	lastRemoveReaction         ReactionAddRequest
	lastJoin                   ChannelJoinRequest
	lastMarkRead               MessageRefRequest
	lastFileList               FileListRequest
	lastDownload               FileDownloadRequest
	lastDeleteFile             string
	lastUnreads                UnreadsRequest
	lastBookmarkAdd            BookmarkAddRequest
	lastBookmarkEdit           BookmarkEditRequest
	lastBookmarkDelete         BookmarkDeleteRequest
	lastBookmarkChannel        string
	lastPresenceUser           string
	lastPresenceSet            string
	editTS                     string
	lastUpload                 FileUploadRequest
	uploadResult               FileUploadResult
}

type fakeHostClient struct {
	users    []User
	channels []Channel
	blobs    map[string]pluginbinding.BlobReadResponse
}

func hostForFactory(factory *capturingFactory) *fakeHostClient {
	host := &fakeHostClient{}
	for _, purpose := range []string{AuthPurposeUser, AuthPurposeBot} {
		client := factory.clients[purpose]
		if client == nil {
			continue
		}
		host.users = append(host.users, client.users...)
		host.channels = append(host.channels, client.channels...)
	}
	return host
}

func withHost(factory *capturingFactory) plugintest.RunOption {
	return plugintest.WithHost(hostForFactory(factory))
}

func (h *fakeHostClient) Secret(purpose string) (pluginbinding.SecretMaterial, error) {
	return pluginbinding.SecretMaterial{Purpose: purpose, Value: purpose}, nil
}

func (h *fakeHostClient) Lookup(input pluginbinding.DatasourceLookupInput) (pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]], error) {
	token := strings.TrimSpace(input.Text)
	if len(input.Terms) > 0 && strings.TrimSpace(input.Terms[0]) != "" {
		token = strings.TrimSpace(input.Terms[0])
	}
	token = strings.TrimPrefix(strings.TrimPrefix(token, "#"), "@")
	var matches []pluginbinding.LookupMatch[any]
	switch strings.TrimSpace(input.Entity) {
	case EntityUser:
		for _, user := range h.users {
			if slackUserMatches(user, token) {
				matches = append(matches, pluginbinding.NewLookupMatch(pluginbinding.LookupSource{Source: "host_index", Plugin: PluginName, Index: DatasourceUsers}, EntityUser, user.ID, 1200, []string{"record.name"}, any(map[string]any{"id": user.ID})))
				break
			}
		}
	case EntityChannel:
		for _, channel := range h.channels {
			if strings.EqualFold(strings.TrimSpace(channel.ID), token) || strings.EqualFold(strings.TrimSpace(channel.Name), token) {
				matches = append(matches, pluginbinding.NewLookupMatch(pluginbinding.LookupSource{Source: "host_index", Plugin: PluginName, Index: DatasourceChannels}, EntityChannel, channel.ID, 1200, []string{"record.name"}, any(map[string]any{"id": channel.ID})))
				break
			}
		}
	}
	return pluginbinding.NewDatasourceLookupResult("host_index", input.Text, input.Terms, matches), nil
}

func (h *fakeHostClient) Search(input pluginbinding.DatasourceSearchInput) (pluginbinding.DatasourceSearchResult[any], error) {
	return pluginbinding.NewDatasourceSearchResult[any]("host_index", input.Query, nil), nil
}

func (h *fakeHostClient) Get(input pluginbinding.DatasourceGetInput) (pluginbinding.DatasourceGetResult[any], error) {
	return pluginbinding.NewDatasourceGetResult[any]("host_index", map[string]any{"id": input.ID}), nil
}

func (h *fakeHostClient) ResolveEndpoint(string) (core.EndpointRef, error) {
	return core.EndpointRef{}, nil
}

func (h *fakeHostClient) HTTP(pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	return pluginbinding.HTTPResponse{}, errors.New("http capability is not configured")
}

func (h *fakeHostClient) BlobRead(input pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error) {
	ref := strings.TrimSpace(input.Ref)
	if blob, ok := h.blobs[ref]; ok {
		return blob, nil
	}
	return pluginbinding.BlobReadResponse{}, errors.New("blob read capability is not configured")
}

func (h *fakeHostClient) BlobWrite(pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, errors.New("blob write capability is not configured")
}

func (h *fakeHostClient) BlobInfo(pluginbinding.BlobInfoRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, errors.New("blob info capability is not configured")
}

func (h *fakeHostClient) EnvLookup(string) (pluginbinding.EnvLookupResponse, error) {
	return pluginbinding.EnvLookupResponse{}, errors.New("env capability is not configured")
}

func (h *fakeHostClient) CapabilityCall(pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	return pluginbinding.ProviderCallResponse{}, errors.New("provider capability is not configured")
}

func (c *fakeClient) AuthTest(_ context.Context) (AuthInfo, error) {
	c.authCalls++
	return c.authInfo, c.authErr
}

func (c *fakeClient) ListUsers(_ context.Context) ([]User, error) {
	c.usersCalls++
	return c.users, c.usersErr
}

func (c *fakeClient) ListChannels(_ context.Context) ([]Channel, error) {
	c.channelsCalls++
	return c.channels, c.channelsErr
}

func (c *fakeClient) ListChannelMembers(_ context.Context, _ string, limit int) ([]User, error) {
	c.channelMembersCalls++
	c.lastMembersLimit = limit
	return c.channelMembers, c.channelMembersErr
}

func (c *fakeClient) ListEmojis(_ context.Context, includeCategories bool) (EmojiSet, error) {
	c.emojisCalls++
	c.lastEmojiIncludeCategories = includeCategories
	return EmojiSet{Custom: c.emojis, Categories: c.emojiCategories}, c.emojisErr
}

func (c *fakeClient) ListBookmarks(_ context.Context, channel string) ([]Bookmark, error) {
	c.bookmarksCalls++
	c.lastBookmarkChannel = channel
	return c.bookmarks, c.bookmarksErr
}

func (c *fakeClient) AddBookmark(_ context.Context, request BookmarkAddRequest) (Bookmark, error) {
	c.addBookmarkCalls++
	c.lastBookmarkAdd = request
	bookmark := c.bookmark
	if bookmark.ID == "" {
		bookmark = Bookmark{ID: "BM1", Channel: request.Channel, Title: request.Title, Link: request.Link, Emoji: strings.Trim(request.Emoji, ":")}
	}
	return bookmark, c.bookmarkErr
}

func (c *fakeClient) EditBookmark(_ context.Context, request BookmarkEditRequest) (Bookmark, error) {
	c.editBookmarkCalls++
	c.lastBookmarkEdit = request
	bookmark := c.bookmark
	if bookmark.ID == "" {
		bookmark = Bookmark{ID: request.BookmarkID, Channel: request.Channel, Title: request.Title, Link: request.Link, Emoji: strings.Trim(request.Emoji, ":")}
	}
	return bookmark, c.bookmarkErr
}

func (c *fakeClient) DeleteBookmark(_ context.Context, request BookmarkDeleteRequest) error {
	c.deleteBookmarkCalls++
	c.lastBookmarkDelete = request
	return c.bookmarkErr
}

func (c *fakeClient) GetPresence(_ context.Context, user string) (Presence, error) {
	c.presenceCalls++
	c.lastPresenceUser = user
	return c.presence, c.presenceErr
}

func (c *fakeClient) SetPresence(_ context.Context, presence string) error {
	c.setPresenceCalls++
	c.lastPresenceSet = presence
	return c.setPresenceErr
}

func (c *fakeClient) SendMessage(_ context.Context, request MessageSendRequest) (string, error) {
	c.sendCalls++
	c.lastSend = request
	return c.sendTS, c.sendErr
}

func (c *fakeClient) EditMessage(_ context.Context, request MessageEditRequest) (string, error) {
	c.editCalls++
	c.lastEdit = request
	if c.editTS != "" {
		return c.editTS, c.editErr
	}
	return request.TS, c.editErr
}

func (c *fakeClient) DeleteMessage(_ context.Context, request MessageRefRequest) error {
	c.deleteCalls++
	c.lastDelete = request
	return c.deleteErr
}

func (c *fakeClient) AddReaction(_ context.Context, request ReactionAddRequest) error {
	c.reactionCalls++
	c.lastReaction = request
	return c.reactionErr
}

func (c *fakeClient) RemoveReaction(_ context.Context, request ReactionAddRequest) error {
	c.removeReactionCalls++
	c.lastRemoveReaction = request
	return c.removeReactionErr
}

func (c *fakeClient) JoinChannel(_ context.Context, request ChannelJoinRequest) error {
	c.joinCalls++
	c.lastJoin = request
	return c.joinErr
}

func (c *fakeClient) MarkRead(_ context.Context, request MessageRefRequest) error {
	c.markReadCalls++
	c.lastMarkRead = request
	return c.markReadErr
}

func (c *fakeClient) LatestMessageTS(_ context.Context, _ string) (string, error) {
	c.latestCalls++
	return c.latestTS, c.threadErr
}

func (c *fakeClient) ListFiles(_ context.Context, request FileListRequest) ([]FileRecord, error) {
	c.filesCalls++
	c.lastFileList = request
	return c.files, c.filesErr
}

func (c *fakeClient) GetFileInfo(_ context.Context, fileID string) (FileRecord, error) {
	c.fileInfoCalls++
	file := c.file
	if file.ID == "" {
		file.ID = fileID
	}
	return file, c.fileErr
}

func (c *fakeClient) DownloadFile(_ context.Context, request FileDownloadRequest) (FileDownloadResult, error) {
	c.downloadCalls++
	c.lastDownload = request
	result := c.downloadResult
	if result.FileID == "" {
		result.FileID = request.FileID
	}
	result.OK = true
	return result, c.downloadErr
}

func (c *fakeClient) DeleteFile(_ context.Context, fileID string) error {
	c.deleteFileCalls++
	c.lastDeleteFile = fileID
	return c.deleteFileErr
}

func (c *fakeClient) ListUnreads(_ context.Context, request UnreadsRequest) ([]UnreadChannel, error) {
	c.unreadsCalls++
	c.lastUnreads = request
	return c.unreads, c.unreadsErr
}

func (c *fakeClient) UploadFile(_ context.Context, request FileUploadRequest) (FileUploadResult, error) {
	c.uploadCalls++
	c.lastUpload = request
	result := c.uploadResult
	if result.FileID == "" {
		result.FileID = "F1"
	}
	if result.Channel == "" {
		result.Channel = request.Channel
	}
	if result.ThreadTS == "" {
		result.ThreadTS = request.ThreadTS
	}
	if result.Filename == "" {
		result.Filename = request.Filename
	}
	if result.Size == 0 {
		result.Size = len(request.Content)
	}
	return result, c.uploadErr
}

func (c *fakeClient) SearchMessages(_ context.Context, _ string, _ int) ([]SearchMessage, int, error) {
	c.searchCalls++
	return c.searchMessages, c.searchTotal, c.searchErr
}

func (c *fakeClient) GetThread(_ context.Context, _, _ string, _, _ int) ([]ThreadMessage, error) {
	c.threadCalls++
	return c.thread, c.threadErr
}
