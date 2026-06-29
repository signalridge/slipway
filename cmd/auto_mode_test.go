package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeAutoConfig sets execution.auto in the project config to the given value.
func writeAutoConfig(t *testing.T, root string, auto bool) {
	t.Helper()
	cfgPath := state.ConfigPath(root)
	cfg, err := model.LoadConfig(cfgPath)
	require.NoError(t, err)
	cfg.Execution.Auto = auto
	require.NoError(t, model.SaveConfig(cfgPath, cfg))
}

// (a) Tri-state flag resolution: explicit flags beat config; absence falls back
// to config (execution.auto).
func TestResolveEffectiveAutoTriState(t *testing.T) {
	t.Parallel()

	t.Run("no flag falls back to config auto=true", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		ensureTestGitRepo(t, root)
		initTestWorkspace(t, root)
		writeAutoConfig(t, root, true)

		cmd := makeRunCmd()
		require.NoError(t, cmd.ParseFlags(nil))
		effective, err := resolveEffectiveAuto(root, cmd, false, false)
		require.NoError(t, err)
		assert.True(t, effective)
	})

	t.Run("no flag falls back to config auto=false", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		ensureTestGitRepo(t, root)
		initTestWorkspace(t, root)
		writeAutoConfig(t, root, false)

		cmd := makeRunCmd()
		require.NoError(t, cmd.ParseFlags(nil))
		effective, err := resolveEffectiveAuto(root, cmd, false, false)
		require.NoError(t, err)
		assert.False(t, effective)
	})

	t.Run("--no-auto beats config auto=true", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		ensureTestGitRepo(t, root)
		initTestWorkspace(t, root)
		writeAutoConfig(t, root, true)

		cmd := makeRunCmd()
		require.NoError(t, cmd.ParseFlags([]string{"--no-auto"}))
		noAuto, err := cmd.Flags().GetBool("no-auto")
		require.NoError(t, err)
		effective, err := resolveEffectiveAuto(root, cmd, false, noAuto)
		require.NoError(t, err)
		assert.False(t, effective)
	})

	t.Run("--auto beats config auto=false", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		ensureTestGitRepo(t, root)
		initTestWorkspace(t, root)
		writeAutoConfig(t, root, false)

		cmd := makeRunCmd()
		require.NoError(t, cmd.ParseFlags([]string{"--auto"}))
		auto, err := cmd.Flags().GetBool("auto")
		require.NoError(t, err)
		effective, err := resolveEffectiveAuto(root, cmd, auto, false)
		require.NoError(t, err)
		assert.True(t, effective)
	})

	t.Run("--auto beats config auto unset", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		ensureTestGitRepo(t, root)
		initTestWorkspace(t, root)
		// No writeAutoConfig: config auto is unset (false by default).

		cmd := makeRunCmd()
		require.NoError(t, cmd.ParseFlags([]string{"--auto"}))
		auto, err := cmd.Flags().GetBool("auto")
		require.NoError(t, err)
		effective, err := resolveEffectiveAuto(root, cmd, auto, false)
		require.NoError(t, err)
		assert.True(t, effective)
	})
}

// TASK C (finding #6): `--no-auto=false` is Changed but its value is false, so it
// must FALL THROUGH to config (not force auto off). A bare `--no-auto` (value
// true) forces off.
func TestResolveEffectiveAutoNoAutoFalseFallsThroughToConfig(t *testing.T) {
	t.Parallel()

	t.Run("--no-auto=false falls through to config auto=true", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		ensureTestGitRepo(t, root)
		initTestWorkspace(t, root)
		writeAutoConfig(t, root, true)

		cmd := makeRunCmd()
		require.NoError(t, cmd.ParseFlags([]string{"--no-auto=false"}))
		require.True(t, cmd.Flags().Changed("no-auto"))
		noAuto, err := cmd.Flags().GetBool("no-auto")
		require.NoError(t, err)
		require.False(t, noAuto)

		effective, err := resolveEffectiveAuto(root, cmd, false, noAuto)
		require.NoError(t, err)
		assert.True(t, effective, "--no-auto=false must not force auto off; it falls through to config")
	})

	t.Run("bare --no-auto forces auto off over config auto=true", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		ensureTestGitRepo(t, root)
		initTestWorkspace(t, root)
		writeAutoConfig(t, root, true)

		cmd := makeRunCmd()
		require.NoError(t, cmd.ParseFlags([]string{"--no-auto"}))
		noAuto, err := cmd.Flags().GetBool("no-auto")
		require.NoError(t, err)
		require.True(t, noAuto)

		effective, err := resolveEffectiveAuto(root, cmd, false, noAuto)
		require.NoError(t, err)
		assert.False(t, effective, "bare --no-auto forces auto off regardless of config")
	})
}

func TestRunCmdRejectsBothAutoAndNoAuto(t *testing.T) {
	t.Parallel()
	cmd := makeRunCmd()
	err := cmd.ParseFlags([]string{"--auto", "--no-auto"})
	// ParseFlags itself succeeds; cobra enforces the mutual exclusion at execute
	// time via ValidateFlagGroups.
	require.NoError(t, err)
	require.Error(t, cmd.ValidateFlagGroups())

	cmd = makeRunCmd()
	cmd.SetArgs([]string{"--auto", "--no-auto"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err = cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auto")
	assert.Contains(t, err.Error(), "no-auto")
}

// (b) auto + non-guardrail softens review_batch and skill_handoff to a
// standing-authorization continuation (not a hard stop).
func TestDeriveConfirmationRequirementAutoSoftensNonGuardrail(t *testing.T) {
	t.Parallel()

	t.Run("review_batch", func(t *testing.T) {
		t.Parallel()
		view := nextView{
			auto:            true,
			GuardrailDomain: "",
			ReviewBatch: &reviewBatchView{
				Skills: []reviewBatchSkillView{{Name: "spec-compliance-review"}},
			},
		}
		req := deriveConfirmationRequirement(view)
		assert.Equal(t, "evidence_continuation", req.Boundary)
		assert.True(t, req.PriorAuthorizationSufficient)
		assert.False(t, req.FreshConfirmationRequired)
		assert.Equal(t, "review_batch", req.Reason)
		assert.NotEmpty(t, req.NextAction)
	})

	t.Run("skill_handoff", func(t *testing.T) {
		t.Parallel()
		view := nextView{
			auto:            true,
			GuardrailDomain: "",
			NextSkill:       &nextSkillView{Name: "wave-orchestration"},
		}
		req := deriveConfirmationRequirement(view)
		assert.Equal(t, "evidence_continuation", req.Boundary)
		assert.True(t, req.PriorAuthorizationSufficient)
		assert.False(t, req.FreshConfirmationRequired)
		assert.Contains(t, req.Reason, "skill_handoff")
		assert.NotEmpty(t, req.NextAction)
	})
}

func TestDeriveConfirmationRequirementAutoSoftensOnlyPurePacingAllowlist(t *testing.T) {
	t.Parallel()

	for _, skillName := range []string{
		progression.SkillResearchOrchestration,
		progression.SkillPlanAudit,
		progression.SkillWaveOrchestration,
		progression.SkillSpecComplianceReview,
		progression.SkillCodeQualityReview,
		progression.SkillIndependentReview,
		progression.SkillShipVerification,
	} {
		skillName := skillName
		t.Run("allowlisted_"+skillName, func(t *testing.T) {
			t.Parallel()
			view := nextView{
				auto:            true,
				GuardrailDomain: "",
				NextSkill:       &nextSkillView{Name: skillName},
			}

			req := deriveConfirmationRequirement(view)
			assert.Equal(t, "evidence_continuation", req.Boundary)
			assert.True(t, req.PriorAuthorizationSufficient)
			assert.False(t, req.FreshConfirmationRequired)
			assert.Equal(t, "skill_handoff:"+skillName, req.Reason)
		})
	}

	for _, skillName := range []string{
		progression.SkillWorktreePreflight,
		progression.SkillIntakeClarification,
		"future-sensitive-review",
	} {
		skillName := skillName
		t.Run("unlisted_"+skillName, func(t *testing.T) {
			t.Parallel()
			view := nextView{
				auto:            true,
				GuardrailDomain: "",
				NextSkill:       &nextSkillView{Name: skillName},
			}

			req := deriveConfirmationRequirement(view)
			assert.Equal(t, "hard_stop", req.Boundary)
			assert.False(t, req.PriorAuthorizationSufficient)
			assert.True(t, req.FreshConfirmationRequired)
			assert.Equal(t, "skill_handoff:"+skillName, req.Reason)
		})
	}
}

func TestDeriveConfirmationRequirementAutoKeepsSecurityReviewHardStop(t *testing.T) {
	t.Parallel()

	t.Run("review_batch with security review stays hard_stop without guardrail", func(t *testing.T) {
		t.Parallel()
		view := nextView{
			auto:            true,
			GuardrailDomain: "",
			ReviewBatch: &reviewBatchView{
				Skills: []reviewBatchSkillView{{Name: progression.SkillSecurityReview}},
			},
		}

		req := deriveConfirmationRequirement(view)
		assert.Equal(t, "hard_stop", req.Boundary)
		assert.False(t, req.PriorAuthorizationSufficient)
		assert.True(t, req.FreshConfirmationRequired)
		assert.Equal(t, "review_batch", req.Reason)
	})

	t.Run("skill_handoff to security review stays hard_stop without guardrail", func(t *testing.T) {
		t.Parallel()
		view := nextView{
			auto:            true,
			GuardrailDomain: "",
			NextSkill:       &nextSkillView{Name: progression.SkillSecurityReview},
		}

		req := deriveConfirmationRequirement(view)
		assert.Equal(t, "hard_stop", req.Boundary)
		assert.False(t, req.PriorAuthorizationSufficient)
		assert.True(t, req.FreshConfirmationRequired)
		assert.Equal(t, "skill_handoff:security-review", req.Reason)
	})

	t.Run("security review blocking name stays hard_stop without guardrail", func(t *testing.T) {
		t.Parallel()
		view := nextView{
			auto:            true,
			GuardrailDomain: "",
			NextSkill: &nextSkillView{
				Name:         progression.SkillShipVerification,
				BlockingName: progression.SkillSecurityReview,
			},
		}

		req := deriveConfirmationRequirement(view)
		assert.Equal(t, "hard_stop", req.Boundary)
		assert.False(t, req.PriorAuthorizationSufficient)
		assert.True(t, req.FreshConfirmationRequired)
		assert.Equal(t, "skill_handoff:security-review", req.Reason)
	})
}

func TestDeriveConfirmationRequirementAutoDoesNotSoftenNonPacingBlockers(t *testing.T) {
	t.Parallel()

	t.Run("sensitive evidence blocker wins over skill handoff", func(t *testing.T) {
		t.Parallel()
		view := nextView{
			auto:            true,
			GuardrailDomain: "",
			NextSkill:       &nextSkillView{Name: "wave-orchestration"},
			Blockers: []model.ReasonCode{
				model.NewReasonCode("sensitive_evidence_missing", "schema_migration:db/migrations/001.sql"),
			},
		}

		req := deriveConfirmationRequirement(view)
		assert.Equal(t, "hard_stop", req.Boundary)
		assert.False(t, req.PriorAuthorizationSufficient)
		assert.True(t, req.FreshConfirmationRequired)
		assert.Equal(t, "blocked_by_governance", req.Reason)
		assert.Equal(t, "blocker_resolution", req.NextActionKind)
	})

	t.Run("scope contract blocker wins over review batch", func(t *testing.T) {
		t.Parallel()
		view := nextView{
			auto:            true,
			GuardrailDomain: "",
			ReviewBatch: &reviewBatchView{
				Skills: []reviewBatchSkillView{{Name: "spec-compliance-review"}},
			},
			Blockers: []model.ReasonCode{
				model.NewReasonCode("scope_contract_drift", "cmd/next.go"),
			},
		}

		req := deriveConfirmationRequirement(view)
		assert.Equal(t, "hard_stop", req.Boundary)
		assert.False(t, req.PriorAuthorizationSufficient)
		assert.True(t, req.FreshConfirmationRequired)
		assert.Equal(t, "blocked_by_governance", req.Reason)
		assert.Equal(t, "blocker_resolution", req.NextActionKind)
	})

	t.Run("required skill blocker still rides skill handoff", func(t *testing.T) {
		t.Parallel()
		view := nextView{
			auto:            true,
			GuardrailDomain: "",
			NextSkill:       &nextSkillView{Name: "wave-orchestration"},
			Blockers: []model.ReasonCode{
				model.NewReasonCode("required_skill_missing", "wave-orchestration"),
			},
		}

		req := deriveConfirmationRequirement(view)
		assert.Equal(t, "evidence_continuation", req.Boundary)
		assert.True(t, req.PriorAuthorizationSufficient)
		assert.False(t, req.FreshConfirmationRequired)
		assert.Equal(t, "skill_handoff:wave-orchestration", req.Reason)
	})

	t.Run("domain review action rides review batch only", func(t *testing.T) {
		t.Parallel()
		view := nextView{
			auto:            true,
			GuardrailDomain: model.GuardrailDomainAuthAuthZ,
			ReviewBatch: &reviewBatchView{
				Skills: []reviewBatchSkillView{{Name: "security-review"}},
			},
			Blockers: []model.ReasonCode{
				model.NewReasonCode("context_origin_handle_invalid", "spec-compliance-review"),
				model.NewReasonCode("governance_action_required", "domain-review: run domain-aware review"),
				model.NewReasonCode("high_risk_check_missing", "auth_authz.safety_baseline"),
			},
		}

		req := deriveConfirmationRequirement(view)
		assert.Equal(t, "hard_stop", req.Boundary)
		assert.False(t, req.PriorAuthorizationSufficient)
		assert.True(t, req.FreshConfirmationRequired)
		assert.Equal(t, "review_batch", req.Reason)
		assert.Equal(t, "review_batch", req.NextActionKind)
	})

	t.Run("domain review action without review batch stays blocker", func(t *testing.T) {
		t.Parallel()
		view := nextView{
			auto:            true,
			GuardrailDomain: "",
			NextSkill:       &nextSkillView{Name: "wave-orchestration"},
			Blockers: []model.ReasonCode{
				model.NewReasonCode("governance_action_required", "domain-review: run domain-aware review"),
			},
		}

		req := deriveConfirmationRequirement(view)
		assert.Equal(t, "hard_stop", req.Boundary)
		assert.False(t, req.PriorAuthorizationSufficient)
		assert.True(t, req.FreshConfirmationRequired)
		assert.Equal(t, "blocked_by_governance", req.Reason)
		assert.Equal(t, "blocker_resolution", req.NextActionKind)
	})

	t.Run("review companion blockers ride ship-verification handoff", func(t *testing.T) {
		t.Parallel()
		view := nextView{
			auto:      false,
			NextSkill: &nextSkillView{Name: progression.SkillShipVerification},
			Blockers: []model.ReasonCode{
				model.NewReasonCode("high_risk_check_missing", "auth_authz.safety_baseline"),
				model.NewReasonCode("context_origin_handle_invalid", ""),
				model.NewReasonCode("governance_action_required", ""),
			},
		}

		req := deriveConfirmationRequirement(view)
		assert.Equal(t, "hard_stop", req.Boundary)
		assert.False(t, req.PriorAuthorizationSufficient)
		assert.True(t, req.FreshConfirmationRequired)
		assert.Equal(t, "skill_handoff:"+progression.SkillShipVerification, req.Reason)
		assert.Equal(t, "skill_handoff", req.NextActionKind)
	})
}

// (c) auto + guardrail domain keeps review_batch and skill_handoff as hard stops.
func TestDeriveConfirmationRequirementAutoKeepsGuardrailHardStop(t *testing.T) {
	t.Parallel()

	t.Run("review_batch stays hard_stop", func(t *testing.T) {
		t.Parallel()
		view := nextView{
			auto:            true,
			GuardrailDomain: model.GuardrailDomainAuthAuthZ,
			ReviewBatch: &reviewBatchView{
				Skills: []reviewBatchSkillView{{Name: "security-review"}},
			},
		}
		req := deriveConfirmationRequirement(view)
		assert.Equal(t, "hard_stop", req.Boundary)
		assert.False(t, req.PriorAuthorizationSufficient)
		assert.True(t, req.FreshConfirmationRequired)
	})

	t.Run("skill_handoff stays hard_stop", func(t *testing.T) {
		t.Parallel()
		view := nextView{
			auto:            true,
			GuardrailDomain: model.GuardrailDomainSecurityCredentials,
			NextSkill:       &nextSkillView{Name: "wave-orchestration"},
		}
		req := deriveConfirmationRequirement(view)
		assert.Equal(t, "hard_stop", req.Boundary)
		assert.False(t, req.PriorAuthorizationSufficient)
		assert.True(t, req.FreshConfirmationRequired)
	})
}

// Auto-off must keep the legacy hard-stop behavior byte-for-byte.
func TestDeriveConfirmationRequirementAutoOffUnchanged(t *testing.T) {
	t.Parallel()

	reviewView := nextView{
		auto: false,
		ReviewBatch: &reviewBatchView{
			Skills: []reviewBatchSkillView{{Name: "spec-compliance-review"}},
		},
	}
	assert.Equal(t, confirmationHardStop("review_batch"), deriveConfirmationRequirement(reviewView))

	skillView := nextView{
		auto:      false,
		NextSkill: &nextSkillView{Name: "wave-orchestration"},
	}
	assert.Equal(t, confirmationHardStop("skill_handoff:wave-orchestration"), deriveConfirmationRequirement(skillView))
}

func TestDeriveConfirmationRequirementAutoOffNonPacingBlockerPrecedesHandoff(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		view nextView
	}{
		{
			name: "skill_handoff",
			view: nextView{
				auto:      false,
				NextSkill: &nextSkillView{Name: progression.SkillWaveOrchestration},
				Blockers: []model.ReasonCode{
					model.NewReasonCode("scope_contract_drift", "cmd/next.go"),
				},
			},
		},
		{
			name: "review_batch",
			view: nextView{
				auto: false,
				ReviewBatch: &reviewBatchView{
					Skills: []reviewBatchSkillView{{Name: progression.SkillSpecComplianceReview}},
				},
				Blockers: []model.ReasonCode{
					model.NewReasonCode("scope_contract_drift", "cmd/next.go"),
				},
			},
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := deriveConfirmationRequirement(tt.view)
			assert.Equal(t, "hard_stop", req.Boundary)
			assert.False(t, req.PriorAuthorizationSufficient)
			assert.True(t, req.FreshConfirmationRequired)
			assert.Equal(t, "blocked_by_governance", req.Reason)
			assert.Equal(t, "blocker_resolution", req.NextActionKind)
		})
	}
}

// (e) A `next` preview under auto records NO auto-confirm side effect: no preset
// mutation, no SaveChange, state unchanged.
func TestNextPreviewUnderAutoHasNoSideEffect(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	writeAutoConfig(t, root, true)

	// A GENUINELY pending preset confirmation: WorkflowPreset is unset (invalid)
	// and a valid preset is suggested, so WorkflowPresetConfirmationPending() is
	// true and the advancing auto path WOULD auto-confirm + SaveChange. (The prior
	// fixture set WorkflowPreset=light, which is valid, so nothing was pending and
	// the no-side-effect assertion held vacuously.)
	newFixture := func(name string) string {
		return createGovernedChangeFixture(t, root, name, func(change *model.Change) {
			change.CurrentState = model.StateS1Plan
			change.PlanSubStep = model.PlanSubStepResearch
			change.WorkflowPreset = ""
			change.SuggestedWorkflowPreset = model.WorkflowPresetStandard
		})
	}

	// Positive control: on the advancing path (preview=false) the same fixture is
	// auto-confirmed and change.yaml IS rewritten, proving the fixture can produce
	// the mutation the preview must suppress (guards against a vacuous test).
	advSlug := newFixture("advancing auto confirms pending preset")
	advChange, err := state.LoadChange(root, advSlug)
	require.NoError(t, err)
	require.True(t, advChange.WorkflowPresetConfirmationPending(), "fixture must start with a pending preset")
	advPath := state.BundleChangeFilePath(root, advSlug)
	advBefore, err := os.ReadFile(advPath)
	require.NoError(t, err)
	_, err = buildNextViewForCommand(root, changeRef{Slug: advSlug}, nextViewOptions{
		AutoSkipEvidence: true,
		Command:          "run",
		Auto:             true,
	})
	require.NoError(t, err)
	advAfter, err := os.ReadFile(advPath)
	require.NoError(t, err)
	require.NotEqual(t, advBefore, advAfter, "advancing auto path must auto-confirm and rewrite change.yaml")
	advConfirmed, err := state.LoadChange(root, advSlug)
	require.NoError(t, err)
	require.False(t, advConfirmed.WorkflowPresetConfirmationPending(), "advancing auto path must confirm the pending preset")

	// The assertion under test: the preview path leaves the pending preset and
	// change.yaml untouched.
	slug := newFixture("next preview no side effect under auto")
	before, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	require.True(t, before.WorkflowPresetConfirmationPending(), "fixture must start with a pending preset")
	changePath := state.BundleChangeFilePath(root, slug)
	beforeBytes, err := os.ReadFile(changePath)
	require.NoError(t, err)

	// preview=true is the `next` query path; auto is threaded for the displayed
	// requirement but must never advance/mutate.
	_, err = buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{
		Preview:          true,
		AutoSkipEvidence: true,
		Command:          "next",
		Auto:             true,
	})
	require.NoError(t, err)

	after, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	assert.Equal(t, before.CurrentState, after.CurrentState)
	assert.Equal(t, before.WorkflowPreset, after.WorkflowPreset)
	assert.Equal(t, before.PlanSubStep, after.PlanSubStep)
	assert.True(t, after.WorkflowPresetConfirmationPending(), "preview must not confirm the pending preset")

	// The authoritative change.yaml must be byte-identical: no SaveChange ran.
	afterBytes, err := os.ReadFile(changePath)
	require.NoError(t, err)
	assert.Equal(t, beforeBytes, afterBytes, "preview must not rewrite change.yaml")
}

// (f) Auto must NOT auto-author the intake Approved Summary. The fixture sits at
// the confirm sub-step with an empty `## Approved Summary`, so the only thing
// between intake and S1 is operator authorship. Under auto the engine must still
// fail closed: it must not write the summary, must not advance out of S0_INTAKE,
// and must keep surfacing the intake_confirmation_incomplete blocker. That
// blocker's presence is itself proof the summary was not auto-authored — the
// engine drops it the moment the section becomes non-empty.
func TestAutoDoesNotAutoWriteIntakeApprovedSummary(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	writeAutoConfig(t, root, true)

	slug := createGovernedChangeFixture(t, root, "auto must not author intake approved summary", func(change *model.Change) {
		change.CurrentState = model.StateS0Intake
		change.IntakeSubStep = model.IntakeSubStepConfirm
	})

	intentPath := filepath.Join(root, "artifacts", "changes", slug, "intent.md")
	const intentBody = "# Intent\n\n## In Scope\nShip the feature.\n\n## Out of Scope\nUnrelated work.\n\n## Acceptance Signals\n- It works.\n\n## Approved Summary\n"
	require.NoError(t, os.WriteFile(intentPath, []byte(intentBody), 0o644))

	// Advancing path with auto on at the Approved-Summary confirm boundary.
	view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{
		AutoSkipEvidence: true,
		Command:          "run",
		Auto:             true,
	})
	require.NoError(t, err)

	after, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	assert.Equal(t, model.StateS0Intake, after.CurrentState, "auto must not advance past the intake authorship boundary")

	// Read intent.md back: the Approved Summary section must stay empty.
	gotIntent, err := os.ReadFile(intentPath)
	require.NoError(t, err)
	assert.Empty(t, approvedSummarySection(string(gotIntent)),
		"auto must not write the intake Approved Summary")

	// The boundary is the specific intake confirmation blocker, not a softened
	// continuation: auto only softens review_batch / non-sensitive skill_handoff.
	assert.True(t, hasReasonCode(view.Blockers, "intake_confirmation_incomplete"),
		"intake confirmation must still block on operator-authored Approved Summary under auto")
}

// approvedSummarySection returns the trimmed body of the `## Approved Summary`
// section, or "" when the heading is absent or the section has no content.
func approvedSummarySection(intent string) string {
	const heading = "## Approved Summary"
	idx := strings.Index(intent, heading)
	if idx < 0 {
		return ""
	}
	body := intent[idx+len(heading):]
	if next := strings.Index(body, "\n## "); next >= 0 {
		body = body[:next]
	}
	return strings.TrimSpace(body)
}

// (finding #5) Config-level execution.auto must reach the real command entry
// points, not just the helper layer. This drives `slipway next` through the root
// cobra command with auto set only via .slipway.yaml. The vehicle is an
// S2_IMPLEMENT change whose pending skill is wave-orchestration (a genuinely
// pure-pacing skill_handoff), so non-guardrail auto softens the boundary to
// evidence_continuation while a guardrail domain — and auto off — keep the
// hard_stop. The guardrail-vs-non-guardrail pair proves both that auto was read
// from config and threaded, and that the guardrail exclusion still fails closed
// through this entry.
//
// intake-clarification is deliberately NOT used as the vehicle: the intake
// approved-summary is a fresh hard gate by design (#357) and must hard-stop even
// under auto, so it can no longer demonstrate the softened path (see
// TestDeriveConfirmationRequirementAutoKeepsIntakeClarificationHardStop). `next`
// is a real command entry that threads execution.auto via resolveEffectiveAuto
// and is preview-only, so it carries no execution side effects.
func TestConfigAutoReachesStageAndHookEntries(t *testing.T) {
	setup := func(t *testing.T, auto bool, guardrail string) (string, string) {
		t.Helper()
		root := t.TempDir()
		ensureTestGitRepo(t, root)
		initTestWorkspace(t, root)
		writeAutoConfig(t, root, auto)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "config auto reaches entries")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		if guardrail != "" {
			change.GuardrailDomain = guardrail
		}
		require.NoError(t, state.SaveChange(root, change))
		writeShipReadyGovernedBundle(t, root, change)
		return root, slug
	}

	t.Run("next_command_s2_wave", func(t *testing.T) {
		nextJSON := func(t *testing.T, root, slug string) string {
			t.Helper()
			var out string
			withWorkspace(t, root, func() {
				cmd := newRootCmd()
				cmd.SetArgs([]string{"next", "--json", "--change", slug})
				var buf bytes.Buffer
				cmd.SetOut(&buf)
				require.NoError(t, cmd.Execute())
				out = buf.String()
			})
			return out
		}

		softRoot, softSlug := setup(t, true, "")
		soft := nextJSON(t, softRoot, softSlug)
		assert.Contains(t, soft, "wave-orchestration")
		assert.Contains(t, soft, "evidence_continuation",
			"config auto must soften the non-guardrail wave-orchestration skill_handoff through the command entry")
		assert.NotContains(t, soft, "hard_stop")

		guardRoot, guardSlug := setup(t, true, string(model.GuardrailDomainAuthAuthZ))
		guard := nextJSON(t, guardRoot, guardSlug)
		assert.Contains(t, guard, "hard_stop",
			"guardrail domain must keep the command entry hard-stop under auto")

		offRoot, offSlug := setup(t, false, "")
		off := nextJSON(t, offRoot, offSlug)
		assert.Contains(t, off, "hard_stop", "auto off must keep the legacy hard_stop")
	})
}

// (g) light auto-pass eligibility is unchanged under auto: the auto-on and
// auto-off advancing paths reach the same outcome for a ship-ready light change.
func TestLightAutoPassEligibilityUnchangedUnderAuto(t *testing.T) {
	t.Parallel()

	build := func(t *testing.T, auto bool) nextView {
		t.Helper()
		root := t.TempDir()
		ensureTestGitRepo(t, root)
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "light autopass unchanged under auto")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.WorkflowPreset = model.WorkflowPresetLight
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		writeShipReadyGovernedBundle(t, root, change)
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		writePassingReviewEvidencePack(t, root, slug, 1)
		writePassingShipVerificationEvidence(t, root, slug, 1)

		// skipAutoPass=true surfaces AutoPassEligible instead of auto-passing.
		view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{
			SkipAutoPass: true,
			Command:      "run",
			Auto:         auto,
		})
		require.NoError(t, err)
		return view
	}

	autoOff := build(t, false)
	autoOn := build(t, true)

	require.NotNil(t, autoOff.Advanced)
	require.NotNil(t, autoOn.Advanced)
	assert.Equal(t, autoOff.Advanced.Action, autoOn.Advanced.Action)
	assert.Equal(t, autoOff.Advanced.AutoPassedStates, autoOn.Advanced.AutoPassedStates)
	assert.Equal(t, autoOff.AutoPassEligible, autoOn.AutoPassEligible)
}
