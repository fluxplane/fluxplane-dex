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

	secret "github.com/fluxplane/fluxplane-secret"
)

const DefaultInstance = "default"

type Grant struct {
	Token        string              `json:"token"`
	Plugin       string              `json:"plugin"`
	Instance     string              `json:"instance"`
	Operations   []string            `json:"operations,omitempty"`
	Capabilities []CapabilityGrant   `json:"capabilities,omitempty"`
	Purposes     []string            `json:"purposes,omitempty"`
	PurposeEnv   map[string][]string `json:"purpose_env,omitempty"`
	ExpiresAt    time.Time           `json:"expires_at"`
	CreatedAt    time.Time           `json:"created_at"`
}

type CapabilityGrant struct {
	Name     string `json:"name"`
	Provider string `json:"provider,omitempty"`
	Action   string `json:"action,omitempty"`
}

type SecretMaterial struct {
	Kind    secret.Kind `json:"kind,omitempty"`
	Value   string      `json:"value"`
	Source  string      `json:"source,omitempty"`
	Purpose string      `json:"purpose,omitempty"`
	Ref     secret.Ref  `json:"ref,omitempty"`
}

// Material converts the runtime wire shape to shared trusted secret material.
func (m SecretMaterial) Material() secret.Material {
	return secret.Material{Ref: m.Ref, Kind: m.Kind, Value: []byte(m.Value)}
}

type StoredSecret = secret.StoredSecret

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
	return s.CreateGrantWithCapabilities(plugin, instance, operations, purposes, nil, ttl)
}

func (s State) CreateGrantWithCapabilities(plugin, instance string, operations []string, purposes []SecretPurpose, capabilities []CapabilityGrant, ttl time.Duration) (Grant, error) {
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
		Token:        token,
		Plugin:       plugin,
		Instance:     instance,
		Operations:   normalizeList(operations),
		Capabilities: normalizeCapabilityGrants(capabilities),
		Purposes:     purposeNames(purposes),
		PurposeEnv:   purposeEnv(purposes),
		CreatedAt:    now,
		ExpiresAt:    now.Add(ttl),
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

func (s State) ValidateCapabilityGrant(plugin, instance, token string, requested CapabilityGrant) error {
	grant, err := s.validateGrantBase(plugin, instance, token)
	if err != nil {
		return err
	}
	requested = normalizeCapabilityGrant(requested)
	if requested.Name == "" {
		return fmt.Errorf("capability grant name is required")
	}
	for _, allowed := range grant.Capabilities {
		if capabilityGrantMatches(allowed, requested) {
			return nil
		}
	}
	return fmt.Errorf("secret grant does not allow capability %q", requested.Name)
}

func (s State) ResolveSecret(ctx context.Context, plugin, instance, purpose, token string) (SecretMaterial, error) {
	grant, err := s.validateGrant(plugin, instance, purpose, token)
	if err != nil {
		return SecretMaterial{}, err
	}
	ref := secret.Plugin(grant.Plugin, grant.Instance, secret.Slot(purpose))
	material, ok, err := s.loadStoredSecretRef(ref)
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

// ResolveSecretRef resolves a shared secret ref after validating the grant for
// plugin-scoped refs. Env refs are resolved from grant purpose env mappings.
func (s State) ResolveSecretRef(ctx context.Context, ref secret.Ref, token string) (SecretMaterial, error) {
	ref = ref.Normalize()
	if ref.Scheme != secret.SchemePlugin {
		return SecretMaterial{}, fmt.Errorf("secret ref scheme %q is unsupported", ref.Scheme)
	}
	return s.ResolveSecret(ctx, ref.Plugin, ref.Instance, string(ref.Slot), token)
}

func (s State) SaveSecret(plugin, instance, purpose string, stored StoredSecret) error {
	return s.SaveSecretRef(secret.Plugin(plugin, NormalizeInstance(instance), secret.Slot(purpose)), stored)
}

// SaveSecretRef persists a shared plugin secret ref using the dex auth store.
func (s State) SaveSecretRef(ref secret.Ref, stored StoredSecret) error {
	ref = ref.Normalize()
	if ref.Scheme != secret.SchemePlugin || ref.Plugin == "" || ref.Slot == "" {
		return fmt.Errorf("plugin secret ref is required")
	}
	stored.Ref = ref
	return secret.NewFileStore(s.AuthDir()).SaveSecret(context.Background(), stored)
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
	return secret.NewFileStore(s.AuthDir()).HasPluginSecrets(plugin, NormalizeInstance(instance))
}

func (s State) validateGrant(plugin, instance, purpose, token string) (Grant, error) {
	grant, err := s.validateGrantBase(plugin, instance, token)
	if err != nil {
		return Grant{}, err
	}
	purpose = strings.TrimSpace(purpose)
	if !contains(grant.Purposes, purpose) {
		return Grant{}, fmt.Errorf("secret grant does not allow purpose %q", purpose)
	}
	return grant, nil
}

func (s State) validateGrantBase(plugin, instance, token string) (Grant, error) {
	plugin = strings.TrimSpace(plugin)
	instance = NormalizeInstance(instance)
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
	return grant, nil
}

func (s State) loadStoredSecret(plugin, instance, purpose string) (SecretMaterial, bool, error) {
	return s.loadStoredSecretRef(secret.Plugin(plugin, NormalizeInstance(instance), secret.Slot(purpose)))
}

func (s State) loadStoredSecretRef(ref secret.Ref) (SecretMaterial, bool, error) {
	material, ok, err := secret.NewFileStore(s.AuthDir()).ResolveSecret(context.Background(), ref)
	if err != nil || !ok {
		return SecretMaterial{}, ok, err
	}
	return SecretMaterial{Kind: material.Kind, Value: string(material.Value), Source: "store", Ref: material.Ref.Normalize()}, true, nil
}

func (s State) grantPath(token string) string {
	return filepath.Join(s.GrantsDir(), pathName(token)+".json")
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
			return SecretMaterial{Kind: secret.KindBearerToken, Value: value, Source: "env:" + key, Ref: secret.Env(key)}, true
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

func normalizeCapabilityGrants(values []CapabilityGrant) []CapabilityGrant {
	seen := map[CapabilityGrant]bool{}
	var out []CapabilityGrant
	for _, value := range values {
		value = normalizeCapabilityGrant(value)
		if value.Name == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func normalizeCapabilityGrant(value CapabilityGrant) CapabilityGrant {
	return CapabilityGrant{
		Name:     strings.TrimSpace(value.Name),
		Provider: strings.TrimSpace(value.Provider),
		Action:   strings.TrimSpace(value.Action),
	}
}

func capabilityGrantMatches(allowed, requested CapabilityGrant) bool {
	allowed = normalizeCapabilityGrant(allowed)
	requested = normalizeCapabilityGrant(requested)
	if allowed.Name != requested.Name {
		return false
	}
	if allowed.Provider != "" && allowed.Provider != "*" && allowed.Provider != requested.Provider {
		return false
	}
	if allowed.Action != "" && allowed.Action != "*" && allowed.Action != requested.Action {
		return false
	}
	return true
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

func pathName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "_"
	}
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}
