package dex

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/protocol"
	"github.com/fluxplane/fluxplane-dex/runtime"
)

// AuthService manages plugin authentication.
type AuthService struct {
	engine *Engine
}

// AuthMethod mirrors core.AuthMethod.
type AuthMethod = core.AuthMethod

// AuthField mirrors core.AuthField.
type AuthField = core.AuthField

// ConnectOptions controls Connect. PrefilledFields short-circuits the
// Prompter for any field whose value is supplied here. AllowPartial=true
// lets Connect succeed even when required fields are missing (matches the
// CLI's --yes flag).
type ConnectOptions struct {
	Instance        string
	PrefilledFields map[string]string
	AllowPartial    bool
}

// ConnectResult summarises what was saved.
type ConnectResult struct {
	Plugin   string
	Instance string
	Saved    []string
	Missing  []string
}

// AutoConnectResult summarises an env-var auto-connect run.
type AutoConnectResult struct {
	Plugin   string   `json:"plugin"`
	Instance string   `json:"instance"`
	Saved    []string `json:"saved,omitempty"`
	Missing  []string `json:"missing,omitempty"`
	Skipped  []string `json:"skipped,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// Methods returns the auth methods declared in the plugin manifest.
func (s *AuthService) Methods(ctx context.Context, plugin string) ([]AuthMethod, error) {
	manifest, err := s.engine.Manifest(ctx, plugin)
	if err != nil {
		return nil, err
	}
	return manifest.Auth, nil
}

// Fields flattens the manifest auth methods into a list of fields, inheriting
// env values from the parent method when a field doesn't declare its own.
func (s *AuthService) Fields(ctx context.Context, plugin string) ([]AuthField, error) {
	methods, err := s.Methods(ctx, plugin)
	if err != nil {
		return nil, err
	}
	var fields []AuthField
	for _, method := range methods {
		for _, field := range method.Fields {
			if len(field.Env) == 0 {
				field.Env = append(field.Env, method.Env...)
			}
			fields = append(fields, field)
		}
	}
	return fields, nil
}

// Test invokes the plugin's auth.test command for the given instance.
func (s *AuthService) Test(ctx context.Context, plugin, instance string) (Response, error) {
	resp, err := s.engine.runner.InvokeInstance(ctx, plugin, instance, protocol.CommandAuthTest, nil)
	if err != nil {
		return resp, err
	}
	if pErr := asPluginError(plugin, resp); pErr != nil {
		return resp, pErr
	}
	return resp, nil
}

// Save persists the given purpose=value pairs as stored secrets. When all
// required fields are present, the plugin is also marked installed
// (unmanaged). Returns counts of saved fields and any fields declared as
// Required but missing from values.
func (s *AuthService) Save(ctx context.Context, plugin, instance string, values map[string]string) (ConnectResult, error) {
	fields, err := s.Fields(ctx, plugin)
	if err != nil {
		return ConnectResult{}, err
	}
	result, err := s.saveValues(plugin, instance, fields, values)
	if err != nil {
		return result, err
	}
	if len(result.Saved) > 0 && len(result.Missing) == 0 {
		if err := s.markInstalled(plugin); err != nil {
			return result, err
		}
	}
	return result, nil
}

// Connect runs an interactive connect flow. For each manifest-declared
// field that isn't pre-filled, it calls Prompter.Input or Prompter.Secret
// based on the field's sensitivity. Saves values, marks the plugin as
// installed. Returns ErrMissingFields when required fields aren't supplied
// and opts.AllowPartial is false.
func (s *AuthService) Connect(ctx context.Context, plugin string, opts ConnectOptions) (ConnectResult, error) {
	if _, ok := s.engine.runner.Marketplace.Resolve(plugin); !ok {
		return ConnectResult{}, fmt.Errorf("%w: %q", ErrPluginNotFound, plugin)
	}
	instance := runtime.NormalizeInstance(opts.Instance)
	fields, err := s.Fields(ctx, plugin)
	if err != nil {
		return ConnectResult{}, err
	}

	values := map[string]string{}
	for k, v := range opts.PrefilledFields {
		if strings.TrimSpace(k) != "" && strings.TrimSpace(v) != "" {
			values[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}

	prompter := s.engine.cfg.Prompter
	_ = prompter.Print(ctx, fmt.Sprintf("Connecting %s/%s", plugin, instance))
	for _, field := range fields {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			continue
		}
		if _, already := values[name]; already {
			continue
		}
		label := name
		if field.Description != "" {
			label = name + " (" + field.Description + ")"
		}
		var value string
		if field.Sensitive || field.Secret {
			value, err = prompter.Secret(ctx, label)
		} else {
			value, err = prompter.Input(ctx, label)
		}
		if err != nil {
			if errors.Is(err, ErrNoPrompter) {
				break
			}
			return ConnectResult{}, err
		}
		value = strings.TrimSpace(value)
		if value != "" {
			values[name] = value
		}
	}

	result, err := s.saveValues(plugin, instance, fields, values)
	if err != nil {
		return result, err
	}
	if len(result.Missing) > 0 && !opts.AllowPartial {
		return result, fmt.Errorf("%w: %s", ErrMissingFields, strings.Join(result.Missing, ", "))
	}
	if len(result.Saved) > 0 {
		if err := s.markInstalled(plugin); err != nil {
			return result, err
		}
	}
	return result, nil
}

// AutoConnect populates fields from environment variables declared in the
// manifest. Mirrors `dex auth connect auto`.
func (s *AuthService) AutoConnect(ctx context.Context, plugin, instance string) (AutoConnectResult, error) {
	if _, ok := s.engine.runner.Marketplace.Resolve(plugin); !ok {
		return AutoConnectResult{}, fmt.Errorf("%w: %q", ErrPluginNotFound, plugin)
	}
	fields, err := s.Fields(ctx, plugin)
	if err != nil {
		return AutoConnectResult{}, err
	}
	instance = runtime.NormalizeInstance(instance)
	result := AutoConnectResult{Plugin: plugin, Instance: instance}
	for _, field := range dedupeAuthFields(fields) {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			continue
		}
		value, ok := firstEnvValue(field.Env)
		if ok {
			kind := "bearer_token"
			if !field.Sensitive && !field.Secret {
				kind = "config"
			}
			if err := s.engine.runner.State.SaveSecret(plugin, instance, name, StoredSecret{Kind: kind, Value: value}); err != nil {
				return result, err
			}
			result.Saved = append(result.Saved, name)
			continue
		}
		if field.Required {
			result.Missing = append(result.Missing, name)
		} else {
			result.Skipped = append(result.Skipped, name)
		}
	}
	if len(result.Saved) > 0 && len(result.Missing) == 0 {
		if err := s.markInstalled(plugin); err != nil {
			return result, err
		}
	}
	return result, nil
}

// HasStored reports whether any stored secret exists for plugin/instance.
func (s *AuthService) HasStored(_ context.Context, plugin, instance string) (bool, error) {
	return s.engine.runner.State.HasStoredAuth(plugin, instance)
}

func (s *AuthService) saveValues(plugin, instance string, fields []AuthField, values map[string]string) (ConnectResult, error) {
	instance = runtime.NormalizeInstance(instance)
	declared := map[string]AuthField{}
	for _, field := range fields {
		declared[field.Name] = field
	}
	result := ConnectResult{Plugin: plugin, Instance: instance}
	for purpose, value := range values {
		field := declared[purpose]
		if field.Name == "" {
			field = AuthField{Name: purpose, Sensitive: true, Secret: true}
		}
		kind := "bearer_token"
		if !field.Sensitive && !field.Secret {
			kind = "config"
		}
		if err := s.engine.runner.State.SaveSecret(plugin, instance, purpose, StoredSecret{Kind: kind, Value: value}); err != nil {
			return result, err
		}
		result.Saved = append(result.Saved, purpose)
	}
	for _, field := range fields {
		if field.Required && strings.TrimSpace(values[field.Name]) == "" {
			result.Missing = append(result.Missing, field.Name)
		}
	}
	return result, nil
}

func (s *AuthService) markInstalled(plugin string) error {
	entry, ok := s.engine.runner.Marketplace.Resolve(plugin)
	if !ok {
		return fmt.Errorf("%w: %q", ErrPluginNotFound, plugin)
	}
	return s.engine.runner.State.MarkPluginInstalled(entry, false)
}

func dedupeAuthFields(fields []AuthField) []AuthField {
	seen := map[string]bool{}
	out := make([]AuthField, 0, len(fields))
	for _, field := range fields {
		name := strings.TrimSpace(field.Name)
		if name == "" || seen[name] {
			continue
		}
		field.Name = name
		seen[name] = true
		out = append(out, field)
	}
	return out
}

func firstEnvValue(candidates []string) (string, bool) {
	for _, key := range candidates {
		if value := strings.TrimSpace(os.Getenv(strings.TrimSpace(key))); value != "" {
			return value, true
		}
	}
	return "", false
}
