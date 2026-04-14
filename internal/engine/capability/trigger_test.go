package capability

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTriggerClauseValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		clause  TriggerClause
		wantErr string
	}{
		{
			name: "command requires value",
			clause: TriggerClause{
				Op:     OpCommand,
				Reason: "why",
			},
			wantErr: "command requires a value",
		},
		{
			name: "reason required at top level",
			clause: TriggerClause{
				Op:    OpCommand,
				Value: "review",
			},
			wantErr: "reason is required",
		},
		{
			name: "all_of needs children",
			clause: TriggerClause{
				Op:     OpAllOf,
				Reason: "why",
			},
			wantErr: "all_of requires children",
		},
		{
			name: "unknown operator rejected",
			clause: TriggerClause{
				Op:     Operator("path_matches"),
				Value:  "foo",
				Reason: "why",
			},
			wantErr: "unknown operator",
		},
		{
			name: "valid all_of accepted",
			clause: TriggerClause{
				Op: OpAllOf,
				Children: []TriggerClause{
					{Op: OpCommand, Value: "review"},
					{Op: OpChangedFilesInclude, Value: "*.go"},
				},
				Reason: "valid",
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.clause.validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestTriggerClauseMatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		clause TriggerClause
		sig    Signals
		want   bool
	}{
		{
			name:   "command match",
			clause: TriggerClause{Op: OpCommand, Value: "review", Reason: "r"},
			sig:    Signals{Command: "review"},
			want:   true,
		},
		{
			name:   "command mismatch",
			clause: TriggerClause{Op: OpCommand, Value: "review", Reason: "r"},
			sig:    Signals{Command: "validate"},
			want:   false,
		},
		{
			name: "all_of requires every child",
			clause: TriggerClause{
				Op: OpAllOf,
				Children: []TriggerClause{
					{Op: OpCommand, Value: "review"},
					{Op: OpHost, Value: "code-quality-review"},
				},
				Reason: "r",
			},
			sig:  Signals{Command: "review", Host: "code-quality-review"},
			want: true,
		},
		{
			name: "any_of matches one",
			clause: TriggerClause{
				Op: OpAnyOf,
				Children: []TriggerClause{
					{Op: OpHost, Value: "goal-verification"},
					{Op: OpHost, Value: "final-closeout"},
				},
				Reason: "r",
			},
			sig:  Signals{Host: "final-closeout"},
			want: true,
		},
		{
			name: "not inverts",
			clause: TriggerClause{
				Op: OpNot,
				Children: []TriggerClause{
					{Op: OpHost, Value: "tdd-governance"},
				},
				Reason: "r",
			},
			sig:  Signals{Host: "code-quality-review"},
			want: true,
		},
		{
			name:   "changed_files_include glob",
			clause: TriggerClause{Op: OpChangedFilesInclude, Value: "*.go", Reason: "r"},
			sig:    Signals{ChangedFiles: []string{"registry.go"}},
			want:   true,
		},
		{
			name:   "changed_files_include dir-prefix",
			clause: TriggerClause{Op: OpChangedFilesInclude, Value: "docs/plans/**/*", Reason: "r"},
			sig:    Signals{ChangedFiles: []string{"docs/plans/2026-04-11-foo.md"}},
			want:   true,
		},
		{
			name:   "blocker_reason match",
			clause: TriggerClause{Op: OpBlockerReason, Value: "missing_red_proof", Reason: "r"},
			sig:    Signals{Blockers: []string{"missing_red_proof"}},
			want:   true,
		},
		{
			name:   "user_text_matches is case-insensitive",
			clause: TriggerClause{Op: OpUserTextMatches, Value: "UNCLEAR", Reason: "r"},
			sig:    Signals{UserText: "scope is unclear"},
			want:   true,
		},
		{
			name:   "path_includes substring",
			clause: TriggerClause{Op: OpPathIncludes, Value: ".github/workflows", Reason: "r"},
			sig:    Signals{Paths: []string{".github/workflows/ci.yml"}},
			want:   true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.clause.Match(tc.sig))
		})
	}
}

func TestMatchGlobAny_SupportsDoubleStarAndBraceExpansion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		pattern string
		files   []string
		want    bool
	}{
		{
			name:    "double-star matches nested markdown under docs/plans",
			pattern: "docs/plans/**/*.md",
			files:   []string{"docs/plans/2026-04-11.md", "docs/plans/archive/2026/a.md"},
			want:    true,
		},
		{
			name:    "double-star auth pattern matches root and nested auth dirs",
			pattern: "**/auth/*",
			files:   []string{"auth/login.go", "pkg/service/auth/token.go"},
			want:    true,
		},
		{
			name:    "github workflows double-star matches deep tree",
			pattern: ".github/workflows/**/*",
			files:   []string{".github/workflows/security/nightly/scan.yml"},
			want:    true,
		},
		{
			name:    "brace expansion matches either yaml extension",
			pattern: "**/*.{yml,yaml}",
			files:   []string{"ci/workflow.yaml"},
			want:    true,
		},
		{
			name:    "single-star does not cross directory boundaries",
			pattern: "*.go",
			files:   []string{"internal/engine/capability/trigger.go"},
			want:    false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := matchGlobAny(tc.files, tc.pattern, nil)
			assert.Equal(t, tc.want, got)
		})
	}
}
