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

func TestSecretPathNamesDoNotCollide(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := state.SaveSecret("example", "prod/work", "access_token", StoredSecret{Value: "slash-instance"}); err != nil {
		t.Fatal(err)
	}
	if err := state.SaveSecret("example", "prod_work", "access_token", StoredSecret{Value: "underscore-instance"}); err != nil {
		t.Fatal(err)
	}
	grant, err := state.CreateGrant("example", "prod_work", nil, []SecretPurpose{{Name: "access_token"}}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	material, err := state.ResolveSecret(context.Background(), "example", "prod_work", "access_token", grant.Token)
	if err != nil {
		t.Fatal(err)
	}
	if material.Value != "underscore-instance" {
		t.Fatalf("resolved colliding instance secret %q", material.Value)
	}

	if err := state.SaveSecret("example", "default", "access/token", StoredSecret{Value: "slash-purpose"}); err != nil {
		t.Fatal(err)
	}
	if err := state.SaveSecret("example", "default", "access_token", StoredSecret{Value: "underscore-purpose"}); err != nil {
		t.Fatal(err)
	}
	purposeGrant, err := state.CreateGrant("example", "default", nil, []SecretPurpose{{Name: "access_token"}}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	purposeMaterial, err := state.ResolveSecret(context.Background(), "example", "default", "access_token", purposeGrant.Token)
	if err != nil {
		t.Fatal(err)
	}
	if purposeMaterial.Value != "underscore-purpose" {
		t.Fatalf("resolved colliding purpose secret %q", purposeMaterial.Value)
	}
}
