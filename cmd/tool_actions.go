package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// usesRe and shaRe are compiled once at package load and shared across the
// pin-actions helpers below. The patterns are identical to the previous
// per-call regexp.MustCompile calls, and a *regexp.Regexp is safe for
// concurrent use by multiple goroutines.
var (
	usesRe = regexp.MustCompile(`^(\s*-?\s*uses:\s*)([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)@([^\s#]+)(.*)$`)
	shaRe  = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

func makePinActionsCmd() *cobra.Command {
	var mappingPath string
	cmd := &cobra.Command{
		Use:   "pin-actions --mapping <mapping.tsv> <workflow.yml> [<workflow.yml>...]",
		Short: "Rewrite GitHub Actions references to pinned SHAs",
		Args: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(mappingPath) == "" || len(args) == 0 {
				return newInvalidUsageError("pin_actions_usage", "Usage: slipway tool pin-actions --mapping <mapping.tsv> <workflow.yml> [<workflow.yml>...]", "Pass a mapping TSV and at least one workflow file.", nil)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPinActions(cmd, mappingPath, args)
		},
	}
	cmd.Flags().StringVar(&mappingPath, "mapping", "", "TSV mapping of owner/repo@ref to 40-character SHA")
	return cmd
}

func runPinActions(cmd *cobra.Command, mappingPath string, files []string) error {
	mapping, err := loadActionPinMapping(mappingPath)
	if err != nil {
		return err
	}
	plans := make([]actionPinRewritePlan, 0, len(files))
	unresolved := false
	for _, file := range files {
		plan, err := planActionPinsRewrite(file, mapping)
		if err == nil {
			plans = append(plans, plan)
			continue
		}
		var unresolvedErr actionPinUnresolvedError
		if asActionPinUnresolved(err, &unresolvedErr) {
			unresolved = true
			fmt.Fprintln(cmd.ErrOrStderr(), unresolvedErr.Error())
			continue
		}
		return err
	}
	if unresolved {
		return newPreconditionError("pin_actions_unresolved", "one or more action references were unresolved; no workflow files were modified", "Add missing owner/repo@ref rows to the mapping TSV.", "", nil)
	}
	for _, plan := range plans {
		if err := writeActionPinsRewrite(plan); err != nil {
			return err
		}
	}
	return nil
}

func loadActionPinMapping(path string) (map[string]string, error) {
	file, err := os.Open(path) // #nosec G304 -- explicit user-supplied helper input path.
	if err != nil {
		return nil, newInvalidUsageError("pin_actions_mapping_missing", fmt.Sprintf("pin-actions: mapping file not found: %s", path), "Pass --mapping with a readable TSV file.", map[string]any{"path": path})
	}
	defer file.Close()

	out := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, newInvalidUsageError("pin_actions_mapping_malformed", fmt.Sprintf("pin-actions: malformed mapping row: %s", line), "Use owner/repo@ref<TAB>sha rows.", nil)
		}
		key := strings.TrimSpace(parts[0])
		sha := strings.TrimSpace(parts[1])
		if !shaRe.MatchString(sha) {
			return nil, newInvalidUsageError("pin_actions_mapping_sha_invalid", fmt.Sprintf("pin-actions: mapping sha not a 40-char hex: %s -> %s", key, sha), "Use lowercase 40-character commit SHAs.", nil)
		}
		out[key] = sha
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

type actionPinUnresolvedError struct {
	key  string
	file string
}

func (e actionPinUnresolvedError) Error() string {
	return fmt.Sprintf("pin-actions: unresolved %s in %s", e.key, e.file)
}

func asActionPinUnresolved(err error, target *actionPinUnresolvedError) bool {
	if err == nil {
		return false
	}
	if typed, ok := err.(actionPinUnresolvedError); ok {
		*target = typed
		return true
	}
	return false
}

type actionPinRewritePlan struct {
	file    string
	body    string
	mode    os.FileMode
	changed bool
}

func planActionPinsRewrite(file string, mapping map[string]string) (actionPinRewritePlan, error) {
	raw, err := os.ReadFile(file) // #nosec G304 -- explicit user-supplied helper input path.
	if err != nil {
		return actionPinRewritePlan{}, newPreconditionError("pin_actions_workflow_unreadable", fmt.Sprintf("pin-actions: cannot read %s", file), "Pass a readable workflow file.", "", map[string]any{"path": file})
	}
	info, err := os.Stat(file)
	if err != nil {
		return actionPinRewritePlan{}, err
	}

	lines := strings.SplitAfter(string(raw), "\n")
	var b strings.Builder
	for _, lineWithEnd := range lines {
		line := strings.TrimSuffix(lineWithEnd, "\n")
		end := ""
		if strings.HasSuffix(lineWithEnd, "\n") {
			end = "\n"
		}
		m := usesRe.FindStringSubmatch(line)
		if m == nil || shaRe.MatchString(m[3]) {
			b.WriteString(line)
			b.WriteString(end)
			continue
		}
		key := m[2] + "@" + m[3]
		sha, ok := mapping[key]
		if !ok {
			return actionPinRewritePlan{}, actionPinUnresolvedError{key: key, file: file}
		}
		b.WriteString(m[1])
		b.WriteString(m[2])
		b.WriteByte('@')
		b.WriteString(sha)
		b.WriteString("  # ")
		b.WriteString(m[3])
		b.WriteString(m[4])
		b.WriteString(end)
	}
	body := b.String()
	return actionPinRewritePlan{
		file:    file,
		body:    body,
		mode:    info.Mode().Perm(),
		changed: body != string(raw),
	}, nil
}

func writeActionPinsRewrite(plan actionPinRewritePlan) error {
	if !plan.changed {
		return nil
	}
	tmp, err := os.CreateTemp(filepath.Dir(plan.file), ".pin-actions-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.WriteString(plan.body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	_ = os.Chmod(tmpName, plan.mode)
	return os.Rename(tmpName, plan.file)
}
