package wave

import (
	"slices"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waveTaskIDGroups maps a computed plan to per-wave task-ID groups. Wave
// numbering is positional: groups[i] holds the task IDs assigned to wave i+1.
// IDs inside each group are sorted so assertions pin wave membership, not
// intra-wave ordering.
func waveTaskIDGroups(waves []Wave) [][]string {
	groups := make([][]string, 0, len(waves))
	for _, w := range waves {
		ids := make([]string, 0, len(w.Nodes))
		for _, node := range w.Nodes {
			ids = append(ids, node.TaskID)
		}
		slices.Sort(ids)
		groups = append(groups, ids)
	}
	return groups
}

func TestPlanWavesAssignsRootsAndFanInDependent(t *testing.T) {
	t.Parallel()

	// Canonical shape: three independent, file-disjoint tasks plus one task
	// depending on all three. The engine computes waves from depends_on and
	// target_files; nothing is declared. Expected: exactly two waves, wave 1
	// holds exactly the three roots, wave 2 holds the dependent task.
	// Input order is deliberately shuffled: assignment uses task-ID tiebreaks.
	nodes := []Node{
		{TaskID: "t-02", TargetFiles: []string{"b.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "t-04", DependsOn: []string{"t-01", "t-02", "t-03"}, TargetFiles: []string{"d.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "t-01", TargetFiles: []string{"a.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "t-03", TargetFiles: []string{"c.go"}, TaskKind: model.TaskKindCode},
	}

	waves, err := PlanWaves(nodes)
	require.NoError(t, err)
	assert.Equal(t, [][]string{
		{"t-01", "t-02", "t-03"},
		{"t-04"},
	}, waveTaskIDGroups(waves))
}

func TestPlanWavesComputesChainAsSingleTaskWaves(t *testing.T) {
	t.Parallel()

	// Pure depends_on chain t-a <- t-b <- t-c must yield three single-task
	// waves in chain order.
	nodes := []Node{
		{TaskID: "t-c", DependsOn: []string{"t-b"}, TargetFiles: []string{"c.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "t-a", TargetFiles: []string{"a.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "t-b", DependsOn: []string{"t-a"}, TargetFiles: []string{"b.go"}, TaskKind: model.TaskKindCode},
	}

	waves, err := PlanWaves(nodes)
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"t-a"}, {"t-b"}, {"t-c"}}, waveTaskIDGroups(waves))
}

func TestPlanWavesAssignsDependentAfterDeepestDependency(t *testing.T) {
	t.Parallel()

	// wave(task) = max(wave of dependencies) + 1: t-03 depends on both the
	// wave-1 root and the wave-2 task, so it must land in wave 3 (not 2).
	nodes := []Node{
		{TaskID: "t-01", TargetFiles: []string{"a.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "t-02", DependsOn: []string{"t-01"}, TargetFiles: []string{"b.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "t-03", DependsOn: []string{"t-01", "t-02"}, TargetFiles: []string{"c.go"}, TaskKind: model.TaskKindCode},
	}

	waves, err := PlanWaves(nodes)
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"t-01"}, {"t-02"}, {"t-03"}}, waveTaskIDGroups(waves))
}

func TestPlanWavesKeepsFileDisjointSiblingsInOneWave(t *testing.T) {
	t.Parallel()

	// Disjoint sibling files in the same directory are not a target conflict;
	// independent tasks must not be over-serialized.
	nodes := []Node{
		{TaskID: "t-01", TargetFiles: []string{"internal/db/schema.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "t-02", TargetFiles: []string{"internal/db/migrate.go"}, TaskKind: model.TaskKindCode},
	}

	waves, err := PlanWaves(nodes)
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"t-01", "t-02"}}, waveTaskIDGroups(waves))
}

func TestPlanWavesBumpsExactTargetConflictDeterministically(t *testing.T) {
	t.Parallel()

	// Two independent tasks sharing the same target file cannot share a wave:
	// the task with the later task ID is bumped to a later wave. The result
	// must be identical across repeated computations and input orderings.
	makeNodes := func() []Node {
		return []Node{
			{TaskID: "t-01", TargetFiles: []string{"shared.go"}, TaskKind: model.TaskKindCode},
			{TaskID: "t-02", TargetFiles: []string{"shared.go"}, TaskKind: model.TaskKindCode},
		}
	}

	first, err := PlanWaves(makeNodes())
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"t-01"}, {"t-02"}}, waveTaskIDGroups(first))

	for i := 0; i < 10; i++ {
		again, err := PlanWaves(makeNodes())
		require.NoError(t, err)
		assert.Equal(t, first, again, "repeated computation %d must produce an identical plan", i)
	}

	nodes := makeNodes()
	reversed := []Node{nodes[1], nodes[0]}
	flipped, err := PlanWaves(reversed)
	require.NoError(t, err)
	assert.Equal(t, first, flipped, "input order must not change the computed plan")
}

func TestPlanWavesBumpsTaskIDLaterConflictWhenReadinessOrderDiffers(t *testing.T) {
	t.Parallel()

	// t-02 becomes topologically ready before t-01 because t-00 is processed
	// before t-99, but both tasks have the same dependency level and target the
	// same file. Conflict resolution is defined by task ID inside the computed
	// level, so t-01 stays in wave 2 and the later ID t-02 is bumped.
	waves, err := PlanWaves([]Node{
		{TaskID: "t-00", TargetFiles: []string{"dep-a.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "t-99", TargetFiles: []string{"dep-b.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "t-01", DependsOn: []string{"t-99"}, TargetFiles: []string{"shared.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "t-02", DependsOn: []string{"t-00"}, TargetFiles: []string{"shared.go"}, TaskKind: model.TaskKindCode},
	})
	require.NoError(t, err)
	assert.Equal(t, [][]string{
		{"t-00", "t-99"},
		{"t-01"},
		{"t-02"},
	}, waveTaskIDGroups(waves))
}

func TestPlanWavesBumpsConflictingTargetsForEachConflictKind(t *testing.T) {
	t.Parallel()

	// Every targetFilesConflict kind forces the later task ID into a later
	// wave instead of failing the plan: exact path, normalization aliases,
	// case-only alias, parent/child containment, and glob overlap.
	tests := []struct {
		name  string
		left  string // target of t-01
		right string // target of t-02
	}{
		{name: "exact path", left: "internal/db/schema.go", right: "internal/db/schema.go"},
		{name: "dot slash alias", left: "a.go", right: "./a.go"},
		{name: "backslash alias", left: `internal\engine\wave.go`, right: "internal/engine/wave.go"},
		{name: "case only alias", left: "Foo.go", right: "foo.go"},
		{name: "parent directory contains child file", left: "internal/engine/progression", right: "internal/engine/progression/advance.go"},
		{name: "glob overlaps concrete file", left: "internal/engine/wave/*.go", right: "internal/engine/wave/parse.go"},
		{name: "double star overlaps nested file", left: "docs/**", right: "docs/guides/workflow.md"},
		{name: "concrete file overlaps glob", left: "cmd/next.go", right: "cmd/*.go"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			waves, err := PlanWaves([]Node{
				{TaskID: "t-01", TargetFiles: []string{tt.left}, TaskKind: model.TaskKindCode},
				{TaskID: "t-02", TargetFiles: []string{tt.right}, TaskKind: model.TaskKindCode},
			})
			require.NoError(t, err)
			assert.Equal(t, [][]string{{"t-01"}, {"t-02"}}, waveTaskIDGroups(waves),
				"conflicting targets must bump the later task ID to a later wave")
		})
	}
}

func TestPlanWavesCascadesBumpsAcrossSharedTarget(t *testing.T) {
	t.Parallel()

	// Three independent tasks over one shared file serialize into three
	// waves in task-ID order: a bumped task re-checks its destination wave
	// for conflicts and keeps moving forward.
	nodes := []Node{
		{TaskID: "t-03", TargetFiles: []string{"shared.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "t-01", TargetFiles: []string{"shared.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "t-02", TargetFiles: []string{"shared.go"}, TaskKind: model.TaskKindCode},
	}

	waves, err := PlanWaves(nodes)
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"t-01"}, {"t-02"}, {"t-03"}}, waveTaskIDGroups(waves))
}

func TestPlanWavesBumpsLaterTaskIDWhenDeferredRootConflictsWithDependent(t *testing.T) {
	t.Parallel()

	// t-03 is a root, but it cannot stay in wave 1 because it conflicts with
	// t-01. When it becomes eligible for wave 2, t-02 is also eligible through
	// its dependency on t-01. Conflict resolution must still keep the lower task
	// ID in the earlier wave and defer the later ID.
	waves, err := PlanWaves([]Node{
		{TaskID: "t-01", TargetFiles: []string{"shared.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "t-02", DependsOn: []string{"t-01"}, TargetFiles: []string{"shared.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "t-03", TargetFiles: []string{"shared.go"}, TaskKind: model.TaskKindCode},
	})
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"t-01"}, {"t-02"}, {"t-03"}}, waveTaskIDGroups(waves))
}

func TestPlanWavesAllowsSharedTargetAcrossDifferentWaves(t *testing.T) {
	t.Parallel()

	// Conflict bumping is a same-wave concern only: waves run sequentially,
	// so a root task may share a target with a task that depends_on pushed
	// into a later wave. t-03 (b.go) stays in wave 1 even though t-02 also
	// targets b.go from wave 2.
	nodes := []Node{
		{TaskID: "t-01", TargetFiles: []string{"a.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "t-02", DependsOn: []string{"t-01"}, TargetFiles: []string{"b.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "t-03", TargetFiles: []string{"b.go"}, TaskKind: model.TaskKindCode},
	}

	waves, err := PlanWaves(nodes)
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"t-01", "t-03"}, {"t-02"}}, waveTaskIDGroups(waves))
}

func TestPlanWavesAllowsTaskIDThatLooksLikeLegacyRunSuffix(t *testing.T) {
	t.Parallel()

	waves, err := PlanWaves([]Node{
		{TaskID: "task-a__legacy", TargetFiles: []string{"a.go"}, TaskKind: model.TaskKindCode},
	})
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"task-a__legacy"}}, waveTaskIDGroups(waves))
}

func TestPlanWavesRejectsDependencyCycles(t *testing.T) {
	t.Parallel()

	// Contract: a depends_on cycle is a hard error and produces no plan. The
	// error must identify the failure as a cycle (case-insensitive substring
	// "cycle").
	tests := []struct {
		name  string
		nodes []Node
	}{
		{
			name: "two task cycle",
			nodes: []Node{
				{TaskID: "t-01", DependsOn: []string{"t-02"}, TargetFiles: []string{"a.go"}, TaskKind: model.TaskKindCode},
				{TaskID: "t-02", DependsOn: []string{"t-01"}, TargetFiles: []string{"b.go"}, TaskKind: model.TaskKindCode},
			},
		},
		{
			name: "self cycle",
			nodes: []Node{
				{TaskID: "t-01", DependsOn: []string{"t-01"}, TargetFiles: []string{"a.go"}, TaskKind: model.TaskKindCode},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			waves, err := PlanWaves(tt.nodes)
			require.Error(t, err)
			assert.Nil(t, waves, "cycle must produce no plan")
			assert.Contains(t, strings.ToLower(err.Error()), "cycle")
		})
	}
}

func TestPlanWavesRejectsUnknownDependencyReference(t *testing.T) {
	t.Parallel()

	// Contract: an unresolved depends_on reference is a hard error and
	// produces no plan. The error must call the dependency unknown
	// (case-insensitive substring "unknown") and name the missing task ID.
	waves, err := PlanWaves([]Node{
		{TaskID: "t-01", DependsOn: []string{"t-missing"}, TargetFiles: []string{"a.go"}, TaskKind: model.TaskKindCode},
	})
	require.Error(t, err)
	assert.Nil(t, waves, "unknown dependency must produce no plan")
	assert.Contains(t, strings.ToLower(err.Error()), "unknown")
	assert.Contains(t, err.Error(), "t-missing")
}

func TestTaskPlanHashesAreStableForWavelessPlan(t *testing.T) {
	t.Parallel()

	base := `# Tasks

Intro prose that is not task metadata.

- [ ] ` + "`t-01`" + ` Build the parser
  - depends_on: []
  - target_files: ["internal/engine/wave/parse.go"]
  - task_kind: code

- [ ] ` + "`t-02`" + ` Cover the parser
  - depends_on: ["t-01"]
  - target_files: ["internal/engine/wave/parse_test.go"]
  - task_kind: test
`

	// Identical tasks; whitespace-only reformatting of unrelated prose.
	reformatted := `# Tasks


Intro prose that is not task metadata.


- [ ] ` + "`t-01`" + ` Build the parser
  - depends_on: []
  - target_files: ["internal/engine/wave/parse.go"]
  - task_kind: code

- [ ] ` + "`t-02`" + ` Cover the parser
  - depends_on: ["t-01"]
  - target_files: ["internal/engine/wave/parse_test.go"]
  - task_kind: test
`

	hashFns := map[string]func(string) (string, error){
		"structural": TaskPlanStructuralHash,
		"scope":      TaskPlanScopeHash,
		"semantic":   TaskPlanSemanticHash,
	}

	for name, hashFn := range hashFns {
		name, hashFn := name, hashFn
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			first, err := hashFn(base)
			require.NoError(t, err)
			assert.NotEmpty(t, first)

			for i := 0; i < 3; i++ {
				again, err := hashFn(base)
				require.NoError(t, err)
				assert.Equal(t, first, again, "repeated calls must hash identically")
			}

			reformattedHash, err := hashFn(reformatted)
			require.NoError(t, err)
			assert.Equal(t, first, reformattedHash,
				"whitespace-only reformatting of unrelated prose must not change the hash")
		})
	}
}

func TestTaskPlanSemanticHashIgnoresCheckboxState(t *testing.T) {
	t.Parallel()

	unchecked := `# Tasks

- [ ] ` + "`task-a`" + ` Implement A
  - target_files: ["cmd/next.go"]
  - task_kind: code
`
	checked := `# Tasks

- [x] ` + "`task-a`" + ` Implement A
  - target_files: ["cmd/next.go"]
  - task_kind: code
`

	left, err := TaskPlanSemanticHash(unchecked)
	require.NoError(t, err)
	right, err := TaskPlanSemanticHash(checked)
	require.NoError(t, err)
	assert.Equal(t, left, right)
}

func TestTaskPlanSemanticHashAllowsEmptyPlan(t *testing.T) {
	t.Parallel()

	hash, err := TaskPlanSemanticHash("")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
}

func TestTaskPlanSemanticHashRejectsMalformedPlan(t *testing.T) {
	t.Parallel()

	_, err := TaskPlanSemanticHash(`# Tasks

- [ ] ` + "`task-a`" + ` Implement A
  - unexpected_key: value
`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown metadata key")
}

func TestTargetCoversPath(t *testing.T) {
	t.Parallel()

	// Directional coverage: TargetCoversPath(targets, file) reports whether
	// file is covered by ANY target, reusing the planner's conflict predicates
	// (exact match, case-insensitive normalization, parent-directory scope, and
	// glob scope). Coverage is the basis the changed-file scope-escape audit
	// (REQ-002) builds on; conflict detection (REQ-003) shares the same
	// predicates.
	tests := []struct {
		name    string
		targets []string
		file    string
		want    bool
	}{
		{name: "exact match", targets: []string{"internal/a.go"}, file: "internal/a.go", want: true},
		{name: "case insensitive match", targets: []string{"internal/Foo.go"}, file: "internal/foo.go", want: true},
		{name: "normalization alias match", targets: []string{`internal\engine\wave.go`}, file: "internal/engine/wave.go", want: true},
		{name: "parent directory covers child file", targets: []string{"internal/engine/"}, file: "internal/engine/wave/wave.go", want: true},
		{name: "glob covers concrete match", targets: []string{"internal/engine/wave/*.go"}, file: "internal/engine/wave/parse.go", want: true},
		{name: "double star covers nested file", targets: []string{"docs/**"}, file: "docs/guides/workflow.md", want: true},
		{name: "malformed glob covers nothing", targets: []string{"internal/["}, file: "cmd/run.go", want: false},
		{name: "covered by one of several targets", targets: []string{"internal/a.go", "internal/b.go"}, file: "internal/b.go", want: true},
		{name: "file outside all targets", targets: []string{"internal/a.go"}, file: "internal/b.go", want: false},
		{name: "sibling file not covered by concrete target", targets: []string{"internal/engine/wave/wave.go"}, file: "internal/engine/wave/parse.go", want: false},
		{name: "child directory does not cover parent file", targets: []string{"internal/engine/wave/"}, file: "internal/engine/progression.go", want: false},
		{name: "empty targets", targets: nil, file: "internal/a.go", want: false},
		{name: "empty file", targets: []string{"internal/a.go"}, file: "", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, TargetCoversPath(tt.targets, tt.file))
		})
	}
}

func TestMalformedPatternConflictAndCoveragePolicies(t *testing.T) {
	t.Parallel()

	const malformedPattern = "internal/["

	assert.True(t, targetPatternMatches(malformedPattern, "internal/a.go"),
		"static conflict detection must treat malformed glob targets as broad to avoid unsafe parallel packing")
	assert.False(t, targetPatternCovers(malformedPattern, "internal/a.go"),
		"changed-file coverage must not let malformed glob targets authorize arbitrary writes")
}

func TestAnalyzeWaveNarrowingCauses(t *testing.T) {
	t.Parallel()

	// AnalyzeWaveNarrowingCauses surfaces conservative, deterministic,
	// non-blocking cues: broad_target_files for glob/directory targets, and
	// fully_serial_plan ONLY when a linear depends_on chain forces every node
	// onto the dependency critical path. Conflict-driven serialization (tasks
	// sharing a concrete file but no depends_on edge) must NOT be reported as
	// fully_serial_plan (REQ-006 scenario 2).
	tests := []struct {
		name  string
		nodes []Node
		want  []string
	}{
		{
			name: "broad directory target and linear chain report both advisories",
			nodes: []Node{
				{TaskID: "t-01", TargetFiles: []string{"internal/engine/"}},
				{TaskID: "t-02", DependsOn: []string{"t-01"}, TargetFiles: []string{"cmd/next.go"}},
				{TaskID: "t-03", DependsOn: []string{"t-02"}, TargetFiles: []string{"cmd/run.go"}},
			},
			want: []string{"broad_target_files:t-01", "fully_serial_plan"},
		},
		{
			name: "broad glob target is reported",
			nodes: []Node{
				{TaskID: "t-01", TargetFiles: []string{"internal/engine/wave/*.go"}},
				{TaskID: "t-02", TargetFiles: []string{"cmd/next.go"}},
			},
			want: []string{"broad_target_files:t-01"},
		},
		{
			name: "file-conflict serialization does NOT report fully_serial_plan",
			nodes: []Node{
				// No depends_on edges: the only reason these serialize is the
				// shared concrete file target, which is honest scheduling.
				{TaskID: "t-01", TargetFiles: []string{"internal/x.go"}},
				{TaskID: "t-02", TargetFiles: []string{"internal/x.go"}},
				{TaskID: "t-03", TargetFiles: []string{"internal/x.go"}},
			},
			want: nil,
		},
		{
			name: "clean file-disjoint parallel plan yields no advisories",
			nodes: []Node{
				{TaskID: "t-01", TargetFiles: []string{"internal/a.go"}},
				{TaskID: "t-02", TargetFiles: []string{"internal/b.go"}},
				{TaskID: "t-03", TargetFiles: []string{"internal/c.go"}},
			},
			want: nil,
		},
		{
			name: "deterministic broad ordering across multiple broad tasks",
			nodes: []Node{
				{TaskID: "t-03", TargetFiles: []string{"docs/**"}},
				{TaskID: "t-01", TargetFiles: []string{"cmd"}},
				{TaskID: "t-02", TargetFiles: []string{"internal/a.go"}},
			},
			want: []string{"broad_target_files:t-01", "broad_target_files:t-03"},
		},
		{
			name: "dotted directory targets are reported",
			nodes: []Node{
				{TaskID: "t-01", TargetFiles: []string{".github/"}},
				{TaskID: "t-02", TargetFiles: []string{"config.d/"}},
				{TaskID: "t-03", TargetFiles: []string{"cmd/main.go"}},
			},
			want: []string{"broad_target_files:t-01", "broad_target_files:t-02"},
		},
		{
			name: "deep fan-in is not fully serial when depth is below node count",
			nodes: []Node{
				{TaskID: "t-01", TargetFiles: []string{"internal/a.go"}},
				{TaskID: "t-02", TargetFiles: []string{"internal/b.go"}},
				{TaskID: "t-03", TargetFiles: []string{"internal/c.go"}},
				// Depends on the three roots: critical path is 2, node count is
				// 4, so the plan is not fully serial.
				{TaskID: "t-04", DependsOn: []string{"t-01", "t-02", "t-03"}, TargetFiles: []string{"internal/d.go"}},
			},
			want: nil,
		},
		{
			// A single-task plan has nothing to parallelize: critical path 1 ==
			// node count 1 must NOT be reported as fully_serial_plan (acceptance:
			// an honest single-task wave passes).
			name:  "single-task plan is not fully serial",
			nodes: []Node{{TaskID: "t-01", TargetFiles: []string{"internal/a.go"}}},
			want:  nil,
		},
		{
			// Conventional extensionless files name one concrete file, not a
			// directory, so they must NOT raise broad_target_files (REQ-006
			// high-confidence cue); bare extensionless directories like `cmd`
			// above still do.
			name: "conventional extensionless files are not broad targets",
			nodes: []Node{
				{TaskID: "t-01", TargetFiles: []string{"Makefile"}},
				{TaskID: "t-02", TargetFiles: []string{"build/Dockerfile"}},
				{TaskID: "t-03", TargetFiles: []string{".github/CODEOWNERS"}},
			},
			want: nil,
		},
		{
			// Nested extensionless paths are ambiguous: scripts/deploy is commonly
			// an executable file, not a directory. Keep the advisory high-confidence
			// by requiring an explicit trailing slash for nested directory targets.
			name:  "nested extensionless executable path is not broad",
			nodes: []Node{{TaskID: "t-01", TargetFiles: []string{"scripts/deploy"}}},
			want:  nil,
		},
		{
			name:  "empty plan yields no advisories",
			nodes: nil,
			want:  nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := AnalyzeWaveNarrowingCauses(tt.nodes)
			assert.Equal(t, tt.want, got)
		})
	}
}
