package toolgen

import (
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
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "slipway-toolgen-codex-home")
	if err == nil {
		_ = os.Setenv("CODEX_HOME", dir)
	}
	code := m.Run()
	if dir != "" {
		_ = os.RemoveAll(dir)
	}
	os.Exit(code)
}
