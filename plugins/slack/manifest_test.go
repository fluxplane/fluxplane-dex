package slack

import (
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
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

func TestManifestDeclaresDatasourceMetadata(t *testing.T) {
	manifest := Manifest()
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
