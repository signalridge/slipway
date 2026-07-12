//go:build !windows

package recoverycmd

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandPreservesCompleteArgvThroughPOSIXShell(t *testing.T) {
	t.Parallel()
	values := []string{
		"",
		"plain",
		"spaces and tabs\t",
		`single'and"double`,
		"Unicode 界",
		"carriage\rreturn",
		"line\nfeed",
		"percent%! ampersand& caret^ dollar$ backtick` semicolon; pipe|",
	}
	argv := append([]string{"sh", "-c", `printf '%s\000' "$@"`, "capture"}, values...)
	rendered := Command(argv...)
	output, err := exec.Command("sh", "-c", rendered).Output()
	require.NoError(t, err)
	expected := strings.Join(values, "\x00") + "\x00"
	assert.Equal(t, []byte(expected), output)
}

func TestCommandRendersCompleteHiddenOperationArgv(t *testing.T) {
	t.Parallel()
	argv := []string{
		"slipway", "run", "answer", "--run", "run with spaces", "--action", `action'"界`,
		"--root", "/tmp/root with spaces/%!&^\r\n界", "--confirm-destructive",
		"--scope-sha256", "sha256:" + strings.Repeat("a", 64), "--text", "confirmed\r\n%!&^",
	}
	rendered := Command(argv...)
	assert.NotEmpty(t, rendered)
	assert.NotContains(t, rendered, "<answer>")
	assert.NotContains(t, rendered, "<file>")
}
