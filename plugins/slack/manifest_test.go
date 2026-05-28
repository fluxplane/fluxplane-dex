package slack

import (
	"testing"

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
