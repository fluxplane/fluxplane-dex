package fluxplaneplugin_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-core/core/resource"
	"github.com/fluxplane/fluxplane-core/orchestration/pluginhost"

	"github.com/fluxplane/fluxplane-dex/fluxplaneplugin"
)

func TestBundlesEmitsOnePerMarketplaceEntry(t *testing.T) {
	e := newEngine(t)
	bundles, err := fluxplaneplugin.Bundles(context.Background(), e)
	if err != nil {
		t.Fatalf("Bundles: %v", err)
	}
	if len(bundles) == 0 {
		t.Fatalf("expected at least one bundle")
	}
	seen := map[string]bool{}
	intentBundles := 0
	for _, b := range bundles {
		if b.Source.ID == "dex-intent" {
			intentBundles++
			if len(b.Reactions) == 0 {
				t.Fatalf("intent bundle has no reactions")
			}
			continue
		}
		if len(b.Plugins) == 0 {
			t.Fatalf("bundle %q missing PluginRef", b.Source.ID)
		}
		if b.Source.ID == "" {
			t.Fatalf("bundle missing source id")
		}
		if !strings.HasPrefix(b.Source.ID, "dex-plugin:") {
			t.Fatalf("bundle source %q not tagged as dex-plugin", b.Source.ID)
		}
		if b.Source.Ecosystem != "dex" {
			t.Fatalf("bundle %q has ecosystem %q, want dex", b.Source.ID, b.Source.Ecosystem)
		}
		name := b.Plugins[0].Name
		if seen[name] {
			t.Fatalf("duplicate bundle for plugin %q", name)
		}
		seen[name] = true
	}
	if intentBundles != 1 {
		t.Fatalf("expected exactly one intent bundle, got %d", intentBundles)
	}
}

// Plugins whose manifest can't be fetched (binary not installed in the
// ephemeral test DEX_HOME) should still emit a discoverable stub with the
// PluginRef and a warning diagnostic.
func TestBundlesStubsUninstalledPlugins(t *testing.T) {
	e := newEngine(t)
	bundles, err := fluxplaneplugin.Bundles(context.Background(), e)
	if err != nil {
		t.Fatalf("Bundles: %v", err)
	}
	var stubs int
	for _, b := range bundles {
		if b.Source.ID == "dex-intent" {
			continue
		}
		// Builtin plugins (websearch, vision) have manifests; everything else
		// in a fresh DEX_HOME stubs.
		if len(b.Operations) == 0 && len(b.OperationSets) == 0 {
			if len(b.Diagnostics) == 0 {
				t.Fatalf("stub bundle for %q has no diagnostic", b.Plugins[0].Name)
			}
			if b.Diagnostics[0].Severity != resource.SeverityWarning {
				t.Fatalf("stub diagnostic severity %q, want warning", b.Diagnostics[0].Severity)
			}
			stubs++
		}
	}
	if stubs == 0 {
		t.Fatalf("expected at least one stubbed plugin in fresh DEX_HOME")
	}
}

func TestRegisterPushesAllPluginsIntoHost(t *testing.T) {
	e := newEngine(t)
	host, err := pluginhost.New()
	if err != nil {
		t.Fatalf("pluginhost.New: %v", err)
	}
	if err := fluxplaneplugin.Register(e, host); err != nil {
		t.Fatalf("Register: %v", err)
	}
	// websearch is a builtin so we can resolve it cleanly without external
	// binaries to verify the host is actually wired.
	res, err := host.Resolve(context.Background(), resource.PluginRef{Name: "websearch"})
	if err != nil {
		t.Fatalf("Host.Resolve: %v", err)
	}
	if len(res.Bundles) == 0 {
		t.Fatalf("expected resolved bundles")
	}
	if len(res.Operations) == 0 {
		t.Fatalf("expected resolved executable operations")
	}
}

func TestRegisterRejectsDuplicates(t *testing.T) {
	e := newEngine(t)
	host, err := pluginhost.New()
	if err != nil {
		t.Fatalf("pluginhost.New: %v", err)
	}
	if err := fluxplaneplugin.Register(e, host); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	if err := fluxplaneplugin.Register(e, host); err == nil {
		t.Fatalf("expected error registering twice")
	}
}

func TestRegisterRejectsNilArgs(t *testing.T) {
	if err := fluxplaneplugin.Register(nil, nil); err == nil {
		t.Fatalf("expected error for nil engine + host")
	}
	host, _ := pluginhost.New()
	if err := fluxplaneplugin.Register(nil, host); err == nil {
		t.Fatalf("expected error for nil engine")
	}
	e := newEngine(t)
	if err := fluxplaneplugin.Register(e, nil); err == nil {
		t.Fatalf("expected error for nil host")
	}
}

func TestBundlesNilEngineErrors(t *testing.T) {
	if _, err := fluxplaneplugin.Bundles(context.Background(), nil); err == nil {
		t.Fatalf("expected error for nil engine")
	}
}
