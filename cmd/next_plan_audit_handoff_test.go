package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runNextDiagnostics drives `next --json --diagnostics` for the given change and
// returns the decoded view. --diagnostics selects the full view that carries
// agent constraints (allowed_operations / required_outputs).
func runNextDiagnostics(t *testing.T, root, slug string) nextView {
	t.Helper()
	cmd := commandForRoot(t, root, makeNextCmd())
	cmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())

	var view nextView
	require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
	return view
}

// TestNextBundleHandoffDoesNotAdvertiseEvidenceBeforeAudit asserts that at
// S1_PLAN/bundle the plan-audit handoff does NOT advertise write_evidence /
// evidence_record. `slipway evidence skill --skill plan-audit` fails closed with
// evidence_skill_wrong_plan_substep until the substep advances to audit, so
// advertising evidence recording at bundle is a dead-end handoff. The
// authoritative next action is to run the lifecycle into S1_PLAN/audit.
//
// issue #229
func TestNextBundleHandoffDoesNotAdvertiseEvidenceBeforeAudit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L3", "plan-audit bundle handoff")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.PlanSubStep = model.PlanSubStepBundle
		require.NoError(t, state.SaveChange(root, change))

		view := runNextDiagnostics(t, root, slug)

		require.NotNil(t, view.NextSkill)
		assert.Equal(t, progression.SkillPlanAudit, view.NextSkill.Name)

		require.NotNil(t, view.Constraints)
		assert.NotContains(t, view.Constraints.AllowedOperations, "write_evidence",
			"bundle handoff must not advertise write_evidence before the substep advances to audit")
		assert.NotContains(t, view.Constraints.RequiredOutputs, "evidence_record",
			"bundle handoff must not require an evidence_record the evidence command rejects at bundle")

		warnings := strings.Join(view.Warnings, "\n")
		assert.Contains(t, warnings, "S1_PLAN/bundle")
		assert.Contains(t, warnings, "slipway run", "warning must point to running into S1_PLAN/audit")
		assert.Contains(t, warnings, "S1_PLAN/audit")
		assert.NotContains(t, warnings, "write plan-audit evidence",
			"warning must not offer recording evidence as a direct action at bundle")
	})
}

// TestNextAuditHandoffStillAdvertisesEvidence asserts the audit-substep behavior
// is unchanged: at S1_PLAN/audit, where `slipway evidence skill --skill
// plan-audit` is accepted, the handoff still advertises write_evidence and
// requires the evidence_record output.
//
// issue #229
func TestNextAuditHandoffStillAdvertisesEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L3", "plan-audit audit handoff")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		view := runNextDiagnostics(t, root, slug)

		require.NotNil(t, view.NextSkill)
		assert.Equal(t, progression.SkillPlanAudit, view.NextSkill.Name)

		require.NotNil(t, view.Constraints)
		assert.Contains(t, view.Constraints.AllowedOperations, "write_evidence",
			"audit handoff must still advertise write_evidence where evidence recording is accepted")
		assert.Contains(t, view.Constraints.RequiredOutputs, "evidence_record",
			"audit handoff must still require the evidence_record output")
	})
}
