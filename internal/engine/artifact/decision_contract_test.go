package artifact

import (
	"os"
	"path/filepath"
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
	require.Len(t, templateBlockers, 5, "every comment-only required section must be flagged")
	for _, b := range templateBlockers {
		assert.Contains(t, b, "decision_section_placeholder:", "the raw instructions decision template must not pass")
	}

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
		assert.Contains(t, res.Message, "decision_section_placeholder")
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
