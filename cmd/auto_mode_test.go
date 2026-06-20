package cmd

import (
	"os"
	"testing"

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
		assert.NotEqual(t, "hard_stop", req.Boundary)
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
		assert.NotEqual(t, "hard_stop", req.Boundary)
		assert.True(t, req.PriorAuthorizationSufficient)
		assert.False(t, req.FreshConfirmationRequired)
		assert.Contains(t, req.Reason, "skill_handoff")
		assert.NotEmpty(t, req.NextAction)
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

// (d) ENTRY-LEVEL: auto-ack injects a response only for a non-guardrail
// human_verify checkpoint; decision/human_action/guardrail stay manual.
func TestAutoAckResumeResponseEntryLevel(t *testing.T) {
	t.Parallel()

	// setupCheckpoint builds an S2_IMPLEMENT change paused on the given checkpoint
	// kind/guardrail. When fresh is true it also writes a passing execution summary
	// for the paused task so projectFreshnessForExecMode reports "fresh"; otherwise
	// the freshness stays unknown and the human_verify auto-ack gate fails closed.
	setupCheckpoint := func(t *testing.T, guardrail, checkpointType string, fresh bool) (string, string, changeRef) {
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

	t.Run("non-guardrail human_verify permits continuation under auto", func(t *testing.T) {
		t.Parallel()
		root, _, ref := setupCheckpoint(t, "", string(model.CheckpointHumanVerify), true)

		// auto-ack injects a standing response so entry validation passes.
		effective, err := autoAckResumeResponse(root, ref, true, false, "")
		require.NoError(t, err)
		assert.Equal(t, autoAcknowledgedResponse, effective)
		require.NoError(t, validateRunEntry(root, ref, false, effective))
	})

	t.Run("decision checkpoint stays manual under auto", func(t *testing.T) {
		t.Parallel()
		root, _, ref := setupCheckpoint(t, "", string(model.CheckpointDecision), false)

		effective, err := autoAckResumeResponse(root, ref, true, false, "")
		require.NoError(t, err)
		assert.Equal(t, "", effective, "decision checkpoint must not be auto-acknowledged")
		err = validateRunEntry(root, ref, false, effective)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "resume_response_required", cliErr.ErrorCode)
	})

	t.Run("human_action checkpoint stays manual under auto", func(t *testing.T) {
		t.Parallel()
		root, _, ref := setupCheckpoint(t, "", string(model.CheckpointHumanAction), false)

		effective, err := autoAckResumeResponse(root, ref, true, false, "")
		require.NoError(t, err)
		assert.Equal(t, "", effective, "human_action checkpoint must not be auto-acknowledged")
		err = validateRunEntry(root, ref, false, effective)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "resume_response_required", cliErr.ErrorCode)
	})

	t.Run("guardrail human_verify stays manual under auto", func(t *testing.T) {
		t.Parallel()
		root, _, ref := setupCheckpoint(t, model.GuardrailDomainAuthAuthZ, string(model.CheckpointHumanVerify), false)

		effective, err := autoAckResumeResponse(root, ref, true, false, "")
		require.NoError(t, err)
		assert.Equal(t, "", effective, "guardrail human_verify must not be auto-acknowledged")
		err = validateRunEntry(root, ref, false, effective)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "resume_response_required", cliErr.ErrorCode)
	})

	t.Run("operator response is never overwritten by auto-ack", func(t *testing.T) {
		t.Parallel()
		root, _, ref := setupCheckpoint(t, "", string(model.CheckpointHumanVerify), false)

		effective, err := autoAckResumeResponse(root, ref, true, false, "operator says ok")
		require.NoError(t, err)
		assert.Equal(t, "operator says ok", effective)
	})

	t.Run("auto off never auto-acks", func(t *testing.T) {
		t.Parallel()
		root, _, ref := setupCheckpoint(t, "", string(model.CheckpointHumanVerify), false)

		effective, err := autoAckResumeResponse(root, ref, false, false, "")
		require.NoError(t, err)
		assert.Equal(t, "", effective)
	})

	// TASK B: a fresh human_verify checkpoint auto-acks; a stale/unknown one must
	// fail closed. With no passing execution summary written, projectFreshnessForExecMode
	// reports unknown freshness, so auto-ack keeps the passed-in empty response.
	t.Run("non-guardrail human_verify with stale/unknown execution stays manual under auto", func(t *testing.T) {
		t.Parallel()
		root, _, ref := setupCheckpoint(t, "", string(model.CheckpointHumanVerify), false)

		effective, err := autoAckResumeResponse(root, ref, true, false, "")
		require.NoError(t, err)
		assert.Equal(t, "", effective, "stale/unknown freshness must not be auto-acknowledged")

		err = validateRunEntry(root, ref, false, effective)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "resume_response_required", cliErr.ErrorCode)
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

	// A change with a pending preset upgrade confirmation: the auto preset
	// auto-confirm would mutate it on an advancing path, so the preview must not.
	slug := createGovernedChangeFixture(t, root, "next preview no side effect under auto", func(change *model.Change) {
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepResearch
		change.WorkflowPreset = model.WorkflowPresetLight
		change.SuggestedWorkflowPreset = model.WorkflowPresetStandard
	})

	before, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	changePath := state.BundleChangeFilePath(root, slug)
	beforeBytes, err := os.ReadFile(changePath)
	require.NoError(t, err)

	// preview=true is the `next` query path; auto is threaded for the displayed
	// requirement but must never advance/mutate.
	_, err = buildNextViewForCommand(root, changeRef{Slug: slug}, "", true, true, false, "next", true)
	require.NoError(t, err)

	after, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	assert.Equal(t, before.CurrentState, after.CurrentState)
	assert.Equal(t, before.WorkflowPreset, after.WorkflowPreset)
	assert.Equal(t, before.PlanSubStep, after.PlanSubStep)

	// The authoritative change.yaml must be byte-identical: no SaveChange ran.
	afterBytes, err := os.ReadFile(changePath)
	require.NoError(t, err)
	assert.Equal(t, beforeBytes, afterBytes, "preview must not rewrite change.yaml")
}

// (f) Auto must NOT auto-write the intake Approved Summary: the intake boundary
// still surfaces a hard-stop confirmation requiring operator authorship.
func TestAutoDoesNotAutoWriteIntakeApprovedSummary(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	writeAutoConfig(t, root, true)

	slug := createIntakeChangeFixture(t, root, "auto must not author intake approved summary")
	before, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	// Advancing path with auto on at intake: the intake confirmation boundary must
	// remain (no Approved Summary auto-authorship), and the change must not jump
	// out of S0_INTAKE.
	view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, "", false, true, false, "run", true)
	require.NoError(t, err)

	after, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	assert.Equal(t, model.StateS0Intake, after.CurrentState, "auto must not advance past the intake authorship boundary")
	assert.Equal(t, before.CurrentState, after.CurrentState)
	// The intake boundary is still a confirmation requirement, not a softened
	// continuation: auto only softens review_batch / non-sensitive skill_handoff.
	assert.True(t, view.ConfirmationRequirement.Required || view.ConfirmationRequirement.Boundary == "hard_stop" ||
		len(view.Blockers) > 0,
		"intake must still require operator confirmation under auto")
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
		view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, "", false, false, true, "run", auto)
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
