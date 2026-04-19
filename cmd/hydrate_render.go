package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/signalridge/slipway/internal/toolgen"
)

const defaultHydrateWarnBytes = 32 * 1024

// hydrateWarnBytes is the advisory threshold for aggregate hydrate output.
// Crossing it surfaces a warning in text output, but does not block render.
var hydrateWarnBytes = defaultHydrateWarnBytes

// normalizeHydrateKeys returns a stable-sorted, deduplicated copy of keys.
// Empty input returns nil so callers and JSON encoders see empty-slice
// elision consistently across text and JSON surfaces.
func normalizeHydrateKeys(keys []string) []string {
	if len(keys) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(keys))
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}

// writeHydrateLine emits a single `Hydrate:` line through the shared
// formatWriter when keys are non-empty. `prefix` lets callers indent nested
// surfaces such as `next` technique hints while preserving one canonical
// delimiter and join policy.
func writeHydrateLine(w *formatWriter, prefix string, keys []string) {
	if len(keys) == 0 {
		return
	}
	w.Writef("%sHydrate: %s\n", prefix, strings.Join(keys, ", "))
}

// selectHydrateKeys narrows a surface's hydrate set to the exact keys the
// operator requested via repeatable --hydrate-ref flags. Requested keys must
// already appear in the surface's advertised Hydrate list; this keeps
// selection deterministic and prevents path-like ad hoc lookups.
func selectHydrateKeys(available, requested []string) ([]string, error) {
	available = normalizeHydrateKeys(available)
	requested = normalizeHydrateKeys(requested)
	if len(requested) == 0 {
		return available, nil
	}
	if len(available) == 0 {
		return nil, newPreconditionError(
			"hydrate_unavailable",
			"this command surface exposes no hydrate references",
			"Run the command without `--hydrate-ref` to inspect available Hydrate keys, or select a mode/view that advertises hydrate references.",
			"",
			map[string]any{"requested_hydrate_refs": requested},
		)
	}

	allowed := make(map[string]struct{}, len(available))
	for _, key := range available {
		allowed[key] = struct{}{}
	}
	for _, key := range requested {
		if _, ok := allowed[key]; ok {
			continue
		}
		return nil, newInvalidUsageError(
			"hydrate_ref_unknown",
			fmt.Sprintf("requested hydrate ref %q is not available on this command surface", key),
			"Run the command without `--hydrate` to inspect the available Hydrate keys, then retry with an exact `--hydrate-ref <skill-id>/<name>` value.",
			map[string]any{
				"requested_hydrate_ref":  key,
				"available_hydrate_refs": available,
			},
		)
	}
	return requested, nil
}

// hydrateReferencePath resolves a hydrate key against the generated workspace
// tree so runtime hydrate output mirrors the files agents actually see under
// `.codex/skills/`, `.claude/skills/`, etc. When callers pass the canonical
// scope root, we follow the current invocation worktree if the scope root does
// not itself carry generated adapters.
func hydrateReferencePath(root, key string) (string, error) {
	skillID, name, ok := strings.Cut(key, "/")
	if !ok || skillID == "" || name == "" {
		return "", newInvalidUsageError(
			"hydrate_key_invalid",
			fmt.Sprintf("hydrate key %q is not in `<skill-id>/<name>` shape", key),
			"Fix the registry entry so it exports a well-formed hydrate key.",
			map[string]any{"key": key},
		)
	}
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return "", newInvalidUsageError(
			"hydrate_key_invalid",
			fmt.Sprintf("hydrate key %q must use a skill-relative basename", key),
			"Fix the registry entry so hydrate names stay within the owning skill's references/ directory.",
			map[string]any{"key": key},
		)
	}
	workspaceRoot := root
	if len(toolgen.DetectExistingTools(workspaceRoot)) == 0 {
		workspaceRoot = invocationWorkspaceRoot(root)
	}
	cfg, err := toolgen.ResolveWorkspaceTool(workspaceRoot)
	if err != nil {
		return "", err
	}
	skillDir := filepath.Dir(toolgen.SkillPath(cfg, skillID))
	return filepath.Join(workspaceRoot, skillDir, "references", filepath.FromSlash(name)), nil
}

// loadHydrateBody returns the file body for a `<skill-id>/<name>` hydrate
// key from the generated workspace tree. Missing files surface
// `hydrate_reference_missing` so operators see which rendered path drifted.
func loadHydrateBody(root, key string) (string, error) {
	refPath, err := hydrateReferencePath(root, key)
	if err != nil {
		return "", err
	}
	body, err := os.ReadFile(refPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", newInvalidUsageError(
				"hydrate_reference_missing",
				fmt.Sprintf("hydrate reference %q is declared but not present on disk at %s", key, refPath),
				"Regenerate the workspace skill tree or remove the stale hydrate entry.",
				map[string]any{"key": key, "path": refPath},
			)
		}
		return "", err
	}
	return string(body), nil
}

// emitHydrateBlocks renders the bodies of each selected hydrate reference
// prefixed by a stable delimiter so consumer tooling can pin a deterministic
// boundary. Fails deterministically when a hydrate key contains `=` or a
// newline (delimiter-unsafe). Aggregate size above hydrateWarnBytes is a text
// warning, not a hard failure.
func emitHydrateBlocks(root string, w io.Writer, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	type block struct {
		key  string
		body string
	}
	blocks := make([]block, 0, len(keys))
	total := 0
	for _, key := range keys {
		if strings.ContainsAny(key, "=\n") {
			return newInvalidUsageError(
				"hydrate_key_unsafe",
				fmt.Sprintf("hydrate key %q contains delimiter-unsafe characters", key),
				"Remove `=` and newline characters from hydrate registry entries.",
				map[string]any{"key": key},
			)
		}
		body, err := loadHydrateBody(root, key)
		if err != nil {
			return err
		}
		total += len(body)
		blocks = append(blocks, block{key: key, body: body})
	}
	printedPreamble := false
	if hydrateWarnBytes > 0 && total > hydrateWarnBytes {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(
			w,
			"WARN hydrate_output_large: selected hydrate bodies total %d bytes exceeds advisory threshold %d; continuing output.\n\n",
			total,
			hydrateWarnBytes,
		); err != nil {
			return err
		}
		printedPreamble = true
	}
	for i, b := range blocks {
		if i == 0 && !printedPreamble {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "===== SLIPWAY HYDRATE: %s =====\n", b.key); err != nil {
			return err
		}
		if _, err := io.WriteString(w, b.body); err != nil {
			return err
		}
		if !strings.HasSuffix(b.body, "\n") {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
	}
	return nil
}
