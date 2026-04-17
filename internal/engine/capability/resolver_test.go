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

func TestResolveSelectsCommandViewRoute(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()

	res := Resolve(reg, Signals{Command: "status"})
	require.NotNil(t, res.Route)
	assert.Equal(t, "incident-response", res.Route.SkillID)
	// View carries the public view alias after the surface-policy cutover.
	assert.Equal(t, "incident", res.Route.View)
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
				// After the surface-policy cutover Supports is host/technique-
				// only; command-auto skills populate SuggestedCapabilities
				// instead (route-surface plan §4.4).
				supports: []supportSnapshot{},
			},
		},
		{
			name: "status",
			sig:  Signals{Command: "status"},
			want: resolutionSnapshot{
				routeSkill: "incident-response",
				routeView:  "incident",
				supports:   []supportSnapshot{},
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
					{skillID: "parallel-executor-contract", kind: AttachmentProcedure},
					{skillID: "root-cause-tracing", kind: AttachmentProcedure},
					{skillID: "tdd-proof", kind: AttachmentProcedure},
				},
				hydrate: []string{
					"root-cause-tracing/condition-based-waiting.md",
					"root-cause-tracing/defense-in-depth.md",
					"root-cause-tracing/failure-patterns.md",
					"root-cause-tracing/hypothesis-testing.md",
					"root-cause-tracing/root-cause-tracing.md",
					"tdd-proof/testing-anti-patterns.md",
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

// TestResolvePlanAuthoringHydrateSurfacesOnPlanAuditHost locks the Wave-3 PR-3
// host-embedded hydrate contract for plan-authoring: when `plan-audit` is the
// active host, the skill's declared reference surfaces through the
// support-path hydrate union.
func TestResolvePlanAuthoringHydrateSurfacesOnPlanAuditHost(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	res := Resolve(reg, Signals{Host: "plan-audit"})
	assert.Contains(t, res.HydrateReferences, "plan-authoring/plan-document-review-prompt.md",
		"plan-authoring hydrate must surface when plan-audit host is active")
}

// TestResolveTddProofHydrateSurfacesOnGovernanceHosts locks the Wave-3 PR-3
// host-embedded hydrate contract for tdd-proof on both of its host bindings
// (`tdd-governance` and `wave-orchestration`).
func TestResolveTddProofHydrateSurfacesOnGovernanceHosts(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()

	resGovernance := Resolve(reg, Signals{Host: "tdd-governance"})
	assert.Contains(t, resGovernance.HydrateReferences, "tdd-proof/testing-anti-patterns.md",
		"tdd-proof hydrate must surface on tdd-governance host")

	resWave := Resolve(reg, Signals{Host: "wave-orchestration"})
	assert.Contains(t, resWave.HydrateReferences, "tdd-proof/testing-anti-patterns.md",
		"tdd-proof hydrate must surface on wave-orchestration host")
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
