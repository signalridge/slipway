package model

import "strings"

// AdvanceIntakeSubStep records an in-state S0_INTAKE substep transition.
func (c *Change) AdvanceIntakeSubStep(next IntakeSubStep) {
	c.IntakeSubStep = next
}

// AdvancePlanSubStep records an in-state S1_PLAN substep transition.
func (c *Change) AdvancePlanSubStep(next PlanSubStep) {
	c.PlanSubStep = next
}

// EnterPlanning records a transition into S1_PLAN and seeds the entry substep.
func (c *Change) EnterPlanning(needsDiscovery bool) []string {
	c.NeedsDiscovery = needsDiscovery
	c.CurrentState = StateS1Plan

	var cleared []string
	if c.IntakeSubStep != IntakeSubStepNone {
		c.IntakeSubStep = IntakeSubStepNone
		cleared = append(cleared, "intake_substep")
	}
	c.PlanSubStep = PlanEntrySubStep(c.NeedsDiscovery)
	return cleared
}

// TransitionTo records a lifecycle transition and clears substeps that no
// longer belong to the destination state.
func (c *Change) TransitionTo(state WorkflowState) []string {
	c.CurrentState = state

	var cleared []string
	if state == StateS0Intake {
		if c.IntakeSubStep == IntakeSubStepNone {
			c.IntakeSubStep = IntakeEntrySubStep()
		}
		if c.PlanSubStep != PlanSubStepNone {
			c.PlanSubStep = PlanSubStepNone
			cleared = append(cleared, "plan_substep")
		}
		return cleared
	}

	if c.IntakeSubStep != IntakeSubStepNone {
		c.IntakeSubStep = IntakeSubStepNone
		cleared = append(cleared, "intake_substep")
	}
	if state == StateS1Plan {
		if c.PlanSubStep == PlanSubStepNone {
			c.PlanSubStep = PlanEntrySubStep(c.NeedsDiscovery)
		}
		return cleared
	}

	if c.PlanSubStep != PlanSubStepNone {
		c.PlanSubStep = PlanSubStepNone
		cleared = append(cleared, "plan_substep")
	}
	return cleared
}

// ClearAutoPassHistory clears the auto-pass trace carried on the current state.
func (c *Change) ClearAutoPassHistory() bool {
	if c.LastAutoPassedStates == nil {
		return false
	}
	c.LastAutoPassedStates = nil
	return true
}

// RecordEvidenceRef records or updates one evidence reference.
func (c *Change) RecordEvidenceRef(key, path string) bool {
	key = strings.TrimSpace(key)
	path = strings.TrimSpace(path)
	if key == "" {
		return false
	}
	if path == "" {
		return c.ClearEvidenceRef(key)
	}
	if c.EvidenceRefs == nil {
		c.EvidenceRefs = map[string]string{}
	}
	if strings.TrimSpace(c.EvidenceRefs[key]) == path {
		return false
	}
	c.EvidenceRefs[key] = path
	return true
}

// ClearEvidenceRef removes one evidence reference, if present.
func (c *Change) ClearEvidenceRef(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" || c.EvidenceRefs == nil {
		return false
	}
	if _, ok := c.EvidenceRefs[key]; !ok {
		return false
	}
	delete(c.EvidenceRefs, key)
	return true
}

// RecordPlanAuditIterations records the current plan-audit iteration count.
func (c *Change) RecordPlanAuditIterations(iterations int) bool {
	if c.PlanAuditIterations == iterations {
		return false
	}
	c.PlanAuditIterations = iterations
	return true
}
