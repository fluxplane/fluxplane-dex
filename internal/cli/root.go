package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/internal/defaults"
	"github.com/fluxplane/fluxplane-dex/protocol"
	"github.com/fluxplane/fluxplane-dex/runtime"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type options struct {
	output          string
	home            string
	instance        string
	marketplacePath string
	devPlugins      []string
	workDir         string
	in              io.Reader
	out             io.Writer
	errOut          io.Writer
}

func Main(args []string) int {
	cmd := NewRootCommand()
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		return 1
	}
	return 0
}

func NewRootCommand() *cobra.Command {
	opts := &options{
		output: "text",
		in:     os.Stdin,
		out:    os.Stdout,
		errOut: os.Stderr,
	}
	if cwd, err := os.Getwd(); err == nil {
		opts.workDir = cwd
	}

	root := &cobra.Command{
		Use:   "dex",
		Short: "Plugin-backed engineering CLI",
		Long:  "dex is a plugin-backed CLI for fast, token-efficient access to external engineering systems.",
	}
	root.SetIn(opts.in)
	root.SetOut(opts.out)
	root.SetErr(opts.errOut)
	root.PersistentFlags().StringVarP(&opts.output, "output", "o", "text", "Output format: text, compact, json, yaml")
	root.PersistentFlags().StringVar(&opts.marketplacePath, "marketplace", "", "Marketplace index path")
	root.PersistentFlags().StringVar(&opts.home, "dex-home", "", "Dex home directory")
	root.PersistentFlags().StringVar(&opts.instance, "instance", "default", "Integration instance name")
	root.PersistentFlags().StringArrayVar(&opts.devPlugins, "dev-plugin", nil, "Development plugin override NAME=PATH")

	root.AddCommand(newPluginCommand(opts))
	root.AddCommand(newOpCommand(opts))
	root.AddCommand(newAuthCommand(opts))
	root.AddCommand(newSecretCommand(opts))
	root.AddCommand(newSearchCommand(opts))
	root.AddCommand(newLookupCommand(opts))
	root.AddCommand(newContextCommand(opts))
	root.AddCommand(newEndpointCommand(opts))
	root.AddCommand(newIndexCommand(opts))
	root.AddCommand(newGitLabCommand(opts))
	root.AddCommand(newSlackCommand(opts))
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "fluxplane-dex dev")
			return err
		},
	})
	return root
}

func newPluginCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "plugin", Short: "Manage dex plugins"}
	cmd.AddCommand(&cobra.Command{
		Use:   "marketplace",
		Short: "Show marketplace index",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			return renderValue(cmd.OutOrStdout(), opts.output, runner.Marketplace.Data())
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "ls",
		Short: "List installed and marketplace plugins",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			installed, err := runner.State.LoadInstalledPlugins()
			if err != nil {
				return err
			}
			return renderValue(cmd.OutOrStdout(), opts.output, map[string]any{"plugins": runner.Marketplace.Plugins(), "installed": installed.Plugins})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "search QUERY",
		Short: "Search marketplace plugins",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			query := strings.Join(args, " ")
			return renderValue(cmd.OutOrStdout(), opts.output, map[string]any{"query": query, "plugins": runtime.SearchPlugins(runner.Marketplace, query)})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "show NAME",
		Short: "Show plugin manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			resp, err := runner.Invoke(cmd.Context(), args[0], protocol.CommandManifest, nil)
			if err != nil {
				return err
			}
			return render(cmd.OutOrStdout(), opts.output, resp.Result)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "install NAME",
		Short: "Install a plugin from marketplace metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			if err := runner.Install(cmd.Context(), args[0]); err != nil {
				return err
			}
			return renderValue(cmd.OutOrStdout(), opts.output, map[string]any{"plugin": args[0], "installed": true})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "remove NAME",
		Short: "Remove a plugin from the local installed registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			removed, err := runner.State.RemoveInstalledPlugin(args[0])
			if err != nil {
				return err
			}
			return renderValue(cmd.OutOrStdout(), opts.output, map[string]any{"plugin": args[0], "removed": removed})
		},
	})
	return cmd
}

func newOpCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "op", Short: "List and run plugin operations"}
	cmd.AddCommand(&cobra.Command{
		Use:   "ls [PLUGIN]",
		Short: "List operations",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			if len(args) == 1 {
				resp, err := runner.Invoke(cmd.Context(), args[0], protocol.CommandOperationsList, nil)
				if err != nil {
					return err
				}
				return render(cmd.OutOrStdout(), opts.output, resp.Result)
			}
			return renderValue(cmd.OutOrStdout(), opts.output, fanout(cmd.Context(), runner, opts.instanceName(), protocol.CommandOperationsList, nil))
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "run NAME [JSON|-]",
		Short: "Run one operation",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			input, err := optionalJSON(cmd.InOrStdin(), args[1:])
			if err != nil {
				return err
			}
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			return callOperation(cmd.Context(), cmd.OutOrStdout(), opts.output, runner, opts.instanceName(), args[0], input)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "batch PLUGIN [JSON|-]",
		Short: "Run multiple operations for one plugin instance",
		Long:  `Run a JSON array of operation calls in one plugin invocation. Each call has id, name, and input fields.`,
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			input, err := optionalJSON(cmd.InOrStdin(), args[1:])
			if err != nil {
				return err
			}
			calls, err := batchCalls(input)
			if err != nil {
				return err
			}
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			result, err := runner.OperationBatch(cmd.Context(), args[0], opts.instanceName(), calls)
			if err != nil {
				return err
			}
			return renderValue(cmd.OutOrStdout(), opts.output, result)
		},
	})
	return cmd
}

func newAuthCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Manage plugin auth"}
	cmd.AddCommand(&cobra.Command{
		Use:   "info PLUGIN",
		Short: "List plugin auth methods",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			resp, err := runner.InvokeInstance(cmd.Context(), args[0], opts.instanceName(), protocol.CommandAuthMethods, nil)
			if err != nil {
				return err
			}
			return render(cmd.OutOrStdout(), opts.output, resp.Result)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "status PLUGIN",
		Short: "Show host-side secret readiness",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			purposes, err := authPurposeSpecs(cmd.Context(), runner, args[0])
			if err != nil {
				return err
			}
			status := runner.State.SecretStatus(args[0], opts.instanceName(), purposes)
			return renderValue(cmd.OutOrStdout(), opts.output, map[string]any{"plugin": args[0], "instance": opts.instanceName(), "status": status})
		},
	})
	connectOpts := struct {
		fields []string
		yes    bool
	}{}
	connect := &cobra.Command{
		Use:   "connect PLUGIN",
		Short: "Connect plugin auth for humans or agents",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			fields, err := authFields(cmd.Context(), runner, args[0])
			if err != nil {
				return err
			}
			values, err := parseConnectFields(connectOpts.fields)
			if err != nil {
				return err
			}
			if len(values) == 0 {
				values, err = promptAuthFields(cmd.InOrStdin(), cmd.OutOrStdout(), args[0], opts.instanceName(), fields)
				if err != nil {
					return err
				}
			}
			saved, missing, err := saveAuthValues(runner.State, args[0], opts.instanceName(), fields, values)
			if err != nil {
				return err
			}
			if len(missing) > 0 && !connectOpts.yes {
				return fmt.Errorf("missing required auth fields: %s", strings.Join(missing, ", "))
			}
			if saved > 0 {
				if err := markPluginAvailable(runner, args[0]); err != nil {
					return err
				}
			}
			return renderValue(cmd.OutOrStdout(), opts.output, map[string]any{"plugin": args[0], "instance": opts.instanceName(), "saved": saved, "missing": missing})
		},
	}
	connect.Flags().StringArrayVarP(&connectOpts.fields, "field", "f", nil, "Secret field as purpose=value")
	connect.Flags().BoolVarP(&connectOpts.yes, "yes", "y", false, "Allow saving partial optional auth values")
	connect.AddCommand(&cobra.Command{
		Use:   "auto [PLUGIN]",
		Short: "Connect auth by reading manifest-declared environment variables",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			if len(args) == 1 {
				plugin, ok := runner.Marketplace.Resolve(args[0])
				if !ok {
					return fmt.Errorf("unknown plugin %q", args[0])
				}
				result, err := autoConnectPlugin(cmd.Context(), runner, plugin.Name, opts.instanceName())
				if err != nil {
					return err
				}
				return renderValue(cmd.OutOrStdout(), opts.output, result)
			}
			results := make([]autoConnectResult, 0, len(runner.Marketplace.Plugins()))
			for _, plugin := range runner.Marketplace.Plugins() {
				result, err := autoConnectPlugin(cmd.Context(), runner, plugin.Name, opts.instanceName())
				if err != nil {
					results = append(results, autoConnectResult{Plugin: plugin.Name, Instance: opts.instanceName(), Error: err.Error()})
					continue
				}
				results = append(results, result)
			}
			return renderValue(cmd.OutOrStdout(), opts.output, map[string]any{"instance": opts.instanceName(), "plugins": results})
		},
	})
	cmd.AddCommand(connect)
	return cmd
}

func newSecretCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "secret", Short: "Broker scoped runtime secrets"}
	getOpts := struct {
		grant   string
		purpose string
	}{}
	get := &cobra.Command{
		Use:   "get PLUGIN",
		Short: "Get secret material for a valid grant",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			material, err := runner.State.ResolveSecret(cmd.Context(), args[0], opts.instanceName(), getOpts.purpose, getOpts.grant)
			if err != nil {
				return err
			}
			return renderValue(cmd.OutOrStdout(), opts.output, material)
		},
	}
	get.Flags().StringVar(&getOpts.grant, "grant", "", "Secret grant token")
	get.Flags().StringVar(&getOpts.purpose, "purpose", "", "Secret purpose")
	_ = get.MarkFlagRequired("grant")
	_ = get.MarkFlagRequired("purpose")
	cmd.AddCommand(get)
	return cmd
}

func newSearchCommand(opts *options) *cobra.Command {
	searchOpts := struct {
		plugin string
		entity string
		limit  int
	}{limit: 20}
	cmd := &cobra.Command{
		Use:   "search QUERY",
		Short: "Search all searchable plugins",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			query := strings.Join(args, " ")
			return renderValue(cmd.OutOrStdout(), opts.output, map[string]any{
				"query":   query,
				"results": fanoutSearch(cmd.Context(), runner, opts.instanceName(), map[string]any{"query": query, "entity": searchOpts.entity, "limit": searchOpts.limit}, searchOpts.plugin),
			})
		},
	}
	cmd.Flags().StringVar(&searchOpts.plugin, "plugin", "", "Search one plugin")
	cmd.Flags().StringVar(&searchOpts.entity, "entity", "", "Filter by entity type")
	cmd.Flags().IntVar(&searchOpts.limit, "limit", 20, "Maximum records per plugin")
	return cmd
}

func newLookupCommand(opts *options) *cobra.Command {
	lookupOpts := struct {
		plugin string
		entity string
		limit  int
	}{limit: 20}
	cmd := &cobra.Command{
		Use:   "lookup TEXT",
		Short: "Lookup canonical datasource references",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			text := strings.Join(args, " ")
			return renderValue(cmd.OutOrStdout(), opts.output, map[string]any{
				"text":    text,
				"results": fanoutLookup(cmd.Context(), runner, opts.instanceName(), map[string]any{"text": text, "entity": lookupOpts.entity, "limit": lookupOpts.limit}, lookupOpts.plugin),
			})
		},
	}
	cmd.Flags().StringVar(&lookupOpts.plugin, "plugin", "", "Lookup in one plugin")
	cmd.Flags().StringVar(&lookupOpts.entity, "entity", "", "Filter by entity type")
	cmd.Flags().IntVar(&lookupOpts.limit, "limit", 20, "Maximum matches per plugin")
	return cmd
}

func newContextCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "context QUERY",
		Short: "Build context from context-provider plugins",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			query := strings.Join(args, " ")
			return renderValue(cmd.OutOrStdout(), opts.output, map[string]any{
				"query":   query,
				"results": fanout(cmd.Context(), runner, opts.instanceName(), protocol.CommandContextBuild, map[string]any{"query": query}),
			})
		},
	}
}

func newEndpointCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "endpoint", Short: "Discover and inspect endpoints"}
	cmd.AddCommand(&cobra.Command{
		Use:   "discover [PRODUCT]",
		Short: "Discover endpoint candidates",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			product := ""
			if len(args) == 1 {
				product = args[0]
			}
			return renderValue(cmd.OutOrStdout(), opts.output, map[string]any{
				"product": product,
				"results": fanout(cmd.Context(), runner, opts.instanceName(), protocol.CommandEndpointsDiscover, map[string]any{"product": product}),
			})
		},
	})
	return cmd
}

func newIndexCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "index", Short: "Manage plugin indexes"}
	buildOpts := struct {
		index  string
		entity string
	}{}
	build := &cobra.Command{
		Use:   "build PLUGIN",
		Short: "Build plugin index",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			input := map[string]any{}
			if strings.TrimSpace(buildOpts.index) != "" {
				input["index"] = strings.TrimSpace(buildOpts.index)
			}
			if strings.TrimSpace(buildOpts.entity) != "" {
				input["entity"] = strings.TrimSpace(buildOpts.entity)
			}
			result, err := runner.BuildIndex(cmd.Context(), args[0], opts.instanceName(), input)
			if err != nil {
				return err
			}
			return renderValue(cmd.OutOrStdout(), opts.output, result)
		},
	}
	build.Flags().StringVar(&buildOpts.index, "index", "", "Index name to build")
	build.Flags().StringVar(&buildOpts.entity, "entity", "", "Entity type to build")
	cmd.AddCommand(build)
	cmd.AddCommand(&cobra.Command{
		Use:   "status PLUGIN",
		Short: "Show plugin index status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			plugin, ok := runner.Marketplace.Resolve(args[0])
			if !ok {
				return fmt.Errorf("unknown plugin %q", args[0])
			}
			status, err := runner.State.IndexStatus(plugin.Name, opts.instanceName())
			if err != nil {
				return err
			}
			return renderValue(cmd.OutOrStdout(), opts.output, status)
		},
	})
	return cmd
}

func newGitLabCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "gl", Aliases: []string{"gitlab"}, Short: "GitLab commands"}
	indexOpts := struct {
		index  string
		entity string
	}{}
	index := &cobra.Command{
		Use:   "index",
		Short: "Build GitLab index",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			input := map[string]any{}
			if strings.TrimSpace(indexOpts.index) != "" {
				input["index"] = strings.TrimSpace(indexOpts.index)
			}
			if strings.TrimSpace(indexOpts.entity) != "" {
				input["entity"] = strings.TrimSpace(indexOpts.entity)
			}
			result, err := runner.BuildIndex(cmd.Context(), "gitlab", opts.instanceName(), input)
			if err != nil {
				return err
			}
			return renderValue(cmd.OutOrStdout(), opts.output, result)
		},
	}
	index.Flags().StringVar(&indexOpts.index, "index", "", "Index name to build")
	index.Flags().StringVar(&indexOpts.entity, "entity", "", "Entity type to build")
	cmd.AddCommand(index)
	mr := &cobra.Command{Use: "mr", Short: "GitLab merge request commands"}
	mrListOpts := struct {
		project string
		state   string
		search  string
		limit   int
	}{state: "opened", limit: 20}
	mrList := &cobra.Command{
		Use:   "ls",
		Short: "List merge requests",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			return callOperation(cmd.Context(), cmd.OutOrStdout(), opts.output, runner, opts.instanceName(), "gitlab.mr.list", map[string]any{
				"project": mrListOpts.project,
				"state":   mrListOpts.state,
				"search":  mrListOpts.search,
				"limit":   mrListOpts.limit,
			})
		},
	}
	mrList.Flags().StringVar(&mrListOpts.project, "project", "", "Project ID or path")
	mrList.Flags().StringVar(&mrListOpts.state, "state", "opened", "Merge request state: opened, closed, merged, all")
	mrList.Flags().StringVar(&mrListOpts.search, "search", "", "Search merge requests")
	mrList.Flags().IntVar(&mrListOpts.limit, "limit", 20, "Maximum merge requests to return")
	mr.AddCommand(mrList)
	mr.AddCommand(&cobra.Command{
		Use:   "show PROJECT!IID",
		Short: "Show merge request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			return callOperation(cmd.Context(), cmd.OutOrStdout(), opts.output, runner, opts.instanceName(), "gitlab.mr.show", map[string]any{"ref": args[0]})
		},
	})
	cmd.AddCommand(mr)

	proj := &cobra.Command{Use: "proj", Aliases: []string{"project"}, Short: "GitLab project commands"}
	proj.AddCommand(&cobra.Command{
		Use:   "ls",
		Short: "List indexed projects",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			return callOperation(cmd.Context(), cmd.OutOrStdout(), opts.output, runner, opts.instanceName(), "gitlab.project.list", map[string]any{})
		},
	})
	proj.AddCommand(&cobra.Command{
		Use:   "show ID_OR_PATH",
		Short: "Show indexed project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			return callOperation(cmd.Context(), cmd.OutOrStdout(), opts.output, runner, opts.instanceName(), "gitlab.project.show", map[string]any{"id": args[0]})
		},
	})
	cmd.AddCommand(proj)
	return cmd
}

func newSlackCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "slack", Short: "Slack commands"}
	indexOpts := struct {
		index  string
		entity string
	}{}
	index := &cobra.Command{
		Use:   "index",
		Short: "Build Slack index",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			input := map[string]any{}
			if strings.TrimSpace(indexOpts.index) != "" {
				input["index"] = strings.TrimSpace(indexOpts.index)
			}
			if strings.TrimSpace(indexOpts.entity) != "" {
				input["entity"] = strings.TrimSpace(indexOpts.entity)
			}
			result, err := runner.BuildIndex(cmd.Context(), "slack", opts.instanceName(), input)
			if err != nil {
				return err
			}
			return renderValue(cmd.OutOrStdout(), opts.output, result)
		},
	}
	index.Flags().StringVar(&indexOpts.index, "index", "", "Index name to build")
	index.Flags().StringVar(&indexOpts.entity, "entity", "", "Entity type to build")
	cmd.AddCommand(index)
	cmd.AddCommand(&cobra.Command{
		Use:   "send CHANNEL TEXT",
		Short: "Send Slack message",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			return callOperation(cmd.Context(), cmd.OutOrStdout(), opts.output, runner, opts.instanceName(), "slack.message.send", map[string]any{"channel": args[0], "text": strings.Join(args[1:], " ")})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "search QUERY",
		Short: "Search Slack",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			return callOperation(cmd.Context(), cmd.OutOrStdout(), opts.output, runner, opts.instanceName(), "slack.search", map[string]any{"query": strings.Join(args, " ")})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "thread CHANNEL TS",
		Short: "Show Slack thread",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			return callOperation(cmd.Context(), cmd.OutOrStdout(), opts.output, runner, opts.instanceName(), "slack.thread", map[string]any{"channel": args[0], "ts": args[1]})
		},
	})
	return cmd
}

func (o *options) runner() (runtime.Runner, error) {
	var marketplace runtime.Marketplace
	var err error
	if strings.TrimSpace(o.marketplacePath) != "" {
		marketplace, err = runtime.LoadMarketplace(o.marketplacePath)
		if err != nil {
			return runtime.Runner{}, err
		}
	} else {
		marketplace, err = runtime.LoadMarketplaceData([]byte(defaults.MarketplaceJSON))
		if err != nil {
			return runtime.Runner{}, err
		}
	}
	home := o.home
	if strings.TrimSpace(home) == "" {
		home = os.Getenv("DEX_HOME")
	}
	state, err := runtime.NewState(home)
	if err != nil {
		return runtime.Runner{}, err
	}
	devPlugins, err := parseDevPlugins(o.devPlugins)
	if err != nil {
		return runtime.Runner{}, err
	}
	return runtime.Runner{Marketplace: marketplace, State: state, DevPlugins: devPlugins, WorkDir: o.workDir, Timeout: 2 * time.Minute}, nil
}

func (o *options) instanceName() string {
	return runtime.NormalizeInstance(o.instance)
}

func parseDevPlugins(values []string) (map[string]string, error) {
	out := map[string]string{}
	for _, value := range values {
		name, path, ok := strings.Cut(value, "=")
		if !ok || strings.TrimSpace(name) == "" || strings.TrimSpace(path) == "" {
			return nil, fmt.Errorf("--dev-plugin must be NAME=PATH")
		}
		out[strings.TrimSpace(name)] = strings.TrimSpace(path)
	}
	return out, nil
}

func fanout(ctx context.Context, runner runtime.Runner, instance, command string, payload any) map[string]any {
	instance = runtime.NormalizeInstance(instance)
	results := map[string]any{}
	plugins := runner.Marketplace.Plugins()
	if command == protocol.CommandDatasourcesSearch || command == protocol.CommandDatasourcesLookup {
		var err error
		capability := "search"
		if command == protocol.CommandDatasourcesLookup {
			capability = "lookup"
		}
		plugins, err = datasourceCapableAvailablePlugins(ctx, runner, instance, capability)
		if err != nil {
			return map[string]any{"error": err.Error()}
		}
	}
	for _, plugin := range plugins {
		resp, err := runner.InvokeInstance(ctx, plugin.Name, instance, command, payload)
		if err != nil {
			results[plugin.Name] = map[string]any{"error": err.Error()}
			continue
		}
		var value any
		_ = json.Unmarshal(resp.Result, &value)
		results[plugin.Name] = value
	}
	return results
}

func fanoutSearch(ctx context.Context, runner runtime.Runner, instance string, payload any, pluginFilter string) map[string]any {
	instance = runtime.NormalizeInstance(instance)
	if strings.TrimSpace(pluginFilter) != "" {
		plugin, ok := runner.Marketplace.Resolve(pluginFilter)
		if !ok {
			return map[string]any{"error": "unknown plugin " + pluginFilter}
		}
		resp, err := runner.InvokeInstance(ctx, plugin.Name, instance, protocol.CommandDatasourcesSearch, payload)
		if err != nil {
			return map[string]any{plugin.Name: map[string]any{"error": err.Error()}}
		}
		var value any
		_ = json.Unmarshal(resp.Result, &value)
		return map[string]any{plugin.Name: value}
	}
	return fanout(ctx, runner, instance, protocol.CommandDatasourcesSearch, payload)
}

func fanoutLookup(ctx context.Context, runner runtime.Runner, instance string, payload any, pluginFilter string) map[string]any {
	instance = runtime.NormalizeInstance(instance)
	if strings.TrimSpace(pluginFilter) != "" {
		plugin, ok := runner.Marketplace.Resolve(pluginFilter)
		if !ok {
			return map[string]any{"error": "unknown plugin " + pluginFilter}
		}
		resp, err := runner.InvokeInstance(ctx, plugin.Name, instance, protocol.CommandDatasourcesLookup, payload)
		if err != nil {
			return map[string]any{plugin.Name: map[string]any{"error": err.Error()}}
		}
		var value any
		_ = json.Unmarshal(resp.Result, &value)
		return map[string]any{plugin.Name: value}
	}
	return fanout(ctx, runner, instance, protocol.CommandDatasourcesLookup, payload)
}

func searchableAvailablePlugins(ctx context.Context, runner runtime.Runner) ([]core.PluginEntry, error) {
	return datasourceCapableAvailablePlugins(ctx, runner, runtime.DefaultInstance, "search")
}

func datasourceCapableAvailablePlugins(ctx context.Context, runner runtime.Runner, instance, capability string) ([]core.PluginEntry, error) {
	var out []core.PluginEntry
	instance = runtime.NormalizeInstance(instance)
	for _, plugin := range runner.Marketplace.Plugins() {
		available, err := pluginAvailable(runner, plugin.Name, instance)
		if err != nil {
			return nil, err
		}
		if !available {
			continue
		}
		manifest, err := pluginManifest(ctx, runner, plugin.Name)
		if err != nil {
			continue
		}
		if manifestHasDatasourceCapability(manifest, capability) {
			out = append(out, plugin)
		}
	}
	return out, nil
}

func pluginAvailable(runner runtime.Runner, plugin, instance string) (bool, error) {
	installed, err := runner.State.IsPluginInstalled(plugin)
	if err != nil {
		return false, err
	}
	if installed {
		return true, nil
	}
	connected, err := runner.State.HasStoredAuth(plugin, instance)
	if err != nil {
		return false, err
	}
	if connected {
		return true, nil
	}
	return runner.State.HasIndexRecords(plugin, instance)
}

func pluginManifest(ctx context.Context, runner runtime.Runner, plugin string) (core.PluginManifest, error) {
	resp, err := runner.InvokeInstance(ctx, plugin, runtime.DefaultInstance, protocol.CommandManifest, nil)
	if err != nil {
		return core.PluginManifest{}, err
	}
	var manifest core.PluginManifest
	if err := json.Unmarshal(resp.Result, &manifest); err != nil {
		return core.PluginManifest{}, err
	}
	return manifest, nil
}

func manifestHasDatasourceCapability(manifest core.PluginManifest, capability string) bool {
	for _, datasource := range manifest.Datasources {
		for _, candidate := range datasource.Capabilities {
			if candidate == capability {
				return true
			}
		}
	}
	return false
}

func invokeAndRender(cmd *cobra.Command, opts *options, plugin, command string, payload any) error {
	runner, err := opts.runner()
	if err != nil {
		return err
	}
	resp, err := runner.InvokeInstance(cmd.Context(), plugin, opts.instanceName(), command, payload)
	if err != nil {
		return err
	}
	return render(cmd.OutOrStdout(), opts.output, resp.Result)
}

func callOperation(ctx context.Context, out io.Writer, output string, runner runtime.Runner, instance, name string, input any) error {
	plugin, _, ok := strings.Cut(name, ".")
	if !ok || strings.TrimSpace(plugin) == "" {
		return fmt.Errorf("operation name must start with plugin prefix, got %q", name)
	}
	inputRaw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	resp, err := runner.InvokeInstance(ctx, plugin, instance, protocol.CommandOperationsCall, protocol.OperationCall{Name: name, Input: inputRaw})
	if err != nil {
		return err
	}
	return render(out, output, resp.Result)
}

func authFields(ctx context.Context, runner runtime.Runner, plugin string) ([]core.AuthField, error) {
	resp, err := runner.InvokeInstance(ctx, plugin, runtime.DefaultInstance, protocol.CommandManifest, nil)
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Auth []struct {
			Env    []string         `json:"env"`
			Fields []core.AuthField `json:"fields"`
		} `json:"auth"`
	}
	if err := json.Unmarshal(resp.Result, &manifest); err != nil {
		return nil, err
	}
	var fields []core.AuthField
	for _, method := range manifest.Auth {
		for _, field := range method.Fields {
			if len(field.Env) == 0 {
				field.Env = append(field.Env, method.Env...)
			}
			fields = append(fields, field)
		}
	}
	return fields, nil
}

type autoConnectResult struct {
	Plugin   string   `json:"plugin"`
	Instance string   `json:"instance"`
	Saved    []string `json:"saved,omitempty"`
	Missing  []string `json:"missing,omitempty"`
	Skipped  []string `json:"skipped,omitempty"`
	Error    string   `json:"error,omitempty"`
}

func autoConnectPlugin(ctx context.Context, runner runtime.Runner, plugin, instance string) (autoConnectResult, error) {
	fields, err := authFields(ctx, runner, plugin)
	if err != nil {
		return autoConnectResult{}, err
	}
	result := autoConnectResult{Plugin: plugin, Instance: runtime.NormalizeInstance(instance)}
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
			if err := runner.State.SaveSecret(plugin, instance, name, runtime.StoredSecret{Kind: kind, Value: value}); err != nil {
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
		if err := markPluginAvailable(runner, plugin); err != nil {
			return result, err
		}
	}
	return result, nil
}

func markPluginAvailable(runner runtime.Runner, pluginName string) error {
	entry, ok := runner.Marketplace.Resolve(pluginName)
	if !ok {
		return fmt.Errorf("unknown plugin %q", pluginName)
	}
	return runner.State.MarkPluginInstalled(entry, false)
}

func dedupeAuthFields(fields []core.AuthField) []core.AuthField {
	seen := map[string]bool{}
	var out []core.AuthField
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

func authPurposeSpecs(ctx context.Context, runner runtime.Runner, plugin string) ([]runtime.SecretPurpose, error) {
	fields, err := authFields(ctx, runner, plugin)
	if err != nil {
		return nil, err
	}
	envByField, err := authFieldEnv(ctx, runner, plugin)
	if err != nil {
		return nil, err
	}
	var purposes []runtime.SecretPurpose
	for _, field := range fields {
		purposes = append(purposes, runtime.SecretPurpose{Name: field.Name, Env: envByField[field.Name]})
	}
	return purposes, nil
}

func authFieldEnv(ctx context.Context, runner runtime.Runner, plugin string) (map[string][]string, error) {
	resp, err := runner.InvokeInstance(ctx, plugin, runtime.DefaultInstance, protocol.CommandManifest, nil)
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Auth []struct {
			Env    []string         `json:"env"`
			Fields []core.AuthField `json:"fields"`
		} `json:"auth"`
	}
	if err := json.Unmarshal(resp.Result, &manifest); err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, method := range manifest.Auth {
		for _, field := range method.Fields {
			if len(field.Env) > 0 {
				out[field.Name] = append(out[field.Name], field.Env...)
			} else {
				out[field.Name] = append(out[field.Name], method.Env...)
			}
		}
	}
	return out, nil
}

func parseConnectFields(raw []string) (map[string]string, error) {
	values := map[string]string{}
	for _, field := range raw {
		purpose, value, ok := strings.Cut(field, "=")
		if !ok || strings.TrimSpace(purpose) == "" || strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("--field must be purpose=value")
		}
		values[strings.TrimSpace(purpose)] = strings.TrimSpace(value)
	}
	return values, nil
}

func promptAuthFields(in io.Reader, out io.Writer, plugin, instance string, fields []core.AuthField) (map[string]string, error) {
	values := map[string]string{}
	reader := bufio.NewReader(in)
	_, _ = fmt.Fprintf(out, "Connecting %s/%s\n", plugin, instance)
	for _, field := range fields {
		if strings.TrimSpace(field.Name) == "" {
			continue
		}
		label := field.Name
		if field.Description != "" {
			label += " (" + field.Description + ")"
		}
		_, _ = fmt.Fprintf(out, "%s: ", label)
		var value string
		var err error
		if field.Sensitive && stdinIsTerminal(in) {
			var data []byte
			data, err = term.ReadPassword(int(syscall.Stdin))
			_, _ = fmt.Fprintln(out)
			value = string(data)
		} else {
			value, err = reader.ReadString('\n')
		}
		if err != nil {
			return nil, err
		}
		value = strings.TrimSpace(value)
		if value != "" {
			values[field.Name] = value
		}
	}
	return values, nil
}

func saveAuthValues(state runtime.State, plugin, instance string, fields []core.AuthField, values map[string]string) (int, []string, error) {
	declared := map[string]core.AuthField{}
	for _, field := range fields {
		declared[field.Name] = field
	}
	saved := 0
	for purpose, value := range values {
		field := declared[purpose]
		if field.Name == "" {
			field = core.AuthField{Name: purpose, Sensitive: true, Secret: true}
		}
		kind := "bearer_token"
		if !field.Sensitive && !field.Secret {
			kind = "config"
		}
		if err := state.SaveSecret(plugin, instance, purpose, runtime.StoredSecret{Kind: kind, Value: value}); err != nil {
			return saved, nil, err
		}
		saved++
	}
	var missing []string
	for _, field := range fields {
		if field.Required && strings.TrimSpace(values[field.Name]) == "" {
			missing = append(missing, field.Name)
		}
	}
	return saved, missing, nil
}

func stdinIsTerminal(in io.Reader) bool {
	file, ok := in.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

func batchCalls(input any) ([]protocol.OperationCall, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var calls []protocol.OperationCall
	if err := json.Unmarshal(data, &calls); err == nil && len(calls) > 0 {
		for i := range calls {
			if calls[i].ID == "" {
				calls[i].ID = fmt.Sprintf("%d", i+1)
			}
			if calls[i].Name == "" {
				return nil, fmt.Errorf("batch call %d is missing name", i+1)
			}
		}
		return calls, nil
	}
	var wrapped protocol.OperationBatch
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil, fmt.Errorf("batch input must be an array of calls or {calls:[...]}")
	}
	return wrapped.Calls, nil
}

func optionalJSON(in io.Reader, args []string) (any, error) {
	if len(args) == 0 {
		return map[string]any{}, nil
	}
	raw := strings.TrimSpace(args[0])
	if raw == "-" {
		data, err := io.ReadAll(bufio.NewReader(in))
		if err != nil {
			return nil, err
		}
		raw = strings.TrimSpace(string(data))
	}
	if raw == "" {
		return map[string]any{}, nil
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, fmt.Errorf("input must be JSON: %w", err)
	}
	return value, nil
}
