package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runInstructions(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := makeInstructionsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestInstructionsReturnsTemplateAndGuidance(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"requirements": "## Requirements",
		"tasks":        "## Task List",
	}
	for name, marker := range cases {
		name, marker := name, marker
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out, err := runInstructions(t, name)
			require.NoError(t, err)
			assert.Contains(t, out, "Authoring instructions")
			assert.Contains(t, out, "## Template")
			assert.Contains(t, out, marker, "should include the artifact template body")
		})
	}
}

func TestInstructionsJSONIncludesTemplateAndGuidance(t *testing.T) {
	t.Parallel()
	out, err := runInstructions(t, "requirements", "--json")
	require.NoError(t, err)

	var view instructionsView
	require.NoError(t, json.Unmarshal([]byte(out), &view))
	assert.Equal(t, "requirements", view.Artifact)
	assert.NotEmpty(t, view.Guidance)
	assert.Contains(t, view.Template, "## Requirements")
	assert.Contains(t, strings.ToUpper(view.Guidance), "MUST")
	// Issue #91: the served template is a rendered exemplar, not the raw
	// Go-template source — its consumer is an authoring skill, so unresolved
	// `{{ … }}` actions must not leak through.
	assert.NotContains(t, view.Template, "{{", "served template must be rendered, not raw Go-template source")
}

func TestInstructionsUnknownArtifactErrors(t *testing.T) {
	t.Parallel()
	_, err := runInstructions(t, "not-an-artifact")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown artifact")
}
