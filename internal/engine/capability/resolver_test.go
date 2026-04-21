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
	// Mode now carries the public surface alias (route-surface plan §4.3).
	assert.Equal(t, "independent-review", res.Route.Mode)
	assert.NotEmpty(t, res.Route.Reason)
}

func TestResolveSelectsCommandPrimaryRoute_Status(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()

	// status no longer has a primary route; default is neutral.
	res := Resolve(reg, Signals{Command: "status"})
	assert.Nil(t, res.Route)
}

func TestResolveIntakeHostDoesNotExposeRetiredScopeSkill(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()

	res := Resolve(reg, Signals{Host: "intake-clarification"})
	assert.Nil(t, res.Route)
	assert.Empty(t, res.Supports)
	assert.Empty(t, res.HydrateReferences)
}

func TestResolveGoalVerificationHostUsesIntentionalSupportSet(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	res := Resolve(reg, Signals{Host: "goal-verification"})

	require.Len(t, res.Supports, 2)
	assert.Equal(t, []Attachment{
		{
			SkillID: "coverage-analysis",
			Kind:    AttachmentChecklist,
			Reason:  "Use when a change needs coverage evaluation. Triggers on validate command, goal-verification host, or coverage-related user text.",
		},
		{
			SkillID: "fresh-verification-evidence",
			Kind:    AttachmentChecklist,
			Reason:  "Use when a change is approaching a verify/closeout gate. Triggers on goal-verification, final-closeout, or any completion-adjacent step.",
		},
	}, res.Supports)
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

func TestResolveEmitsHydrateForAutoRoute(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	// health retains its primary route with incident-response hydrate refs.
	res := Resolve(reg, Signals{Command: "health"})
	require.NotNil(t, res.Route)
	require.Equal(t, "incident-response", res.Route.SkillID)
	assert.NotEmpty(t, res.HydrateReferences)
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
				// After the surface-policy cutover Supports is host/technique-
				// only for routed commands.
				supports: []supportSnapshot{},
			},
		},
		{
			name: "status",
			sig:  Signals{Command: "status"},
			want: resolutionSnapshot{
				supports: []supportSnapshot{},
			},
		},
		{
			name: "intake-clarification",
			sig:  Signals{Host: "intake-clarification"},
			want: resolutionSnapshot{
				supports: []supportSnapshot{},
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
					{skillID: "root-cause-tracing", kind: AttachmentProcedure},
				},
				hydrate: []string{
					"root-cause-tracing/condition-based-waiting.md",
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

// TestResolvePlanAuditDoesNotSurfaceRetiredPlanAuthoringHydrate ensures the
// absorbed authoring skill no longer leaks through the plan-audit host path.
func TestResolvePlanAuditDoesNotSurfaceRetiredPlanAuthoringHydrate(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	res := Resolve(reg, Signals{Host: "plan-audit"})
	for _, key := range res.HydrateReferences {
		assert.NotContains(t, key, "plan-authoring/",
			"retired plan-authoring hydrate must not surface on plan-audit")
	}
}

// TestResolveRetiredTddProofHydrateDoesNotLeak ensures host absorption removed
// the old tdd-proof hydrate keys from both governance paths.
func TestResolveRetiredTddProofHydrateDoesNotLeak(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()

	resGovernance := Resolve(reg, Signals{Host: "tdd-governance"})
	for _, key := range resGovernance.HydrateReferences {
		assert.NotContains(t, key, "tdd-proof/",
			"retired tdd-proof hydrate must not surface on tdd-governance")
	}

	resWave := Resolve(reg, Signals{Host: "wave-orchestration"})
	for _, key := range resWave.HydrateReferences {
		assert.NotContains(t, key, "tdd-proof/",
			"retired tdd-proof hydrate must not surface on wave-orchestration")
	}
}

// TestResolveCiTriageNeverSurfacesHydrate enforces the Wave-3 PR-3 negative
// invariant: ci-triage is a scripts-only suggested-only skill with no
// HydrateReferences, so its hydrate footprint is empty on every selection
// path it owns (suggested on repair, and also on arbitrary hosts).
func TestResolveCiTriageNeverSurfacesHydrate(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	for _, sig := range []Signals{
		{Command: "repair"},
		{Host: "code-quality-review"},
		{Host: "plan-audit"},
	} {
		res := Resolve(reg, sig)
		for _, key := range res.HydrateReferences {
			assert.NotContains(t, key, "ci-triage/",
				"ci-triage must never surface hydrate keys (signals=%+v, key=%s)", sig, key)
		}
	}
}

// TestResolveReviewCommentTriageNeverSurfacesHydrate enforces the matching
// negative invariant for review-comment-triage.
func TestResolveReviewCommentTriageNeverSurfacesHydrate(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	for _, sig := range []Signals{
		{Command: "repair"},
		{Host: "code-quality-review"},
		{Host: "plan-audit"},
	} {
		res := Resolve(reg, sig)
		for _, key := range res.HydrateReferences {
			assert.NotContains(t, key, "review-comment-triage/",
				"review-comment-triage must never surface hydrate keys (signals=%+v, key=%s)", sig, key)
		}
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
