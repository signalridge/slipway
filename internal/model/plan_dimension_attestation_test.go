package model

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanDimensionAttestationsFromVerification(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, root, "internal/engine/progression/advance_governed.go")
	writeTestFile(t, root, "internal/model/placeholder.go")
	writeTestFile(t, root, "internal/todo/queue.go")
	writeTestFile(t, root, "artifacts/changes/example/requirements.md")

	tests := []struct {
		name      string
		record    VerificationRecord
		wantCodes []string
		want      map[PlanDimensionName]string
	}{
		{
			name: "valid required attestations",
			record: VerificationRecord{References: []string{
				"dim:decision_soundness=pass:internal/engine/progression/advance_governed.go:1305",
				"dim:consistency=pass:artifacts/changes/example/requirements.md#requirements",
			}},
			want: map[PlanDimensionName]string{
				PlanDimensionDecisionSoundness: PlanDimensionVerdictPass,
				PlanDimensionConsistency:       PlanDimensionVerdictPass,
			},
		},
		{
			name: "line ranges and columns are stripped before path resolution",
			record: VerificationRecord{References: []string{
				"dim:decision_soundness=pass:internal/engine/progression/advance_governed.go:1305-1320",
				"dim:consistency=pass:artifacts/changes/example/requirements.md:12:5",
			}},
			want: map[PlanDimensionName]string{
				PlanDimensionDecisionSoundness: PlanDimensionVerdictPass,
				PlanDimensionConsistency:       PlanDimensionVerdictPass,
			},
		},
		{
			name: "line and column ranges are stripped before path resolution",
			record: VerificationRecord{References: []string{
				"dim:decision_soundness=pass:internal/engine/progression/advance_governed.go:1305:10-1320:4",
				"dim:consistency=pass:artifacts/changes/example/requirements.md:12-14",
			}},
			want: map[PlanDimensionName]string{
				PlanDimensionDecisionSoundness: PlanDimensionVerdictPass,
				PlanDimensionConsistency:       PlanDimensionVerdictPass,
			},
		},
		{
			name: "duplicate same verdict is idempotent",
			record: VerificationRecord{References: []string{
				"dim:consistency=pass:artifacts/changes/example/requirements.md",
				"dim:consistency=pass:artifacts/changes/example/requirements.md",
			}},
			wantCodes: []string{"plan_dimension_decision_soundness_unattested"},
			want: map[PlanDimensionName]string{
				PlanDimensionConsistency: PlanDimensionVerdictPass,
			},
		},
		{
			name: "conflicting verdict fails closed",
			record: VerificationRecord{References: []string{
				"dim:consistency=pass:artifacts/changes/example/requirements.md",
				"dim:consistency=fail:artifacts/changes/example/requirements.md",
			}},
			wantCodes: []string{
				"plan_dimension_attestation_conflict",
				"plan_dimension_decision_soundness_unattested",
			},
		},
		{
			name: "malformed token rejected",
			record: VerificationRecord{References: []string{
				"dim:consistency-pass:artifacts/changes/example/requirements.md",
			}},
			wantCodes: []string{
				"plan_dimension_attestation_invalid",
				"plan_dimension_decision_soundness_unattested",
				"plan_dimension_consistency_unattested",
			},
		},
		{
			name: "placeholder evidence rejected",
			record: VerificationRecord{References: []string{
				"dim:consistency=pass:<path/to/evidence>",
			}},
			wantCodes: []string{
				"plan_dimension_attestation_invalid",
				"plan_dimension_decision_soundness_unattested",
			},
		},
		{
			name: "legitimate todo and placeholder path segments are allowed",
			record: VerificationRecord{References: []string{
				"dim:decision_soundness=pass:internal/todo/queue.go",
				"dim:consistency=pass:internal/model/placeholder.go",
			}},
			want: map[PlanDimensionName]string{
				PlanDimensionDecisionSoundness: PlanDimensionVerdictPass,
				PlanDimensionConsistency:       PlanDimensionVerdictPass,
			},
		},
		{
			name: "traversal evidence rejected",
			record: VerificationRecord{References: []string{
				"dim:consistency=pass:../requirements.md",
			}},
			wantCodes: []string{
				"plan_dimension_attestation_invalid",
				"plan_dimension_decision_soundness_unattested",
			},
		},
		{
			name: "absolute evidence rejected",
			record: VerificationRecord{References: []string{
				"dim:consistency=pass:/tmp/requirements.md",
			}},
			wantCodes: []string{
				"plan_dimension_attestation_invalid",
				"plan_dimension_decision_soundness_unattested",
			},
		},
		{
			name: "unresolvable evidence rejected distinctly",
			record: VerificationRecord{References: []string{
				"dim:consistency=pass:docs/missing.md",
			}},
			wantCodes: []string{
				"plan_dimension_attestation_evidence_unresolvable",
				"plan_dimension_decision_soundness_unattested",
			},
		},
		{
			name: "decision soundness may not cite artifacts evidence",
			record: VerificationRecord{References: []string{
				"dim:decision_soundness=pass:artifacts/changes/example/requirements.md",
				"dim:consistency=pass:artifacts/changes/example/requirements.md",
			}},
			wantCodes: []string{"plan_dimension_decision_soundness_evidence_invalid"},
			want: map[PlanDimensionName]string{
				PlanDimensionConsistency: PlanDimensionVerdictPass,
			},
		},
		{
			name: "decision soundness may not cite top-level artifacts directory",
			record: VerificationRecord{References: []string{
				"dim:decision_soundness=pass:artifacts",
				"dim:consistency=pass:artifacts/changes/example/requirements.md",
			}},
			wantCodes: []string{"plan_dimension_decision_soundness_evidence_invalid"},
			want: map[PlanDimensionName]string{
				PlanDimensionConsistency: PlanDimensionVerdictPass,
			},
		},
		{
			name: "decision soundness may not cite artifacts directory with trailing slash",
			record: VerificationRecord{References: []string{
				"dim:decision_soundness=pass:artifacts/",
				"dim:consistency=pass:artifacts/changes/example/requirements.md",
			}},
			wantCodes: []string{"plan_dimension_decision_soundness_evidence_invalid"},
			want: map[PlanDimensionName]string{
				PlanDimensionConsistency: PlanDimensionVerdictPass,
			},
		},
		{
			name: "attestation evidence must be a regular file",
			record: VerificationRecord{References: []string{
				"dim:decision_soundness=pass:internal",
				"dim:consistency=pass:artifacts/changes/example/requirements.md",
			}},
			wantCodes: []string{"plan_dimension_attestation_evidence_unresolvable"},
			want: map[PlanDimensionName]string{
				PlanDimensionConsistency: PlanDimensionVerdictPass,
			},
		},
		{
			name: "fail verdict maps to dimension failure",
			record: VerificationRecord{References: []string{
				"dim:decision_soundness=fail:internal/engine/progression/advance_governed.go",
				"dim:consistency=fail:artifacts/changes/example/requirements.md",
			}},
			wantCodes: []string{
				"plan_dimension_decision_unsound",
				"plan_dimension_consistency_failed",
			},
			want: map[PlanDimensionName]string{
				PlanDimensionDecisionSoundness: PlanDimensionVerdictFail,
				PlanDimensionConsistency:       PlanDimensionVerdictFail,
			},
		},
		{
			name: "no required tokens reports unattested",
			record: VerificationRecord{References: []string{
				"plan-audit:pass",
			}},
			wantCodes: []string{
				"plan_dimension_decision_soundness_unattested",
				"plan_dimension_consistency_unattested",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, blockers := RequiredPlanDimensionAttestationBlockers(root, tt.record)
			assert.ElementsMatch(t, tt.wantCodes, reasonCodeNames(blockers))
			for name, verdict := range tt.want {
				attestation, ok := got.Attestations[name]
				require.Truef(t, ok, "expected dimension %q", name)
				assert.Equal(t, verdict, attestation.Verdict)
			}
		})
	}
}

func writeTestFile(t *testing.T, root, rel string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("test"), 0o644))
}

func reasonCodeNames(reasons []ReasonCode) []string {
	out := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		out = append(out, reason.Code)
	}
	return out
}
