package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/toolgen"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var check bool
	var write bool
	flag.BoolVar(&check, "check", false, "fail if docs/SURFACE-MANIFEST.json is stale")
	flag.BoolVar(&write, "write", false, "rewrite docs/SURFACE-MANIFEST.json")
	flag.Parse()

	repoRoot, err := fsutil.FindRepoRoot("")
	if err != nil {
		return err
	}
	return runWithOptions(repoRoot, manifestCommandOptions{
		check:  check,
		write:  write,
		stdout: os.Stdout,
	})
}

type manifestCommandOptions struct {
	check  bool
	write  bool
	stdout io.Writer
}

func runWithOptions(repoRoot string, opts manifestCommandOptions) error {
	if opts.check && opts.write {
		return errors.New("choose only one of --check or --write")
	}
	if opts.stdout == nil {
		opts.stdout = io.Discard
	}
	manifestPath := filepath.Join(repoRoot, toolgen.SurfaceManifestPath)

	live, err := toolgen.EncodeSurfaceManifest(toolgen.BuildSurfaceManifest())
	if err != nil {
		return fmt.Errorf("encode surface manifest: %w", err)
	}

	switch {
	case opts.write:
		if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil { // #nosec G301 -- docs directory is a committed project artifact location.
			return fmt.Errorf("create manifest directory: %w", err)
		}
		if err := os.WriteFile(manifestPath, live, 0o644); err != nil { // #nosec G306 -- committed docs artifact.
			return fmt.Errorf("write %s: %w", toolgen.SurfaceManifestPath, err)
		}
		fmt.Fprintf(opts.stdout, "wrote %s\n", toolgen.SurfaceManifestPath)
		return nil
	case opts.check:
		committed, err := os.ReadFile(manifestPath) // #nosec G304 -- manifestPath is repo root plus docs/SURFACE-MANIFEST.json.
		if err != nil {
			return fmt.Errorf("%s is missing or unreadable; run `go run ./internal/toolgen/cmd/gen-surface-manifest --write`: %w",
				toolgen.SurfaceManifestPath,
				err)
		}
		if bytes.Equal(committed, live) {
			fmt.Fprintf(opts.stdout, "%s is up to date\n", toolgen.SurfaceManifestPath)
			return nil
		}
		return fmt.Errorf("%s is stale; run `go run ./internal/toolgen/cmd/gen-surface-manifest --write`\n%s",
			toolgen.SurfaceManifestPath,
			surfaceManifestDiff(committed, live))
	default:
		_, err := opts.stdout.Write(live)
		return err
	}
}

func surfaceManifestDiff(committed, live []byte) string {
	committedRows := decodeRows(committed)
	liveRows := decodeRows(live)

	committedSet := map[string]struct{}{}
	for _, row := range committedRows {
		committedSet[surfaceManifestRowKey(row)] = struct{}{}
	}
	liveSet := map[string]struct{}{}
	for _, row := range liveRows {
		liveSet[surfaceManifestRowKey(row)] = struct{}{}
	}

	var lines []string
	for _, row := range liveRows {
		key := surfaceManifestRowKey(row)
		if _, ok := committedSet[key]; !ok {
			lines = append(lines, "+ "+key)
		}
	}
	for _, row := range committedRows {
		key := surfaceManifestRowKey(row)
		if _, ok := liveSet[key]; !ok {
			lines = append(lines, "- "+key)
		}
	}
	if len(lines) == 0 {
		return "manifest row keys are unchanged; committed JSON differs in metadata or ordering"
	}
	return strings.Join(lines, "\n")
}

func decodeRows(raw []byte) []toolgen.SurfaceManifestRow {
	var manifest toolgen.SurfaceManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil
	}
	return manifest.Rows
}

func surfaceManifestRowKey(row toolgen.SurfaceManifestRow) string {
	return row.Kind + "/" + row.Name
}
