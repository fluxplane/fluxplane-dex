package runtime

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func TestEndpointRegistryStoresAndFiltersEndpoints(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	record, err := state.SaveEndpoint(core.EndpointRef{URL: "http://prometheus.monitoring.svc:9090", Product: "prometheus", Source: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if record.ID == "" || record.Protocol != "http" {
		t.Fatalf("record = %#v", record)
	}
	endpoints, err := state.ListEndpoints("prometheus")
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) != 1 || endpoints[0].ID != record.ID {
		t.Fatalf("endpoints = %#v", endpoints)
	}
	_, ok, err := state.GetEndpoint(record.ID)
	if err != nil || !ok {
		t.Fatalf("get ok=%v err=%v", ok, err)
	}
	removed, err := state.RemoveEndpoint(record.ID)
	if err != nil || !removed {
		t.Fatalf("remove=%v err=%v", removed, err)
	}
}

func TestEndpointRegistryStoresHealthAndPreservesOnUpdate(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveEndpoint(core.EndpointRef{ID: "dev", URL: "kubernetes://context/dev", Product: "kubernetes"}); err != nil {
		t.Fatal(err)
	}
	checkedAt := time.Date(2026, 5, 28, 1, 2, 3, 0, time.UTC)
	updated, err := state.SaveEndpointHealth("dev", EndpointHealth{OK: true, CheckedAt: checkedAt, Method: "kubernetes.cluster.test", DurationMS: 42, Details: json.RawMessage(`{"server_version":"v1.30.0"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if updated.LastHealth == nil || !updated.LastHealth.OK || updated.LastHealth.Method != "kubernetes.cluster.test" {
		t.Fatalf("updated = %#v", updated)
	}
	if _, err := state.SaveEndpoint(core.EndpointRef{ID: "dev", URL: "kubernetes://context/dev", Product: "kubernetes", Source: "test"}); err != nil {
		t.Fatal(err)
	}
	record, ok, err := state.GetEndpoint("dev")
	if err != nil || !ok {
		t.Fatalf("get ok=%v err=%v", ok, err)
	}
	var details map[string]string
	if record.LastHealth != nil {
		_ = json.Unmarshal(record.LastHealth.Details, &details)
	}
	if record.LastHealth == nil || record.LastHealth.DurationMS != 42 || details["server_version"] != "v1.30.0" {
		t.Fatalf("record = %#v", record)
	}
}

func TestRunnerResolvesEndpointRefIntoOperationInput(t *testing.T) {
	state, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.SaveEndpoint(core.EndpointRef{
		ID:            "mysql-dev",
		URL:           "mysql://127.0.0.1:3306/app",
		Product:       "mysql",
		CredentialRef: "kubernetes://apps/secrets/mysql",
	}); err != nil {
		t.Fatal(err)
	}
	runner := Runner{State: state}
	call := protocol.OperationCall{Name: "sql.query", Input: json.RawMessage(`{"endpoint_ref":"mysql-dev","query":"select 1"}`)}
	changed, err := runner.resolveOperationEndpointRef(&call)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected endpoint_ref to be resolved")
	}
	var input map[string]any
	if err := json.Unmarshal(call.Input, &input); err != nil {
		t.Fatal(err)
	}
	if input["url"] != "mysql://127.0.0.1:3306/app" || input["credential_ref"] != "kubernetes://apps/secrets/mysql" || input["endpoint_product"] != "mysql" {
		t.Fatalf("input = %#v", input)
	}
}
