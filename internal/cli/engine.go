package cli

import (
	"context"
	"fmt"
	"strings"

	dex "github.com/fluxplane/fluxplane-dex"
	"github.com/fluxplane/fluxplane-dex/internal/defaults"
	"github.com/fluxplane/fluxplane-dex/runtime"
	"github.com/spf13/cobra"
)

// engine constructs a *dex.Engine bound to the given cobra command's IO
// streams. The CLI is the canonical consumer of the public dex package;
// this helper wires the terminal-backed Prompter and an event sink that
// writes plugin progress to errOut.
func (o *options) engine(cmd *cobra.Command) (*dex.Engine, error) {
	in := o.in
	out := o.out
	errOut := o.errOut
	if cmd != nil {
		in = cmd.InOrStdin()
		out = cmd.OutOrStdout()
		errOut = cmd.ErrOrStderr()
	}

	cfg := dex.Config{
		WorkDir:         o.home,
		MarketplacePath: o.marketplacePath,
		Prompter:        &TerminalPrompter{In: in, Out: out},
	}
	if strings.TrimSpace(o.marketplacePath) == "" {
		cfg.MarketplaceJSON = []byte(defaults.MarketplaceJSON)
	}
	devPlugins, err := parseDevPlugins(o.devPlugins)
	if err != nil {
		return nil, fmt.Errorf("parse --dev-plugin: %w", err)
	}
	cfg.DevPlugins = devPlugins
	if errOut != nil {
		sink := cliEventSink(errOut)
		cfg.Events = func(ctx context.Context, event dex.PluginEvent) {
			sink(ctx, runtime.PluginEvent(event))
		}
	}
	return dex.New(cfg)
}
