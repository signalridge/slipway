package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedIssue275UnknownMetadataKey seeds an S2_IMPLEMENT change with a materialized
// wave plan and recorded execution evidence, then injects an unknown task
// metadata key into tasks.md so repair/validate/run/next observe tasks.md
// parse-failure drift (issue #275 residual root cause).
func seedIssue275UnknownMetadataKey(t *testing.T, root string) string {
	t.Helper()
	slug := createGovernedRequest(t, root, "L2", "issue275 repair drift guidance")
	issue227SeedTwoWaveExecution(t, root, slug)
	for _, tc := range []struct{ id, file string }{
		{"task-01", "cmd/checkpoint.go"},
		{"task-02", "cmd/evidence.go"},
	} {
		c := commandForRoot(t, root, makeEvidenceCmd())
		c.SetArgs([]string{
			"task", "--change", slug, "--task-id", tc.id,
			"--run-summary-version", "1", "--task-kind", "code", "--verdict", "pass",
			"--evidence-ref", "test:" + tc.id, "--changed-file", tc.file, "--target-file", tc.file,
		})
		require.NoError(t, c.Execute())
	}
	sk := commandForRoot(t, root, makeEvidenceCmd())
	sk.SetArgs([]string{
		"skill", "--change", slug, "--skill", progression.SkillWaveOrchestration,
		"--verdict", model.VerificationVerdictPass, "--reference", "wave-orchestration:pass", "--notes", "ok",
	})
	require.NoError(t, sk.Execute())

	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	bundlePath, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md",
		[]byte("\n"+
			"- [ ] `task-01` first wave task\n"+
			"  - depends_on: []\n"+
			"  - target_files: [\"cmd/checkpoint.go\"]\n"+
			"  - task_kind: code\n\n"+
			"- [ ] `task-02` second wave task\n"+
			"  - depends_on: [\"task-01\"]\n"+
			"  - target_files: [\"cmd/evidence.go\"]\n"+
			"  - task_kind: code\n"+
			"  - scope_amendment: x\n")))
	return slug
}

func repairUnknownKeyDrift(t *testing.T, root string) (repairSummary, *repairDriftFinding) {
	t.Helper()
	rc := commandForRoot(t, root, makeRepairCmd())
	rc.SetArgs([]string{"--json"})
	var rbuf bytes.Buffer
	rc.SetOut(&rbuf)
	rc.SetErr(&rbuf)
	require.NoError(t, rc.Execute(), "repair: %s", rbuf.String())

	var sum repairSummary
	require.NoError(t, json.Unmarshal(rbuf.Bytes(), &sum))
	for i := range sum.UnrepairedDrift {
		if strings.Contains(sum.UnrepairedDrift[i].Reason, "unknown metadata key") {
			return sum, &sum.UnrepairedDrift[i]
		}
	}
	return sum, nil
}

// TestIssue275RepairDriftRoutesToFixingTasks asserts the issue #275 residual fix:
// `slipway repair --json` drift guidance for a tasks.md unknown-metadata-key parse
// failure routes the operator to fixing tasks.md, not to the generic "run slipway
// run" default (REQ-001), and stays fail-closed (REQ-002): a second repair still
// reports the same drift rather than silently mutating tasks.md.
func TestIssue275RepairDriftRoutesToFixingTasks(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		seedIssue275UnknownMetadataKey(t, root)

		sum, found := repairUnknownKeyDrift(t, root)
		require.NotNilf(t, found, "expected an unknown-metadata-key drift finding; got %#v", sum.UnrepairedDrift)
		assert.Contains(t, found.NextAction, "tasks.md", "repair drift must route to fixing tasks.md")
		assert.NotContains(t, found.NextAction, "slipway run",
			"repair drift for a tasks.md parse failure must not route to the generic slipway run default")

		// REQ-002 fail-closed: repair must not silently fix/mutate governed
		// tasks.md — a second repair still surfaces the same unrepaired drift.
		_, again := repairUnknownKeyDrift(t, root)
		assert.NotNil(t, again, "repair must stay fail-closed: the unknown-key drift persists, not silently repaired")
	})
}

// TestIssue275ParseFailureGuidanceConsistentAcrossSurfaces asserts REQ-003: none of
// repair/validate/run/next route this drift class to the misleading default
// "...repair the current lifecycle evidence and continue alignment" guidance, and
// each surface that reports the drift points the operator at tasks.md.
func TestIssue275ParseFailureGuidanceConsistentAcrossSurfaces(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := seedIssue275UnknownMetadataKey(t, root)

		const bugPhrase = "repair the current lifecycle evidence and continue alignment"
		for _, tc := range []struct {
			name string
			cmd  *cobra.Command
			args []string
		}{
			{"repair", makeRepairCmd(), []string{"--json"}},
			{"validate", makeValidateCmd(), []string{"--json", "--change", slug}},
			{"run", makeRunCmd(), []string{"--json", "--change", slug}},
			{"next", makeNextCmd(), []string{"--json", "--change", slug}},
		} {
			c := commandForRoot(t, root, tc.cmd)
			c.SetArgs(tc.args)
			var buf bytes.Buffer
			c.SetOut(&buf)
			c.SetErr(&buf)
			// Some surfaces exit non-zero on a fail-closed blocker; assert on output.
			_ = c.Execute()
			out := buf.String()
			assert.NotContainsf(t, out, bugPhrase,
				"%s must not emit the misleading default guidance for tasks.md parse-failure drift", tc.name)
			// Each surface must point at the actionable cause, not a generic
			// dead-end: repair/validate name tasks.md directly, while run/next
			// fail closed with a wave-plan-derivation error that names the
			// unknown metadata key the operator must remove from tasks.md.
			actionable := strings.Contains(out, "tasks.md") || strings.Contains(out, "unknown metadata key")
			assert.Truef(t, actionable,
				"%s guidance for this drift should point at the tasks.md unknown-metadata-key cause; got: %s", tc.name, out)
		}
	})
}
