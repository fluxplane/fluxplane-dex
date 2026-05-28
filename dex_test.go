package dex_test

import (
	"context"
	"errors"
	"testing"

	dex "github.com/fluxplane/fluxplane-dex"
)

func newEngine(t *testing.T) *dex.Engine {
	t.Helper()
	e, err := dex.New(dex.Config{WorkDir: t.TempDir()})
	if err != nil {
		t.Fatalf("dex.New: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	return e
}

func TestNewLoadsBundledMarketplace(t *testing.T) {
	e := newEngine(t)
	plugins := e.Plugins().All(context.Background())
	if len(plugins) == 0 {
		t.Fatalf("expected bundled marketplace to have plugins")
	}
}

func TestPluginsSearchFiltersByQuery(t *testing.T) {
	e := newEngine(t)
	matches := e.Plugins().Search(context.Background(), "websearch")
	found := false
	for _, p := range matches {
		if p.Name == "websearch" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected to find 'websearch' in search results, got %v", matches)
	}
}

func TestManifestBuiltinPlugin(t *testing.T) {
	e := newEngine(t)
	manifest, err := e.Manifest(context.Background(), "websearch")
	if err != nil {
		t.Fatalf("Manifest(websearch): %v", err)
	}
	if manifest.Name != "websearch" {
		t.Fatalf("unexpected manifest name %q", manifest.Name)
	}
	if len(manifest.Operations) == 0 {
		t.Fatalf("expected websearch manifest to declare operations")
	}
}

func TestOperationsListBuiltin(t *testing.T) {
	e := newEngine(t)
	ops, err := e.Operations().List(context.Background(), "websearch")
	if err != nil {
		t.Fatalf("Operations().List: %v", err)
	}
	if len(ops) == 0 {
		t.Fatalf("expected at least one operation declared by websearch")
	}
}

func TestNoopPrompterReturnsErrNoPrompter(t *testing.T) {
	e := newEngine(t)
	p := e.Prompter()
	if _, err := p.Input(context.Background(), "label"); !errors.Is(err, dex.ErrNoPrompter) {
		t.Fatalf("expected ErrNoPrompter, got %v", err)
	}
	if _, err := p.Secret(context.Background(), "label"); !errors.Is(err, dex.ErrNoPrompter) {
		t.Fatalf("expected ErrNoPrompter, got %v", err)
	}
}

func TestUnknownPluginInstall(t *testing.T) {
	e := newEngine(t)
	_, err := e.Plugins().Install(context.Background(), "does-not-exist")
	if !errors.Is(err, dex.ErrPluginNotFound) {
		t.Fatalf("expected ErrPluginNotFound, got %v", err)
	}
}

func TestAuthMethodsBuiltin(t *testing.T) {
	e := newEngine(t)
	methods, err := e.Auth().Methods(context.Background(), "websearch")
	if err != nil {
		t.Fatalf("Auth().Methods: %v", err)
	}
	_ = methods // builtin may have zero auth methods; we just assert it doesn't error
}

type scriptedPrompter struct {
	inputs  []string
	secrets []string
	printed []string
}

func (p *scriptedPrompter) Confirm(context.Context, string) (bool, error) { return true, nil }
func (p *scriptedPrompter) Input(_ context.Context, _ string) (string, error) {
	if len(p.inputs) == 0 {
		return "", nil
	}
	v := p.inputs[0]
	p.inputs = p.inputs[1:]
	return v, nil
}
func (p *scriptedPrompter) Secret(_ context.Context, _ string) (string, error) {
	if len(p.secrets) == 0 {
		return "", nil
	}
	v := p.secrets[0]
	p.secrets = p.secrets[1:]
	return v, nil
}
func (p *scriptedPrompter) Print(_ context.Context, msg string) error {
	p.printed = append(p.printed, msg)
	return nil
}

func TestPrompterReceivesPrintFromConnect(t *testing.T) {
	prompter := &scriptedPrompter{}
	e, err := dex.New(dex.Config{WorkDir: t.TempDir(), Prompter: prompter})
	if err != nil {
		t.Fatalf("dex.New: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	// websearch declares no auth fields, so Connect should be a no-op but
	// still emit the "Connecting <plugin>/<instance>" print.
	if _, err := e.Auth().Connect(context.Background(), "websearch", dex.ConnectOptions{AllowPartial: true}); err != nil {
		t.Fatalf("Auth().Connect: %v", err)
	}
	if len(prompter.printed) == 0 {
		t.Fatalf("expected prompter to receive at least one Print call")
	}
}
