package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func makeFindVariantCmd() *cobra.Command {
	var opts variantOptions
	cmd := &cobra.Command{
		Use:   "find-variant --engine=<codeql|semgrep> --language=<lang>",
		Short: "Emit starter variant-analysis query scaffolds",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFindVariant(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.engine, "engine", "", "Variant engine: codeql or semgrep")
	cmd.Flags().StringVar(&opts.language, "language", "", "Target language")
	cmd.Flags().StringVar(&opts.seedFile, "seed-file", "", "Seed vulnerable file path")
	cmd.Flags().StringVar(&opts.seedLine, "seed-line", "", "Seed vulnerable line")
	cmd.Flags().StringVar(&opts.variantName, "variant-name", "", "Variant query name")
	cmd.Flags().StringVar(&opts.originalBug, "original-bug", "", "Original bug identifier")
	return cmd
}

type variantOptions struct {
	engine      string
	language    string
	seedFile    string
	seedLine    string
	variantName string
	originalBug string
}

func runFindVariant(cmd *cobra.Command, opts variantOptions) error {
	opts.engine = strings.ToLower(strings.TrimSpace(opts.engine))
	opts.language = strings.ToLower(strings.TrimSpace(opts.language))
	if opts.engine == "" || opts.language == "" {
		return newInvalidUsageError("find_variant_missing_required_flags", "error: --engine and --language are required", "Pass --engine=<codeql|semgrep> and --language=<lang>.", nil)
	}
	opts.seedFile = defaultString(opts.seedFile, "TODO-seed-file")
	opts.seedLine = defaultString(opts.seedLine, "TODO-seed-line")
	opts.variantName = defaultString(opts.variantName, "TODO-variant-name")
	opts.originalBug = defaultString(opts.originalBug, "TODO-original-bug-id")

	var body string
	switch opts.engine {
	case "codeql":
		body = codeqlVariantScaffold(opts)
	case "semgrep":
		body = semgrepVariantScaffold(opts)
	default:
		return newInvalidUsageError("find_variant_unsupported", fmt.Sprintf("unsupported engine/language pair: %s/%s", opts.engine, opts.language), "Supported engines are codeql and semgrep.", nil)
	}
	if body == "" {
		return newInvalidUsageError("find_variant_unsupported", fmt.Sprintf("unsupported engine/language pair: %s/%s", opts.engine, opts.language), "Use a supported language for the selected engine.", nil)
	}
	fmt.Fprint(cmd.OutOrStdout(), body)
	return nil
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func codeqlVariantScaffold(opts variantOptions) string {
	// Each language maps to its top-level import plus the correct taint-tracking
	// module path. The module path is not uniform: Python's modern dataflow lives
	// under `.new`, and Java/C++ sit under `semmle.code.<lang>` rather than
	// `semmle.<lang>`. The TaintTracking import is a best-effort starting point;
	// pin it to the CodeQL library version in use when filling the TODOs.
	lang, ok := map[string]struct{ topImport, taintImport string }{
		"python":     {"python", "semmle.python.dataflow.new.TaintTracking"},
		"go":         {"go", "semmle.go.dataflow.TaintTracking"},
		"java":       {"java", "semmle.code.java.dataflow.TaintTracking"},
		"javascript": {"javascript", "semmle.javascript.dataflow.TaintTracking"},
		"cpp":        {"cpp", "semmle.code.cpp.dataflow.TaintTracking"},
	}[opts.language]
	if !ok {
		return ""
	}
	return fmt.Sprintf(`/**
 * @name %s
 * @description Variants of %s
 * @kind path-problem
 * @problem.severity error
 * @precision high
 * @tags security variant-analysis
 */

// Seed: %s:%s
// TODO(seed): paste the minimized original vulnerable snippet here.

import %s
import %s

// TODO(source): narrow to the exact accessor path the seed exercises.
// TODO(sink): pin the dangerous API the seed calls.
// TODO(sanitizer): model every real mitigation used in this repo.

from TaintTracking::PathNode src, TaintTracking::PathNode snk
where src = snk
select snk, src, snk, "TODO: replace with real variant dataflow."
`, opts.variantName, opts.originalBug, opts.seedFile, opts.seedLine, lang.topImport, lang.taintImport)
}

func semgrepVariantScaffold(opts variantOptions) string {
	switch opts.language {
	case "python", "go", "java", "javascript", "cpp":
	default:
		return ""
	}
	return fmt.Sprintf(`rules:
  - id: variant-taint-%s
    mode: taint
    languages: [%s]
    message: "TODO: variant of %s from %s:%s"
    severity: ERROR
    pattern-sources:
      - pattern: |
          TODO(source)
    pattern-sinks:
      - pattern: |
          TODO(sink)
    pattern-sanitizers:
      - pattern: |
          TODO(sanitizer)
`, opts.language, opts.language, opts.originalBug, opts.seedFile, opts.seedLine)
}
