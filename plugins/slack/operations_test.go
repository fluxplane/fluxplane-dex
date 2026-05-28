package slack

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

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
	plugin := testPlugin(factory, nil)

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
	plugin := testPlugin(factory, nil)

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
	get := func(_ pluginbinding.Context, purpose string) (pluginbinding.SecretMaterial, error) {
		if purpose == "user_token" {
			return pluginbinding.SecretMaterial{}, errors.New("missing user token")
		}
		return pluginbinding.SecretMaterial{Purpose: purpose, Value: purpose}, nil
	}
	plugin := testPlugin(factory, get)

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
	plugin := testPlugin(factory, nil)

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
	plugin := testPlugin(factory, nil)

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
	plugin := testPlugin(factory, nil)

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
	plugin := testPlugin(factory, nil)

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
	plugin := testPlugin(factory, nil)

	out := plugintest.RunOK[InfoResult](t, plugin, OperationInfo, map[string]any{})
	if out.Status != "ok" || out.Count != 2 || len(out.Tokens) != 2 {
		t.Fatalf("info result = %#v", out)
	}
	if out.Tokens[0].Purpose != AuthPurposeUser || !out.Tokens[0].OK || out.Tokens[0].TeamID != "T1" || out.Tokens[0].UserID != "U1" {
		t.Fatalf("user token info = %#v", out.Tokens[0])
	}
	if out.Tokens[1].Purpose != AuthPurposeBot || !out.Tokens[1].OK || out.Tokens[1].BotID != "B1" {
		t.Fatalf("bot token info = %#v", out.Tokens[1])
	}
	if factory.clients["user_token"].authCalls != 1 || factory.clients["bot_token"].authCalls != 1 {
		t.Fatalf("auth calls user=%d bot=%d", factory.clients["user_token"].authCalls, factory.clients["bot_token"].authCalls)
	}
}

func TestServiceInfoReportsPartialTokenFailure(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {authErr: slackapi.SlackErrorResponse{Err: "invalid_auth"}},
			"bot_token":  {authInfo: AuthInfo{Team: "Example", TeamID: "T1", UserID: "Ubot", BotID: "B1"}},
		},
	}
	plugin := testPlugin(factory, nil)

	out := plugintest.RunOK[InfoResult](t, plugin, OperationInfo, map[string]any{})
	if out.Status != "partial" || out.Count != 2 {
		t.Fatalf("info result = %#v", out)
	}
	if out.Tokens[0].Purpose != AuthPurposeUser || out.Tokens[0].OK || out.Tokens[0].Error == "" {
		t.Fatalf("user token failure = %#v", out.Tokens[0])
	}
	if out.Tokens[1].Purpose != AuthPurposeBot || !out.Tokens[1].OK || out.Tokens[1].TeamID != "T1" {
		t.Fatalf("bot token info = %#v", out.Tokens[1])
	}
}

func TestServiceInfoFailsWhenNoUserOrBotTokenConfigured(t *testing.T) {
	factory := &capturingFactory{clients: map[string]*fakeClient{}}
	get := func(_ pluginbinding.Context, purpose string) (pluginbinding.SecretMaterial, error) {
		return pluginbinding.SecretMaterial{}, errors.New("missing " + purpose)
	}
	plugin := testPlugin(factory, get)

	if err := plugintest.RunError(t, plugin, OperationInfo, map[string]any{}); err == nil || err.Code != "secret" {
		t.Fatalf("missing token err = %#v", err)
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
	plugin := testPlugin(factory, nil)

	send := plugintest.RunOK[MessageSendResult](t, plugin, OperationMessageSend, map[string]any{"channel": "C1", "text": "hello"})
	if !send.OK || send.TS != "1710000000.123456" || factory.clients["bot_token"].sendCalls != 1 {
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

func TestServiceUploadFileUsesBotTokenAndFilePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chart.png")
	if err := os.WriteFile(path, []byte("png bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"bot_token": {
				uploadResult: FileUploadResult{OK: true, FileID: "F1", Permalink: "https://example.slack.com/files/F1"},
			},
		},
	}
	plugin := testPlugin(factory, nil)

	out := plugintest.RunOK[FileUploadResult](t, plugin, OperationFileUpload, map[string]any{"channel": "C1", "thread_ts": "1710000000.123456", "file_path": path, "initial_comment": "graph", "alt_text": "Latency chart"})
	if !out.OK || out.FileID != "F1" || factory.clients["bot_token"].uploadCalls != 1 {
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

func TestServiceUploadFileAcceptsBase64ContentBytes(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"bot_token": {uploadResult: FileUploadResult{OK: true, FileID: "F2"}},
		},
	}
	plugin := testPlugin(factory, nil)

	out := plugintest.RunOK[FileUploadResult](t, plugin, OperationFileUpload, map[string]any{"channel": "C1", "filename": "chart.png", "content_bytes": "cG5nIGJ5dGVz"})
	if !out.OK || out.FileID != "F2" {
		t.Fatalf("upload result = %#v", out)
	}
	if string(factory.clients["bot_token"].lastUpload.Content) != "png bytes" || factory.clients["bot_token"].lastUpload.Filename != "chart.png" {
		t.Fatalf("upload request = %#v", factory.clients["bot_token"].lastUpload)
	}
}

func TestServiceUploadFileRequiresExactlyOneContentSource(t *testing.T) {
	plugin := testPlugin(&capturingFactory{clients: map[string]*fakeClient{"bot_token": {}}}, nil)

	if err := plugintest.RunError(t, plugin, OperationFileUpload, map[string]any{"channel": "C1", "filename": "chart.png"}); err == nil || err.Code != "bad_input" {
		t.Fatalf("missing content err = %#v", err)
	}
	if err := plugintest.RunError(t, plugin, OperationFileUpload, map[string]any{"channel": "C1", "file_path": "chart.png", "filename": "chart.png", "content_bytes": "cG5n"}); err == nil || err.Code != "bad_input" {
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
	plugin := testPlugin(factory, nil)

	thread := plugintest.RunOK[ThreadResult](t, plugin, OperationThread, map[string]any{"channel": "C1", "ts": "1710000000.123456", "limit": 2})
	if thread.Count != 2 || len(thread.Messages) != 2 {
		t.Fatalf("thread result = %#v", thread)
	}
	if thread.Messages[0].TS != "1710000000.123456" || thread.Messages[1].TS != "1710000001.123456" {
		t.Fatalf("thread messages = %#v", thread.Messages)
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
	plugin := testPlugin(factory, nil)

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
	plugin := testPlugin(&capturingFactory{clients: map[string]*fakeClient{"user_token": {}}}, nil)

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
	plugin := testPlugin(factory, nil)

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
	plugin := testPlugin(factory, nil)

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

func TestServiceChannelMembersDatasourceFallsBackToBotToken(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {channelMembersErr: slackapi.SlackErrorResponse{Err: "missing_scope"}},
			"bot_token":  {channelMembers: []User{{ID: "U1", Name: "timo"}}},
		},
	}
	plugin := testPlugin(factory, nil)

	out := plugintest.DatasourceSearchOK[ChannelMembersDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceChannelMembers, "channel": "C1", "limit": 1})
	if out.Count != 1 || out.Records[0].ID != "C1:U1" {
		t.Fatalf("channel members result = %#v", out)
	}
	if factory.created["user_token"] == 0 || factory.created["bot_token"] == 0 {
		t.Fatalf("expected preferred token fallback: %#v", factory.created)
	}
}

func testPlugin(factory *capturingFactory, get pluginbinding.SecretGetter) *pluginbinding.Plugin {
	if get == nil {
		get = func(_ pluginbinding.Context, purpose string) (pluginbinding.SecretMaterial, error) {
			return pluginbinding.SecretMaterial{Purpose: purpose, Value: purpose}, nil
		}
	}
	return NewPluginWithService(Service{SecretGetter: get, ClientFactory: factory.newClient})
}

type capturingFactory struct {
	clients map[string]*fakeClient
	created map[string]int
}

func (f *capturingFactory) newClient(material pluginbinding.SecretMaterial) (Client, error) {
	if f.created == nil {
		f.created = map[string]int{}
	}
	f.created[material.Purpose]++
	client := f.clients[material.Purpose]
	if client == nil {
		return nil, errors.New("unexpected token " + material.Purpose)
	}
	return client, nil
}

type fakeClient struct {
	authInfo            AuthInfo
	users               []User
	channels            []Channel
	channelMembers      []User
	searchMessages      []SearchMessage
	thread              []ThreadMessage
	sendTS              string
	searchTotal         int
	authErr             error
	usersErr            error
	channelsErr         error
	channelMembersErr   error
	sendErr             error
	uploadErr           error
	searchErr           error
	threadErr           error
	authCalls           int
	usersCalls          int
	channelsCalls       int
	channelMembersCalls int
	lastMembersLimit    int
	sendCalls           int
	uploadCalls         int
	searchCalls         int
	threadCalls         int
	lastSend            MessageSendRequest
	lastUpload          FileUploadRequest
	uploadResult        FileUploadResult
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

func (c *fakeClient) SendMessage(_ context.Context, request MessageSendRequest) (string, error) {
	c.sendCalls++
	c.lastSend = request
	return c.sendTS, c.sendErr
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
