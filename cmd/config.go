package cmd

import (
	"errors"
	"fmt"
	"text/tabwriter"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

// configShortDescription is the one-line summary for the `config` command. The
// `config` command is not part of the toolgen command registry that drives the
// generated skill/adapter surfaces, so its Short is sourced here rather than via
// desc(); the command-description contract test enumerates only registry-backed
// commands and does not cover `config`.
const configShortDescription = "Inspect and set repo-level Slipway configuration keys"

// configGetView is the JSON shape emitted by `config get --json`.
type configGetView struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// makeConfigCmd builds the `config` command. It is registered alongside its
// siblings in newRootCmd() and listed in the root help groups, so it is
// discoverable from `slipway --help`, not only `slipway help config`.
func makeConfigCmd() *cobra.Command {
	var listJSON bool
	cmd := &cobra.Command{
		Use:   "config",
		Short: configShortDescription,
		// Mirror the root command: errors surface as structured CLIError JSON on
		// stderr, never a usage wall on stdout. Subcommands inherit these.
		SilenceUsage:  true,
		SilenceErrors: true,
		Long: configShortDescription + `

Keys are the dotted leaves of .slipway.yaml (the same surface strict decoding
accepts). With no subcommand, config lists every key; use list/get/set to read
or update individual keys.

config set rewrites .slipway.yaml as deterministic YAML; comments and the
original key ordering are not preserved.

  config [list] [--json]   Enumerate every key (name, type, default,
                           allowed-values, scope).
  config get <key> [--json]
                           Print the resolved effective value for a key.
  config set <key> <value> Validate and persist a key to .slipway.yaml.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConfigList(cmd, listJSON)
		},
	}
	cmd.Flags().BoolVar(&listJSON, "json", false, "JSON output")
	cmd.AddCommand(makeConfigListCmd())
	cmd.AddCommand(makeConfigGetCmd())
	cmd.AddCommand(makeConfigSetCmd())
	return cmd
}

func makeConfigListCmd() *cobra.Command {
	var jsonFlag bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List every configuration key with its type, default, allowed values, and scope",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConfigList(cmd, jsonFlag)
		},
	}
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "JSON output")
	return cmd
}

func makeConfigGetCmd() *cobra.Command {
	var jsonFlag bool
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Print the resolved effective value for a configuration key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigGet(cmd, args[0], jsonFlag)
		},
	}
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "JSON output")
	return cmd
}

func makeConfigSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Validate and persist a configuration key to .slipway.yaml",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigSet(cmd, args[0], args[1])
		},
	}
	return cmd
}

// loadConfigForCommand resolves the project root for the command and loads the
// effective .slipway.yaml, reusing the shared root and config-load helpers so
// uninitialized-workspace and parse-failure errors stay consistent with the
// rest of the CLI.
func loadConfigForCommand(cmd *cobra.Command) (string, model.Config, error) {
	root, err := projectRootFromCommand(cmd)
	if err != nil {
		return "", model.Config{}, err
	}
	cfg, err := loadConfigAtRootWithStderr(root, cmd.ErrOrStderr())
	if err != nil {
		return "", model.Config{}, err
	}
	return root, cfg, nil
}

func runConfigList(cmd *cobra.Command, jsonFlag bool) error {
	catalog := model.ConfigCatalog()
	if jsonFlag {
		return encodeJSONResponse(cmd, catalog)
	}
	return writeConfigCatalogText(cmd, catalog)
}

func writeConfigCatalogText(cmd *cobra.Command, catalog []model.ConfigCatalogEntry) error {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "KEY\tTYPE\tDEFAULT\tALLOWED\tSCOPE\tDESCRIPTION"); err != nil {
		return err
	}
	for _, entry := range catalog {
		allowed := "-"
		if len(entry.AllowedValues) > 0 {
			allowed = joinConfigAllowed(entry.AllowedValues)
		}
		def := entry.Default
		if def == "" {
			def = "-"
		}
		description := entry.Description
		if description == "" {
			description = "-"
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", entry.Name, entry.Type, def, allowed, entry.Scope, description); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func joinConfigAllowed(values []string) string {
	out := ""
	for i, v := range values {
		if i > 0 {
			out += "|"
		}
		out += v
	}
	return out
}

func runConfigGet(cmd *cobra.Command, key string, jsonFlag bool) error {
	_, cfg, err := loadConfigForCommand(cmd)
	if err != nil {
		return err
	}
	value, err := model.ConfigGetValue(cfg, key)
	if err != nil {
		return newConfigKeyError(key, err)
	}
	if jsonFlag {
		return encodeJSONResponse(cmd, configGetView{Key: key, Value: value})
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), value)
	return err
}

func runConfigSet(cmd *cobra.Command, key, value string) error {
	root, cfg, err := loadConfigForCommand(cmd)
	if err != nil {
		return err
	}
	// ConfigSetValue parses the typed value by dotted key and runs the same
	// strict Config.Validate() contract. It returns a copy and never mutates cfg
	// on failure, so an invalid value cannot reach the SaveConfig atomic write.
	updated, err := model.ConfigSetValue(cfg, key, value)
	if err != nil {
		// An unknown key reports the same stable code as `config get` so the two
		// read/write paths do not disagree on what an unknown key is; only a
		// parseable-but-invalid value uses config_value_invalid.
		if errors.Is(err, model.ErrUnknownConfigKey) {
			return newConfigKeyError(key, err)
		}
		return newConfigSetValueError(key, err)
	}
	if err := model.SaveConfig(state.ConfigPath(root), updated); err != nil {
		return newStateIntegrityError(
			"config_write_failure",
			fmt.Sprintf("failed to persist .slipway.yaml: %v", err),
			"Check filesystem permissions for the workspace, then retry `slipway config set`.",
			"",
			map[string]any{"path": state.ConfigPath(root), "key": key},
		)
	}
	displayCfg := updated
	displayCfg.Normalize()
	displayValue, err := model.ConfigGetValue(displayCfg, key)
	if err != nil {
		return newStateIntegrityError(
			"config_value_resolution_failure",
			fmt.Sprintf("failed to resolve persisted config value for %q: %v", key, err),
			"Run `slipway config get` for the key to inspect the persisted value.",
			"",
			map[string]any{"path": state.ConfigPath(root), "key": key},
		)
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "set %s = %s\n", key, displayValue)
	return err
}

// newConfigKeyError wraps an unknown/unresolvable config key error from the
// catalog into a CLIError so it emits to stderr with a non-zero exit. The
// offending key is carried in the structured Details map (not just the wrapped
// Message prose) so callers and tests can key on a stable field.
func newConfigKeyError(key string, err error) error {
	return newInvalidUsageError(
		"config_key_unknown",
		err.Error(),
		"Run `slipway config list` to see every settable key.",
		map[string]any{"key": key},
	)
}

// newConfigSetValueError wraps a parse/validation rejection from ConfigSetValue
// (an unknown key, an unparseable value, or a Config.Validate() rejection) into
// a CLIError. The .slipway.yaml file is never written on this path, so the
// existing config stays byte-for-byte unchanged. The offending key is carried in
// the structured Details map so callers and tests can key on a stable field.
func newConfigSetValueError(key string, err error) error {
	return newInvalidUsageError(
		"config_value_invalid",
		err.Error(),
		"Run `slipway config list` to see each key's type and allowed values, then retry with a valid value.",
		map[string]any{"key": key},
	)
}
