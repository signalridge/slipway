package good

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBehaviorOutputContainsStatus(t *testing.T) {
	output := renderStatus()
	if !strings.Contains(output, "done-ready") {
		t.Fatal("missing status")
	}
}

func TestGeneratedSurfaceContract(t *testing.T) {
	surface := renderGeneratedSurface()
	if !strings.Contains(surface, "slipway run") {
		t.Fatal("missing generated command")
	}
}

func TestGoldenOutput(t *testing.T) {
	golden, err := os.ReadFile("testdata/status.golden")
	if err != nil {
		t.Fatal(err)
	}

	if got := renderStatus(); got != string(golden) {
		t.Fatalf("renderStatus() = %q, want golden output", got)
	}
}

func TestContractText(t *testing.T) {
	payload := renderJSONContract()
	if !strings.Contains(payload, `"state":"done_ready"`) {
		t.Fatal("missing contract field")
	}
}

func TestOrderingAssertionOnNonTimingValues(t *testing.T) {
	assert.Less(t, 1, 2)
}

func renderStatus() string {
	return "state: done-ready"
}

func renderGeneratedSurface() string {
	return "usage: slipway run --json"
}

func renderJSONContract() string {
	return `{"state":"done_ready"}`
}
