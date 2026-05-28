package dex

import (
	"context"

	"github.com/fluxplane/fluxplane-dex/runtime"
)

// SecretService manages stored auth/secret material under <WorkDir>/auth.
type SecretService struct {
	engine *Engine
}

// StoredSecret is a piece of saved secret material.
type StoredSecret = runtime.StoredSecret

// SecretMaterial is the resolved secret returned to plugins (via grants).
type SecretMaterial = runtime.SecretMaterial

// Save persists a secret value at (plugin, instance, purpose). If instance
// is empty, the default instance is used.
func (s *SecretService) Save(_ context.Context, plugin, instance, purpose string, secret StoredSecret) error {
	return s.engine.runner.State.SaveSecret(plugin, instance, purpose, secret)
}

// Has reports whether any stored secret exists for the given plugin/instance.
func (s *SecretService) Has(_ context.Context, plugin, instance string) (bool, error) {
	return s.engine.runner.State.HasStoredAuth(plugin, instance)
}

// Status returns a status map (purpose -> "stored"|"env"|"missing") for the
// given purposes.
func (s *SecretService) Status(_ context.Context, plugin, instance string, purposes []runtime.SecretPurpose) map[string]string {
	return s.engine.runner.State.SecretStatus(plugin, instance, purposes)
}
