package toolgen

import (
	"fmt"
	"os"
	"testing"
)

// TestMain hardens the whole package against the class of bug where a test
// silently mutates the developer's real legacy Codex prompt store. Codex command
// surfaces are project skills now, but refresh still prunes retired host-global
// slipway-* prompts from $CODEX_HOME/prompts (else ~/.codex/prompts). Defaulting
// CODEX_HOME to a throwaway directory makes that cleanup safe; individual tests
// still override it with t.Setenv when they need to assert on legacy cleanup.
//
// If the sandbox cannot be established the package aborts rather than running
// with an unset CODEX_HOME — running on would leave the very escape hatch this
// guard exists to close, letting a Codex-generating test prune the real ~/.codex
// prompt store.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "slipway-toolgen-codex-home")
	if err != nil {
		fmt.Fprintf(os.Stderr, "toolgen TestMain: cannot create CODEX_HOME sandbox: %v\n", err)
		os.Exit(1)
	}
	if err := os.Setenv("CODEX_HOME", dir); err != nil {
		fmt.Fprintf(os.Stderr, "toolgen TestMain: cannot set CODEX_HOME: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
