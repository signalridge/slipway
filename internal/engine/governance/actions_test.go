package governance

import (
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
)

func makeControl(id model.ControlID, mode model.ControlMode, scope model.ControlScope) model.ControlActivation {
	return model.ControlActivation{
		ControlID:    id,
		Mode:         mode,
		Scope:        scope,
		Active:       true,
		TriggeredBy:  []string{"test"},
		PolicySource: model.BuiltinPolicySource,
	}
}

func TestResolveRequiredActionsEmpty(t *testing.T) {
	t.Parallel()
	actions := ResolveRequiredActions(RequiredActionsInput{})
	assert.Empty(t, actions)
}

func TestResolveRequiredActionsFromControls(t *testing.T) {
	t.Parallel()
	controls := []model.ControlActivation{
		makeControl(model.ControlDomainReview, model.ControlModeBlocking, model.ControlScopeReview),
		makeControl(model.ControlWorktreeIsolation, model.ControlModeBlocking, model.ControlScopeExecution),
		makeControl(model.ControlRollbackRequired, model.ControlModeAdvisory, model.ControlScopeRelease),
	}

	actions := ResolveRequiredActions(RequiredActionsInput{
		ActiveControls: controls,
	})

	assert.Len(t, actions, 3)

	var ids []model.ControlID
	for _, a := range actions {
		ids = append(ids, a.ControlID)
		assert.NotEmpty(t, a.Description)
	}
	assert.Contains(t, ids, model.ControlDomainReview)
	assert.Contains(t, ids, model.ControlWorktreeIsolation)
	assert.Contains(t, ids, model.ControlRollbackRequired)
}

func TestRequiredActionsSatisfaction(t *testing.T) {
	t.Parallel()
	controls := []model.ControlActivation{
		makeControl(model.ControlDomainReview, model.ControlModeBlocking, model.ControlScopeReview),
		makeControl(model.ControlWorktreeIsolation, model.ControlModeBlocking, model.ControlScopeExecution),
	}

	// Not satisfied.
	actions := ResolveRequiredActions(RequiredActionsInput{
		ActiveControls: controls,
	})
	assert.NotEmpty(t, UnsatisfiedBlockingActions(actions))
	assert.Len(t, UnsatisfiedBlockingActions(actions), 2)

	// Partially satisfied.
	actions = ResolveRequiredActions(RequiredActionsInput{
		ActiveControls:        controls,
		DomainReviewDone:      true,
		WorktreePreflightDone: false,
	})
	assert.NotEmpty(t, UnsatisfiedBlockingActions(actions))
	assert.Len(t, UnsatisfiedBlockingActions(actions), 1)

	// Fully satisfied.
	actions = ResolveRequiredActions(RequiredActionsInput{
		ActiveControls:        controls,
		DomainReviewDone:      true,
		WorktreePreflightDone: true,
	})
	assert.Empty(t, UnsatisfiedBlockingActions(actions))
}

func TestAdvisoryActionsDoNotBlock(t *testing.T) {
	t.Parallel()
	controls := []model.ControlActivation{
		makeControl(model.ControlRollbackRequired, model.ControlModeAdvisory, model.ControlScopeRelease),
	}

	actions := ResolveRequiredActions(RequiredActionsInput{
		ActiveControls: controls,
	})

	assert.Len(t, actions, 1)
	assert.False(t, actions[0].Satisfied)
	// Advisory controls should not appear in unsatisfied blockers.
	assert.Empty(t, UnsatisfiedBlockingActions(actions))
}

func TestIndependentReviewActionDescribesPostExecutionTiming(t *testing.T) {
	t.Parallel()
	// Independent review is a review-scope gate (S3): it runs on execution
	// evidence, not before execution. The wording must not tell agents to run it
	// "before further execution" at S2, which contradicts the gate and the
	// review command's execution-summary requirement (issue #36, comment 1).
	controls := []model.ControlActivation{
		makeControl(model.ControlIndependentReview, model.ControlModeBlocking, model.ControlScopeReview),
	}

	actions := ResolveRequiredActions(RequiredActionsInput{
		ActiveControls: controls,
	})

	if assert.Len(t, actions, 1) {
		desc := actions[0].Description
		assert.NotContains(t, desc, "before further execution")
		assert.Contains(t, desc, "after wave execution")
	}
}

func TestSecurityReviewActionSatisfaction(t *testing.T) {
	t.Parallel()

	controls := []model.ControlActivation{
		makeControl(model.ControlSecurityReview, model.ControlModeBlocking, model.ControlScopeReview),
	}

	actions := ResolveRequiredActions(RequiredActionsInput{
		ActiveControls: controls,
	})
	if assert.Len(t, actions, 1) {
		assert.False(t, actions[0].Satisfied)
		assert.Contains(t, actions[0].Description, "security review")
	}
	assert.NotEmpty(t, UnsatisfiedBlockingActions(actions))

	actions = ResolveRequiredActions(RequiredActionsInput{
		ActiveControls:     controls,
		SecurityReviewDone: true,
		SecurityReviewSatisfiedBy: []SatisfiedBy{
			{Kind: "skill_evidence", Name: "security-review"},
		},
	})
	if assert.Len(t, actions, 1) {
		assert.True(t, actions[0].Satisfied)
		assert.Equal(t, "security-review", actions[0].SatisfiedBy[0].Name)
	}
	assert.Empty(t, UnsatisfiedBlockingActions(actions))
}

func TestAdvisoryModeOverrideDoesNotBlock(t *testing.T) {
	t.Parallel()
	// Simulate independent-review activated as advisory via mode override.
	controls := []model.ControlActivation{
		makeControl(model.ControlIndependentReview, model.ControlModeAdvisory, model.ControlScopeReview),
		makeControl(model.ControlDomainReview, model.ControlModeBlocking, model.ControlScopeReview),
	}

	actions := ResolveRequiredActions(RequiredActionsInput{
		ActiveControls: controls,
	})

	assert.Len(t, actions, 2)
	// Only domain-review (blocking, unsatisfied) should be a blocker.
	blocking := UnsatisfiedBlockingActions(actions)
	assert.Len(t, blocking, 1)
	assert.Equal(t, model.ControlDomainReview, blocking[0].ControlID)
}

func TestReescalatedBlockingControlBlocks(t *testing.T) {
	t.Parallel()
	// Simulate rollback-required re-escalated to blocking via mode override.
	controls := []model.ControlActivation{
		makeControl(model.ControlRollbackRequired, model.ControlModeBlocking, model.ControlScopeRelease),
	}

	actions := ResolveRequiredActions(RequiredActionsInput{
		ActiveControls: controls,
	})

	assert.Len(t, actions, 1)
	assert.False(t, actions[0].Satisfied)
	// Now it should block since mode is blocking.
	assert.NotEmpty(t, UnsatisfiedBlockingActions(actions))
}

func TestExplorationControlSatisfaction(t *testing.T) {
	t.Parallel()
	controls := []model.ControlActivation{
		makeControl(model.ControlResearch, model.ControlModeBlocking, model.ControlScopeDiscovery),
	}

	// Not satisfied without research.
	actions := ResolveRequiredActions(RequiredActionsInput{
		ActiveControls: controls,
	})
	assert.NotEmpty(t, UnsatisfiedBlockingActions(actions))

	// Not satisfied without research structure.
	actions = ResolveRequiredActions(RequiredActionsInput{
		ActiveControls: controls,
		IntentExists:   true,
		ScopeConfirmed: true,
	})
	assert.NotEmpty(t, UnsatisfiedBlockingActions(actions), "research control requires research.md structure validation")

	// Satisfied when all three conditions met.
	actions = ResolveRequiredActions(RequiredActionsInput{
		ActiveControls:      controls,
		IntentExists:        true,
		ScopeConfirmed:      true,
		ResearchStructureOK: true,
	})
	assert.Empty(t, UnsatisfiedBlockingActions(actions))

	// #377: present-but-stale research evidence un-satisfies the control even when
	// intent, scope, and structure are all in place — effective freshness, not mere
	// structural presence, drives satisfaction.
	actions = ResolveRequiredActions(RequiredActionsInput{
		ActiveControls:        controls,
		IntentExists:          true,
		ScopeConfirmed:        true,
		ResearchStructureOK:   true,
		ResearchEvidenceStale: true,
	})
	assert.NotEmpty(t, UnsatisfiedBlockingActions(actions),
		"stale research-orchestration evidence must un-satisfy the research control")
}

func TestClarificationControlSatisfaction(t *testing.T) {
	t.Parallel()
	controls := []model.ControlActivation{
		makeControl(model.ControlClarification, model.ControlModeBlocking, model.ControlScopeDiscovery),
	}

	// Not satisfied — has blocking questions.
	actions := ResolveRequiredActions(RequiredActionsInput{
		ActiveControls:           controls,
		HasBlockingOpenQuestions: true,
	})
	assert.NotEmpty(t, UnsatisfiedBlockingActions(actions))

	// Satisfied — no blocking questions.
	actions = ResolveRequiredActions(RequiredActionsInput{
		ActiveControls:           controls,
		HasBlockingOpenQuestions: false,
	})
	assert.Empty(t, UnsatisfiedBlockingActions(actions))
}
