package cmd

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/toolgen"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestTemplateFlagsMatchCobraCommands verifies that every --flag referenced
// in generated skill/command templates actually exists on the corresponding
// Cobra command. This prevents template-CLI drift (D8).
func TestTemplateFlagsMatchCobraCommands(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, toolgen.Generate(root, []string{"claude"}, true))

	// Map command names to their Cobra constructors.
	cmds := map[string]*cobra.Command{
		"init":      makeInitCmd(),
		"intake":    makeIntakeCmd(),
		"new":       makeNewCmd(),
		"next":      makeNextCmd(),
		"plan":      makePlanCmd(),
		"status":    makeStatusCmd(),
		"done":      makeDoneCmd(),
		"cancel":    makeCancelCmd(),
		"fix":       makeFixCmd(),
		"implement": makeImplementCmd(),
		"review":    makeReviewCmd(),
		"validate":  makeValidateCmd(),
		"health":    makeHealthCmd(),
		"run":       makeRunCmd(),
		"abort":     makeAbortCmd(),
		"repair":    makeRepairCmd(),
		"evidence":  makeEvidenceCmd(),
		"config":    makeConfigCmd(),
	}

	// Collect registered flags per command.
	cmdFlags := map[string]map[string]bool{}
	for name, cmd := range cmds {
		flags := collectCommandFlags(cmd)
		// Always allow -h/--help (inherited from Cobra).
		flags["help"] = true
		cmdFlags[name] = flags
	}

	// Pattern: `slipway <cmd> --<flag>` or `slipway <cmd> ... --<flag>`
	re := regexp.MustCompile("`slipway ([a-z][a-z-]*)(?:\\s[^`]*?)\\s--([a-z][a-z-]*)`")
	standaloneFlagRe := regexp.MustCompile("`--([a-z][a-z-]*)`")

	// Walk all generated skill files.
	skillsDir := filepath.Join(root, ".claude", "skills")
	err := filepath.Walk(skillsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".md" {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		matches := re.FindAllStringSubmatch(string(content), -1)
		for _, m := range matches {
			cmdName, flagName := m[1], m[2]
			flags, ok := cmdFlags[cmdName]
			if !ok {
				continue // unknown command; covered by existing command name test
			}
			relPath, _ := filepath.Rel(root, path)
			assert.True(t, flags[flagName],
				"template %s references `slipway %s --%s` but flag --%s is not registered on the %s command",
				relPath, cmdName, flagName, flagName, cmdName)
		}
		commandName := generatedSkillCommandName(path)
		if flags, ok := cmdFlags[commandName]; ok {
			for _, m := range standaloneFlagRe.FindAllStringSubmatch(flagsSection(string(content)), -1) {
				flagName := m[1]
				relPath, _ := filepath.Rel(root, path)
				assert.True(t, flags[flagName],
					"command entry %s references standalone flag --%s but that flag is not registered on the %s command",
					relPath, flagName, commandName)
			}
		}
		return nil
	})
	require.NoError(t, err)

	// Also check command entry files.
	commandsDir := filepath.Join(root, ".claude", "commands", "slipway")
	err = filepath.Walk(commandsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".md" {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		matches := re.FindAllStringSubmatch(string(content), -1)
		for _, m := range matches {
			cmdName, flagName := m[1], m[2]
			flags, ok := cmdFlags[cmdName]
			if !ok {
				continue
			}
			relPath, _ := filepath.Rel(root, path)
			assert.True(t, flags[flagName],
				"command entry %s references `slipway %s --%s` but flag --%s is not registered",
				relPath, cmdName, flagName, flagName, cmdName)
		}
		commandName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if flags, ok := cmdFlags[commandName]; ok {
			for _, m := range standaloneFlagRe.FindAllStringSubmatch(flagsSection(string(content)), -1) {
				flagName := m[1]
				relPath, _ := filepath.Rel(root, path)
				assert.True(t, flags[flagName],
					"command entry %s references standalone flag --%s but that flag is not registered on the %s command",
					relPath, flagName, commandName)
			}
		}
		return nil
	})
	require.NoError(t, err)
}

func generatedSkillCommandName(path string) string {
	dirName := filepath.Base(filepath.Dir(path))
	return strings.TrimPrefix(dirName, "slipway-")
}

func flagsSection(content string) string {
	const heading = "## Flags"
	start := strings.Index(content, heading)
	if start < 0 {
		return ""
	}
	section := content[start+len(heading):]
	if next := strings.Index(section, "\n## "); next >= 0 {
		section = section[:next]
	}
	return section
}

func collectCommandFlags(cmd *cobra.Command) map[string]bool {
	flags := map[string]bool{}
	if cmd == nil {
		return flags
	}
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		flags[f.Name] = true
	})
	for _, child := range cmd.Commands() {
		for flagName := range collectCommandFlags(child) {
			flags[flagName] = true
		}
	}
	return flags
}

func TestDoneAllReadyFlagUsageMatchesBulkBehavior(t *testing.T) {
	t.Parallel()
	flag := makeDoneCmd().Flags().Lookup("all-ready")
	require.NotNil(t, flag)
	assert.Equal(t, "Archive every active change that is done-ready", flag.Usage)
}

func TestHydrateRefHelpUsesReferencePlaceholder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cmd  *cobra.Command
	}{
		{name: "status", cmd: makeStatusCmd()},
		{name: "review", cmd: makeReviewCmd()},
		{name: "health", cmd: makeHealthCmd()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			tt.cmd.SetOut(&out)
			require.NoError(t, tt.cmd.Help())

			help := out.String()
			assert.Contains(t, help, "--hydrate-ref <skill-id>/<name>")
			assert.NotContains(t, help, "--hydrate-ref --hydrate")
		})
	}
}

func TestGeneratedCommandEntriesExposeChangeSelectorForSupportedCommands(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, toolgen.Generate(root, []string{"claude"}, true))

	commandsDir := filepath.Join(root, ".claude", "commands", "slipway")
	for _, id := range []string{
		"abort",
		"cancel",
		"done",
		"fix",
		"implement",
		"intake",
		"next",
		"plan",
		"preset",
		"review",
		"run",
		"status",
		"validate",
	} {
		raw, err := os.ReadFile(filepath.Join(commandsDir, id+".md"))
		require.NoError(t, err)
		assert.Contains(t, string(raw), "--change <slug>", "generated command entry for %s must surface the explicit change selector", id)
	}
}

func TestGeneratedCommandSkillIDsResolveOnLiveRootCommands(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	var toolIDs []string
	for _, cfg := range toolgen.Registry() {
		if cfg.CommandSkillSurface {
			toolIDs = append(toolIDs, cfg.ID)
		}
	}
	require.NotEmpty(t, toolIDs)
	require.NoError(t, toolgen.Generate(root, toolIDs, true))

	liveRootCommands := map[string]struct{}{}
	for _, cmd := range newRootCmd().Commands() {
		liveRootCommands[cmd.Name()] = struct{}{}
	}

	found := map[string]struct{}{}
	for _, cfg := range toolgen.Registry() {
		if !cfg.CommandSkillSurface {
			continue
		}
		skillsRoot := filepath.Join(root, cfg.SkillsDir)
		err := filepath.WalkDir(skillsRoot, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() || d.Name() != "SKILL.md" {
				return walkErr
			}
			meta, ok, err := generatedCommandSkillMetadata(path)
			if err != nil || !ok {
				return err
			}
			found[cfg.ID+"/"+meta.CommandID] = struct{}{}
			_, resolves := liveRootCommands[meta.CommandID]
			assert.Truef(t, resolves,
				"%s generated command skill %s points at command_id %q, but newRootCmd().Commands() does not register it",
				cfg.ID, path, meta.CommandID)
			assert.Equal(t, "slipway-"+meta.CommandID, filepath.Base(filepath.Dir(path)),
				"%s generated command skill path must match command_id", cfg.ID)
			return nil
		})
		require.NoError(t, err)
	}
	require.NotEmpty(t, found, "expected generated command-skill metadata for at least one host")
}

type commandSkillMetadata struct {
	CommandID string `yaml:"command_id"`
	Surface   string `yaml:"surface"`
}

func generatedCommandSkillMetadata(path string) (commandSkillMetadata, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return commandSkillMetadata{}, false, err
	}
	block, ok := leadingFrontmatterBlock(string(raw))
	if !ok {
		return commandSkillMetadata{}, false, nil
	}
	var meta commandSkillMetadata
	if err := yaml.Unmarshal([]byte(block), &meta); err != nil {
		return commandSkillMetadata{}, false, err
	}
	if strings.TrimSpace(meta.CommandID) == "" || strings.TrimSpace(meta.Surface) != "skill" {
		return commandSkillMetadata{}, false, nil
	}
	meta.CommandID = strings.TrimSpace(meta.CommandID)
	meta.Surface = strings.TrimSpace(meta.Surface)
	return meta, true, nil
}

func leadingFrontmatterBlock(content string) (string, bool) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.TrimLeft(content, "\n")
	if !strings.HasPrefix(content, "---\n") {
		return "", false
	}
	rest := strings.TrimPrefix(content, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", false
	}
	return rest[:end], true
}

// TestCobraFlagsCoveredByRegistryArguments is the reverse contract of
// TestTemplateFlagsMatchCobraCommands. The forward test prevents a template
// from naming a flag that does not exist; this reverse test prevents a real
// Cobra flag from silently dropping out of the generated command reference.
// Every non-hidden, non-help flag a command registers MUST appear in that
// command's toolgen.CommandArguments(id) string (the source the reference and
// codex prompts render from), unless it is explicitly exempted below.
func TestCobraFlagsCoveredByRegistryArguments(t *testing.T) {
	t.Parallel()

	cmds := map[string]*cobra.Command{
		"new":          makeNewCmd(),
		"intake":       makeIntakeCmd(),
		"plan":         makePlanCmd(),
		"implement":    makeImplementCmd(),
		"next":         makeNextCmd(),
		"run":          makeRunCmd(),
		"status":       makeStatusCmd(),
		"done":         makeDoneCmd(),
		"fix":          makeFixCmd(),
		"init":         makeInitCmd(),
		"cancel":       makeCancelCmd(),
		"review":       makeReviewCmd(),
		"validate":     makeValidateCmd(),
		"preset":       makePresetCmd(),
		"abort":        makeAbortCmd(),
		"repair":       makeRepairCmd(),
		"evidence":     makeEvidenceCmd(),
		"health":       makeHealthCmd(),
		"codebase-map": makeCodebaseMapCmd(),
		"config":       makeConfigCmd(),
	}

	// Flags intentionally omitted from the human-facing Arguments summary.
	// Keep this list small and justified — it is the only sanctioned way for a
	// real flag to be absent from the reference.
	exempt := map[string]map[string]bool{
		// `evidence task` exposes ledger override flags in Cobra for black-box
		// recovery/debugging, but generated Arguments intentionally teach the
		// host-owned public task evidence surface.
		"evidence task": {
			"captured-at":         true,
			"changed-file":        true,
			"evidence-ref":        true,
			"no-op-justification": true,
			"run-summary-version": true,
			"session-id":          true,
			"task-id":             true,
			"task-kind":           true,
			"target-file":         true,
		},
	}

	for id, cmd := range cmds {
		args := toolgen.CommandArguments(id)
		require.NotEmptyf(t, args, "registry Arguments missing for command %q", id)
		for _, ref := range collectVisibleFlagRefs(cmd, []string{id}) {
			if ref.name == "help" || exempt[ref.commandPath][ref.name] {
				continue
			}
			// Bound the match so "--hydrate" is not satisfied by "--hydrate-ref".
			re := regexp.MustCompile("--" + regexp.QuoteMeta(ref.name) + "([^a-zA-Z0-9-]|$)")
			assert.Truef(t, re.MatchString(args),
				"command path %q registers flag --%s but it is absent from registry Arguments %q; add it to commandRegistry[%q].Arguments or to the exemption list",
				ref.commandPath, ref.name, args, id)
		}
	}
}

type visibleFlagRef struct {
	commandPath string
	name        string
}

// collectVisibleFlagRefs is collectCommandFlags restricted to non-hidden flags,
// recursing into subcommands while preserving the command path (e.g.
// `evidence task`) so exemptions can stay narrow.
func collectVisibleFlagRefs(cmd *cobra.Command, path []string) []visibleFlagRef {
	if cmd == nil {
		return nil
	}
	var refs []visibleFlagRef
	commandPath := strings.Join(path, " ")
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if !f.Hidden {
			refs = append(refs, visibleFlagRef{
				commandPath: commandPath,
				name:        f.Name,
			})
		}
	})
	for _, child := range cmd.Commands() {
		childPath := append(append([]string{}, path...), child.Name())
		refs = append(refs, collectVisibleFlagRefs(child, childPath)...)
	}
	return refs
}
