package slack

import (
	"context"
	"encoding/json"
	"errors"
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

	search := plugintest.RunOK[SearchResult](t, plugin, OperationSearch, map[string]any{"query": "hello", "limit": 5})
	if search.Count != 1 || len(search.Messages) != 1 || search.Messages[0].Channel != "C1" || factory.clients["user_token"].searchCalls != 1 {
		t.Fatalf("search result = %#v calls=%d", search, factory.clients["user_token"].searchCalls)
	}

	thread := plugintest.RunOK[ThreadResult](t, plugin, OperationThread, map[string]any{"channel": "C1", "ts": "1710000000.123456"})
	if thread.Count != 1 || thread.Messages[0].Text != "root" || factory.clients["user_token"].threadCalls != 1 {
		t.Fatalf("thread result = %#v calls=%d", thread, factory.clients["user_token"].threadCalls)
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
	users          []User
	channels       []Channel
	searchMessages []SearchMessage
	thread         []ThreadMessage
	sendTS         string
	searchTotal    int
	usersErr       error
	channelsErr    error
	sendErr        error
	searchErr      error
	threadErr      error
	usersCalls     int
	channelsCalls  int
	sendCalls      int
	searchCalls    int
	threadCalls    int
}

func (c *fakeClient) ListUsers(_ context.Context) ([]User, error) {
	c.usersCalls++
	return c.users, c.usersErr
}

func (c *fakeClient) ListChannels(_ context.Context) ([]Channel, error) {
	c.channelsCalls++
	return c.channels, c.channelsErr
}

func (c *fakeClient) SendMessage(_ context.Context, _, _ string) (string, error) {
	c.sendCalls++
	return c.sendTS, c.sendErr
}

func (c *fakeClient) SearchMessages(_ context.Context, _ string, _ int) ([]SearchMessage, int, error) {
	c.searchCalls++
	return c.searchMessages, c.searchTotal, c.searchErr
}

func (c *fakeClient) GetThread(_ context.Context, _, _ string, _ int) ([]ThreadMessage, error) {
	c.threadCalls++
	return c.thread, c.threadErr
}
