package cmd

import (
	"bytes"
	"encoding/json"
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
		progression.SkillIntakeClarification,
		progression.SkillResearchOrchestration,
		progression.SkillPlanAudit,
		progression.SkillWaveOrchestration,
		progression.SkillSpecComplianceReview,
		progression.SkillCodeQualityReview,
		progression.SkillIndependentReview,
		progression.SkillGoalVerification,
		progression.SkillFinalCloseout,
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
				Name:         progression.SkillGoalVerification,
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
				model.NewReasonCode("closeout_assurance_attestation_missing", ""),
				model.NewReasonCode("closeout_reviewer_independence_missing", ""),
				model.NewReasonCode("context_origin_handle_invalid", "spec-compliance-review"),
				model.NewReasonCode("governance_action_required", "domain-review: run domain-aware review"),
				model.NewReasonCode("high_risk_check_missing", "auth_authz.safety_baseline"),
				model.NewReasonCode("verification_evidence_missing", ""),
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

	t.Run("review companion blockers ride goal verification handoff", func(t *testing.T) {
		t.Parallel()
		view := nextView{
			auto:      false,
			NextSkill: &nextSkillView{Name: "goal-verification"},
			Blockers: []model.ReasonCode{
				model.NewReasonCode("closeout_assurance_attestation_missing", ""),
				model.NewReasonCode("closeout_reviewer_independence_missing", ""),
				model.NewReasonCode("high_risk_check_missing", "auth_authz.safety_baseline"),
				model.NewReasonCode("verification_evidence_missing", ""),
			},
		}

		req := deriveConfirmationRequirement(view)
		assert.Equal(t, "hard_stop", req.Boundary)
		assert.False(t, req.PriorAuthorizationSufficient)
		assert.True(t, req.FreshConfirmationRequired)
		assert.Equal(t, "skill_handoff:goal-verification", req.Reason)
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

// TASK D (findings #1, #2): the resume_checkpoint confirmation contract under
// auto. Only a FRESH, non-guardrail human_verify checkpoint softens to an
// evidence_continuation; every other kind, a guardrail domain, or stale
// freshness keeps the manual hard_stop. This mirrors run.autoAckEligibleCheckpoint
// so the next contract never diverges from what run actually does.
func TestDeriveConfirmationRequirementResumeCheckpointAutoMatrix(t *testing.T) {
	t.Parallel()

	const freshness, stale = "fresh", "stale"

	makeView := func(guardrail, checkpointType, fresh string) nextView {
		return nextView{
			auto:            true,
			GuardrailDomain: guardrail,
			InputContext: nextContext{
				ResumeCheckpoint: &resumeCheckpoint{
					CheckpointType: checkpointType,
					Freshness:      fresh,
					PausedTaskID:   "t-01",
				},
			},
		}
	}

	t.Run("fresh non-guardrail human_verify softens to evidence_continuation", func(t *testing.T) {
		t.Parallel()
		req := deriveConfirmationRequirement(makeView("", string(model.CheckpointHumanVerify), freshness))
		assert.Equal(t, "evidence_continuation", req.Boundary)
		assert.True(t, req.PriorAuthorizationSufficient)
		assert.Equal(t, "slipway run", req.NextCommand)
		assert.Equal(t, "resume_checkpoint", req.Reason)
	})

	t.Run("stale non-guardrail human_verify stays hard_stop", func(t *testing.T) {
		t.Parallel()
		req := deriveConfirmationRequirement(makeView("", string(model.CheckpointHumanVerify), stale))
		assert.Equal(t, "hard_stop", req.Boundary)
		assert.Equal(t, "resume_checkpoint", req.Reason)
	})

	t.Run("fresh decision checkpoint stays hard_stop", func(t *testing.T) {
		t.Parallel()
		req := deriveConfirmationRequirement(makeView("", string(model.CheckpointDecision), freshness))
		assert.Equal(t, "hard_stop", req.Boundary)
		assert.Equal(t, "resume_checkpoint", req.Reason)
	})

	t.Run("fresh human_action checkpoint stays hard_stop", func(t *testing.T) {
		t.Parallel()
		req := deriveConfirmationRequirement(makeView("", string(model.CheckpointHumanAction), freshness))
		assert.Equal(t, "hard_stop", req.Boundary)
		assert.Equal(t, "resume_checkpoint", req.Reason)
	})

	t.Run("fresh guardrail human_verify stays hard_stop", func(t *testing.T) {
		t.Parallel()
		req := deriveConfirmationRequirement(makeView(model.GuardrailDomainAuthAuthZ, string(model.CheckpointHumanVerify), freshness))
		assert.Equal(t, "hard_stop", req.Boundary)
		assert.Equal(t, "resume_checkpoint", req.Reason)
	})
}

func setupAutoModeCheckpoint(t *testing.T, guardrail, checkpointType string, fresh bool) (string, string, changeRef) {
	t.Helper()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	slug := createGovernedRequest(t, root, "L2", "auto ack entry "+checkpointType+" "+guardrail)
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	change.GuardrailDomain = guardrail
	require.NoError(t, state.SaveChange(root, change))
	plan, err := state.MaterializeWavePlan(root, change)
	require.NoError(t, err)
	require.NotEmpty(t, plan.Waves)
	require.NotEmpty(t, plan.Waves[0].Tasks)
	taskID := plan.Waves[0].Tasks[0].TaskID
	change.ActiveCheckpoint = &model.ActiveCheckpoint{
		PausedTaskID:    taskID,
		PausedWaveIndex: plan.Waves[0].WaveIndex,
		CheckpointType:  checkpointType,
	}
	if model.CheckpointKind(checkpointType) == model.CheckpointDecision {
		change.ActiveCheckpoint.AllowedResponses = []string{"approve", "reject"}
	}
	require.NoError(t, state.SaveChange(root, change))
	if fresh {
		writePassingExecutionSummary(t, root, slug, 1, taskID)
		// The injected-response entry path validates active-checkpoint authority
		// against the wave plan, which requires materialized wave run evidence.
		writePassingWaveEvidence(t, root, slug, 1)
	}
	return root, slug, changeRef{Slug: slug}
}

func requireCheckpointResolvedEvent(t *testing.T, events []state.LifecycleEvent) state.LifecycleEvent {
	t.Helper()
	for _, event := range events {
		if event.EventType == "checkpoint.resolved" {
			return event
		}
	}
	require.FailNow(t, "expected checkpoint.resolved lifecycle event", "events=%+v", events)
	return state.LifecycleEvent{}
}

// (d) ENTRY-LEVEL: auto-ack injects a response only for a non-guardrail
// human_verify checkpoint; decision/human_action/guardrail stay manual.
func TestAutoAckResumeResponseEntryLevel(t *testing.T) {
	t.Parallel()

	t.Run("non-guardrail human_verify permits continuation under auto", func(t *testing.T) {
		t.Parallel()
		root, _, ref := setupAutoModeCheckpoint(t, "", string(model.CheckpointHumanVerify), true)

		// auto-ack injects a standing response so entry validation passes.
		effective, autoAcknowledged, err := autoAckResumeResponse(root, ref, true, false, "")
		require.NoError(t, err)
		assert.Equal(t, autoAcknowledgedResponse, effective)
		assert.True(t, autoAcknowledged)
		require.NoError(t, validateRunEntry(root, ref, false, effective))
	})

	t.Run("decision checkpoint stays manual under auto", func(t *testing.T) {
		t.Parallel()
		root, _, ref := setupAutoModeCheckpoint(t, "", string(model.CheckpointDecision), false)

		effective, autoAcknowledged, err := autoAckResumeResponse(root, ref, true, false, "")
		require.NoError(t, err)
		assert.Equal(t, "", effective, "decision checkpoint must not be auto-acknowledged")
		assert.False(t, autoAcknowledged)
		err = validateRunEntry(root, ref, false, effective)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "resume_response_required", cliErr.ErrorCode)
	})

	t.Run("human_action checkpoint stays manual under auto", func(t *testing.T) {
		t.Parallel()
		root, _, ref := setupAutoModeCheckpoint(t, "", string(model.CheckpointHumanAction), false)

		effective, autoAcknowledged, err := autoAckResumeResponse(root, ref, true, false, "")
		require.NoError(t, err)
		assert.Equal(t, "", effective, "human_action checkpoint must not be auto-acknowledged")
		assert.False(t, autoAcknowledged)
		err = validateRunEntry(root, ref, false, effective)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "resume_response_required", cliErr.ErrorCode)
	})

	t.Run("guardrail human_verify stays manual under auto", func(t *testing.T) {
		t.Parallel()
		root, _, ref := setupAutoModeCheckpoint(t, model.GuardrailDomainAuthAuthZ, string(model.CheckpointHumanVerify), false)

		effective, autoAcknowledged, err := autoAckResumeResponse(root, ref, true, false, "")
		require.NoError(t, err)
		assert.Equal(t, "", effective, "guardrail human_verify must not be auto-acknowledged")
		assert.False(t, autoAcknowledged)
		err = validateRunEntry(root, ref, false, effective)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "resume_response_required", cliErr.ErrorCode)
	})

	t.Run("operator response is never overwritten by auto-ack", func(t *testing.T) {
		t.Parallel()
		root, _, ref := setupAutoModeCheckpoint(t, "", string(model.CheckpointHumanVerify), false)

		effective, autoAcknowledged, err := autoAckResumeResponse(root, ref, true, false, "operator says ok")
		require.NoError(t, err)
		assert.Equal(t, "operator says ok", effective)
		assert.False(t, autoAcknowledged)
	})

	t.Run("auto off never auto-acks", func(t *testing.T) {
		t.Parallel()
		root, _, ref := setupAutoModeCheckpoint(t, "", string(model.CheckpointHumanVerify), false)

		effective, autoAcknowledged, err := autoAckResumeResponse(root, ref, false, false, "")
		require.NoError(t, err)
		assert.Equal(t, "", effective)
		assert.False(t, autoAcknowledged)
	})

	// TASK B: a fresh human_verify checkpoint auto-acks; a stale/unknown one must
	// fail closed. With no passing execution summary written, projectFreshnessForExecMode
	// reports unknown freshness, so auto-ack keeps the passed-in empty response.
	t.Run("non-guardrail human_verify with stale/unknown execution stays manual under auto", func(t *testing.T) {
		t.Parallel()
		root, _, ref := setupAutoModeCheckpoint(t, "", string(model.CheckpointHumanVerify), false)

		effective, autoAcknowledged, err := autoAckResumeResponse(root, ref, true, false, "")
		require.NoError(t, err)
		assert.Equal(t, "", effective, "stale/unknown freshness must not be auto-acknowledged")
		assert.False(t, autoAcknowledged)

		err = validateRunEntry(root, ref, false, effective)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "resume_response_required", cliErr.ErrorCode)
	})
}

func TestRunCommandAutoFlagsExerciseRealEntryPath(t *testing.T) {
	t.Parallel()

	t.Run("--auto auto-acknowledges fresh non-guardrail human_verify", func(t *testing.T) {
		t.Parallel()
		root, slug, _ := setupAutoModeCheckpoint(t, "", string(model.CheckpointHumanVerify), true)

		cmd := commandForRoot(t, root, makeRunCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics", "--auto"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		require.NoError(t, cmd.Execute(), buf.String())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		require.NotNil(t, view.InputContext.ResumeCheckpoint)
		assert.Equal(t, autoAcknowledgedResponse, view.InputContext.ResumeCheckpoint.UserResponsePayload)

		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Nil(t, change.ActiveCheckpoint, "run --auto must consume the eligible checkpoint through the real command path")

		events, err := state.ReadLifecycleEvents(root, change)
		require.NoError(t, err)
		event := requireCheckpointResolvedEvent(t, events)
		assert.True(t, lifecycleEventHasSideEffect(event, "active_checkpoint_cleared"))
		assert.True(t, lifecycleEventHasSideEffect(event, autoCheckpointAcknowledgedSideEffect))
	})

	t.Run("manual literal auto-acknowledged response stays manual", func(t *testing.T) {
		t.Parallel()
		root, slug, _ := setupAutoModeCheckpoint(t, "", string(model.CheckpointHumanVerify), true)

		cmd := commandForRoot(t, root, makeRunCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics", "--auto", "--resume-response", autoAcknowledgedResponse})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		require.NoError(t, cmd.Execute(), buf.String())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		require.NotNil(t, view.InputContext.ResumeCheckpoint)
		assert.Equal(t, autoAcknowledgedResponse, view.InputContext.ResumeCheckpoint.UserResponsePayload)

		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Nil(t, change.ActiveCheckpoint, "manual resume response must still consume the checkpoint")

		events, err := state.ReadLifecycleEvents(root, change)
		require.NoError(t, err)
		event := requireCheckpointResolvedEvent(t, events)
		assert.True(t, lifecycleEventHasSideEffect(event, "active_checkpoint_cleared"))
		assert.False(t, lifecycleEventHasSideEffect(event, autoCheckpointAcknowledgedSideEffect))
	})

	t.Run("--no-auto overrides config auto and keeps checkpoint manual", func(t *testing.T) {
		t.Parallel()
		root, slug, _ := setupAutoModeCheckpoint(t, "", string(model.CheckpointHumanVerify), true)
		writeAutoConfig(t, root, true)

		cmd := commandForRoot(t, root, makeRunCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics", "--no-auto"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		err := cmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "resume_response_required", cliErr.ErrorCode)

		change, loadErr := state.LoadChange(root, slug)
		require.NoError(t, loadErr)
		assert.NotNil(t, change.ActiveCheckpoint, "run --no-auto must not consume the checkpoint even when execution.auto is true")
	})
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
	_, err = buildNextViewForCommand(root, changeRef{Slug: advSlug}, "", false, true, false, "run", true, false)
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
	_, err = buildNextViewForCommand(root, changeRef{Slug: slug}, "", true, true, false, "next", true, false)
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
	view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, "", false, true, false, "run", true, false)
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
// points, not just the helper layer. These drive `slipway intake` through the
// root cobra command and the `session-start` hook through makeHookCmd, with auto
// set only via .slipway.yaml. At S0 intake the next skill is intake-clarification
// (a skill_handoff), so non-guardrail auto softens the boundary to
// evidence_continuation while a guardrail domain — and auto off — keep the
// hard_stop. The guardrail-vs-non-guardrail pair proves both that auto was read
// from config and threaded, and that the guardrail exclusion still fails closed
// through these entries.
func TestConfigAutoReachesStageAndHookEntries(t *testing.T) {
	setup := func(t *testing.T, auto bool, guardrail string) string {
		t.Helper()
		root := t.TempDir()
		ensureTestGitRepo(t, root)
		initTestWorkspace(t, root)
		writeAutoConfig(t, root, auto)
		slug := createIntakeChangeFixture(t, root, "config auto reaches entries")
		if guardrail != "" {
			change, err := state.LoadChange(root, slug)
			require.NoError(t, err)
			change.GuardrailDomain = guardrail
			require.NoError(t, state.SaveChange(root, change))
		}
		return root
	}

	t.Run("stage_command_intake", func(t *testing.T) {
		stageJSON := func(t *testing.T, root string) string {
			t.Helper()
			var out string
			withWorkspace(t, root, func() {
				cmd := newRootCmd()
				cmd.SetArgs([]string{"intake", "--json"})
				var buf bytes.Buffer
				cmd.SetOut(&buf)
				require.NoError(t, cmd.Execute())
				out = buf.String()
			})
			return out
		}

		soft := stageJSON(t, setup(t, true, ""))
		assert.Contains(t, soft, "evidence_continuation",
			"config auto must soften the non-guardrail intake skill_handoff through the stage command")
		assert.NotContains(t, soft, "hard_stop")

		guard := stageJSON(t, setup(t, true, string(model.GuardrailDomainAuthAuthZ)))
		assert.Contains(t, guard, "hard_stop",
			"guardrail domain must keep the stage entry hard-stop under auto")

		off := stageJSON(t, setup(t, false, ""))
		assert.Contains(t, off, "hard_stop", "auto off must keep the legacy hard_stop")
	})

	t.Run("session_start_hook", func(t *testing.T) {
		hookJSON := func(t *testing.T, root string) string {
			t.Helper()
			var out string
			withWorkspace(t, root, func() {
				cmd := makeHookCmd()
				cmd.SetArgs([]string{"session-start", "--tool", "claude"})
				var buf bytes.Buffer
				cmd.SetOut(&buf)
				require.NoError(t, cmd.Execute())
				out = buf.String()
			})
			return out
		}

		soft := hookJSON(t, setup(t, true, ""))
		assert.Contains(t, soft, "evidence_continuation",
			"config auto must thread into the session-start hook view")
		assert.NotContains(t, soft, "hard_stop")

		guard := hookJSON(t, setup(t, true, string(model.GuardrailDomainAuthAuthZ)))
		assert.Contains(t, guard, "hard_stop",
			"guardrail domain must keep the hook view hard-stop under auto")

		off := hookJSON(t, setup(t, false, ""))
		assert.Contains(t, off, "hard_stop", "auto off must keep the hook view hard-stop")
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

		slug := createGovernedRequest(t, root, "L2", "light autopass unchanged under auto")
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
		writePassingGoalVerificationEvidence(t, root, slug, 1)

		// skipAutoPass=true surfaces AutoPassEligible instead of auto-passing.
		view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, "", false, false, true, "run", auto, false)
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
