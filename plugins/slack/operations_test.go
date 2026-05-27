package slack

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/fluxplane/fluxplane-dex/plugins/internal/pluginutil"
	"github.com/fluxplane/fluxplane-dex/protocol"
	slackapi "github.com/slack-go/slack"
)

func TestOperationRunnerIndexBuildUsesUserTokenFirst(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				users:    []User{{ID: "U1", Name: "timo", RealName: "Timo Friedl", DisplayName: "Timo"}},
				channels: []Channel{{ID: "C1", Name: "general", IsChannel: true}},
			},
		},
	}
	runner := testRunner(factory, nil)

	result := runner.Run(protocol.Request{Instance: "default"}, callWithInput("slack.index.build", map[string]any{}), nil)
	if !result.OK {
		t.Fatalf("operation failed: %#v", result.Error)
	}
	var out struct {
		Indexes []struct {
			Index   string            `json:"index"`
			Records []json.RawMessage `json:"records"`
		} `json:"indexes"`
	}
	decodeResult(t, result, &out)
	if len(out.Indexes) != 2 || out.Indexes[0].Index != "slack.users" || out.Indexes[1].Index != "slack.channels" {
		t.Fatalf("indexes = %#v", out.Indexes)
	}
	if len(out.Indexes[0].Records) != 1 || len(out.Indexes[1].Records) != 1 {
		t.Fatalf("records = %#v", out.Indexes)
	}
	if factory.created["bot_token"] != 0 {
		t.Fatalf("bot token should not be used: %#v", factory.created)
	}
}

func TestOperationRunnerIndexBuildFallsBackToBotToken(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {usersErr: slackapi.SlackErrorResponse{Err: "missing_scope"}, channelsErr: slackapi.SlackErrorResponse{Err: "missing_scope"}},
			"bot_token": {
				users:    []User{{ID: "U1", Name: "timo"}, {ID: "U2", Name: "deleted", Deleted: true}},
				channels: []Channel{{ID: "C1", Name: "general", IsChannel: true}},
			},
		},
	}
	runner := testRunner(factory, nil)

	result := runner.Run(protocol.Request{Instance: "default"}, callWithInput("slack.index.build", map[string]any{}), nil)
	if !result.OK {
		t.Fatalf("operation failed: %#v", result.Error)
	}
	var out struct {
		Indexes []struct {
			Index   string            `json:"index"`
			Records []json.RawMessage `json:"records"`
		} `json:"indexes"`
	}
	decodeResult(t, result, &out)
	if factory.created["user_token"] == 0 || factory.created["bot_token"] == 0 {
		t.Fatalf("expected user then bot token: %#v", factory.created)
	}
	if len(out.Indexes[0].Records) != 1 {
		t.Fatalf("deleted users should be filtered: %#v", out.Indexes[0].Records)
	}
}

func TestOperationRunnerIndexBuildFallsBackWhenUserTokenMissing(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"bot_token": {users: []User{{ID: "U1", Name: "timo"}}},
		},
	}
	get := func(_ protocol.Request, purpose string, _ map[string]pluginutil.SecretMaterial) (pluginutil.SecretMaterial, error) {
		if purpose == "user_token" {
			return pluginutil.SecretMaterial{}, errors.New("missing user token")
		}
		return pluginutil.SecretMaterial{Purpose: purpose, Value: purpose}, nil
	}
	runner := testRunner(factory, get)

	result := runner.Run(protocol.Request{Instance: "default"}, callWithInput("slack.index.build", map[string]any{"entity": "slack.user"}), nil)
	if !result.OK {
		t.Fatalf("operation failed: %#v", result.Error)
	}
	if factory.created["bot_token"] != 1 {
		t.Fatalf("expected bot token fallback: %#v", factory.created)
	}
}

func TestOperationRunnerIndexBuildDoesNotFallbackOnNetworkError(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {usersErr: errors.New("network down")},
			"bot_token":  {users: []User{{ID: "U1"}}},
		},
	}
	runner := testRunner(factory, nil)

	result := runner.Run(protocol.Request{Instance: "default"}, callWithInput("slack.index.build", map[string]any{"entity": "slack.user"}), nil)
	if result.OK {
		t.Fatalf("expected failure")
	}
	if factory.created["bot_token"] != 0 {
		t.Fatalf("bot token should not be used for non-auth error: %#v", factory.created)
	}
}

func TestOperationRunnerIndexBuildCanTargetOneIndex(t *testing.T) {
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {channels: []Channel{{ID: "C1", Name: "general", IsChannel: true}}},
		},
	}
	runner := testRunner(factory, nil)

	result := runner.Run(protocol.Request{Instance: "default"}, callWithInput("slack.index.build", map[string]any{"index": "slack.channels"}), nil)
	if !result.OK {
		t.Fatalf("operation failed: %#v", result.Error)
	}
	var out struct {
		Indexes []struct {
			Index   string            `json:"index"`
			Records []json.RawMessage `json:"records"`
		} `json:"indexes"`
	}
	decodeResult(t, result, &out)
	if len(out.Indexes) != 1 || out.Indexes[0].Index != "slack.channels" || len(out.Indexes[0].Records) != 1 {
		t.Fatalf("targeted output = %#v", out.Indexes)
	}
	if factory.clients["user_token"].usersCalls != 0 || factory.clients["user_token"].channelsCalls != 1 {
		t.Fatalf("unexpected client calls: %#v", factory.clients["user_token"])
	}
}

func testRunner(factory *capturingFactory, get SecretGetter) OperationRunner {
	if get == nil {
		get = func(_ protocol.Request, purpose string, _ map[string]pluginutil.SecretMaterial) (pluginutil.SecretMaterial, error) {
			return pluginutil.SecretMaterial{Purpose: purpose, Value: purpose}, nil
		}
	}
	return OperationRunner{SecretGetter: get, ClientFactory: factory.newClient}
}

func callWithInput(name string, input map[string]any) protocol.OperationCall {
	raw, _ := json.Marshal(input)
	return protocol.OperationCall{Name: name, Input: raw}
}

func decodeResult(t *testing.T, result protocol.OperationResult, target any) {
	t.Helper()
	if err := json.Unmarshal(result.Result, target); err != nil {
		t.Fatal(err)
	}
}

type capturingFactory struct {
	clients map[string]*fakeClient
	created map[string]int
}

func (f *capturingFactory) newClient(material pluginutil.SecretMaterial) (Client, error) {
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
	users         []User
	channels      []Channel
	usersErr      error
	channelsErr   error
	usersCalls    int
	channelsCalls int
}

func (c *fakeClient) ListUsers(_ context.Context) ([]User, error) {
	c.usersCalls++
	return c.users, c.usersErr
}

func (c *fakeClient) ListChannels(_ context.Context) ([]Channel, error) {
	c.channelsCalls++
	return c.channels, c.channelsErr
}
