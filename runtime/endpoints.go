package runtime

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-dex/core"
)

type EndpointRecord struct {
	core.EndpointRef
	LastHealth *EndpointHealth `json:"last_health,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

type EndpointHealth struct {
	OK         bool            `json:"ok"`
	CheckedAt  time.Time       `json:"checked_at"`
	Method     string          `json:"method,omitempty"`
	DurationMS int64           `json:"duration_ms,omitempty"`
	Error      string          `json:"error,omitempty"`
	Details    json.RawMessage `json:"details,omitempty"`
}

type EndpointRegistry struct {
	Endpoints []EndpointRecord `json:"endpoints"`
}

func (s State) EndpointsPath() string {
	return filepath.Join(s.Home, "endpoints", "registry.json")
}

func (s State) LoadEndpoints() (EndpointRegistry, error) {
	var registry EndpointRegistry
	data, err := os.ReadFile(s.EndpointsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return registry, nil
		}
		return registry, err
	}
	if err := json.Unmarshal(data, &registry); err != nil {
		return registry, err
	}
	sort.Slice(registry.Endpoints, func(i, j int) bool { return registry.Endpoints[i].ID < registry.Endpoints[j].ID })
	return registry, nil
}

func (s State) SaveEndpoint(ref core.EndpointRef) (EndpointRecord, error) {
	ref = NormalizeEndpointRef(ref)
	if ref.ID == "" {
		return EndpointRecord{}, fmt.Errorf("endpoint id is required")
	}
	if ref.URL == "" {
		return EndpointRecord{}, fmt.Errorf("endpoint url is required")
	}
	if parsed, err := url.Parse(ref.URL); err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return EndpointRecord{}, fmt.Errorf("invalid endpoint url %q", ref.URL)
	}
	registry, err := s.LoadEndpoints()
	if err != nil {
		return EndpointRecord{}, err
	}
	now := time.Now().UTC()
	record := EndpointRecord{EndpointRef: ref, CreatedAt: now, UpdatedAt: now}
	for i := range registry.Endpoints {
		if registry.Endpoints[i].ID == ref.ID {
			record.CreatedAt = registry.Endpoints[i].CreatedAt
			record.LastHealth = registry.Endpoints[i].LastHealth
			registry.Endpoints[i] = record
			if err := s.writeEndpoints(registry); err != nil {
				return EndpointRecord{}, err
			}
			return record, nil
		}
	}
	registry.Endpoints = append(registry.Endpoints, record)
	if err := s.writeEndpoints(registry); err != nil {
		return EndpointRecord{}, err
	}
	return record, nil
}

func (s State) SaveEndpointHealth(id string, health EndpointHealth) (EndpointRecord, error) {
	registry, err := s.LoadEndpoints()
	if err != nil {
		return EndpointRecord{}, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return EndpointRecord{}, fmt.Errorf("endpoint id is required")
	}
	if health.CheckedAt.IsZero() {
		health.CheckedAt = time.Now().UTC()
	} else {
		health.CheckedAt = health.CheckedAt.UTC()
	}
	now := time.Now().UTC()
	for i := range registry.Endpoints {
		if registry.Endpoints[i].ID == id {
			registry.Endpoints[i].LastHealth = &health
			registry.Endpoints[i].UpdatedAt = now
			if err := s.writeEndpoints(registry); err != nil {
				return EndpointRecord{}, err
			}
			return registry.Endpoints[i], nil
		}
	}
	return EndpointRecord{}, fmt.Errorf("unknown endpoint %q", id)
}

func (s State) SaveEndpointCandidate(candidate core.EndpointCandidate) (EndpointRecord, error) {
	return s.SaveEndpoint(core.EndpointRef{
		ID:            firstEndpointNonEmpty(candidate.ID, endpointID(candidate.Product, candidate.URL)),
		URL:           candidate.URL,
		Product:       candidate.Product,
		Protocol:      candidate.Protocol,
		Source:        candidate.Source,
		CredentialRef: candidate.CredentialRef,
		Labels:        candidate.Labels,
		Annotations:   candidate.Annotations,
	})
}

func (s State) ListEndpoints(product string) ([]EndpointRecord, error) {
	registry, err := s.LoadEndpoints()
	if err != nil {
		return nil, err
	}
	product = strings.TrimSpace(product)
	if product == "" {
		return registry.Endpoints, nil
	}
	var out []EndpointRecord
	for _, endpoint := range registry.Endpoints {
		if endpoint.Product == product {
			out = append(out, endpoint)
		}
	}
	return out, nil
}

func (s State) GetEndpoint(id string) (EndpointRecord, bool, error) {
	registry, err := s.LoadEndpoints()
	if err != nil {
		return EndpointRecord{}, false, err
	}
	id = strings.TrimSpace(id)
	for _, endpoint := range registry.Endpoints {
		if endpoint.ID == id {
			return endpoint, true, nil
		}
	}
	return EndpointRecord{}, false, nil
}

func (s State) RemoveEndpoint(id string) (bool, error) {
	registry, err := s.LoadEndpoints()
	if err != nil {
		return false, err
	}
	id = strings.TrimSpace(id)
	next := registry.Endpoints[:0]
	removed := false
	for _, endpoint := range registry.Endpoints {
		if endpoint.ID == id {
			removed = true
			continue
		}
		next = append(next, endpoint)
	}
	registry.Endpoints = next
	if err := s.writeEndpoints(registry); err != nil {
		return false, err
	}
	return removed, nil
}

func (s State) writeEndpoints(registry EndpointRegistry) error {
	if err := os.MkdirAll(filepath.Dir(s.EndpointsPath()), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal endpoints: %w", err)
	}
	return os.WriteFile(s.EndpointsPath(), data, 0o600)
}

func NormalizeEndpointRef(ref core.EndpointRef) core.EndpointRef {
	ref.ID = strings.TrimSpace(ref.ID)
	ref.URL = strings.TrimRight(strings.TrimSpace(ref.URL), "/")
	ref.Product = strings.TrimSpace(ref.Product)
	ref.Protocol = strings.TrimSpace(ref.Protocol)
	ref.Source = strings.TrimSpace(ref.Source)
	ref.CredentialRef = strings.TrimSpace(ref.CredentialRef)
	if ref.Protocol == "" {
		if parsed, err := url.Parse(ref.URL); err == nil {
			ref.Protocol = parsed.Scheme
		}
	}
	if ref.Source == "" {
		ref.Source = "manual"
	}
	if ref.ID == "" {
		ref.ID = endpointID(ref.Product, ref.URL)
	}
	return ref
}

func endpointID(product, rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	host := ""
	if err == nil {
		host = parsed.Host
	}
	id := strings.Trim(strings.ToLower(product+"-"+host), "-")
	if id == "" {
		id = strings.TrimSpace(rawURL)
	}
	replacer := strings.NewReplacer("://", "-", "/", "-", ":", "-", ".", "-", "_", "-")
	id = replacer.Replace(id)
	id = strings.Trim(id, "-")
	if id == "" {
		id = "endpoint"
	}
	return id
}

func firstEndpointNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
