package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewOriginHandleFromVerification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		record     VerificationRecord
		wantOK     bool
		wantSkill  string
		wantHandle string
	}{
		{
			name: "valid single token",
			record: VerificationRecord{
				References: []string{"review_origin:skill=spec-compliance-review=ctx-abc"},
			},
			wantOK:     true,
			wantSkill:  "spec-compliance-review",
			wantHandle: "ctx-abc",
		},
		{
			name: "handle containing equals splits on first only",
			record: VerificationRecord{
				References: []string{"review_origin:skill=code-quality-review=ctx=99"},
			},
			wantOK:     true,
			wantSkill:  "code-quality-review",
			wantHandle: "ctx=99",
		},
		{
			name: "surrounding punctuation and quotes trimmed",
			record: VerificationRecord{
				References: []string{"`review_origin:skill=spec-compliance-review=ctx-abc`."},
			},
			wantOK:     true,
			wantSkill:  "spec-compliance-review",
			wantHandle: "ctx-abc",
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
				References: []string{"review_context:skill=x=y"},
			},
			wantOK: false,
		},
		{
			name: "missing handle",
			record: VerificationRecord{
				References: []string{"review_origin:skill=spec-compliance-review="},
			},
			wantOK: false,
		},
		{
			name: "missing skill",
			record: VerificationRecord{
				References: []string{"review_origin:skill==ctx"},
			},
			wantOK: false,
		},
		{
			name: "repeated identical token is idempotent",
			record: VerificationRecord{
				References: []string{
					"review_origin:skill=spec-compliance-review=ctx-abc",
					"review_origin:skill=spec-compliance-review=ctx-abc",
				},
			},
			wantOK:     true,
			wantSkill:  "spec-compliance-review",
			wantHandle: "ctx-abc",
		},
		{
			name: "two different handles fail closed",
			record: VerificationRecord{
				References: []string{
					"review_origin:skill=spec-compliance-review=ctx-abc",
					"review_origin:skill=spec-compliance-review=ctx-def",
				},
			},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := ReviewOriginHandleFromVerification(tt.record)
			require.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				assert.Equal(t, ReviewOriginHandle{}, got)
				return
			}
			assert.Equal(t, tt.wantSkill, got.Skill)
			assert.Equal(t, tt.wantHandle, got.Handle)
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

	assert.Equal(t, "review_origin:skill=", ReviewOriginReferencePrefix)
	assert.Equal(t, "degraded_dispatch_justification:wave=", WaveDegradedJustificationReferencePrefix)
}
