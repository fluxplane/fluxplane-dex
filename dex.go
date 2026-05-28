// Package dex is the public library entrypoint to the dex engine. Construct
// an Engine via New and access functionality through the service accessors:
// Auth, Plugins, Operations, Datasources, Endpoints, Secrets, Index, Context.
package dex

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/internal/defaults"
	"github.com/fluxplane/fluxplane-dex/protocol"
	"github.com/fluxplane/fluxplane-dex/runtime"
)

// Config configures a new Engine. WorkDir overrides $DEX_HOME (default
// ~/.dex). MarketplacePath, if set, is loaded from disk; otherwise
// MarketplaceJSON is used; otherwise the bundled defaults.MarketplaceJSON.
type Config struct {
	WorkDir         string
	MarketplacePath string
	MarketplaceJSON []byte
	DevPlugins      map[string]string
	Timeout         time.Duration
	HostCommand     string
	Prompter        Prompter
	Events          EventSink
	Logger          *slog.Logger
}

// DefaultTimeout is applied when Config.Timeout is zero.
const DefaultTimeout = 2 * time.Minute

// Engine is the dex library handle. It is safe to share across goroutines —
// the underlying Runner and State are stateless except for filesystem-backed
// state under WorkDir.
type Engine struct {
	runner runtime.Runner
	cfg    Config

	auth        *AuthService
	plugins     *PluginService
	operations  *OperationService
	datasources *DatasourceService
	endpoints   *EndpointService
	secrets     *SecretService
	index       *IndexService
	contexts    *ContextService
}

// New constructs an Engine from cfg. The work directory is created if it
// doesn't exist. The marketplace is loaded with this precedence:
// MarketplacePath > MarketplaceJSON > bundled defaults.
func New(cfg Config) (*Engine, error) {
	marketplace, err := loadMarketplace(cfg)
	if err != nil {
		return nil, err
	}

	home := strings.TrimSpace(cfg.WorkDir)
	if home == "" {
		home = os.Getenv("DEX_HOME")
	}
	state, err := runtime.NewState(home)
	if err != nil {
		return nil, err
	}

	if cfg.Prompter == nil {
		cfg.Prompter = NoopPrompter{}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}

	runner := runtime.Runner{
		Marketplace: marketplace,
		State:       state,
		DevPlugins:  cfg.DevPlugins,
		WorkDir:     cfg.WorkDir,
		Timeout:     cfg.Timeout,
		HostCommand: cfg.HostCommand,
	}
	if cfg.Events != nil {
		sink := cfg.Events
		runner.EventSink = func(ctx context.Context, event runtime.PluginEvent) {
			sink(ctx, PluginEvent(event))
		}
	}

	return &Engine{runner: runner, cfg: cfg}, nil
}

// Close releases resources held by the engine. Currently a no-op; reserved
// for future use (long-running plugin connections, caches, etc).
func (e *Engine) Close() error {
	return nil
}

// Marketplace returns the resolved marketplace catalog.
func (e *Engine) Marketplace() runtime.Marketplace {
	return e.runner.Marketplace
}

// Runner returns the underlying runtime.Runner. Exposed for advanced
// use-cases (custom command invocation) and tests; library callers should
// prefer the service accessors.
func (e *Engine) Runner() runtime.Runner {
	return e.runner
}

// Prompter returns the configured Prompter (never nil — NoopPrompter when
// unset).
func (e *Engine) Prompter() Prompter {
	return e.cfg.Prompter
}

// Logger returns the configured logger (never nil).
func (e *Engine) Logger() *slog.Logger {
	return e.cfg.Logger
}

// Auth returns the auth management service.
func (e *Engine) Auth() *AuthService {
	if e.auth == nil {
		e.auth = &AuthService{engine: e}
	}
	return e.auth
}

// Plugins returns the plugin management service.
func (e *Engine) Plugins() *PluginService {
	if e.plugins == nil {
		e.plugins = &PluginService{engine: e}
	}
	return e.plugins
}

// Operations returns the operation invocation service.
func (e *Engine) Operations() *OperationService {
	if e.operations == nil {
		e.operations = &OperationService{engine: e}
	}
	return e.operations
}

// Datasources returns the datasource query service.
func (e *Engine) Datasources() *DatasourceService {
	if e.datasources == nil {
		e.datasources = &DatasourceService{engine: e}
	}
	return e.datasources
}

// Endpoints returns the endpoint management service.
func (e *Engine) Endpoints() *EndpointService {
	if e.endpoints == nil {
		e.endpoints = &EndpointService{engine: e}
	}
	return e.endpoints
}

// Secrets returns the secret management service.
func (e *Engine) Secrets() *SecretService {
	if e.secrets == nil {
		e.secrets = &SecretService{engine: e}
	}
	return e.secrets
}

// Index returns the index management service.
func (e *Engine) Index() *IndexService {
	if e.index == nil {
		e.index = &IndexService{engine: e}
	}
	return e.index
}

// Contexts returns the context-build service.
func (e *Engine) Contexts() *ContextService {
	if e.contexts == nil {
		e.contexts = &ContextService{engine: e}
	}
	return e.contexts
}

func loadMarketplace(cfg Config) (runtime.Marketplace, error) {
	if strings.TrimSpace(cfg.MarketplacePath) != "" {
		return runtime.LoadMarketplace(cfg.MarketplacePath)
	}
	data := cfg.MarketplaceJSON
	if len(data) == 0 {
		data = []byte(defaults.MarketplaceJSON)
	}
	return runtime.LoadMarketplaceData(data)
}

// Manifest fetches the manifest for a plugin by name (or alias). Plugins
// must be installed (or available via DevPlugins / local_path).
func (e *Engine) Manifest(ctx context.Context, plugin string) (core.PluginManifest, error) {
	resp, err := e.runner.InvokeInstance(ctx, plugin, runtime.DefaultInstance, protocol.CommandManifest, nil)
	if err != nil {
		return core.PluginManifest{}, err
	}
	return decodeManifest(resp)
}
