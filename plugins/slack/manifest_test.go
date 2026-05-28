package slack

import (
	"encoding/json"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func TestManifestQuality(t *testing.T) {
	plugintest.AssertManifestQuality(t, Manifest())
}

func TestManifestUsesTokenSetAuthMethod(t *testing.T) {
	manifest := Manifest()
	if len(manifest.Auth) != 1 {
		t.Fatalf("auth methods = %#v", manifest.Auth)
	}
	method := manifest.Auth[0]
	if method.Name != AuthMethodTokenSet {
		t.Fatalf("auth method = %q, want %q", method.Name, AuthMethodTokenSet)
	}
	if method.Name == AuthPurposeBot {
		t.Fatalf("%q is a token purpose, not an auth method", AuthPurposeBot)
	}
	fields := map[string]bool{}
	for _, field := range method.Fields {
		fields[field.Name] = true
	}
	for _, purpose := range []string{AuthPurposeUser, AuthPurposeBot, AuthPurposeApp} {
		if !fields[purpose] {
			t.Fatalf("missing auth field %q in %#v", purpose, method.Fields)
		}
	}
}

func TestManifestWriteOperationsExposeRoleInput(t *testing.T) {
	manifest := Manifest()
	for _, operation := range manifest.Operations {
		if !operationHasEffect(operation, core.OperationEffectWrite) {
			continue
		}
		var schema struct {
			Properties map[string]struct {
				Enum []string `json:"enum"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(operation.Input, &schema); err != nil {
			t.Fatalf("%s input schema: %v", operation.Name, err)
		}
		role, ok := schema.Properties["role"]
		if !ok {
			t.Fatalf("%s input schema missing role: %s", operation.Name, string(operation.Input))
		}
		if len(role.Enum) != 2 || role.Enum[0] != SlackRoleBot || role.Enum[1] != SlackRoleUser {
			t.Fatalf("%s role enum = %#v", operation.Name, role.Enum)
		}
	}
}

func operationHasEffect(operation core.OperationSpec, effect core.OperationEffect) bool {
	for _, candidate := range operation.Effects {
		if candidate == effect {
			return true
		}
	}
	return false
}

func TestManifestThreadOperationsExposeRefInput(t *testing.T) {
	manifest := Manifest()
	operations := map[string]core.OperationSpec{}
	for _, operation := range manifest.Operations {
		operations[operation.Name] = operation
	}
	for _, name := range []string{OperationThread} {
		var schema struct {
			Properties map[string]any `json:"properties"`
			Required   []string       `json:"required"`
		}
		if err := json.Unmarshal(operations[name].Input, &schema); err != nil {
			t.Fatalf("%s input schema: %v", name, err)
		}
		if _, ok := schema.Properties["ref"]; !ok {
			t.Fatalf("%s input schema missing ref: %s", name, string(operations[name].Input))
		}
		for _, field := range schema.Required {
			if field == "channel" || field == "ts" {
				t.Fatalf("%s should not require %s when ref is available: %s", name, field, string(operations[name].Input))
			}
		}
	}
}

func TestManifestDeclaresDatasourceMetadata(t *testing.T) {
	manifest := Manifest()
	if manifest.Metadata[pluginbinding.ManifestProtocolKey] != protocol.Version {
		t.Fatalf("protocol metadata = %#v", manifest.Metadata)
	}
	byEntity := map[string]core.DatasourceSpec{}
	for _, datasource := range manifest.Datasources {
		byEntity[datasource.Entity] = datasource
	}
	channel := byEntity[EntityChannel]
	if channel.EntitySchema == nil || channel.EntitySchema.IDField != "channel_id" || channel.EntitySchema.TitleField != "title" {
		t.Fatalf("channel entity schema = %#v", channel.EntitySchema)
	}
	if channel.Fallback != core.DatasourceFallbackHostIndexFirst {
		t.Fatalf("channel fallback = %q", channel.Fallback)
	}
	if channel.Completion == nil || len(channel.Completion.Fields) == 0 {
		t.Fatalf("channel completion = %#v", channel.Completion)
	}
	for _, tc := range []struct {
		entity string
		id     string
		title  string
	}{
		{EntityMessage, "message_id", "title"},
		{EntityThreadMessage, "thread_message_id", "title"},
		{EntityChannelMember, "channel_member_id", "title"},
	} {
		datasource := byEntity[tc.entity]
		if datasource.EntitySchema == nil || datasource.EntitySchema.IDField != tc.id || datasource.EntitySchema.TitleField != tc.title {
			t.Fatalf("%s entity schema = %#v", tc.entity, datasource.EntitySchema)
		}
		if datasource.Completion == nil || len(datasource.Completion.Fields) == 0 {
			t.Fatalf("%s completion = %#v", tc.entity, datasource.Completion)
		}
		if datasource.Fallback != core.DatasourceFallbackNone {
			t.Fatalf("%s fallback = %q", tc.entity, datasource.Fallback)
		}
	}
}
