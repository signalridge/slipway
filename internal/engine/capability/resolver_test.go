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

func TestResolveShipVerificationHostUsesIntentionalSupportSet(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	res := Resolve(reg, Signals{Host: "ship-verification"})

	require.Len(t, res.Supports, 1)
	assert.Equal(t, []Attachment{
		{
			SkillID: "coverage-analysis",
			Kind:    AttachmentChecklist,
			Reason:  "Use when evaluating test coverage of a change's new and modified lines (not the whole codebase). Triggers on the `slipway validate` command, the ship-verification host, or coverage-related user text.",
		},
	}, res.Supports)
}

func TestResolveVerifyHostsDoNotExposeRetiredFreshEvidenceSupport(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()

	for _, host := range []string{"tdd-governance"} {
		host := host
		t.Run(host, func(t *testing.T) {
			t.Parallel()
			res := Resolve(reg, Signals{Host: host})
			assert.Empty(t, res.Supports)
		})
	}
}

// final-closeout was merged into the terminal ship-verification gate. The
// retired host name must resolve to nothing — no route, no support set, no
// hydrate references leak through it.
func TestResolveRetiredFinalCloseoutHostResolvesToNothing(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()

	res := Resolve(reg, Signals{Host: "final-closeout"})
	assert.Nil(t, res.Route)
	assert.Empty(t, res.Supports)
	assert.Empty(t, res.HydrateReferences)
}

func TestResolveNoMatchReturnsEmpty(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	res := Resolve(reg, Signals{})
	assert.Nil(t, res.Route)
	assert.Empty(t, res.Supports)
}

func TestResolveHostCapabilityRequirement(t *testing.T) {
	t.Parallel()

	t.Run("skill without a host capability contract resolves nil", func(t *testing.T) {
		t.Parallel()

		// context-assembly is registered but declares no host-capability contract
		// and is not part of the subagent-dispatch lever, so it must resolve nil.
		assert.Nil(t, ResolveHostCapabilityRequirement("context-assembly", Signals{}))
	})

	tests := []struct {
		name             string
		signals          Signals
		wantAvailability string
		wantFallback     bool
		wantFallbackMode string
	}{
		{
			name:             "undeclared host capability remains unknown",
			signals:          Signals{},
			wantAvailability: "unknown",
		},
		{
			name: "explicit subagent capability is available",
			signals: Signals{
				HostCapabilities: []string{"subagent"},
			},
			wantAvailability: "available",
		},
		{
			name: "delegation alias satisfies subagent",
			signals: Signals{
				HostCapabilities: []string{"delegation"},
			},
			wantAvailability: "available",
		},
		{
			name: "explicit none is unavailable",
			signals: Signals{
				HostCapabilities: []string{"none"},
			},
			wantAvailability: "unavailable",
		},
		{
			name: "manual independent review fallback is explicit",
			signals: Signals{
				HostCapabilities: []string{"none"},
				Fallbacks:        []string{"manual_independent_review"},
			},
			wantAvailability: "unavailable",
			wantFallback:     true,
			wantFallbackMode: "manual_independent_review",
		},
		{
			name: "same-context degraded fallback is explicit",
			signals: Signals{
				HostCapabilities: []string{"none"},
				Fallbacks:        []string{"same_context_degraded"},
			},
			wantAvailability: "unavailable",
			wantFallback:     true,
			wantFallbackMode: "same_context_degraded",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := ResolveHostCapabilityRequirement("independent-review", tt.signals)
			require.NotNil(t, req)
			assert.Equal(t, "independent-review", req.SkillID)
			assert.Equal(t, "subagent", req.Capability)
			assert.True(t, req.Required)
			assert.Equal(t, tt.wantAvailability, req.Availability)
			assert.Equal(t, tt.wantFallback, req.FallbackSelected)
			assert.Equal(t, tt.wantFallbackMode, req.FallbackMode)
			assert.NotEmpty(t, req.EvidenceRequirement)
			assert.NotEmpty(t, req.Remediation)
		})
	}
}

// TestResolveHostCapabilitySubagentDispatchLever covers the built-in
// subagent-dispatch contract that surfaces a host-delegation prerequisite for
// the governance skills that REQUIRE a fresh subagent but are not registered in
// the capability catalog (plan-audit at S1; spec-compliance-review and
// code-quality-review at S3). Registering them in the catalog would drag in the
// surface-manifest / install-profile / generation machinery, so the contract is
// a targeted lever consulted when the registry carries no host-capability
// contract for the skill. (#339 / #369)
func TestResolveHostCapabilitySubagentDispatchLever(t *testing.T) {
	t.Parallel()

	leverSkills := []string{"plan-audit", "spec-compliance-review", "code-quality-review"}
	for _, skillID := range leverSkills {
		skillID := skillID
		t.Run(skillID+"/unknown surfaces continuable prerequisite", func(t *testing.T) {
			t.Parallel()
			req := ResolveHostCapabilityRequirement(skillID, Signals{})
			require.NotNil(t, req, "%s must resolve a built-in subagent-dispatch contract", skillID)
			assert.Equal(t, skillID, req.SkillID)
			assert.Equal(t, "subagent", req.Capability)
			assert.True(t, req.Required)
			assert.Equal(t, "unknown", req.Availability)
			assert.False(t, req.FallbackSelected)
			assert.NotEmpty(t, req.EvidenceRequirement)
			assert.NotEmpty(t, req.Remediation)
		})
		t.Run(skillID+"/explicit none is unavailable", func(t *testing.T) {
			t.Parallel()
			req := ResolveHostCapabilityRequirement(skillID, Signals{HostCapabilities: []string{"none"}})
			require.NotNil(t, req)
			assert.Equal(t, "unavailable", req.Availability)
			assert.False(t, req.FallbackSelected)
		})
		t.Run(skillID+"/declared subagent is available", func(t *testing.T) {
			t.Parallel()
			req := ResolveHostCapabilityRequirement(skillID, Signals{HostCapabilities: []string{"subagent"}})
			require.NotNil(t, req)
			assert.Equal(t, "available", req.Availability)
		})
		t.Run(skillID+"/generic same_context_degraded fallback is selectable", func(t *testing.T) {
			t.Parallel()
			req := ResolveHostCapabilityRequirement(skillID, Signals{
				HostCapabilities: []string{"none"},
				Fallbacks:        []string{"same_context_degraded"},
			})
			require.NotNil(t, req)
			assert.Equal(t, "unavailable", req.Availability)
			assert.True(t, req.FallbackSelected)
			assert.Equal(t, "same_context_degraded", req.FallbackMode)
		})
	}

	t.Run("registry contract still wins for registered security-review", func(t *testing.T) {
		t.Parallel()
		req := ResolveHostCapabilityRequirement("security-review", Signals{HostCapabilities: []string{"none"}})
		require.NotNil(t, req, "security-review must carry a registry host-capability contract")
		assert.Equal(t, "subagent", req.Capability)
		assert.Equal(t, "unavailable", req.Availability)
	})

	t.Run("skills without any contract still resolve to nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, ResolveHostCapabilityRequirement("context-assembly", Signals{}))
		assert.Nil(t, ResolveHostCapabilityRequirement("no-such-skill", Signals{}))
	})
}

func TestResolveHostCapabilityRequirementUsesRegistryContract(t *testing.T) {
	t.Parallel()

	reg, err := NewRegistry(Skill{
		ID:                "fresh-context-review",
		Domain:            DomainReviewQuality,
		Function:          "fresh context review",
		Tier:              TierT1,
		PrimaryAttachment: AttachmentProcedure,
		Summary:           "Use when testing registry-owned host capabilities. Triggers on tests.",
		Evidence:          EvidenceVerdict,
		Bindings: []Binding{
			{Type: BindingCommandAuto, Target: "review", Attachment: AttachmentReportSchema},
		},
		HostCapabilities: []HostCapabilityContract{
			{
				Capability:          "isolated_context",
				Required:            true,
				FallbackModes:       []string{"manual_review"},
				EvidenceRequirement: "record review evidence from an isolated context",
				Remediation:         "Run in a host with isolated_context, or select manual_review.",
			},
		},
	})
	require.NoError(t, err)

	req := ResolveHostCapabilityRequirementFromRegistry(reg, "fresh-context-review", Signals{
		HostCapabilities: []string{"none"},
		Fallbacks:        []string{"manual_review"},
	})
	require.NotNil(t, req)
	assert.Equal(t, "fresh-context-review", req.SkillID)
	assert.Equal(t, "isolated_context", req.Capability)
	assert.Equal(t, "unavailable", req.Availability)
	assert.True(t, req.FallbackSelected)
	assert.Equal(t, "manual_review", req.FallbackMode)

	delegationOnly := ResolveHostCapabilityRequirementFromRegistry(reg, "fresh-context-review", Signals{
		HostCapabilities: []string{"delegation"},
	})
	require.NotNil(t, delegationOnly)
	assert.Equal(t, "unavailable", delegationOnly.Availability)
}

func TestResolveReviewHostDoesNotAttachPromotedReviewHosts(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	res := Resolve(reg, Signals{Host: "code-quality-review"})
	require.NotEmpty(t, res.Supports)

	for _, s := range res.Supports {
		assert.NotContains(t, []string{"independent-review", "security-review"}, s.SkillID,
			"promoted S3 review hosts must not be attached as host-embedded supports")
	}
	for _, key := range res.HydrateReferences {
		assert.NotContains(t, key, "security-review/",
			"promoted security-review host hydrate refs must not leak through code-quality-review support")
	}
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

func TestResolvePreservesRouteAndSupportsInvariant(t *testing.T) {
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
					{skillID: "multi-reviewer-calibration", kind: AttachmentProcedure},
				},
			},
		},
		{
			name: "wave-orchestration",
			sig:  Signals{Host: "wave-orchestration"},
			want: resolutionSnapshot{
				supports: []supportSnapshot{
					{skillID: "root-cause-tracing", kind: AttachmentProcedure},
					{skillID: "test-design", kind: AttachmentProcedure},
				},
				hydrate: []string{
					"root-cause-tracing/condition-based-waiting.md",
					"root-cause-tracing/hypothesis-testing.md",
					"root-cause-tracing/root-cause-tracing.md",
					"test-design/behavior-vs-implementation.md",
					"test-design/case-enumeration.md",
					"test-design/property-reasoning.md",
					"test-design/test-data.md",
					"test-design/test-doubles.md",
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

// TestResolveCiTriageNeverSurfacesHydrate enforces the negative
// invariant: ci-triage is a scripts-only suggested-only skill with no
// HydrateReferences, so its hydrate footprint is empty on every selection
// path it owns (suggested on fix, and also on arbitrary hosts).
func TestResolveCiTriageNeverSurfacesHydrate(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	for _, sig := range []Signals{
		{Command: "fix"},
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
		{Command: "fix"},
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
