package runtime

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const DefaultInstance = "default"

type Grant struct {
	Token      string              `json:"token"`
	Plugin     string              `json:"plugin"`
	Instance   string              `json:"instance"`
	Operations []string            `json:"operations,omitempty"`
	Purposes   []string            `json:"purposes,omitempty"`
	PurposeEnv map[string][]string `json:"purpose_env,omitempty"`
	ExpiresAt  time.Time           `json:"expires_at"`
	CreatedAt  time.Time           `json:"created_at"`
}

type SecretMaterial struct {
	Kind    string `json:"kind,omitempty"`
	Value   string `json:"value"`
	Source  string `json:"source,omitempty"`
	Purpose string `json:"purpose,omitempty"`
}

type StoredSecret struct {
	Kind      string            `json:"kind,omitempty"`
	Value     string            `json:"value"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	UpdatedAt time.Time         `json:"updated_at"`
}

func NormalizeInstance(instance string) string {
	instance = strings.TrimSpace(instance)
	if instance == "" {
		return DefaultInstance
	}
	return instance
}

func (s State) GrantsDir() string {
	return filepath.Join(s.Home, "grants")
}

type SecretPurpose struct {
	Name string
	Env  []string
}

func (s State) CreateGrant(plugin, instance string, operations []string, purposes []SecretPurpose, ttl time.Duration) (Grant, error) {
	plugin = strings.TrimSpace(plugin)
	instance = NormalizeInstance(instance)
	if plugin == "" {
		return Grant{}, fmt.Errorf("secret grant plugin is empty")
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	token, err := randomToken()
	if err != nil {
		return Grant{}, err
	}
	now := time.Now().UTC()
	grant := Grant{
		Token:      token,
		Plugin:     plugin,
		Instance:   instance,
		Operations: normalizeList(operations),
		Purposes:   purposeNames(purposes),
		PurposeEnv: purposeEnv(purposes),
		CreatedAt:  now,
		ExpiresAt:  now.Add(ttl),
	}
	if err := os.MkdirAll(s.GrantsDir(), 0o700); err != nil {
		return Grant{}, err
	}
	data, err := json.MarshalIndent(grant, "", "  ")
	if err != nil {
		return Grant{}, err
	}
	if err := os.WriteFile(s.grantPath(token), data, 0o600); err != nil {
		return Grant{}, err
	}
	return grant, nil
}

func (s State) ResolveSecret(ctx context.Context, plugin, instance, purpose, token string) (SecretMaterial, error) {
	grant, err := s.validateGrant(plugin, instance, purpose, token)
	if err != nil {
		return SecretMaterial{}, err
	}
	material, ok, err := s.loadStoredSecret(grant.Plugin, grant.Instance, purpose)
	if err != nil {
		return SecretMaterial{}, err
	}
	if ok {
		material.Purpose = purpose
		return material, nil
	}
	material, ok = envSecret(grant, purpose)
	if ok {
		material.Purpose = purpose
		return material, nil
	}
	return SecretMaterial{}, fmt.Errorf("secret material not found for %s/%s purpose %q", grant.Plugin, grant.Instance, purpose)
}

func (s State) SaveSecret(plugin, instance, purpose string, secret StoredSecret) error {
	plugin = strings.TrimSpace(plugin)
	instance = NormalizeInstance(instance)
	purpose = strings.TrimSpace(purpose)
	if plugin == "" || purpose == "" {
		return fmt.Errorf("plugin and purpose are required")
	}
	if strings.TrimSpace(secret.Value) == "" {
		return fmt.Errorf("secret value is empty")
	}
	if secret.Kind == "" {
		secret.Kind = "bearer_token"
	}
	secret.UpdatedAt = time.Now().UTC()
	dir := filepath.Dir(s.secretPath(plugin, instance, purpose))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(secret, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.secretPath(plugin, instance, purpose), data, 0o600)
}

func (s State) SecretStatus(plugin, instance string, purposes []SecretPurpose) map[string]string {
	instance = NormalizeInstance(instance)
	out := map[string]string{}
	for _, purpose := range purposes {
		name := strings.TrimSpace(purpose.Name)
		if name == "" {
			continue
		}
		if _, ok, err := s.loadStoredSecret(plugin, instance, name); err == nil && ok {
			out[name] = "stored"
			continue
		}
		if _, ok := envSecret(Grant{PurposeEnv: map[string][]string{name: purpose.Env}}, name); ok {
			out[name] = "env"
			continue
		}
		out[name] = "missing"
	}
	return out
}

func (s State) HasStoredAuth(plugin, instance string) (bool, error) {
	dir := filepath.Join(s.AuthDir(), safeName(plugin), safeName(NormalizeInstance(instance)))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		purpose := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		if _, ok, err := s.loadStoredSecret(plugin, instance, purpose); err != nil {
			return false, err
		} else if ok {
			return true, nil
		}
	}
	return false, nil
}

func (s State) validateGrant(plugin, instance, purpose, token string) (Grant, error) {
	plugin = strings.TrimSpace(plugin)
	instance = NormalizeInstance(instance)
	purpose = strings.TrimSpace(purpose)
	token = strings.TrimSpace(token)
	if token == "" {
		return Grant{}, fmt.Errorf("secret grant is required")
	}
	data, err := os.ReadFile(s.grantPath(token))
	if err != nil {
		if os.IsNotExist(err) {
			return Grant{}, fmt.Errorf("secret grant is invalid")
		}
		return Grant{}, err
	}
	var grant Grant
	if err := json.Unmarshal(data, &grant); err != nil {
		return Grant{}, err
	}
	if time.Now().UTC().After(grant.ExpiresAt) {
		return Grant{}, fmt.Errorf("secret grant is expired")
	}
	if grant.Plugin != plugin || grant.Instance != instance {
		return Grant{}, fmt.Errorf("secret grant does not match plugin instance")
	}
	if !contains(grant.Purposes, purpose) {
		return Grant{}, fmt.Errorf("secret grant does not allow purpose %q", purpose)
	}
	return grant, nil
}

func (s State) loadStoredSecret(plugin, instance, purpose string) (SecretMaterial, bool, error) {
	data, err := os.ReadFile(s.secretPath(plugin, NormalizeInstance(instance), purpose))
	if err != nil {
		if os.IsNotExist(err) {
			return SecretMaterial{}, false, nil
		}
		return SecretMaterial{}, false, err
	}
	var stored StoredSecret
	if err := json.Unmarshal(data, &stored); err != nil {
		return SecretMaterial{}, false, err
	}
	if strings.TrimSpace(stored.Value) == "" {
		return SecretMaterial{}, false, nil
	}
	return SecretMaterial{Kind: stored.Kind, Value: stored.Value, Source: "store"}, true, nil
}

func (s State) grantPath(token string) string {
	return filepath.Join(s.GrantsDir(), safeName(token)+".json")
}

func (s State) secretPath(plugin, instance, purpose string) string {
	return filepath.Join(s.AuthDir(), safeName(plugin), safeName(instance), safeName(purpose)+".json")
}

func randomToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func envSecret(grant Grant, purpose string) (SecretMaterial, bool) {
	for _, key := range grant.PurposeEnv[strings.TrimSpace(purpose)] {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return SecretMaterial{Kind: "bearer_token", Value: value, Source: "env:" + key}, true
		}
	}
	return SecretMaterial{}, false
}

func normalizeList(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func purposeNames(purposes []SecretPurpose) []string {
	var names []string
	for _, purpose := range purposes {
		if strings.TrimSpace(purpose.Name) != "" {
			names = append(names, purpose.Name)
		}
	}
	return normalizeList(names)
}

func purposeEnv(purposes []SecretPurpose) map[string][]string {
	out := map[string][]string{}
	for _, purpose := range purposes {
		name := strings.TrimSpace(purpose.Name)
		if name == "" {
			continue
		}
		out[name] = normalizeList(purpose.Env)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func safeName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "_"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}
