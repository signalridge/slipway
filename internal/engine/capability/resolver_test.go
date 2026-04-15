package capability

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveSelectsCommandRoute(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()

	res := Resolve(reg, Signals{Command: "review"})
	require.NotNil(t, res.Route)
	assert.Equal(t, "independent-review", res.Route.SkillID)
	assert.Equal(t, "independent-review", res.Route.Mode)
	assert.NotEmpty(t, res.Route.Reason)
}

func TestResolveSelectsCommandViewRoute(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()

	res := Resolve(reg, Signals{Command: "status"})
	require.NotNil(t, res.Route)
	assert.Equal(t, "incident-response", res.Route.SkillID)
	assert.Equal(t, "incident-response", res.Route.View)
	assert.NotEmpty(t, res.Route.Reason)
}

func TestResolveAttachesIntakeSupportOnIntakeHost(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()

	res := Resolve(reg, Signals{Host: "intake-clarification"})
	assert.Nil(t, res.Route)
	require.NotEmpty(t, res.Supports)
	assert.Equal(t, "scope-clarification", res.Supports[0].SkillID)
	assert.Equal(t, AttachmentPosture, res.Supports[0].Kind)
	assert.NotEmpty(t, res.Supports[0].Reason)
}

func TestResolveCapsSupportsAtThree(t *testing.T) {
	t.Parallel()
	// Signals that match several skills' triggers at once.
	reg := DefaultRegistry()
	res := Resolve(reg, Signals{
		Host:         "goal-verification",
		Command:      "validate",
		ChangedFiles: []string{"docs/plans/2026-04-11-foo.md"},
		Blockers:     []string{"stale_verification_evidence"},
	})
	assert.LessOrEqual(t, len(res.Supports), 3)
}

func TestResolveNoMatchReturnsEmpty(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	res := Resolve(reg, Signals{})
	assert.Nil(t, res.Route)
	assert.Empty(t, res.Supports)
}

func TestResolveReviewHostAttachesIndependentReview(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	res := Resolve(reg, Signals{Host: "code-quality-review"})
	require.NotEmpty(t, res.Supports)
	foundIR := false
	for _, s := range res.Supports {
		if s.SkillID == "independent-review" {
			foundIR = true
			assert.NotEmpty(t, s.Kind)
		}
	}
	assert.True(t, foundIR, "expected independent-review attached at code-quality-review host")
}

func TestResolveTiebreakStaysReserved(t *testing.T) {
	t.Parallel()
	// llm_tiebreak is reserved for B7+ and must not leak from the resolver
	// even after PR-4a populates hydrate_references for routed and
	// support-path resolutions.
	reg := DefaultRegistry()
	res := Resolve(reg, Signals{
		Host:    "plan-audit",
		Command: "review",
	})
	assert.Nil(t, res.LLMTiebreak)
}

func TestResolveEmitsHydrateForAutoRoute(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	res := Resolve(reg, Signals{Command: "status"})
	require.NotNil(t, res.Route)
	require.Equal(t, "incident-response", res.Route.SkillID)
	assert.Contains(t, res.HydrateReferences, "incident-response/incident-severity-matrix.md")
	assert.Contains(t, res.HydrateReferences, "incident-response/incident-response-framework.md")
	// Keys are sorted.
	sorted := append([]string(nil), res.HydrateReferences...)
	for i := 1; i < len(sorted); i++ {
		assert.LessOrEqual(t, sorted[i-1], sorted[i], "hydrate references must be sorted")
	}
}

func TestResolveEmitsHydrateForSupportPath(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	res := Resolve(reg, Signals{Host: "wave-orchestration"})
	require.NotEmpty(t, res.Supports)
	assert.Contains(t, res.HydrateReferences, "root-cause-tracing/root-cause-tracing.md")
}

func TestResolveDoesNotLeakManualOnlyHydrateOnImplicitReview(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	res := Resolve(reg, Signals{Command: "review"})
	assert.Empty(t, res.HydrateReferences, "manual-only hydrate should surface only through explicit --mode")
}

func TestResolveHydrateDedupesAndSortsAcrossRouteAndSupports(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	res := Resolve(reg, Signals{
		Command: "status",
		Host:    "plan-audit",
	})
	seen := make(map[string]int, len(res.HydrateReferences))
	for _, k := range res.HydrateReferences {
		seen[k]++
	}
	for k, n := range seen {
		assert.Equal(t, 1, n, "hydrate key %q must appear exactly once", k)
	}
}

func TestResolvePR4aPreservesRouteAndSupportsInvariant(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	type supportSnapshot struct {
		skillID string
		kind    AttachmentMode
	}
	type resolutionSnapshot struct {
		routeSkill string
		routeMode  string
		routeView  string
		supports   []supportSnapshot
		hydrate    []string
	}
	cases := []struct {
		name string
		sig  Signals
		want resolutionSnapshot
	}{
		{
			name: "review",
			sig:  Signals{Command: "review"},
			want: resolutionSnapshot{
				routeSkill: "independent-review",
				routeMode:  "independent-review",
				supports: []supportSnapshot{
					{skillID: "differential-review", kind: AttachmentProcedure},
					{skillID: "gha-security-review", kind: AttachmentChecklist},
					{skillID: "multi-reviewer-calibration", kind: AttachmentChecklist},
				},
			},
		},
		{
			name: "status",
			sig:  Signals{Command: "status"},
			want: resolutionSnapshot{
				routeSkill: "incident-response",
				routeView:  "incident-response",
				supports: []supportSnapshot{
					{skillID: "ci-triage", kind: AttachmentChecklist},
					{skillID: "git-recovery", kind: AttachmentChecklist},
					{skillID: "performance-profiling", kind: AttachmentChecklist},
				},
				hydrate: []string{
					"incident-response/communication-templates.md",
					"incident-response/incident-response-framework.md",
					"incident-response/incident-severity-matrix.md",
					"incident-response/rca-frameworks-guide.md",
					"incident-response/regulatory-deadlines.md",
					"incident-response/sla-management-guide.md",
				},
			},
		},
		{
			name: "intake-clarification",
			sig:  Signals{Host: "intake-clarification"},
			want: resolutionSnapshot{
				supports: []supportSnapshot{
					{skillID: "scope-clarification", kind: AttachmentPosture},
				},
			},
		},
		{
			name: "code-quality-review",
			sig:  Signals{Host: "code-quality-review"},
			want: resolutionSnapshot{
				supports: []supportSnapshot{
					{skillID: "independent-review", kind: AttachmentProcedure},
					{skillID: "multi-reviewer-calibration", kind: AttachmentProcedure},
					{skillID: "security-review", kind: AttachmentChecklist},
				},
				hydrate: []string{
					"security-review/authentication.md",
					"security-review/authorization.md",
					"security-review/infrastructure-docker.md",
					"security-review/injection.md",
					"security-review/ssrf.md",
					"security-review/xss.md",
				},
			},
		},
		{
			name: "wave-orchestration",
			sig:  Signals{Host: "wave-orchestration"},
			want: resolutionSnapshot{
				supports: []supportSnapshot{
					{skillID: "tdd-proof", kind: AttachmentProcedure},
					{skillID: "parallel-executor-contract", kind: AttachmentProcedure},
					{skillID: "root-cause-tracing", kind: AttachmentProcedure},
				},
				hydrate: []string{
					"root-cause-tracing/condition-based-waiting.md",
					"root-cause-tracing/defense-in-depth.md",
					"root-cause-tracing/failure-patterns.md",
					"root-cause-tracing/hypothesis-testing.md",
					"root-cause-tracing/root-cause-tracing.md",
				},
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			res := Resolve(reg, tc.sig)
			if tc.want.routeSkill == "" {
				assert.Nil(t, res.Route)
			} else {
				require.NotNil(t, res.Route)
				assert.Equal(t, tc.want.routeSkill, res.Route.SkillID)
				assert.Equal(t, tc.want.routeMode, res.Route.Mode)
				assert.Equal(t, tc.want.routeView, res.Route.View)
				assert.NotEmpty(t, res.Route.Reason)
			}

			gotSupports := make([]supportSnapshot, 0, len(res.Supports))
			for _, s := range res.Supports {
				assert.NotEmpty(t, s.Reason)
				gotSupports = append(gotSupports, supportSnapshot{skillID: s.SkillID, kind: s.Kind})
			}
			assert.Equal(t, tc.want.supports, gotSupports)
			assert.Equal(t, tc.want.hydrate, res.HydrateReferences)
			assert.True(t, slices.IsSorted(res.HydrateReferences), "hydrate references should stay stable-sorted")
		})
	}
}

func TestHydrateReferenceKeysForSkillReturnsSkillRelativeKeys(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	keys := HydrateReferenceKeysForSkill(reg, "gha-security-review")
	require.NotEmpty(t, keys)
	for _, k := range keys {
		assert.Contains(t, k, "gha-security-review/")
	}
	assert.Empty(t, HydrateReferenceKeysForSkill(reg, "does-not-exist"))
	assert.Empty(t, HydrateReferenceKeysForSkill(nil, "gha-security-review"))
}
