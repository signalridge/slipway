package artifact

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskSubstanceBlockers(t *testing.T) {
	t.Parallel()

	assert.NotEmpty(t, TaskSubstanceBlockers(""), "empty tasks must be rejected")

	mechanical := "# Tasks\n## Task List\n- [ ] `t-01` Pending task objective\n  - wave: 1\n  - covers: [REQ-001]\n"
	assert.NotEmpty(t, TaskSubstanceBlockers(mechanical), "placeholder objective must be rejected")

	copiedInstructionObjective := "# Tasks\n## Task List\n- [ ] `t-01` <concrete task objective>\n  - wave: 1\n  - target_files: [\"internal/engine/artifact/tasks_contract.go\"]\n  - covers: [REQ-001]\n"
	assert.NotEmpty(t, TaskSubstanceBlockers(copiedInstructionObjective),
		"copied instructions objective placeholder must be rejected")

	authored := "# Tasks\n## Task List\n- [ ] `t-01` Implement the substance gate in requirements_contract.go\n  - wave: 1\n  - target_files: [\"internal/engine/artifact/requirements_contract.go\"]\n  - covers: [REQ-001]\n"
	assert.Empty(t, TaskSubstanceBlockers(authored), "authored tasks must pass")

	// Blocker #1 regression (issue #91): the rendered authoring-guidance comment
	// may itself mention the placeholder sentinel. Because the gate parses the
	// checklist instead of scanning the whole file, an authored tasks.md that
	// keeps the comment must still pass.
	withComment := "# Tasks\n\n## Task List\n\n<!--\nReplace the seeded placeholder objective below; a placeholder tasks list\n(\"Pending task objective\") is rejected by the tasks substance gate.\n-->\n\n- [ ] `t-01` Implement the parser-based tasks substance gate\n  - wave: 1\n  - target_files: [\"internal/engine/artifact/tasks_contract.go\"]\n  - covers: [REQ-001]\n"
	assert.Empty(t, TaskSubstanceBlockers(withComment), "authored tasks retaining the guidance comment must pass")

	// Blocker #2 regression (issue #91): non-empty prose with no checklist task is
	// not substantive and must not be reported valid.
	noTasks := "# Tasks\n\n## Task List\n\nThe author will do the work later.\n"
	assert.NotEmpty(t, TaskSubstanceBlockers(noTasks), "tasks.md with no checklist task must be rejected")

	// A tasks.md that violates the checkbox-native contract must be rejected
	// rather than silently passing.
	malformed := "# Tasks\n\n## Task List\n\n- [ ] `t-01` Do the thing\n  - bogus_key: nope\n"
	assert.NotEmpty(t, TaskSubstanceBlockers(malformed), "unparseable tasks.md must be rejected")

	// issue #119: target_files are a kind-agnostic execution/scope boundary.
	// This code task is just the smallest fixture for the missing-target case.
	// The objective is concrete, so the target_files blocker is the only one.
	codeNoTargets := "# Tasks\n## Task List\n- [ ] `t-01` Implement the parser-based code gate in tasks_contract.go\n  - wave: 1\n  - task_kind: code\n  - covers: [REQ-001]\n"
	codeNoTargetsBlockers := TaskSubstanceBlockers(codeNoTargets)
	require.Len(t, codeNoTargetsBlockers, 1, "a code task with no target_files yields exactly the target_files blocker")
	assert.Contains(t, codeNoTargetsBlockers[0], "target_files",
		"the blocker must name the missing target_files")

	// The same code task passes once it names the files it will change.
	codeWithTargets := "# Tasks\n## Task List\n- [ ] `t-01` Implement the parser-based code gate in tasks_contract.go\n  - wave: 1\n  - target_files: [\"internal/engine/artifact/tasks_contract.go\"]\n  - task_kind: code\n  - covers: [REQ-001]\n"
	assert.Empty(t, TaskSubstanceBlockers(codeWithTargets), "a code task naming target_files must pass")

	// target_files are the execution/scope boundary for every task kind, not
	// just code tasks. A verification task with no target_files would otherwise
	// make the contract view say "valid" while plan readiness blocks it.
	verificationNoTargets := "# Tasks\n## Task List\n- [ ] `t-01` Verify the code gate rejects an empty target_files list\n  - wave: 1\n  - task_kind: verification\n  - covers: [REQ-001]\n"
	verificationNoTargetsBlockers := TaskSubstanceBlockers(verificationNoTargets)
	require.Len(t, verificationNoTargetsBlockers, 1, "a non-code task with no target_files yields exactly the target_files blocker")
	assert.Contains(t, verificationNoTargetsBlockers[0], "target_files",
		"the blocker must name the missing target_files")

	placeholderTargets := "# Tasks\n## Task List\n- [ ] `t-01` Implement the parser-based code gate in tasks_contract.go\n  - wave: 1\n  - target_files: [<path/to/file.go>]\n  - task_kind: code\n  - covers: [REQ-001]\n"
	placeholderTargetBlockers := TaskSubstanceBlockers(placeholderTargets)
	require.Len(t, placeholderTargetBlockers, 1, "a placeholder target_files entry yields exactly the target_files blocker")
	assert.Contains(t, placeholderTargetBlockers[0], "placeholder target_files",
		"the blocker must name the copied target_files placeholder")

	template, err := RenderArtifactExample("tasks.md")
	require.NoError(t, err)
	assert.Contains(t, template, "<!--", "regression fixture should exercise authoring guidance comments")
	assert.NotEmpty(t, TaskSubstanceBlockers(template), "instructions template comments must not satisfy the tasks gate")
}

func TestEvaluateTasksContract(t *testing.T) {
	t.Parallel()

	write := func(t *testing.T, content string) string {
		t.Helper()
		root := t.TempDir()
		slug := "tasks-contract"
		bundleDir := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.MkdirAll(bundleDir, 0o755))
		if content != "" {
			require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(content), 0o644))
		}
		return bundleDir
	}

	t.Run("missing", func(t *testing.T) {
		t.Parallel()
		bundleDir := write(t, "")
		res, err := EvaluateTasksContract(bundleDir)
		require.NoError(t, err)
		assert.Equal(t, TasksContractStatusMissing, res.Status)
	})

	t.Run("invalid placeholder", func(t *testing.T) {
		t.Parallel()
		bundleDir := write(t, "# Tasks\n## Task List\n- [ ] `t-01` Pending task objective\n  - wave: 1\n  - covers: [REQ-001]\n")
		res, err := EvaluateTasksContract(bundleDir)
		require.NoError(t, err)
		assert.Equal(t, TasksContractStatusInvalid, res.Status)
		assert.Contains(t, res.Message, "placeholder")
	})

	t.Run("valid authored", func(t *testing.T) {
		t.Parallel()
		bundleDir := write(t, "# Tasks\n## Task List\n- [ ] `t-01` Implement the tasks substance validator and wire it into validate\n  - wave: 1\n  - target_files: [\"internal/engine/artifact/tasks_contract.go\"]\n  - covers: [REQ-001]\n")
		res, err := EvaluateTasksContract(bundleDir)
		require.NoError(t, err)
		assert.Equal(t, TasksContractStatusValid, res.Status)
	})

	t.Run("valid authored keeping guidance comment", func(t *testing.T) {
		t.Parallel()
		bundleDir := write(t, "# Tasks\n\n## Task List\n\n<!--\nReplace the seeded placeholder objective below; a placeholder tasks list\n(\"Pending task objective\") is rejected by the tasks substance gate.\n-->\n\n- [ ] `t-01` Implement the tasks substance validator and wire it into validate\n  - wave: 1\n  - target_files: [\"internal/engine/artifact/tasks_contract.go\"]\n  - covers: [REQ-001]\n")
		res, err := EvaluateTasksContract(bundleDir)
		require.NoError(t, err)
		assert.Equal(t, TasksContractStatusValid, res.Status)
	})

	t.Run("invalid no checklist task", func(t *testing.T) {
		t.Parallel()
		bundleDir := write(t, "# Tasks\n\n## Task List\n\nThe author will do the work later.\n")
		res, err := EvaluateTasksContract(bundleDir)
		require.NoError(t, err)
		assert.Equal(t, TasksContractStatusInvalid, res.Status)
		assert.Contains(t, res.Message, "no checklist task")
	})

	t.Run("invalid instructions template", func(t *testing.T) {
		t.Parallel()
		template, err := RenderArtifactExample("tasks.md")
		require.NoError(t, err)
		bundleDir := write(t, template)
		res, err := EvaluateTasksContract(bundleDir)
		require.NoError(t, err)
		assert.Equal(t, TasksContractStatusInvalid, res.Status)
		assert.Contains(t, res.Message, "no checklist task")
	})

	t.Run("invalid code task missing target_files", func(t *testing.T) {
		t.Parallel()
		bundleDir := write(t, "# Tasks\n## Task List\n- [ ] `t-01` Implement the parser-based code gate in tasks_contract.go\n  - wave: 1\n  - task_kind: code\n  - covers: [REQ-001]\n")
		res, err := EvaluateTasksContract(bundleDir)
		require.NoError(t, err)
		assert.Equal(t, TasksContractStatusInvalid, res.Status)
		assert.Contains(t, res.Message, "target_files")
	})

	t.Run("invalid verification task missing target_files", func(t *testing.T) {
		t.Parallel()
		bundleDir := write(t, "# Tasks\n## Task List\n- [ ] `t-01` Verify target_files contract parity\n  - wave: 1\n  - task_kind: verification\n  - covers: [REQ-001]\n")
		res, err := EvaluateTasksContract(bundleDir)
		require.NoError(t, err)
		assert.Equal(t, TasksContractStatusInvalid, res.Status)
		assert.Contains(t, res.Message, "target_files")
	})

	t.Run("invalid placeholder target_files", func(t *testing.T) {
		t.Parallel()
		bundleDir := write(t, "# Tasks\n## Task List\n- [ ] `t-01` Implement target_files placeholder rejection\n  - wave: 1\n  - target_files: [<path/to/file.go>]\n  - task_kind: code\n  - covers: [REQ-001]\n")
		res, err := EvaluateTasksContract(bundleDir)
		require.NoError(t, err)
		assert.Equal(t, TasksContractStatusInvalid, res.Status)
		assert.Contains(t, res.Message, "placeholder target_files")
	})
}
