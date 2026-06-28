package artifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecisionSubstanceBlockers(t *testing.T) {
	t.Parallel()

	// Regression (issue #119): the unedited template served by `slipway
	// instructions decision` is the scaffold the authoring skill must replace —
	// every required section holds only its <!-- ... --> guidance comment, so the
	// substance gate must reject it.
	template, err := RenderArtifactExample("decision.md")
	require.NoError(t, err)
	templateBlockers := DecisionSubstanceBlockers(template)
	require.Len(t, templateBlockers, 1,
		"comment-only required sections must fail closed at the structure layer")
	assert.Contains(t, templateBlockers[0], "decision_structure_invalid:")
	assert.Contains(t, templateBlockers[0], "non-empty content")

	// Legacy scaffold regression: existing active changes may still carry the
	// pre-#119 seeded prose. It is not an HTML comment, but it is still unwritten
	// scaffold and must not satisfy the decision contract.
	legacySeeded := "# Decision\n\n" +
		"## Alternatives Considered\nPending investigation. Replace with concrete alternatives, tradeoffs, and the selected direction after research or code inspection.\n\n" +
		"## Selected Approach\nPending investigation. Record the selected approach only after the alternatives have concrete evidence.\n\n" +
		"## Interfaces and Data Flow\nPending investigation. Name changed interfaces and data flows, or write \"none\" after inspection.\n\n" +
		"## Rollout and Rollback\nPending investigation. Write the concrete rollback path and verification command after implementation scope is known.\n\n" +
		"## Risk\nPending investigation. List concrete risks only after inspecting the affected code and contracts.\n"
	legacyBlockers := DecisionSubstanceBlockers(legacySeeded)
	require.Len(t, legacyBlockers, 5, "every legacy seeded decision section must be flagged")
	assert.Contains(t, legacyBlockers, "decision_section_placeholder:## Alternatives Considered")
	assert.Contains(t, legacyBlockers, "decision_section_placeholder:## Selected Approach")
	assert.Contains(t, legacyBlockers, "decision_section_placeholder:## Interfaces and Data Flow")
	assert.Contains(t, legacyBlockers, "decision_section_placeholder:## Rollout and Rollback")
	assert.Contains(t, legacyBlockers, "decision_section_placeholder:## Risk")

	// An authored decision with real content in every section passes.
	authored := "# Decision\n\n" +
		"## Alternatives Considered\nA) keep polling; B) push via webhook. B wins on latency.\n\n" +
		"## Selected Approach\nWebhook push, grounded in alternative B above.\n\n" +
		"## Interfaces and Data Flow\nAdds POST /hook; events flow producer -> queue -> consumer.\n\n" +
		"## Rollout and Rollback\nFeature flag webhook_push; rollback flips it off. Verify with `go test ./...`.\n\n" +
		"## Risk\nDuplicate delivery; mitigated by idempotency keys.\n"
	assert.Empty(t, DecisionSubstanceBlockers(authored), "an authored decision must pass")

	// Authored sections that keep the guidance comment alongside real prose pass:
	// the floor strips HTML comments and judges the remaining authored text. A
	// terse "none" for Interfaces and Data Flow is a legitimate answer.
	authoredKeepingComments := "# Decision\n\n" +
		"## Alternatives Considered\n<!-- At least two real approaches with tradeoffs. -->\nA vs B; B wins.\n\n" +
		"## Selected Approach\n<!-- guidance -->\nB.\n\n" +
		"## Interfaces and Data Flow\nnone\n\n" +
		"## Rollout and Rollback\nFlag flip; verify `go test ./...`.\n\n" +
		"## Risk\nDuplicate delivery; idempotency keys.\n"
	assert.Empty(t, DecisionSubstanceBlockers(authoredKeepingComments),
		"authored sections keeping the guidance comment must pass")

	// A missing required section is rejected by the structure check.
	missingSection := "# Decision\n\n## Alternatives Considered\nA vs B.\n\n" +
		"## Selected Approach\nB.\n\n## Interfaces and Data Flow\nnone\n\n## Risk\nlow.\n"
	missingBlockers := DecisionSubstanceBlockers(missingSection)
	require.NotEmpty(t, missingBlockers, "a missing required section must be rejected")
	assert.Contains(t, missingBlockers[0], "decision_structure_invalid")

	// A structurally empty section is rejected.
	emptySection := "# Decision\n\n## Alternatives Considered\n\n## Selected Approach\nB.\n\n" +
		"## Interfaces and Data Flow\nnone\n\n## Rollout and Rollback\nFlag.\n\n## Risk\nLow.\n"
	assert.NotEmpty(t, DecisionSubstanceBlockers(emptySection), "a structurally empty section must be rejected")
}

func TestReadArtifactContractSource(t *testing.T) {
	t.Parallel()

	bundleDir := t.TempDir()
	sourcePath := ResolveArtifactPath(bundleDir, "decision.md")
	source, ok, err := readArtifactContractSource(bundleDir, "decision.md")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, sourcePath, source.Path)
	assert.Empty(t, source.Content)

	require.NoError(t, os.WriteFile(sourcePath, []byte("# Decision\n\nAuthored content.\n"), 0o644))
	source, ok, err = readArtifactContractSource(bundleDir, "decision.md")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, sourcePath, source.Path)
	assert.Equal(t, "# Decision\n\nAuthored content.\n", source.Content)
}

func TestEvaluateDecisionContract(t *testing.T) {
	t.Parallel()

	write := func(t *testing.T, content string) string {
		t.Helper()
		root := t.TempDir()
		slug := "decision-contract"
		bundleDir := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.MkdirAll(bundleDir, 0o755))
		if content != "" {
			require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "decision.md"), []byte(content), 0o644))
		}
		return bundleDir
	}

	t.Run("missing", func(t *testing.T) {
		t.Parallel()
		bundleDir := write(t, "")
		res, err := EvaluateDecisionContract(bundleDir)
		require.NoError(t, err)
		assert.Equal(t, DecisionContractStatusMissing, res.Status)
	})

	t.Run("invalid template-only", func(t *testing.T) {
		t.Parallel()
		template, err := RenderArtifactExample("decision.md")
		require.NoError(t, err)
		bundleDir := write(t, template)
		res, err := EvaluateDecisionContract(bundleDir)
		require.NoError(t, err)
		assert.Equal(t, DecisionContractStatusInvalid, res.Status)
		assert.Contains(t, res.Message, "decision_structure_invalid")
	})

	t.Run("valid authored", func(t *testing.T) {
		t.Parallel()
		authored := "# Decision\n\n## Alternatives Considered\nA vs B; B wins.\n\n" +
			"## Selected Approach\nB.\n\n## Interfaces and Data Flow\nnone\n\n" +
			"## Rollout and Rollback\nFlag flip; verify tests.\n\n## Risk\nLow.\n"
		bundleDir := write(t, authored)
		res, err := EvaluateDecisionContract(bundleDir)
		require.NoError(t, err)
		assert.Equal(t, DecisionContractStatusValid, res.Status)
	})
}

func TestParseDecisionContractStatus(t *testing.T) {
	t.Parallel()

	authored := func(statusSection string) string {
		return "# Decision\n\n" +
			statusSection +
			"## Alternatives Considered\nA vs B; B wins.\n\n" +
			"## Selected Approach\nB.\n\n" +
			"## Interfaces and Data Flow\nnone\n\n" +
			"## Rollout and Rollback\nFlag flip; verify tests.\n\n" +
			"## Risk\nLow.\n"
	}

	tests := []struct {
		name          string
		content       string
		expectedBlock string
	}{
		{
			name:    "missing status stays compatible",
			content: authored(""),
		},
		{
			name:    "accepted status is live",
			content: authored("## Status\nAccepted\n\n"),
		},
		{
			name:    "status label accepts live status",
			content: authored("## Status\nStatus: Accepted\n\n"),
		},
		{
			name:    "earlier live status remains compatible before later live status",
			content: authored("## Status\nAccepted\n\n## State\nProposed\n\n"),
		},
		{
			name:    "state alias parses proposed",
			content: authored("## State\nProposed\n\n"),
		},
		{
			name:          "superseded status is rejected",
			content:       authored("## Lifecycle\nSuperseded by DEC-001\n\n"),
			expectedBlock: "decision_status_rejected:superseded",
		},
		{
			name:          "deprecated status is rejected",
			content:       authored("## Stage\nDeprecated\n\n"),
			expectedBlock: "decision_status_rejected:deprecated",
		},
		{
			name:          "mixed live and superseded status is rejected",
			content:       authored("## Status\nAccepted, superseded by DEC-001\n\n"),
			expectedBlock: "decision_status_rejected:superseded",
		},
		{
			name:          "later lifecycle alias can reject accepted status",
			content:       authored("## Status\nAccepted\n\n## Lifecycle\nSuperseded by DEC-001\n\n"),
			expectedBlock: "decision_status_rejected:superseded",
		},
		{
			name:          "earlier rejected status wins over later unknown status",
			content:       authored("## Status\nSuperseded by DEC-001\n\n## State\nRetired-ish\n\n"),
			expectedBlock: "decision_status_rejected:superseded",
		},
		{
			name:          "later state alias can fail closed after accepted status",
			content:       authored("## Status\nAccepted\n\n## State\nRetired-ish\n\n"),
			expectedBlock: "decision_status_unknown:retired ish",
		},
		{
			name:          "lowercase status heading is explicit",
			content:       authored("## status\nSuperseded\n\n"),
			expectedBlock: "decision_status_rejected:superseded",
		},
		{
			name:          "punctuated status heading is explicit",
			content:       authored("## Status:\nRetired-ish\n\n"),
			expectedBlock: "decision_status_unknown:retired ish",
		},
		{
			name:          "uppercase lifecycle heading with closing hashes is explicit",
			content:       authored("## LIFECYCLE ##\nDeprecated\n\n"),
			expectedBlock: "decision_status_rejected:deprecated",
		},
		{
			name:          "unknown explicit status fails closed",
			content:       authored("## Status\nRetired-ish\n\n"),
			expectedBlock: "decision_status_unknown:retired ish",
		},
		{
			name:          "inactive is not active",
			content:       authored("## Status\nInactive\n\n"),
			expectedBlock: "decision_status_unknown:inactive",
		},
		{
			name:          "unaccepted is not accepted",
			content:       authored("## Status\nunaccepted\n\n"),
			expectedBlock: "decision_status_unknown:unaccepted",
		},
		{
			name:          "drafted is not draft",
			content:       authored("## Status\ndrafted\n\n"),
			expectedBlock: "decision_status_unknown:drafted",
		},
		{
			name:          "empty explicit status fails closed",
			content:       authored("## Status\n\n"),
			expectedBlock: "decision_status_unknown:empty",
		},
		{
			name:          "empty status alias can fail closed before accepted status",
			content:       authored("## Status\n\n## State\nAccepted\n\n"),
			expectedBlock: "decision_status_unknown:empty",
		},
		{
			name:          "comment only status alias can fail closed before accepted status",
			content:       authored("## Status\n<!-- guidance -->\n\n## State\nAccepted\n\n"),
			expectedBlock: "decision_status_unknown:empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parsed := ParseDecisionContract(tt.content)

			require.NotEmpty(t, parsed.Decisions)
			assert.Contains(t, parsed.Decisions[0], "Selected Approach:")
			if tt.expectedBlock == "" {
				assert.Empty(t, parsed.StatusBlockers)
			} else {
				assert.Contains(t, parsed.StatusBlockers, tt.expectedBlock)
			}
		})
	}
}

func TestShouldRejectDecisionStatusNormalizationProperties(t *testing.T) {
	t.Parallel()

	rejectedStatuses := []string{"superseded", "deprecated", "rejected"}
	variants := []func(string) string{
		func(s string) string { return s },
		strings.ToUpper,
		func(s string) string { return "  " + s + "  " },
		func(s string) string { return s + "." },
		func(s string) string { return "[" + s + "]" },
		func(s string) string { return "status: " + s },
		func(s string) string { return "accepted, " + s + " by DEC-001" },
	}

	for _, status := range rejectedStatuses {
		for _, variant := range variants {
			input := variant(status)
			t.Run(input, func(t *testing.T) {
				t.Parallel()
				assert.True(t, ShouldRejectDecisionStatus(input), "variant %q must reject", input)
			})
		}
	}

	for _, live := range []string{"accepted", "approved", "proposed", "draft", "active", ""} {
		t.Run("live "+live, func(t *testing.T) {
			t.Parallel()
			assert.False(t, ShouldRejectDecisionStatus(live))
		})
	}
}
