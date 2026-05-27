package runtime

import (
	"context"
	"testing"
	"time"
)

func TestSecretGrantAllowsScopedPurpose(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := state.SaveSecret("example", "work", "access_token", StoredSecret{Value: "token"}); err != nil {
		t.Fatal(err)
	}
	grant, err := state.CreateGrant("example", "work", []string{"example.list"}, []SecretPurpose{{Name: "access_token"}}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	material, err := state.ResolveSecret(context.Background(), "example", "work", "access_token", grant.Token)
	if err != nil {
		t.Fatal(err)
	}
	if material.Value != "token" {
		t.Fatalf("material value = %q", material.Value)
	}
}

func TestSecretGrantRejectsWrongInstance(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	grant, err := state.CreateGrant("example", "work", []string{"example.list"}, []SecretPurpose{{Name: "access_token"}}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.ResolveSecret(context.Background(), "example", "other", "access_token", grant.Token); err == nil {
		t.Fatal("expected wrong instance to be rejected")
	}
}

func TestSecretGrantRejectsWrongPurpose(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	grant, err := state.CreateGrant("example", "default", []string{"example.search"}, []SecretPurpose{{Name: "primary_token"}}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.ResolveSecret(context.Background(), "example", "default", "other_token", grant.Token); err == nil {
		t.Fatal("expected wrong purpose to be rejected")
	}
}
