package toolgen

import (
	"fmt"
	"os"
	"testing"
)

// TestMain hardens the whole package against the class of bug where a test
// silently rewrites the developer's real Codex prompt store. Codex prompts are
// host-global (resolved from $CODEX_HOME/prompts, else ~/.codex/prompts), so any
// test that generates Codex surfaces without an explicit sandbox would mutate
// the host. Defaulting CODEX_HOME to a throwaway directory makes that
// impossible; individual tests still override it with t.Setenv when they need to
// assert on the generated prompts.
//
// If the sandbox cannot be established the package aborts rather than running
// with an unset CODEX_HOME — running on would leave the very escape hatch this
// guard exists to close, letting a Codex-generating test write to the real
// ~/.codex store.
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
