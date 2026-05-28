package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
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
	root.AddCommand(newShortcutCommand(opts))
	root.AddCommand(newSearchCommand(opts))
	root.AddCommand(newLookupCommand(opts))
	root.AddCommand(newContextCommand(opts))
	root.AddCommand(newEndpointCommand(opts))
	root.AddCommand(newIndexCommand(opts))
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

type shortcutView struct {
	Plugin      string         `json:"plugin"`
	Use         string         `json:"use"`
	Description string         `json:"description,omitempty"`
	Target      string         `json:"target"`
	Operation   string         `json:"operation,omitempty"`
	Datasource  string         `json:"datasource,omitempty"`
	Capability  string         `json:"capability,omitempty"`
	Entity      string         `json:"entity,omitempty"`
	Defaults    map[string]any `json:"defaults,omitempty"`
}

type endpointCandidateView struct {
	Index int `json:"index"`
	core.EndpointCandidate
}

type endpointDiscoveryPluginView struct {
	Candidates []endpointCandidateView `json:"candidates,omitempty"`
	Error      string                  `json:"error,omitempty"`
}

type endpointDiscoveryView struct {
	Product    string                                 `json:"product,omitempty"`
	Plugin     string                                 `json:"plugin,omitempty"`
	Candidates []endpointCandidateView                `json:"candidates,omitempty"`
	Results    map[string]endpointDiscoveryPluginView `json:"results,omitempty"`
	Saved      []runtime.EndpointRecord               `json:"saved,omitempty"`
}

func newShortcutCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "shortcut", Short: "Inspect CLI shortcut bindings"}
	cmd.AddCommand(&cobra.Command{
		Use:   "ls [PLUGIN]",
		Short: "List shortcut bindings",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			var plugin string
			if len(args) == 1 {
				entry, ok := runner.Marketplace.Resolve(args[0])
				if !ok {
					return fmt.Errorf("unknown plugin %q", args[0])
				}
				plugin = entry.Name
			}
			return renderValue(cmd.OutOrStdout(), opts.output, map[string]any{"shortcuts": shortcutViews(runner.Marketplace, plugin)})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "show USE",
		Short: "Show one shortcut binding",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			use := strings.Join(args, " ")
			for _, shortcut := range shortcutViews(runner.Marketplace, "") {
				if shortcut.Use == use {
					return renderValue(cmd.OutOrStdout(), opts.output, shortcut)
				}
			}
			return fmt.Errorf("unknown shortcut %q", use)
		},
	})
	return cmd
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
		Short: "List marketplace, installed, and activated plugins",
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
			statuses, err := runner.State.PluginStatuses(runner.Marketplace)
			if err != nil {
				return err
			}
			return renderValue(cmd.OutOrStdout(), opts.output, map[string]any{"plugins": runner.Marketplace.Plugins(), "installed": installed.Plugins, "status": statuses})
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
		Use:   "status NAME",
		Short: "Show plugin installation and activation status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			entry, ok := runner.Marketplace.Resolve(args[0])
			if !ok {
				return fmt.Errorf("unknown plugin %q", args[0])
			}
			status, err := runner.State.PluginStatus(entry)
			if err != nil {
				return err
			}
			return renderValue(cmd.OutOrStdout(), opts.output, status)
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
		Use:   "activate NAME",
		Short: "Activate an installed or builtin plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			entry, ok := runner.Marketplace.Resolve(args[0])
			if !ok {
				return fmt.Errorf("unknown plugin %q", args[0])
			}
			if err := runner.State.ActivatePlugin(entry); err != nil {
				return err
			}
			status, err := runner.State.PluginStatus(entry)
			if err != nil {
				return err
			}
			return renderValue(cmd.OutOrStdout(), opts.output, status)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "deactivate NAME",
		Short: "Deactivate a plugin without removing its install record",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			entry, ok := runner.Marketplace.Resolve(args[0])
			if !ok {
				return fmt.Errorf("unknown plugin %q", args[0])
			}
			changed, err := runner.State.DeactivatePlugin(entry.Name)
			if err != nil {
				return err
			}
			status, err := runner.State.PluginStatus(entry)
			if err != nil {
				return err
			}
			return renderValue(cmd.OutOrStdout(), opts.output, map[string]any{"changed": changed, "status": status})
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
		Use:   "show NAME",
		Short: "Show one operation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			operation, err := findOperationSpec(cmd.Context(), runner, args[0])
			if err != nil {
				return err
			}
			return renderValue(cmd.OutOrStdout(), opts.output, operation)
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
		Use:   "ls [PRODUCT]",
		Short: "List registered endpoints",
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
			endpoints, err := runner.State.ListEndpoints(product)
			if err != nil {
				return err
			}
			return renderValue(cmd.OutOrStdout(), opts.output, map[string]any{"product": product, "endpoints": endpoints})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "show ID",
		Short: "Show a registered endpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			endpoint, ok, err := runner.State.GetEndpoint(args[0])
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("unknown endpoint %q", args[0])
			}
			return renderValue(cmd.OutOrStdout(), opts.output, endpoint)
		},
	})
	addOpts := struct {
		id          string
		product     string
		protocol    string
		source      string
		credential  string
		labels      []string
		annotations []string
	}{}
	add := &cobra.Command{
		Use:   "add URL",
		Short: "Register an endpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			labels, err := parseStringMapFlags(addOpts.labels)
			if err != nil {
				return err
			}
			annotations, err := parseStringMapFlags(addOpts.annotations)
			if err != nil {
				return err
			}
			record, err := runner.State.SaveEndpoint(core.EndpointRef{
				ID:            addOpts.id,
				URL:           args[0],
				Product:       addOpts.product,
				Protocol:      addOpts.protocol,
				Source:        addOpts.source,
				CredentialRef: addOpts.credential,
				Labels:        labels,
				Annotations:   annotations,
			})
			if err != nil {
				return err
			}
			return renderValue(cmd.OutOrStdout(), opts.output, record)
		},
	}
	add.Flags().StringVar(&addOpts.id, "id", "", "Endpoint ID")
	add.Flags().StringVar(&addOpts.product, "product", "", "Product name")
	add.Flags().StringVar(&addOpts.protocol, "protocol", "", "Endpoint protocol")
	add.Flags().StringVar(&addOpts.source, "source", "manual", "Endpoint source")
	add.Flags().StringVar(&addOpts.credential, "credential-ref", "", "Credential reference")
	add.Flags().StringArrayVar(&addOpts.labels, "label", nil, "Endpoint label key=value")
	add.Flags().StringArrayVar(&addOpts.annotations, "annotation", nil, "Endpoint annotation key=value")
	cmd.AddCommand(add)
	cmd.AddCommand(&cobra.Command{
		Use:   "remove ID",
		Short: "Remove a registered endpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			removed, err := runner.State.RemoveEndpoint(args[0])
			if err != nil {
				return err
			}
			return renderValue(cmd.OutOrStdout(), opts.output, map[string]any{"id": args[0], "removed": removed})
		},
	})
	importOpts := struct {
		from        string
		candidate   int
		id          string
		source      string
		labels      []string
		annotations []string
	}{}
	importCmd := &cobra.Command{
		Use:   "import [JSON|-]",
		Short: "Import a discovered endpoint candidate",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := opts.runner()
			if err != nil {
				return err
			}
			raw, err := endpointImportInput(cmd.InOrStdin(), importOpts.from, args)
			if err != nil {
				return err
			}
			candidate, err := endpointCandidateFromImport(raw, importOpts.candidate)
			if err != nil {
				return err
			}
			if strings.TrimSpace(importOpts.id) != "" {
				candidate.ID = strings.TrimSpace(importOpts.id)
			}
			if strings.TrimSpace(importOpts.source) != "" {
				candidate.Source = strings.TrimSpace(importOpts.source)
			}
			labels, err := parseStringMapFlags(importOpts.labels)
			if err != nil {
				return err
			}
			annotations, err := parseStringMapFlags(importOpts.annotations)
			if err != nil {
				return err
			}
			candidate.Labels = mergeStringMaps(candidate.Labels, labels)
			candidate.Annotations = mergeStringMaps(candidate.Annotations, annotations)
			record, err := runner.State.SaveEndpointCandidate(candidate)
			if err != nil {
				return err
			}
			return renderValue(cmd.OutOrStdout(), opts.output, record)
		},
	}
	importCmd.Flags().StringVar(&importOpts.from, "from", "", "Read candidate JSON from file")
	importCmd.Flags().IntVar(&importOpts.candidate, "candidate", 0, "Candidate index to import from discovery output")
	importCmd.Flags().StringVar(&importOpts.id, "id", "", "Endpoint ID override")
	importCmd.Flags().StringVar(&importOpts.source, "source", "", "Endpoint source override")
	importCmd.Flags().StringArrayVar(&importOpts.labels, "label", nil, "Endpoint label key=value")
	importCmd.Flags().StringArrayVar(&importOpts.annotations, "annotation", nil, "Endpoint annotation key=value")
	cmd.AddCommand(importCmd)
	discoverOpts := struct {
		plugin      string
		contextName string
		namespace   string
		limit       int
		interactive bool
	}{}
	discover := &cobra.Command{
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
			input := map[string]any{"product": product}
			if strings.TrimSpace(discoverOpts.contextName) != "" {
				input["context"] = strings.TrimSpace(discoverOpts.contextName)
			}
			if strings.TrimSpace(discoverOpts.namespace) != "" {
				input["namespace"] = strings.TrimSpace(discoverOpts.namespace)
			}
			if discoverOpts.limit > 0 {
				input["limit"] = discoverOpts.limit
			}
			result, err := discoverEndpoints(cmd.Context(), runner, opts.instanceName(), product, discoverOpts.plugin, input)
			if err != nil {
				return err
			}
			if discoverOpts.interactive {
				saved, err := importEndpointSelections(cmd.InOrStdin(), cmd.ErrOrStderr(), runner.State, result.Candidates)
				if err != nil {
					return err
				}
				result.Saved = saved
			}
			return renderValue(cmd.OutOrStdout(), opts.output, result)
		},
	}
	discover.Flags().StringVar(&discoverOpts.plugin, "plugin", "", "Discover with one plugin")
	discover.Flags().StringVar(&discoverOpts.contextName, "context", "", "Discovery context")
	discover.Flags().StringVar(&discoverOpts.namespace, "namespace", "", "Discovery namespace")
	discover.Flags().IntVar(&discoverOpts.limit, "limit", 0, "Maximum candidates")
	discover.Flags().BoolVar(&discoverOpts.interactive, "interactive", false, "Interactively select candidates to import")
	cmd.AddCommand(discover)
	return cmd
}

func parseStringMapFlags(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := map[string]string{}
	for _, value := range values {
		key, val, ok := strings.Cut(value, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, fmt.Errorf("expected key=value, got %q", value)
		}
		out[key] = strings.TrimSpace(val)
	}
	return out, nil
}

func discoverEndpoints(ctx context.Context, runner runtime.Runner, instance, product, pluginFilter string, input map[string]any) (endpointDiscoveryView, error) {
	view := endpointDiscoveryView{Product: product, Results: map[string]endpointDiscoveryPluginView{}}
	var plugins []core.PluginEntry
	if strings.TrimSpace(pluginFilter) != "" {
		plugin, ok := runner.Marketplace.Resolve(pluginFilter)
		if !ok {
			return endpointDiscoveryView{}, fmt.Errorf("unknown plugin %q", pluginFilter)
		}
		plugins = []core.PluginEntry{plugin}
		view.Plugin = plugin.Name
	} else {
		var err error
		plugins, err = endpointDiscovererAvailablePlugins(ctx, runner, instance)
		if err != nil {
			return endpointDiscoveryView{}, err
		}
	}
	for _, plugin := range plugins {
		resp, err := runner.InvokeInstance(ctx, plugin.Name, instance, protocol.CommandEndpointsDiscover, input)
		if err != nil {
			view.Results[plugin.Name] = endpointDiscoveryPluginView{Error: err.Error()}
			continue
		}
		if !resp.OK {
			message := "endpoint discovery failed"
			if resp.Error != nil && strings.TrimSpace(resp.Error.Message) != "" {
				message = resp.Error.Message
			}
			view.Results[plugin.Name] = endpointDiscoveryPluginView{Error: message}
			continue
		}
		var result struct {
			Candidates []core.EndpointCandidate `json:"candidates"`
		}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			view.Results[plugin.Name] = endpointDiscoveryPluginView{Error: err.Error()}
			continue
		}
		pluginView := endpointDiscoveryPluginView{}
		for _, candidate := range result.Candidates {
			index := len(view.Candidates) + 1
			candidateView := endpointCandidateView{Index: index, EndpointCandidate: candidate}
			view.Candidates = append(view.Candidates, candidateView)
			pluginView.Candidates = append(pluginView.Candidates, candidateView)
		}
		view.Results[plugin.Name] = pluginView
	}
	if len(view.Results) == 0 || strings.TrimSpace(pluginFilter) != "" {
		view.Results = nil
	}
	return view, nil
}

func endpointImportInput(in io.Reader, from string, args []string) ([]byte, error) {
	if strings.TrimSpace(from) != "" {
		return os.ReadFile(strings.TrimSpace(from))
	}
	if len(args) > 0 && strings.TrimSpace(args[0]) != "-" {
		return []byte(strings.TrimSpace(args[0])), nil
	}
	data, err := io.ReadAll(bufio.NewReader(in))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func endpointCandidateFromImport(raw []byte, selectedIndex int) (core.EndpointCandidate, error) {
	candidates, err := endpointCandidatesFromImport(raw)
	if err != nil {
		return core.EndpointCandidate{}, err
	}
	if len(candidates) == 0 {
		return core.EndpointCandidate{}, fmt.Errorf("endpoint import JSON did not contain candidates")
	}
	if selectedIndex > 0 {
		for i, candidate := range candidates {
			if candidate.Index == selectedIndex || i+1 == selectedIndex {
				return candidate.EndpointCandidate, nil
			}
		}
		return core.EndpointCandidate{}, fmt.Errorf("candidate %d not found", selectedIndex)
	}
	if len(candidates) == 1 {
		return candidates[0].EndpointCandidate, nil
	}
	return core.EndpointCandidate{}, fmt.Errorf("multiple candidates found; pass --candidate")
}

func endpointCandidatesFromImport(raw []byte) ([]endpointCandidateView, error) {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return nil, fmt.Errorf("endpoint import input is empty")
	}
	var single endpointCandidateView
	if err := json.Unmarshal(raw, &single); err == nil && strings.TrimSpace(single.URL) != "" {
		if single.Index == 0 {
			single.Index = 1
		}
		return []endpointCandidateView{single}, nil
	}
	var view endpointDiscoveryView
	if err := json.Unmarshal(raw, &view); err != nil {
		return nil, fmt.Errorf("endpoint import input must be JSON: %w", err)
	}
	var out []endpointCandidateView
	out = append(out, view.Candidates...)
	if len(out) == 0 {
		for _, result := range view.Results {
			out = append(out, result.Candidates...)
		}
	}
	seen := map[int]bool{}
	for i := range out {
		if out[i].Index == 0 || seen[out[i].Index] {
			out[i].Index = i + 1
		}
		seen[out[i].Index] = true
	}
	return out, nil
}

func importEndpointSelections(in io.Reader, out io.Writer, state runtime.State, candidates []endpointCandidateView) ([]runtime.EndpointRecord, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no endpoint candidates to import")
	}
	for _, candidate := range candidates {
		if _, err := fmt.Fprintf(out, "%d\t%s\t%s\t%s\n", candidate.Index, candidate.Product, candidate.URL, candidate.CredentialRef); err != nil {
			return nil, err
		}
	}
	if _, err := fmt.Fprint(out, "Select endpoint candidates to import: "); err != nil {
		return nil, err
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return nil, err
	}
	selected, err := parseEndpointSelection(line)
	if err != nil {
		return nil, err
	}
	byIndex := map[int]core.EndpointCandidate{}
	for _, candidate := range candidates {
		byIndex[candidate.Index] = candidate.EndpointCandidate
	}
	var saved []runtime.EndpointRecord
	for _, index := range selected {
		candidate, ok := byIndex[index]
		if !ok {
			return nil, fmt.Errorf("candidate %d not found", index)
		}
		record, err := state.SaveEndpointCandidate(candidate)
		if err != nil {
			return nil, err
		}
		saved = append(saved, record)
	}
	return saved, nil
}

func parseEndpointSelection(value string) ([]int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("no endpoint candidates selected")
	}
	seen := map[int]bool{}
	var out []int
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		startText, endText, isRange := strings.Cut(part, "-")
		start, err := parsePositiveInt(startText)
		if err != nil {
			return nil, fmt.Errorf("invalid selection %q", part)
		}
		end := start
		if isRange {
			end, err = parsePositiveInt(endText)
			if err != nil || end < start {
				return nil, fmt.Errorf("invalid selection range %q", part)
			}
		}
		for i := start; i <= end; i++ {
			if !seen[i] {
				seen[i] = true
				out = append(out, i)
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no endpoint candidates selected")
	}
	return out, nil
}

func parsePositiveInt(value string) (int, error) {
	out, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || out <= 0 {
		return 0, fmt.Errorf("expected positive integer")
	}
	return out, nil
}

func mergeStringMaps(base, override map[string]string) map[string]string {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}
	out := map[string]string{}
	for key, value := range base {
		out[key] = value
	}
	for key, value := range override {
		out[key] = value
	}
	return out
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

func shortcutViews(marketplace runtime.Marketplace, pluginFilter string) []shortcutView {
	var out []shortcutView
	for _, plugin := range marketplace.Plugins() {
		if pluginFilter != "" && plugin.Name != pluginFilter {
			continue
		}
		for _, command := range plugin.Commands {
			target := strings.TrimSpace(command.Target)
			if target == "" {
				switch {
				case strings.TrimSpace(command.Operation) != "":
					target = "operation"
				case strings.TrimSpace(command.Datasource) != "":
					target = "datasource"
				default:
					target = "command"
				}
			}
			out = append(out, shortcutView{
				Plugin:      plugin.Name,
				Use:         command.Use,
				Description: command.Description,
				Target:      target,
				Operation:   command.Operation,
				Datasource:  command.Datasource,
				Capability:  command.Capability,
				Entity:      command.Entity,
				Defaults:    command.Defaults,
			})
		}
	}
	return out
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
	} else if command == protocol.CommandEndpointsDiscover {
		var err error
		plugins, err = endpointDiscovererAvailablePlugins(ctx, runner, instance)
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

func endpointDiscovererAvailablePlugins(ctx context.Context, runner runtime.Runner, instance string) ([]core.PluginEntry, error) {
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
		if len(manifest.Endpoints) > 0 {
			out = append(out, plugin)
		}
	}
	return out, nil
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
	entry, ok := runner.Marketplace.Resolve(plugin)
	if !ok {
		return false, nil
	}
	activated, err := runner.State.IsPluginActivated(entry.Name)
	if err != nil {
		return false, err
	}
	if activated {
		return true, nil
	}
	return false, nil
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

func findOperationSpec(ctx context.Context, runner runtime.Runner, name string) (core.OperationSpec, error) {
	plugin := strings.TrimSpace(strings.SplitN(name, ".", 2)[0])
	if plugin != "" && plugin != name {
		manifest, err := pluginManifest(ctx, runner, plugin)
		if err == nil {
			for _, operation := range manifest.Operations {
				if operation.Name == name {
					return operation, nil
				}
			}
			return core.OperationSpec{}, fmt.Errorf("unknown operation %q", name)
		}
	}
	for _, entry := range runner.Marketplace.Plugins() {
		manifest, err := pluginManifest(ctx, runner, entry.Name)
		if err != nil {
			continue
		}
		for _, operation := range manifest.Operations {
			if operation.Name == name {
				return operation, nil
			}
		}
	}
	return core.OperationSpec{}, fmt.Errorf("unknown operation %q", name)
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
	if !resp.OK {
		if resp.Error != nil {
			return fmt.Errorf("%s", resp.Error.Message)
		}
		return fmt.Errorf("operation %s failed", name)
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
