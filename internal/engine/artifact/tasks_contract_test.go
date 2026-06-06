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
}

func TestEvaluateTasksContract(t *testing.T) {
	t.Parallel()

	write := func(t *testing.T, content string) (string, string) {
		t.Helper()
		root := t.TempDir()
		slug := "tasks-contract"
		bundleDir := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.MkdirAll(bundleDir, 0o755))
		if content != "" {
			require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(content), 0o644))
		}
		return bundleDir, slug
	}

	t.Run("missing", func(t *testing.T) {
		t.Parallel()
		bundleDir, slug := write(t, "")
		res, err := EvaluateTasksContract(bundleDir, slug)
		require.NoError(t, err)
		assert.Equal(t, TasksContractStatusMissing, res.Status)
	})

	t.Run("invalid placeholder", func(t *testing.T) {
		t.Parallel()
		bundleDir, slug := write(t, "# Tasks\n## Task List\n- [ ] `t-01` Pending task objective\n  - wave: 1\n  - covers: [REQ-001]\n")
		res, err := EvaluateTasksContract(bundleDir, slug)
		require.NoError(t, err)
		assert.Equal(t, TasksContractStatusInvalid, res.Status)
		assert.Contains(t, res.Message, "placeholder")
	})

	t.Run("valid authored", func(t *testing.T) {
		t.Parallel()
		bundleDir, slug := write(t, "# Tasks\n## Task List\n- [ ] `t-01` Implement the tasks substance validator and wire it into validate\n  - wave: 1\n  - target_files: [\"internal/engine/artifact/tasks_contract.go\"]\n  - covers: [REQ-001]\n")
		res, err := EvaluateTasksContract(bundleDir, slug)
		require.NoError(t, err)
		assert.Equal(t, TasksContractStatusValid, res.Status)
	})

	t.Run("valid authored keeping guidance comment", func(t *testing.T) {
		t.Parallel()
		bundleDir, slug := write(t, "# Tasks\n\n## Task List\n\n<!--\nReplace the seeded placeholder objective below; a placeholder tasks list\n(\"Pending task objective\") is rejected by the tasks substance gate.\n-->\n\n- [ ] `t-01` Implement the tasks substance validator and wire it into validate\n  - wave: 1\n  - target_files: [\"internal/engine/artifact/tasks_contract.go\"]\n  - covers: [REQ-001]\n")
		res, err := EvaluateTasksContract(bundleDir, slug)
		require.NoError(t, err)
		assert.Equal(t, TasksContractStatusValid, res.Status)
	})

	t.Run("invalid no checklist task", func(t *testing.T) {
		t.Parallel()
		bundleDir, slug := write(t, "# Tasks\n\n## Task List\n\nThe author will do the work later.\n")
		res, err := EvaluateTasksContract(bundleDir, slug)
		require.NoError(t, err)
		assert.Equal(t, TasksContractStatusInvalid, res.Status)
		assert.Contains(t, res.Message, "no checklist task")
	})
}
