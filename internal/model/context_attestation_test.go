package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextOriginHandlesFromVerification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		record VerificationRecord
		wantOK bool
		want   map[string]string
	}{
		{
			name: "valid single token",
			record: VerificationRecord{
				References: []string{"context_origin:stage=review=ctx-abc"},
			},
			wantOK: true,
			want:   map[string]string{"review": "ctx-abc"},
		},
		{
			name: "multiple distinct stages",
			record: VerificationRecord{
				References: []string{
					"context_origin:stage=review=ctx-abc",
					"context_origin:stage=executor=ctx-def",
				},
			},
			wantOK: true,
			want:   map[string]string{"review": "ctx-abc", "executor": "ctx-def"},
		},
		{
			name: "handle containing equals splits on first only",
			record: VerificationRecord{
				References: []string{"context_origin:stage=review=ctx=99"},
			},
			wantOK: true,
			want:   map[string]string{"review": "ctx=99"},
		},
		{
			name: "surrounding punctuation and quotes trimmed",
			record: VerificationRecord{
				References: []string{"`context_origin:stage=review=ctx-abc`."},
			},
			wantOK: true,
			want:   map[string]string{"review": "ctx-abc"},
		},
		{
			name: "no token present",
			record: VerificationRecord{
				References: []string{"tool:go-test"},
			},
			wantOK: false,
		},
		{
			name: "wrong prefix",
			record: VerificationRecord{
				References: []string{"review_origin:skill=x=y"},
			},
			wantOK: false,
		},
		{
			name: "missing handle",
			record: VerificationRecord{
				References: []string{"context_origin:stage=review="},
			},
			wantOK: false,
		},
		{
			name: "missing stage",
			record: VerificationRecord{
				References: []string{"context_origin:stage==ctx"},
			},
			wantOK: false,
		},
		{
			name: "repeated identical token is idempotent",
			record: VerificationRecord{
				References: []string{
					"context_origin:stage=review=ctx-abc",
					"context_origin:stage=review=ctx-abc",
				},
			},
			wantOK: true,
			want:   map[string]string{"review": "ctx-abc"},
		},
		{
			name: "two different handles for same stage fail closed",
			record: VerificationRecord{
				References: []string{
					"context_origin:stage=review=ctx-abc",
					"context_origin:stage=review=ctx-def",
				},
			},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := ContextOriginHandlesFromVerification(tt.record)
			require.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				assert.Nil(t, got)
				return
			}
			want := make(map[string]string, len(tt.want))
			for stage, handle := range tt.want {
				want[stage] = handle
			}
			gotMap := make(map[string]string, len(got))
			for stage, h := range got {
				assert.Equal(t, stage, h.Stage)
				gotMap[stage] = h.Handle
			}
			assert.Equal(t, want, gotMap)
		})
	}
}

func TestPlanOriginHandleFromVerification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		record     VerificationRecord
		wantOK     bool
		wantHandle string
	}{
		{
			name: "valid plan origin token",
			record: VerificationRecord{
				References: []string{"plan_origin:ctx-author"},
			},
			wantOK:     true,
			wantHandle: "ctx-author",
		},
		{
			name: "surrounding punctuation trimmed",
			record: VerificationRecord{
				References: []string{"`plan_origin:ctx-author`."},
			},
			wantOK:     true,
			wantHandle: "ctx-author",
		},
		{
			name: "handle with embedded colon preserved",
			record: VerificationRecord{
				References: []string{"plan_origin:ctx:author:1"},
			},
			wantOK:     true,
			wantHandle: "ctx:author:1",
		},
		{
			name: "no token present",
			record: VerificationRecord{
				References: []string{"tool:go-test"},
			},
			wantOK: false,
		},
		{
			name: "empty handle fails",
			record: VerificationRecord{
				References: []string{"plan_origin:"},
			},
			wantOK: false,
		},
		{
			name: "repeated identical token is idempotent",
			record: VerificationRecord{
				References: []string{
					"plan_origin:ctx-author",
					"plan_origin:ctx-author",
				},
			},
			wantOK:     true,
			wantHandle: "ctx-author",
		},
		{
			name: "two different handles fail closed",
			record: VerificationRecord{
				References: []string{
					"plan_origin:ctx-author",
					"plan_origin:ctx-other",
				},
			},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := PlanOriginHandleFromVerification(tt.record)
			require.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				assert.Equal(t, ContextOriginHandle{}, got)
				return
			}
			assert.Equal(t, StageContextPlanOrigin, got.Stage)
			assert.Equal(t, tt.wantHandle, got.Handle)
		})
	}
}

func TestAuditOriginHandleFromVerification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		record     VerificationRecord
		wantOK     bool
		wantHandle string
	}{
		{
			name: "valid audit origin token",
			record: VerificationRecord{
				References: []string{"audit_origin:ctx-auditor"},
			},
			wantOK:     true,
			wantHandle: "ctx-auditor",
		},
		{
			name: "no token present",
			record: VerificationRecord{
				References: []string{"plan_origin:ctx-author"},
			},
			wantOK: false,
		},
		{
			name: "empty handle fails",
			record: VerificationRecord{
				References: []string{"audit_origin:"},
			},
			wantOK: false,
		},
		{
			name: "two different handles fail closed",
			record: VerificationRecord{
				References: []string{
					"audit_origin:ctx-auditor",
					"audit_origin:ctx-other",
				},
			},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := AuditOriginHandleFromVerification(tt.record)
			require.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				assert.Equal(t, ContextOriginHandle{}, got)
				return
			}
			assert.Equal(t, StageContextAuditOrigin, got.Stage)
			assert.Equal(t, tt.wantHandle, got.Handle)
		})
	}
}

func TestReviewContextOriginHandleFromVerification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		record     VerificationRecord
		wantOK     bool
		wantHandle string
	}{
		{
			name: "valid review origin token",
			record: VerificationRecord{
				References: []string{"context_origin:stage=review=ctx-reviewer"},
			},
			wantOK:     true,
			wantHandle: "ctx-reviewer",
		},
		{
			name: "other context origin stages do not satisfy review origin",
			record: VerificationRecord{
				References: []string{"context_origin:stage=executor=ctx-executor"},
			},
			wantOK: false,
		},
		{
			name: "conflicting review handles fail closed",
			record: VerificationRecord{
				References: []string{
					"context_origin:stage=review=ctx-reviewer-a",
					"context_origin:stage=review=ctx-reviewer-b",
				},
			},
			wantOK: false,
		},
		{
			name: "review handle may coexist with non-review stages",
			record: VerificationRecord{
				References: []string{
					"context_origin:stage=review=ctx-reviewer",
					"context_origin:stage=executor=ctx-executor",
				},
			},
			wantOK:     true,
			wantHandle: "ctx-reviewer",
		},
		{
			name: "review handle reads through coexisting multiple distinct fix handles",
			record: VerificationRecord{
				References: []string{
					"context_origin:stage=review=ctx-review",
					"context_origin:stage=fix=ctx-fix-1",
					"context_origin:stage=fix=ctx-fix-2",
				},
			},
			wantOK:     true,
			wantHandle: "ctx-review",
		},
		{
			name: "record with only multiple fix handles has no review handle without failing closed",
			record: VerificationRecord{
				References: []string{
					"context_origin:stage=fix=ctx-fix-1",
					"context_origin:stage=fix=ctx-fix-2",
				},
			},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := ReviewContextOriginHandleFromVerification(tt.record)
			require.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				assert.Equal(t, ContextOriginHandle{}, got)
				return
			}
			assert.Equal(t, StageContextReview, got.Stage)
			assert.Equal(t, tt.wantHandle, got.Handle)
		})
	}
}

func TestFixContextOriginHandleSetFromVerification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		record VerificationRecord
		want   map[string]struct{}
	}{
		{
			name: "single fix handle flattened to a set",
			record: VerificationRecord{
				References: []string{"context_origin:stage=fix=ctx-fix-1"},
			},
			want: map[string]struct{}{"ctx-fix-1": {}},
		},
		{
			name: "distinct fix handles deduplicated",
			record: VerificationRecord{
				References: []string{
					"context_origin:stage=fix=ctx-fix-1",
					"context_origin:stage=fix=ctx-fix-2",
					"context_origin:stage=fix=ctx-fix-1",
				},
			},
			want: map[string]struct{}{"ctx-fix-1": {}, "ctx-fix-2": {}},
		},
		{
			name: "fix handles coexisting with other stages are still collected",
			record: VerificationRecord{
				References: []string{
					"context_origin:stage=review=ctx-review",
					"context_origin:stage=fix=ctx-fix-1",
					"context_origin:stage=fix=ctx-fix-2",
				},
			},
			want: map[string]struct{}{"ctx-fix-1": {}, "ctx-fix-2": {}},
		},
		{
			name: "no fix token yields non-nil empty set",
			record: VerificationRecord{
				References: []string{"context_origin:stage=review=ctx-review"},
			},
			want: map[string]struct{}{},
		},
		{
			name: "empty references yields non-nil empty set",
			record: VerificationRecord{
				References: nil,
			},
			want: map[string]struct{}{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := FixContextOriginHandleSetFromVerification(tt.record)
			assert.NotNil(t, got)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExecutorParticipantHandleSetFromVerification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		record VerificationRecord
		want   map[string]struct{}
	}{
		{
			name: "single executor handle flattened to a set",
			record: VerificationRecord{
				References: []string{"executor_agent:wave=1:task=t-01:ctx-exec-a"},
			},
			want: map[string]struct{}{"ctx-exec-a": {}},
		},
		{
			name: "distinct handles across waves and tasks deduped",
			record: VerificationRecord{
				References: []string{
					"executor_agent:wave=1:task=t-01:ctx-exec-a",
					"executor_agent:wave=1:task=t-02:ctx-exec-b",
					"executor_agent:wave=2:task=t-03:ctx-exec-a",
				},
			},
			want: map[string]struct{}{"ctx-exec-a": {}, "ctx-exec-b": {}},
		},
		{
			name: "blank collapse value from conflicting handles is dropped",
			record: VerificationRecord{
				References: []string{
					"executor_agent:wave=1:task=t-01:ctx-exec-a",
					"executor_agent:wave=1:task=t-01:ctx-exec-b",
				},
			},
			want: map[string]struct{}{},
		},
		{
			name: "conflict drop leaves a valid sibling handle",
			record: VerificationRecord{
				References: []string{
					"executor_agent:wave=1:task=t-01:ctx-exec-a",
					"executor_agent:wave=1:task=t-01:ctx-exec-b",
					"executor_agent:wave=1:task=t-02:ctx-exec-c",
				},
			},
			want: map[string]struct{}{"ctx-exec-c": {}},
		},
		{
			name: "no executor tokens yields empty set",
			record: VerificationRecord{
				References: []string{"tool:go-test"},
			},
			want: map[string]struct{}{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ExecutorParticipantHandleSetFromVerification(tt.record)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCrossStageContextCollisions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		participants map[string]ContextParticipant
		ownedStages  map[string]struct{}
		want         [][2]string
	}{
		{
			name: "two single-handle participants sharing a handle collide",
			participants: map[string]ContextParticipant{
				StageContextGoal:   {Handle: "ctx-shared"},
				StageContextReview: {Handle: "ctx-shared"},
			},
			ownedStages: map[string]struct{}{StageContextGoal: {}, StageContextReview: {}},
			want:        [][2]string{{StageContextGoal, StageContextReview}},
		},
		{
			name: "distinct single handles do not collide",
			participants: map[string]ContextParticipant{
				StageContextGoal:   {Handle: "ctx-a"},
				StageContextReview: {Handle: "ctx-b"},
			},
			ownedStages: map[string]struct{}{StageContextGoal: {}, StageContextReview: {}},
			want:        nil,
		},
		{
			name: "single handle inside executor set collides",
			participants: map[string]ContextParticipant{
				StageContextReview:   {Handle: "ctx-exec-a"},
				StageContextExecutor: {HandleSet: map[string]struct{}{"ctx-exec-a": {}, "ctx-exec-b": {}}},
			},
			ownedStages: map[string]struct{}{StageContextReview: {}},
			want:        [][2]string{{StageContextExecutor, StageContextReview}},
		},
		{
			name: "single handle outside executor set does not collide",
			participants: map[string]ContextParticipant{
				StageContextReview:   {Handle: "ctx-fresh"},
				StageContextExecutor: {HandleSet: map[string]struct{}{"ctx-exec-a": {}}},
			},
			ownedStages: map[string]struct{}{StageContextReview: {}},
			want:        nil,
		},
		{
			name: "collision on an edge with no owned endpoint is filtered out",
			participants: map[string]ContextParticipant{
				StageContextGoal:     {Handle: "ctx-shared"},
				StageContextReview:   {Handle: "ctx-shared"},
				StageContextCloseout: {Handle: "ctx-closeout"},
			},
			ownedStages: map[string]struct{}{StageContextCloseout: {}},
			want:        nil,
		},
		{
			name: "collision retained when one endpoint is owned",
			participants: map[string]ContextParticipant{
				StageContextReview:   {Handle: "ctx-shared"},
				StageContextGoal:     {Handle: "ctx-shared"},
				StageContextCloseout: {Handle: "ctx-closeout"},
			},
			ownedStages: map[string]struct{}{StageContextGoal: {}},
			want:        [][2]string{{StageContextGoal, StageContextReview}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := CrossStageContextCollisions(tt.participants, tt.ownedStages)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDegradedDispatchJustificationsFromVerification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		record VerificationRecord
		want   map[int]struct{}
	}{
		{
			name: "valid justification",
			record: VerificationRecord{
				References: []string{"degraded_dispatch_justification:wave=2:tool_unavailable=Task tool unreachable"},
			},
			want: map[int]struct{}{2: {}},
		},
		{
			name: "multiple waves",
			record: VerificationRecord{
				References: []string{
					"degraded_dispatch_justification:wave=1:tool_unavailable=Task tool unreachable",
					"degraded_dispatch_justification:wave=3:tool_unavailable=spawn failed",
				},
			},
			want: map[int]struct{}{1: {}, 3: {}},
		},
		{
			name: "empty detail is ignored",
			record: VerificationRecord{
				References: []string{"degraded_dispatch_justification:wave=1:tool_unavailable="},
			},
			want: nil,
		},
		{
			name: "missing tool_unavailable segment is ignored",
			record: VerificationRecord{
				References: []string{"degraded_dispatch_justification:wave=1"},
			},
			want: nil,
		},
		{
			name: "wave index zero is ignored",
			record: VerificationRecord{
				References: []string{"degraded_dispatch_justification:wave=0:tool_unavailable=Task tool unreachable"},
			},
			want: nil,
		},
		{
			name: "non-numeric wave index is ignored",
			record: VerificationRecord{
				References: []string{"degraded_dispatch_justification:wave=two:tool_unavailable=Task tool unreachable"},
			},
			want: nil,
		},
		{
			name: "no matching token",
			record: VerificationRecord{
				References: []string{"dispatch_mode:wave=1:degraded_sequential"},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := DegradedDispatchJustificationsFromVerification(tt.record)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestContextAttestationPrefixConstsArePinned(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "context_origin:stage=", ContextOriginReferencePrefix)
	assert.Equal(t, "plan_origin:", PlanOriginReferencePrefix)
	assert.Equal(t, "audit_origin:", AuditOriginReferencePrefix)
	assert.Equal(t, "degraded_dispatch_justification:wave=", WaveDegradedJustificationReferencePrefix)

	assert.Equal(t, "executor", StageContextExecutor)
	assert.Equal(t, "plan_origin", StageContextPlanOrigin)
	assert.Equal(t, "audit_origin", StageContextAuditOrigin)
	assert.Equal(t, "review", StageContextReview)
	assert.Equal(t, "fix", StageContextFix)
	assert.Equal(t, "goal", StageContextGoal)
	assert.Equal(t, "closeout", StageContextCloseout)
}
