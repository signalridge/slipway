package bad

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSourceGrep(t *testing.T) {
	raw, err := os.ReadFile("internal/state/delete.go")
	if err != nil {
		t.Fatal(err)
	}

	source := string(raw)
	if !strings.Contains(source, "func Delete") { // want "source-grep test reads .go files"
		t.Fatal("missing delete implementation")
	}
}

func TestSourceGrepInlineConversion(t *testing.T) {
	raw, err := os.ReadFile("cmd/root.go")
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(raw), "cobra.Command") { // want "source-grep test reads .go files"
		t.Fatal("source grep")
	}
}

func TestElapsedAssertion(t *testing.T) {
	start := time.Now()
	if time.Since(start) > time.Second { // want "elapsed/timing assertion uses time.Since"
		t.Fatal("too slow")
	}
}

func TestElapsedVariableComparison(t *testing.T) {
	start := time.Now()
	elapsed := time.Since(start)
	if elapsed < 10*time.Millisecond { // want "elapsed/timing assertion uses time.Since"
		t.Fatal("too fast")
	}
}

func TestAssertLessTimeSince(t *testing.T) {
	start := time.Now()
	assert.Less(t, time.Since(start), time.Second) // want "elapsed/timing assertion uses time.Since"
}

func TestRequireLessElapsedVariable(t *testing.T) {
	start := time.Now()
	elapsed := time.Since(start)
	require.Less(t, elapsed, time.Second) // want "elapsed/timing assertion uses time.Since"
}

func TestAssertGreaterOrEqualElapsedVariable(t *testing.T) {
	start := time.Now()
	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, time.Millisecond) // want "elapsed/timing assertion uses time.Since"
}
